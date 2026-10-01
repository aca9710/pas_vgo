// Package routers replica los routers de FastAPI (pagos, devoluciones,
// estado) con net/http (ServeMux Go 1.22+).
package routers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"pasarela/config"
	"pasarela/database"
	"pasarela/httpclient"
	"pasarela/models"
	"pasarela/pagosadmin"
	"pasarela/utils"
)

// =============================================================================
// AdminPagos global (equivalente a get_admpagos de routers/pagos.py)
// =============================================================================

var (
	adminMu sync.Mutex
	admin   *pagosadmin.AdminPagos
)

// SetAdmin inicializa el AdminPagos global (llamado desde main).
func SetAdmin(a *pagosadmin.AdminPagos) {
	adminMu.Lock()
	defer adminMu.Unlock()
	admin = wireAdmin(a)
}

// wireAdmin conecta a un AdminPagos recien creado el callback que limpia los
// marcadores de Redis. Las claves (pasarela:listaid / pasarela:por_notificar)
// son de este paquete, por eso pagosadmin no importa redisclient.
func wireAdmin(a *pagosadmin.AdminPagos) *pagosadmin.AdminPagos {
	a.SetOnGuardado(func(externalid string) {
		ctx := context.Background()
		removeListaID(ctx, externalid)
		removePorNotificar(ctx, externalid)
		// Liberar caches auxiliares de notificaciones para este externalid (evita
		// que notifjson/notifPr crezcan indefinidamente).
		admpagos := getAdmin()
		admpagos.LimpiarNotif(externalid)
	})
	return a
}

// getAdmin devuelve el AdminPagos, inicializandolo perezosamente si falta.
func getAdmin() *pagosadmin.AdminPagos {
	adminMu.Lock()
	defer adminMu.Unlock()
	if admin == nil {
		admin = wireAdmin(pagosadmin.New(database.Get()))
	}
	return admin
}

// =============================================================================
// Helpers HTTP
// =============================================================================

// writeJSON serializa v como JSON con el status indicado.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError500 replica el error 500 de FastAPI: {"detail": msg}.
func writeError500(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": msg})
}

// writeAuthError replica el HTTPException 401 de dependencies.verify_auth.
func writeAuthError(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"detail": map[string]any{
			"estado": 8,
			"msg":    "Acceso denegado, Usuario, Password o Source no válido",
		},
	})
}

// writeValidationError emite un 422 estilo FastAPI.
func writeValidationError(w http.ResponseWriter, ve *models.ValidationError) {
	writeJSON(w, http.StatusUnprocessableEntity, ve)
}

// Limites de tamano de payload. El legacy (FastAPI/httpx) no los tenia:
// io.ReadAll sin tope permitia un body ilimitado en la entrada y una respuesta
// ilimitada de ETECSA.
const (
	maxRequestBody  = 1 << 20 // 1 MiB
	maxResponseBody = 4 << 20 // 4 MiB
)

// decodeJSON lee el body, verifica campos requeridos (como pydantic) y
// deserializa en v. Devuelve false si ya se escribio la respuesta de error.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, required []string) bool {
	// MaxBytesReader corta la lectura al superarse el limite (y cierra el body),
	// de modo que un body gigante no se buffers en memoria.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail": []models.ValidationErrorItem{{Loc: []string{"body"}, Msg: "There was an error parsing the body", Type: "json_invalid"}},
		})
		return false
	}

	// Detectar campos requeridos ausentes (pydantic: "Field required").
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail": []models.ValidationErrorItem{{Loc: []string{"body"}, Msg: "There was an error parsing the body", Type: "json_invalid"}},
		})
		return false
	}
	var errs []models.ValidationErrorItem
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			errs = append(errs, models.ValidationErrorItem{Loc: []string{"body", f}, Msg: "Field required", Type: "missing"})
		}
	}
	if len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"detail": errs})
		return false
	}

	if err := json.Unmarshal(body, v); err != nil {
		// Error de tipo (p.ej. Amount como string) -> 422 estilo pydantic.
		if ute, ok := err.(*json.UnmarshalTypeError); ok {
			typ := ute.Type.String()
			switch ute.Type.Kind() {
			case reflect.Float64, reflect.Float32:
				typ = "float"
			case reflect.String:
				typ = "string"
			case reflect.Bool:
				typ = "bool"
			case reflect.Int, reflect.Int64:
				typ = "integer"
			}
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"detail": []models.ValidationErrorItem{{
					Loc:  []string{"body", ute.Field},
					Msg:  fmt.Sprintf("Input should be a valid %s", typ),
					Type: "type_error",
				}},
			})
			return false
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail": []models.ValidationErrorItem{{Loc: []string{"body"}, Msg: "There was an error parsing the body", Type: "json_invalid"}},
		})
		return false
	}
	return true
}

// verifyAuth replica dependencies.verify_auth (headers username/password/source).
func verifyAuth(r *http.Request) bool {
	headers := map[string]string{
		"username": r.Header.Get("username"),
		"password": r.Header.Get("password"),
		"source":   r.Header.Get("source"),
	}
	return utils.ValidarUsuario(headers, time.Now())
}

// clientIP replica request.client.host (IP directa del peer).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// =============================================================================
// Llamadas HTTP a ETECSA (httpx.AsyncClient con data= y headers)
// =============================================================================

// doRequest ejecuta una peticion HTTP y parsea la respuesta JSON.
func doRequest(ctx context.Context, method, url, body string, headers map[string]string) (map[string]any, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpclient.Get().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// La respuesta de ETECSA tambien tiene tope: un host comprometido (o un
	// MITM) no puede agotar la memoria del proceso.
	data, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, maxResponseBody))
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// postJSON replica client.post(url, data=body, headers=headers).
func postJSON(ctx context.Context, url, body string, headers map[string]string) (map[string]any, error) {
	return doRequest(ctx, http.MethodPost, url, body, headers)
}

// getJSON replica client.get(url, headers=headers).
func getJSON(ctx context.Context, url string, headers map[string]string) (map[string]any, error) {
	return doRequest(ctx, http.MethodGet, url, "", headers)
}

// =============================================================================
// Conversiones
// =============================================================================

func toInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int8:
		return int(t)
	case int16:
		return int(t)
	case int32:
		return int(t)
	case int64:
		return int(t)
	case uint8:
		return int(t)
	case uint16:
		return int(t)
	case uint32:
		return int(t)
	case uint64:
		return int(t)
	case float32:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint8:
		return int64(t)
	case uint16:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float32:
		return float64(t)
	case float64:
		return t
	case int:
		return float64(t)
	case int8:
		return float64(t)
	case int16:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case uint8:
		return float64(t)
	case uint16:
		return float64(t)
	case uint32:
		return float64(t)
	case uint64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

// toString convierte un valor a string (para columnas varchar).
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float64:
		return fmt.Sprintf("%v", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int32:
		return fmt.Sprintf("%d", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// orEmpty devuelve "" si v es nil, si no el valor (para campos mixtos).
func orEmpty(v any) any {
	if v == nil {
		return ""
	}
	return v
}

// orEmptyStr devuelve "" si v es nil, si no str(v) (como str() de Python).
func orEmptyStr(v any) any {
	if v == nil {
		return ""
	}
	return toString(v)
}

// orZero devuelve 0 si v es nil, si no el valor.
func orZero(v any) any {
	if v == nil {
		return 0
	}
	return v
}

// orMinusOne devuelve -1 si v es nil, si no el valor.
func orMinusOne(v any) any {
	if v == nil {
		return -1
	}
	return v
}

// =============================================================================
// Registro de rutas (equivalente a app.include_router)
// =============================================================================

// RegisterRoutes registra todos los endpoints en el ServeMux.
// Se registran variantes con y sin slash final (FastAPI redirige, aqui se
// sirven ambas directamente).
func RegisterRoutes(mux *http.ServeMux) {
	// Pagos
	mux.HandleFunc("POST /pago/", PagoHandler)
	mux.HandleFunc("POST /pago", PagoHandler)
	mux.HandleFunc("POST /pago_a/", PagoAHandler)
	mux.HandleFunc("POST /pago_a", PagoAHandler)
	mux.HandleFunc("POST /cancelar/", CancelarHandler)
	mux.HandleFunc("POST /cancelar", CancelarHandler)
	mux.HandleFunc("POST /notificapagos/", NotificacionHandler)
	mux.HandleFunc("POST /notificapagos", NotificacionHandler)

	// Devoluciones
	mux.HandleFunc("POST /devolucion/", DevolucionHandler)
	mux.HandleFunc("POST /devolucion", DevolucionHandler)
	mux.HandleFunc("POST /notificaciondevol/", NotificacionDevolHandler)
	mux.HandleFunc("POST /notificaciondevol", NotificacionDevolHandler)

	// Estado
	mux.HandleFunc("GET /estadoordenpago/{externalid}/{source}/", EstadoOrdenHandler)
	mux.HandleFunc("GET /estadoordenpago/{externalid}/{source}", EstadoOrdenHandler)
	mux.HandleFunc("GET /estadoordenpagolocal/{externalid}/{source}/", EstadoOrdenLocalHandler)
	mux.HandleFunc("GET /estadoordenpagolocal/{externalid}/{source}", EstadoOrdenLocalHandler)
	mux.HandleFunc("GET /estadodevolucion/{externalid}/{source}/{tmid}/", EstadoDevolucionHandler)
	mux.HandleFunc("GET /estadodevolucion/{externalid}/{source}/{tmid}", EstadoDevolucionHandler)
}

// Cfg es un alias para acceder a la configuracion desde los handlers.
var _ = config.Cfg