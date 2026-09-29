package config

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// =============================================================================
// SafeMap
//
// Nota: SafeMap expone Get/Set/Contains/Len/Items. NO tiene metodo Delete
// (a diferencia de lo supuesto), asi que no hay Delete que probar.
// =============================================================================

func TestSafeMap(t *testing.T) {
	m := NewSafeMap()
	if m == nil {
		t.Fatal("NewSafeMap devolvio nil")
	}
	if got := m.Len(); got != 0 {
		t.Fatalf("mapa nuevo deberia tener len 0, obtuve %d", got)
	}
	if m.Contains("clave") {
		t.Fatal("mapa nuevo no deberia contener claves")
	}
	if v := m.Get("clave"); v != nil {
		t.Fatalf("Get de clave inexistente deberia ser nil, obtuve %v", v)
	}

	m.Set("caja", 1)
	m.Set("phone", "5355550000")
	m.Set("external_id", "caja-123")

	if got := m.Len(); got != 3 {
		t.Fatalf("len esperado 3, obtuve %d", got)
	}
	if !m.Contains("caja") {
		t.Error("deberia contener 'caja'")
	}
	if v := m.Get("caja"); v != 1 {
		t.Errorf("Get(caja) = %v, esperaba 1", v)
	}
	if v := m.Get("phone"); v != "5355550000" {
		t.Errorf("Get(phone) = %v, esperaba 5355550000", v)
	}

	// Set sobreescribe el valor previo.
	m.Set("caja", 2)
	if v := m.Get("caja"); v != 2 {
		t.Errorf("Get(caja) tras re-Set = %v, esperaba 2", v)
	}

	// Items devuelve una copia: mutarla no afecta al mapa original.
	items := m.Items()
	items["fantasma"] = true
	if m.Contains("fantasma") {
		t.Error("Items() deberia devolver una copia independiente")
	}
	if got := len(items); got != 4 {
		t.Errorf("len(items) = %d, esperaba 4 (3 originales + 1 agregado a la copia)", got)
	}
}

func TestSafeMapConcurrent(t *testing.T) {
	m := NewSafeMap()
	const goroutines = 16
	const iterations = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("g%d-%d", g, i)
				m.Set(key, i)
				_ = m.Get(key)
				_ = m.Contains(key)
				_ = m.Len()
				_ = m.Items()
			}
		}(g)
	}
	wg.Wait()

	want := goroutines * iterations
	if got := m.Len(); got != want {
		t.Errorf("len final = %d, esperaba %d", got, want)
	}
	// Cada goroutine debe haber dejado su primera clave visible.
	if v := m.Get("g0-0"); v != 0 {
		t.Errorf("Get(g0-0) = %v, esperaba 0", v)
	}
}

// =============================================================================
// DICRES / GetErrorList
// =============================================================================

func TestDICRES(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "En proceso"},
		{1, "ok"},
		{2, "Fondos insuficientes"},
		{5, "Limite de compra excedido"},
		{18, "Pago aceptado"},
		{26, "Devolución efectuada"},
		{40, "Telefono en lista negra hasta %s"}, // placeholder %s, como en config.py
		{55, "La solicitud de pago no existe en el sistema"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("codigo %d", c.code), func(t *testing.T) {
			got, ok := DICRES[c.code]
			if !ok {
				t.Fatalf("DICRES[%d] no existe", c.code)
			}
			if got != c.want {
				t.Errorf("DICRES[%d] = %q, esperaba %q", c.code, got, c.want)
			}
		})
	}

	// Las claves reales van de 0 a 41 y de 50 a 55; no existe la 100.
	if _, ok := DICRES[100]; ok {
		t.Error("DICRES[100] no deberia existir (claves reales: 0-41 y 50-55)")
	}
}

func TestGetErrorList(t *testing.T) {
	list := GetErrorList()
	if list == nil {
		t.Fatal("GetErrorList devolvio nil")
	}
	if len(list) != len(DICRES) {
		t.Errorf("longitud de GetErrorList = %d, DICRES = %d", len(list), len(DICRES))
	}
	for k, v := range DICRES {
		got, ok := list[k]
		if !ok || got != v {
			t.Errorf("GetErrorList()[%d] = %q (ok=%v), esperaba %q", k, got, ok, v)
		}
	}
}

// =============================================================================
// ProjectRoot
// =============================================================================

func TestProjectRoot(t *testing.T) {
	root := ProjectRoot()
	if root == "" {
		t.Fatal("ProjectRoot devolvio cadena vacia")
	}
	if !filepath.IsAbs(root) {
		t.Errorf("ProjectRoot deberia devolver un path absoluto, obtuve %q", root)
	}
}

// =============================================================================
// Load
//
// Se usan t.Setenv para controlar las variables (godotenv no pisa variables ya
// definidas), de modo que los tests no dependen del .env del repo ni del entorno.
//
// Nota: los nombres reales en Go son Cfg.APIPort (no API_PORT), Cfg.DSN,
// Cfg.APIHost, Cfg.Debug, etc.
// =============================================================================

var allEnvKeys = []string{
	"DSN", "API_HOST", "API_PORT", "DEBUG", "PASARELA_URL", "ORDENPAGO",
	"DEVOLUCION", "ESTADODEVOL", "ESTADOORDEN", "NOTIF_PAGO", "URL_RES_DEVOL",
	"SECRET_KEY", "SEMILLA_AUTH", "REDIS_DSN", "PG_MIN_SIZE", "PG_MAX_SIZE",
}

// resetCfg restaura la instancia global Cfg a su estado inicial (el literal
// documentado en config.go). Necesario porque Load() usa Cfg.<campo> como
// valor por defecto: si un test anterior cargo otros valores, los defaults
// dejan de ser los literales iniciales.
func resetCfg() {
	Cfg = Config{
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
}

func TestLoadPopulatesCfg(t *testing.T) {
	resetCfg()
	env := map[string]string{
		"DSN":           "host=test dbname=test user=u password=p port=5432",
		"API_HOST":      "127.0.0.1",
		"API_PORT":      "6123",
		"DEBUG":         "true",
		"PASARELA_URL":  "http://pasarela.test",
		"ORDENPAGO":     "http://etecsa.test/payorder/",
		"DEVOLUCION":    "http://etecsa.test/refundPay/",
		"ESTADODEVOL":   "http://etecsa.test/getStatusRefundOrder/",
		"ESTADOORDEN":   "http://etecsa.test/getStatusOrder/",
		"NOTIF_PAGO":    "http://etecsa.test/notificapagos/",
		"URL_RES_DEVOL": "http://etecsa.test/notificaciondevol/",
		"SECRET_KEY":    "clave-secreta-test",
		"SEMILLA_AUTH":  "semilla-test",
		"REDIS_DSN":     "redis://redis-test:7000/1",
		"PG_MIN_SIZE":   "2",
		"PG_MAX_SIZE":   "7",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	Load()

	if Cfg.DSN != env["DSN"] {
		t.Errorf("Cfg.DSN = %q, esperaba %q", Cfg.DSN, env["DSN"])
	}
	if Cfg.APIHost != env["API_HOST"] {
		t.Errorf("Cfg.APIHost = %q, esperaba %q", Cfg.APIHost, env["API_HOST"])
	}
	if Cfg.APIPort != 6123 {
		t.Errorf("Cfg.APIPort = %d, esperaba 6123", Cfg.APIPort)
	}
	if !Cfg.Debug {
		t.Error("Cfg.Debug deberia ser true")
	}
	if Cfg.PasarelaURL != env["PASARELA_URL"] {
		t.Errorf("Cfg.PasarelaURL = %q, esperaba %q", Cfg.PasarelaURL, env["PASARELA_URL"])
	}
	if Cfg.OrdenPago != env["ORDENPAGO"] {
		t.Errorf("Cfg.OrdenPago = %q, esperaba %q", Cfg.OrdenPago, env["ORDENPAGO"])
	}
	if Cfg.Devolucion != env["DEVOLUCION"] {
		t.Errorf("Cfg.Devolucion = %q, esperaba %q", Cfg.Devolucion, env["DEVOLUCION"])
	}
	if Cfg.EstadoDevol != env["ESTADODEVOL"] {
		t.Errorf("Cfg.EstadoDevol = %q, esperaba %q", Cfg.EstadoDevol, env["ESTADODEVOL"])
	}
	if Cfg.EstadoOrden != env["ESTADOORDEN"] {
		t.Errorf("Cfg.EstadoOrden = %q, esperaba %q", Cfg.EstadoOrden, env["ESTADOORDEN"])
	}
	if Cfg.NotifPago != env["NOTIF_PAGO"] {
		t.Errorf("Cfg.NotifPago = %q, esperaba %q", Cfg.NotifPago, env["NOTIF_PAGO"])
	}
	if Cfg.URLResDevol != env["URL_RES_DEVOL"] {
		t.Errorf("Cfg.URLResDevol = %q, esperaba %q", Cfg.URLResDevol, env["URL_RES_DEVOL"])
	}
	if Cfg.SecretKey != env["SECRET_KEY"] {
		t.Errorf("Cfg.SecretKey = %q, esperaba %q", Cfg.SecretKey, env["SECRET_KEY"])
	}
	if Cfg.SemillaAuth != env["SEMILLA_AUTH"] {
		t.Errorf("Cfg.SemillaAuth = %q, esperaba %q", Cfg.SemillaAuth, env["SEMILLA_AUTH"])
	}
	if Cfg.RedisDSN != env["REDIS_DSN"] {
		t.Errorf("Cfg.RedisDSN = %q, esperaba %q", Cfg.RedisDSN, env["REDIS_DSN"])
	}
	if Cfg.PgMinSize != 2 {
		t.Errorf("Cfg.PgMinSize = %d, esperaba 2", Cfg.PgMinSize)
	}
	if Cfg.PgMaxSize != 7 {
		t.Errorf("Cfg.PgMaxSize = %d, esperaba 7", Cfg.PgMaxSize)
	}
}

func TestLoadDefaults(t *testing.T) {
	resetCfg()
	// Forzar variables vacias: getEnv/getEnvInt/getEnvBool tratan "" como no
	// definido y caen al default, y godotenv no pisa variables ya definidas.
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}

	Load()

	if Cfg.DSN != "" {
		t.Errorf("Cfg.DSN default = %q, esperaba cadena vacia", Cfg.DSN)
	}
	if Cfg.APIHost != "0.0.0.0" {
		t.Errorf("Cfg.APIHost default = %q, esperaba 0.0.0.0", Cfg.APIHost)
	}
	if Cfg.APIPort != 5081 {
		t.Errorf("Cfg.APIPort default = %d, esperaba 5081", Cfg.APIPort)
	}
	if Cfg.Debug {
		t.Error("Cfg.Debug default deberia ser false")
	}
	if Cfg.PasarelaURL != "" {
		t.Errorf("Cfg.PasarelaURL default = %q, esperaba cadena vacia", Cfg.PasarelaURL)
	}
	if Cfg.OrdenPago != "" {
		t.Errorf("Cfg.OrdenPago default = %q, esperaba cadena vacia", Cfg.OrdenPago)
	}
	if Cfg.Devolucion != "" {
		t.Errorf("Cfg.Devolucion default = %q, esperaba cadena vacia", Cfg.Devolucion)
	}
	if Cfg.EstadoDevol != "" {
		t.Errorf("Cfg.EstadoDevol default = %q, esperaba cadena vacia", Cfg.EstadoDevol)
	}
	if Cfg.EstadoOrden != "" {
		t.Errorf("Cfg.EstadoOrden default = %q, esperaba cadena vacia", Cfg.EstadoOrden)
	}
	if Cfg.NotifPago != "" {
		t.Errorf("Cfg.NotifPago default = %q, esperaba cadena vacia", Cfg.NotifPago)
	}
	if Cfg.URLResDevol != "" {
		t.Errorf("Cfg.URLResDevol default = %q, esperaba cadena vacia", Cfg.URLResDevol)
	}
	if Cfg.SecretKey != "" {
		t.Errorf("Cfg.SecretKey default = %q, esperaba cadena vacia", Cfg.SecretKey)
	}
	if Cfg.SemillaAuth != "" {
		t.Errorf("Cfg.SemillaAuth default = %q, esperaba cadena vacia", Cfg.SemillaAuth)
	}
	if Cfg.RedisDSN != "redis://localhost:6379/0" {
		t.Errorf("Cfg.RedisDSN default = %q, esperaba redis://localhost:6379/0", Cfg.RedisDSN)
	}
	if Cfg.PgMinSize != 10 {
		t.Errorf("Cfg.PgMinSize default = %d, esperaba 10", Cfg.PgMinSize)
	}
	if Cfg.PgMaxSize != 100 {
		t.Errorf("Cfg.PgMaxSize default = %d, esperaba 100", Cfg.PgMaxSize)
	}
}