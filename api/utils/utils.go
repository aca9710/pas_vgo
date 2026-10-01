// Package utils replica utils.py: generadores de id, autenticacion,
// logging (traza/ver_error/data) y helpers de base de datos.
package utils

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"pasarela/config"
	"pasarela/database"
)

// =============================================================================
// Generadores de ID
// =============================================================================

const alfaChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// IDAlfaGen replica utils.id_alfagen: (size-4) caracteres aleatorios
// mayusculas+digitos, seguidos de MMSS y un espacio.
func IDAlfaGen(size int) string {
	n := size - 4
	if n < 0 {
		n = 0
	}
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alfaChars))))
		b[i] = alfaChars[idx.Int64()]
	}
	txthora := time.Now().Format("0405") // MMSS
	return string(b) + txthora + " "
}

// =============================================================================
// Origen (UID-IDOPERACION)
// =============================================================================

// ObtenerOrigen separa un ExternalId en uid e id_operacion.
// Replica utils.obtener_origen: el guion debe estar en posicion > 0.
func ObtenerOrigen(externalID string) (string, string, error) {
	guion := strings.Index(externalID, "-")
	if guion <= 0 {
		return "", "", fmt.Errorf("Formato inválido")
	}
	return externalID[:guion], externalID[guion+1:], nil
}

// =============================================================================
// Autenticacion
// =============================================================================

// fechaAuth devuelve dia, mes, anio sin ceros a la izquierda
// (replica el strftime "%d,%m,%Y" + strip de ceros de utils.py).
func fechaAuth(now time.Time) (string, string, string) {
	parts := strings.Split(now.Format("02,01,2006"), ",")
	for i := 0; i < 2; i++ {
		if len(parts[i]) == 2 && parts[i][0] == '0' {
			parts[i] = parts[i][1:]
		}
	}
	return parts[0], parts[1], parts[2]
}

// passwordAuth calcula base64(sha512(f"{usuario}{dia}{mes}{año}{semilla}{source}")).
func passwordAuth(usuario, semilla, source string, now time.Time) string {
	dia, mes, anio := fechaAuth(now)
	password := fmt.Sprintf("%s%s%s%s%s%s", usuario, dia, mes, anio, semilla, source)
	hash := sha512.Sum512([]byte(password))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// ValidarUsuario replica utils.validar_usuario.
func ValidarUsuario(headers map[string]string, now time.Time) bool {
	usuario := headers["username"]
	rpassword := headers["password"]
	source := headers["source"]

	if usuario == "" || rpassword == "" || source == "" {
		return false
	}

	semilla := config.Cfg.SemillaAuth
	if semilla == "" {
		semilla = "externalpayment"
	}

	return passwordAuth(usuario, semilla, source, now) == rpassword
}

// PreparaConexion replica utils.prepara_conexion: headers de autenticacion
// para las llamadas a ETECSA.
func PreparaConexion(semilla, usuario, source string, now time.Time) map[string]string {
	if semilla == "" {
		semilla = "externalpayment"
	}
	if usuario == "" {
		usuario = "cimex"
	}
	if source == "" {
		source = "30010"
	}
	return map[string]string{
		"Content-Type": "application/json",
		"username":     usuario,
		"source":       source,
		"password":     passwordAuth(usuario, semilla, source, now),
	}
}

// =============================================================================
// Logging (traza / ver_error / data)
// =============================================================================

// txthora12 devuelve la hora en formato "%I:%M:%S a.m./p.m.".
func txthora12(t time.Time) string {
	ampm := "a.m."
	if t.Hour() > 11 {
		ampm = "p.m."
	}
	return t.Format("03:04:05") + " " + ampm
}

// Traza replica utils.traza: escribe en logs/traza_YYYYMMDD.log.
func Traza(texto, idTraza string) {
	defer func() { _ = recover() }()

	now := time.Now()
	txtfecha := now.Format("02/01/2006 ") // %d/%m/%Y
	txthora := txthora12(now)

	texto = strings.ReplaceAll(texto, "*f1", fmt.Sprintf("\n%s, %s        ", txtfecha, txthora))

	logDir := filepath.Join(config.ProjectRoot(), "logs")
	_ = os.MkdirAll(logDir, 0o755)

	fichero := filepath.Join(logDir, fmt.Sprintf("traza_%s.log", now.Format("20060102")))
	f, err := os.OpenFile(fichero, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(fmt.Sprintf("%s, %s  %s %s \n", txtfecha, txthora, idTraza, texto))
}

// VerError replica utils.ver_error: escribe en log/error_YYYYMMDD.log y
// devuelve el texto (con stack trace si lo hay).
func VerError(texto string) string {
	defer func() { _ = recover() }()

	now := time.Now()
	txtfecha := now.Format("02/01/2006 ")
	txthora := txthora12(now)

	logDir := filepath.Join(config.ProjectRoot(), "log")
	_ = os.MkdirAll(logDir, 0o755)

	fichero := filepath.Join(logDir, fmt.Sprintf("error_%s.log", now.Format("20060102")))
	f, err := os.OpenFile(fichero, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ""
	}
	defer f.Close()

	_, _ = f.WriteString(fmt.Sprintf("%s\n%s, %s \n", strings.Repeat("-", 78), txtfecha, txthora))
	_, _ = f.WriteString(texto + "\n")

	// Equivalente al traceback de Python: stack del goroutine actual.
	stack := string(debug.Stack())
	_, _ = f.WriteString(stack + "\n")
	return texto
}

// Data replica utils.data: escribe lineas en data/{nfichero}_YYYYMMDD.txt.
func Data(nfichero string, dato []string) {
	now := time.Now()
	nombre := fmt.Sprintf("%s_%s.txt", nfichero, now.Format("20060102"))
	fichero := filepath.Join(config.ProjectRoot(), "data", nombre)
	_ = os.MkdirAll(filepath.Dir(fichero), 0o755)

	f, err := os.OpenFile(fichero, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	for _, linea := range dato {
		_, _ = f.WriteString(linea + "\n")
	}
}

// =============================================================================
// Helpers de base de datos (chk_url / chk_msg / ipvalido / ipvalidotv)
// =============================================================================

// toInt convierte un valor normalizado (int64/float64/int) a int.
func toInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case nil:
		return 0
	default:
		return 0
	}
}

// ChkURL replica utils.chk_url: busca/inserta la URL y devuelve su id.
func ChkURL(ctx context.Context, url string, urlList *config.SafeMap, db *database.Database) (int, error) {
	u := strings.TrimSpace(url)
	if len(u) == 0 {
		u = "------"
	}
	if v := urlList.Get(u); v != nil {
		return toInt(v), nil
	}

	rows, err := db.Select(ctx, "SELECT id, url FROM public.url WHERE url = $1", u)
	if err != nil {
		return 0, err
	}
	var idurl int
	if len(rows) == 0 {
		v, err := db.InsertReturning(ctx, "INSERT INTO public.url(url) VALUES ($1) RETURNING id", u)
		if err != nil {
			return 0, err
		}
		idurl = toInt(v)
	} else {
		idurl = toInt(rows[0]["id"])
	}
	urlList.Set(u, idurl)
	return idurl, nil
}

// ChkMsg replica utils.chk_msg: busca/inserta el mensaje y devuelve su id.
func ChkMsg(ctx context.Context, msg string, msgList *config.SafeMap, db *database.Database) (int, error) {
	if v := msgList.Get(msg); v != nil {
		return toInt(v), nil
	}

	rows, err := db.Select(ctx, "SELECT id FROM public.msg WHERE texto = $1", msg)
	if err != nil {
		return 0, err
	}
	var idmsg int
	if len(rows) == 0 {
		v, err := db.InsertReturning(ctx, "INSERT INTO public.msg(texto) VALUES ($1) RETURNING id", msg)
		if err != nil {
			return 0, err
		}
		idmsg = toInt(v)
	} else {
		idmsg = toInt(rows[0]["id"])
	}
	msgList.Set(msg, idmsg)
	return idmsg, nil
}

// GetErrorList devuelve DICRES (replica utils.get_error_list).
func GetErrorList() map[int]string {
	return config.GetErrorList()
}

// IPValido replica utils.ipvalido: consulta listaip.
func IPValido(ctx context.Context, ip string, produccion bool, db *database.Database) (bool, error) {
	rows, err := db.Select(ctx,
		"SELECT id, uid, nombre, produccion, activa FROM listaip WHERE ip = $1 AND activa = true AND produccion = $2",
		ip, produccion)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// IPValidoTV replica utils.ipvalidotv: comprueba si la ip es substring de
// produccion/desarrollo de la tienda (mismo comportamiento que Python).
func IPValidoTV(ctx context.Context, uid, ip string, db *database.Database) (bool, error) {
	rows, err := db.Select(ctx, "SELECT produccion, desarrollo FROM tiendas WHERE uid = $1", uid)
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		produccion, _ := r["produccion"].(string)
		desarrollo, _ := r["desarrollo"].(string)
		if strings.Contains(produccion, ip) || strings.Contains(desarrollo, ip) {
			return true, nil
		}
	}
	return false, nil
}