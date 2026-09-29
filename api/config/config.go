// Package config replica config.py: Settings (pydantic-settings) y los
// diccionarios thread-safe (ThreadSafeDict) usados por la aplicacion.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

// =============================================================================
// SafeMap — equivalente Go de ThreadSafeDict (config.py)
// =============================================================================

// SafeMap es un mapa thread-safe con locks para operaciones atomicas.
type SafeMap struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewSafeMap() *SafeMap {
	return &SafeMap{data: make(map[string]any)}
}

func (m *SafeMap) Get(key string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key]
}

func (m *SafeMap) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *SafeMap) Contains(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok
}

func (m *SafeMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Items devuelve una copia de los pares (key, value).
func (m *SafeMap) Items() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]any, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out
}

// =============================================================================
// Instancias globales thread-safe (reemplazan los diccionarios simples)
// =============================================================================

var (
	MsgList   = NewSafeMap()
	URLList   = NewSafeMap()
	URLValido = NewSafeMap()
)

// =============================================================================
// Settings — equivalente Go de pydantic Settings (config.py)
// =============================================================================

type Config struct {
	DSN        string
	APIHost    string
	APIPort    int
	Debug      bool
	PasarelaURL string
	OrdenPago  string
	Devolucion string
	EstadoDevol string
	EstadoOrden string
	NotifPago  string
	URLResDevol string
	SecretKey  string
	SemillaAuth string
	RedisDSN   string
	PgMinSize  int
	PgMaxSize  int
}

// Cfg es la instancia global de configuracion (equivalente a settings).
var Cfg = Config{
	DSN:         "",
	APIHost:     "0.0.0.0",
	APIPort:     5081,
	Debug:       false,
	PasarelaURL: "",
	OrdenPago:   "",
	Devolucion:  "",
	EstadoDevol: "",
	EstadoOrden: "",
	NotifPago:   "",
	URLResDevol: "",
	SecretKey:   "",
	SemillaAuth: "",
	RedisDSN:    "redis://localhost:6379/0",
	PgMinSize:   10,
	PgMaxSize:   100,
}

// getEnv lee una variable de entorno con valor por defecto.
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvInt lee una variable de entorno entera con valor por defecto.
func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// getEnvBool lee una variable de entorno booleana con valor por defecto.
func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// Load carga la configuracion desde .env (o ../.env como fallback) y las
// variables de entorno. Equivalente a Settings() de pydantic-settings.
func Load() {
	// pydantic-settings lee ".env" relativo al cwd. Si no existe (p.ej. al
	// ejecutar el binario desde golang/), se intenta ../.env (raiz del repo).
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	Cfg.DSN = getEnv("DSN", Cfg.DSN)
	Cfg.APIHost = getEnv("API_HOST", Cfg.APIHost)
	Cfg.APIPort = getEnvInt("API_PORT", Cfg.APIPort)
	Cfg.Debug = getEnvBool("DEBUG", Cfg.Debug)
	Cfg.PasarelaURL = getEnv("PASARELA_URL", Cfg.PasarelaURL)
	Cfg.OrdenPago = getEnv("ORDENPAGO", Cfg.OrdenPago)
	Cfg.Devolucion = getEnv("DEVOLUCION", Cfg.Devolucion)
	Cfg.EstadoDevol = getEnv("ESTADODEVOL", Cfg.EstadoDevol)
	Cfg.EstadoOrden = getEnv("ESTADOORDEN", Cfg.EstadoOrden)
	Cfg.NotifPago = getEnv("NOTIF_PAGO", Cfg.NotifPago)
	Cfg.URLResDevol = getEnv("URL_RES_DEVOL", Cfg.URLResDevol)
	Cfg.SecretKey = getEnv("SECRET_KEY", Cfg.SecretKey)
	Cfg.SemillaAuth = getEnv("SEMILLA_AUTH", Cfg.SemillaAuth)
	Cfg.RedisDSN = getEnv("REDIS_DSN", Cfg.RedisDSN)
	Cfg.PgMinSize = getEnvInt("PG_MIN_SIZE", Cfg.PgMinSize)
	Cfg.PgMaxSize = getEnvInt("PG_MAX_SIZE", Cfg.PgMaxSize)
}

// =============================================================================
// DICRES — codigos de error (config.py)
// =============================================================================

var DICRES = map[int]string{
	0:  "En proceso", 1: "ok", 2: "Fondos insuficientes", 3: "Operación inválida",
	4:  "Licencia no válida", 5: "Limite de compra excedido", 6: "Origen no registrado",
	7:  "Error de conexión", 8: "Acceso denegado, Usuario, Password o Source no válido",
	9:  "Faltan datos", 10: "Error interno, ver traza", 11: "Cancelado por el usuario",
	12: "Falta separador origen-No.operacion", 13: "UrlResponse no puede estar vacio",
	14: "Transacción duplicada", 15: "Solicitud de pago aceptada por etecsa",
	16: "Solicitud de pago no es aceptada por etecsa", 17: "La respuesta de ETECSA no se entiende",
	18: "Pago aceptado", 19: "Transacción desconocida", 20: "Error de conexión al intentar notificar al origen",
	21: "Base de datos corrupta, ver traza", 22: "Notificación procesada",
	23: "Devolución de pago aceptada", 24: "Devolución de pago no es aceptada por etecsa",
	25: "Dirección IP del origen no es válida", 26: "Devolución efectuada", 27: "Devolución denegada",
	28: "La estructura del JSON no esta correcta", 29: "El identificador de la caja no es válido",
	30: "Operacion Nueva", 31: "Operacion notificada exitosa", 32: "Operacion fallida sin notificar",
	33: "Operacion fallida", 34: "Operacion pendiente rollback", 35: "Operacion rollback ok",
	36: "El usuario no escaneo el qr", 37: "El importe de la devolucion excede lo pagado",
	38: "Notificacion repetida, aceptada", 39: "Notificacion repetida, no se acepta",
	40: "Telefono en lista negra hasta %s", 41: "El campo UrlResponse no contiene un URL valido",
	50: "Solicitud de pago no enviada",
	51: "Solicitud de pago aceptada por transfermovil",
	52: "Solicitud de pago no aceptada por transfermovil",
	53: "Solicitud de pago notificada",
	54: "Solicitud de pago vencida por tiempo",
	55: "La solicitud de pago no existe en el sistema",
}

// GetErrorList devuelve DICRES (equivalente a utils.get_error_list()).
func GetErrorList() map[int]string {
	return DICRES
}

// ProjectRoot devuelve la raiz del proyecto (directorio padre de golang/).
func ProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// Si estamos dentro de golang/, subimos un nivel.
	if filepath.Base(wd) == "golang" {
		return filepath.Dir(wd)
	}
	return wd
}