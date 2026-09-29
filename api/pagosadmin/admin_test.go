// Package pagosadmin: tests unitarios de la gestion en memoria de pagos.
//
// Solo se prueban metodos que NO requieren PostgreSQL/Redis/red:
//   - DataPago: Vencido, Notifica, Notificado (operan sobre campos del struct).
//   - AdminPagos: New, chkLock, lock, NotificaJSON, logLocked, Add,
//     Vencimiento, Notifica (con <= 20 pagos), NumOrden/ExistePago en su rama
//     en memoria, y ProcesarLista solo en estados sin trabajo (LOCK o mapas
//     vacios).
//
// Se omiten (requieren *database.Database vivo):
//   - DataPago.Guarda / initAsync: INSERT a pagos + ChkURL/ChkMsg sobre la BD.
//   - AdminPagos.NumOrden rama "no en memoria": a.db.Execute.
//   - AdminPagos.ExistePago rama "no en memoria": a.db.Select.
//   - AdminPagos.ProcesarLista con pagos vencidos/notificaciones: Guarda + queries.
//   - AdminPagos.Notifica cuando len(solPagos) > 20: dispara ProcesarLista.
//
// New(nil) es valido aqui porque los metodos probados nunca tocan a.db.
//
// logLocked con > 100 entradas llama a utils.Data (escribe data/*.txt bajo
// config.ProjectRoot()); TestMain hace chdir a un directorio temporal para que
// esas escrituras no toquen el repo (mismo patron que utils/utils_test.go).
package pagosadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"pasarela/models"
)

// TestMain redirige el cwd a un dir temporal (utils.Data escribe bajo
// config.ProjectRoot() cuando logLocked supera las 100 lineas) y lo restaura.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pagosadmin-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "pagosadmin: MkdirTemp: %v\n", err)
		os.Exit(1)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pagosadmin: Getwd: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintf(os.Stderr, "pagosadmin: Chdir: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.Chdir(oldWd)
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// =============================================================================
// Helpers
// =============================================================================

// pagoManual construye un DataPago via literal de struct (sin NewDataPago,
// que no toca BD pero introduce dependencias de tiempo/parseo innecesarias).
func pagoManual(externalID string, venceDesdeAhora time.Duration) *DataPago {
	return &DataPago{
		vence:      time.Now().Add(venceDesdeAhora),
		ExternalId: externalID,
		Status:     "0",
	}
}

func reqPago(ext string) *models.SolicitudPagoRequest {
	return &models.SolicitudPagoRequest{
		Amount:      25.50,
		Phone:       "5355550000",
		Currency:    "CUP",
		Description: "pago de prueba",
		ExternalId:  ext,
		Source:      "30010",
		ValidTime:   "60",
	}
}

func notifReq(ext string) *models.NotificacionRequest {
	return &models.NotificacionRequest{
		Phone:      "5355550001",
		ExternalId: ext,
		TmId:       "TM-777",
		BankId:     "BANK-1",
		Status:     "18",
		Source:     "30010",
		Msg:        "Pago aceptado",
		Bank:       "BANDEC",
	}
}

// =============================================================================
// DataPago — metodos puros sobre campos del struct
// =============================================================================

func TestDataPagoVencido(t *testing.T) {
	cases := []struct {
		name         string
		venceDesde   time.Duration
		wantVencido  bool
	}{
		{"vence en el futuro lejano", 24 * time.Hour, false},
		{"vence en el futuro cercano", 5 * time.Second, false},
		{"vence en el pasado", -1 * time.Second, true},
		{"vence hace mucho", -24 * time.Hour, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pagoManual("uid-1", tc.venceDesde)
			if got := p.Vencido(); got != tc.wantVencido {
				t.Fatalf("Vencido() = %v, quiere %v", got, tc.wantVencido)
			}
		})
	}

	t.Run("valor cero (vence sin asignar) esta vencido", func(t *testing.T) {
		if !(&DataPago{}).Vencido() {
			t.Fatal("DataPago vacio deberia estar vencido (vence zero)")
		}
	})
}

func TestDataPagoNotifica(t *testing.T) {
	t.Run("external id coincide: actualiza y marca notificado", func(t *testing.T) {
		p := pagoManual("uid-42", time.Hour)
		if p.Notificado() {
			t.Fatal("pago recien creado no deberia estar notificado")
		}

		n := notifReq("uid-42")
		if !p.Notifica(n) {
			t.Fatal("Notifica deberia devolver true cuando el external id coincide")
		}

		if p.Phone != n.Phone || p.Status != n.Status || p.TmId != n.TmId ||
			p.BankId != n.BankId || p.Bank != n.Bank {
			t.Fatalf("campos no actualizados: %+v", p)
		}
		if !p.Notificado() {
			t.Fatal("tras Notifica(), Notificado() deberia ser true")
		}
	})

	t.Run("external id distinto: no actualiza nada", func(t *testing.T) {
		p := pagoManual("uid-42", time.Hour)
		p.Phone = "5350000111"
		p.Status = "0"

		n := notifReq("uid-999") // distinto
		if p.Notifica(n) {
			t.Fatal("Notifica con external id distinto deberia devolver false")
		}
		if p.Phone != "5350000111" || p.Status != "0" || p.TmId != "" || p.BankId != "" || p.Bank != "" {
			t.Fatalf("Notifica con id distinto no deberia tocar campos: %+v", p)
		}
		if p.Notificado() {
			t.Fatal("sin coincidencia no deberia marcarse como notificado")
		}
	})
}

func TestDataPagoNotificado(t *testing.T) {
	t.Run("inicia en false", func(t *testing.T) {
		if pagoManual("uid-1", time.Hour).Notificado() {
			t.Fatal("Notificado() deberia ser false al inicio")
		}
	})

	t.Run("true tras notifica", func(t *testing.T) {
		p := pagoManual("uid-1", time.Hour)
		p.Notifica(notifReq("uid-1"))
		if !p.Notificado() {
			t.Fatal("Notificado() deberia ser true tras Notifica")
		}
	})

	t.Run("respeto del flag via literal (p.estado notificado=true)", func(t *testing.T) {
		p := &DataPago{notificado: true, ExternalId: "uid-1"}
		if !p.Notificado() {
			t.Fatal("Notificado() deberia reflejar la bandera puesta por literal")
		}
	})
}

// =============================================================================
// AdminPagos — New y metodos puros
// =============================================================================

func TestNew(t *testing.T) {
	a := New(nil) // nil db: los metodos que probamos nunca lo usan
	if a == nil {
		t.Fatal("New(nil) no deberia devolver nil")
	}
	if a.db != nil {
		t.Fatal("New(nil) deberia guardar db = nil")
	}
	if a.nsalvaPagos != "pagos" {
		t.Errorf("nsalvaPagos = %q, quiere %q", a.nsalvaPagos, "pagos")
	}
	if a.nsalvaNotif != "not_pagos" {
		t.Errorf("nsalvaNotif = %q, quiere %q", a.nsalvaNotif, "not_pagos")
	}
	if a.solPagos == nil || len(a.solPagos) != 0 {
		t.Errorf("solPagos deberia ser un mapa vacio inicializado, got %#v", a.solPagos)
	}
	if a.notificaciones == nil || len(a.notificaciones) != 0 {
		t.Errorf("notificaciones deberia ser un mapa vacio inicializado, got %#v", a.notificaciones)
	}
	if a.notifPr == nil || len(a.notifPr) != 0 {
		t.Errorf("notifPr deberia ser un mapa vacio inicializado, got %#v", a.notifPr)
	}
	if a.notifjson == nil || len(a.notifjson) != 0 {
		t.Errorf("notifjson deberia ser un mapa vacio inicializado, got %#v", a.notifjson)
	}
	if a.LOCK {
		t.Error("LOCK deberia iniciar en false")
	}
	if a.vence.IsZero() {
		t.Error("vence deberia iniciar con un timestamp (time.Now)")
	}
}

func TestChkLock(t *testing.T) {
	t.Run("vence en el pasado libera el lock", func(t *testing.T) {
		a := New(nil)
		a.LOCK = true
		a.vence = time.Now().Add(-time.Second)
		a.chkLock()
		if a.LOCK {
			t.Fatal("chkLock con vence expirado deberia poner LOCK = false")
		}
	})

	t.Run("vence en el futuro conserva el lock", func(t *testing.T) {
		a := New(nil)
		a.LOCK = true
		a.vence = time.Now().Add(time.Minute)
		a.chkLock()
		if !a.LOCK {
			t.Fatal("chkLock con vence vigente no deberia tocar LOCK")
		}
	})

	t.Run("lock apagado permanece apagado", func(t *testing.T) {
		a := New(nil)
		a.vence = time.Now().Add(-time.Second)
		a.chkLock() // no deberia paniquear ni cambiar nada
		if a.LOCK {
			t.Fatal("LOCK deberia seguir en false")
		}
	})
}

func TestLock(t *testing.T) {
	t.Run("enable=true adquiere con vencimiento 120s", func(t *testing.T) {
		a := New(nil)
		a.lock(true)
		if !a.LOCK {
			t.Fatal("lock(true) deberia poner LOCK = true")
		}
		restante := time.Until(a.vence)
		if restante < 119*time.Second || restante > 120*time.Second {
			t.Fatalf("vence deberia ser ~120s en el futuro, restante %v", restante)
		}
	})

	t.Run("enable=false libera", func(t *testing.T) {
		a := New(nil)
		a.lock(true)
		a.lock(false)
		if a.LOCK {
			t.Fatal("lock(false) deberia poner LOCK = false")
		}
	})

	t.Run("lock liberado sigue expirado correctamente", func(t *testing.T) {
		a := New(nil)
		a.lock(true)
		// Fuerza expiracion y re-adquiere: vence se reasigna a now+120s.
		a.vence = time.Now().Add(-time.Second)
		a.lock(true)
		if !a.LOCK {
			t.Fatal("lock(true) tras expirar deberia readquirir")
		}
		if restante := time.Until(a.vence); restante < 119*time.Second {
			t.Fatalf("vence deberia renovarse a ~120s, restante %v", restante)
		}
	})
}

func TestNumOrdenEnMemoria(t *testing.T) {
	// Rama "en memoria": solo muta el OrderId del DataPago, no toca la BD.
	// La rama "no en memoria" llama a.db.Execute y se omite (requiere DB).
	ctx := context.Background()

	a := New(nil)
	ext := "uid-1-op-2"
	a.solPagos[ext] = pagoManual(ext, time.Hour)

	a.NumOrden(ctx, 9876, ext)

	if got := a.solPagos[ext].OrderId; got != 9876 {
		t.Fatalf("OrderId = %d, quiere 9876", got)
	}
}

func TestNotificaJSON(t *testing.T) {
	a := New(nil)

	t.Run("sin entrada devuelve {}", func(t *testing.T) {
		if got := a.NotificaJSON("uid-1"); got != "{}" {
			t.Fatalf("NotificaJSON sin entrada = %#v, quiere \"{}\"", got)
		}
	})

	t.Run("devuelve el valor guardado", func(t *testing.T) {
		a.notifjson["uid-2"] = `{"status":"ok"}`
		if got := a.NotificaJSON("uid-2"); got != `{"status":"ok"}` {
			t.Fatalf("NotificaJSON = %#v, quiere el valor guardado", got)
		}
	})

	t.Run("valor nil guardado se devuelve tal cual (no {})", func(t *testing.T) {
		a.notifjson["uid-3"] = nil
		if got := a.NotificaJSON("uid-3"); got != nil {
			t.Fatalf("NotificaJSON con valor nil deberia devolver nil, obtuve %#v", got)
		}
	})
}

func TestExistePagoEnMemoria(t *testing.T) {
	// Rama "en memoria": devuelve respuesta idempotente sin tocar BD.
	// La rama "no en memoria" llama a.db.Select y se omite (requiere DB).
	ctx := context.Background()

	a := New(nil)
	ext := "uid-1-op"
	a.solPagos[ext] = pagoManual(ext, time.Hour)

	got, err := a.ExistePago(ctx, ext)
	if err != nil {
		t.Fatalf("ExistePago (en memoria) error inesperado: %v", err)
	}
	want := map[string]any{
		"transaction_id": ext,
		"status":         "0",
		"message":        "Transacción idempotente",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExistePago = %#v, quiere %#v", got, want)
	}
}

func TestLogLocked(t *testing.T) {
	t.Run("tipo 'p' agrega linea con ip y data a trazapagos", func(t *testing.T) {
		a := New(nil)
		n := notifReq("uid-1")

		if got := a.logLocked("p", n, "192.168.1.10"); got != nil {
			t.Fatalf("logLocked('p') deberia devolver nil, obtuve %#v", got)
		}
		data, _ := json.Marshal(n)
		wantLine := fmt.Sprintf(`"ip": "%s", "data": %s`, "192.168.1.10", string(data))

		if len(a.trazapagos) != 1 {
			t.Fatalf("trazapagos len = %d, quiere 1", len(a.trazapagos))
		}
		if a.trazapagos[0] != wantLine {
			t.Fatalf("linea = %q, quiere %q", a.trazapagos[0], wantLine)
		}
		if len(a.trazanotif) != 0 {
			t.Fatalf("tipo 'p' no deberia tocar trazanotif, len = %d", len(a.trazanotif))
		}
	})

	t.Run("tipo 'n' agrega el JSON crudo a trazanotif", func(t *testing.T) {
		a := New(nil)
		n := notifReq("uid-2")

		if got := a.logLocked("n", n, ""); got != nil {
			t.Fatalf("logLocked('n') deberia devolver nil, obtuve %#v", got)
		}
		data, _ := json.Marshal(n)
		if len(a.trazanotif) != 1 || a.trazanotif[0] != string(data) {
			t.Fatalf("trazanotif = %#v, quiere [%q]", a.trazanotif, string(data))
		}
		if len(a.trazapagos) != 0 {
			t.Fatalf("tipo 'n' no deberia tocar trazapagos, len = %d", len(a.trazapagos))
		}
	})

	t.Run("flush tras 100 lineas tipo 'p' escribe archivo y resetea", func(t *testing.T) {
		root, err := os.Getwd() // TestMain hizo chdir a un dir temporal
		if err != nil {
			t.Fatal(err)
		}
		a := New(nil)
		for i := 0; i < 101; i++ {
			a.logLocked("p", notifReq(fmt.Sprintf("uid-flush-%d", i)), "10.0.0.1")
		}
		if len(a.trazapagos) != 0 {
			t.Fatalf("tras el flush trazapagos deberia quedar vacio, len = %d", len(a.trazapagos))
		}
		path := filepath.Join(root, "data", fmt.Sprintf("pagos_%s.txt", time.Now().Format("20060102")))
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("utils.Data no escribio %s: %v", path, err)
		}
		if !strings.Contains(string(content), `"uid-flush-0"`) {
			t.Fatalf("el archivo de data no contiene la traza esperada: %q", string(content))
		}
	})

	t.Run("flush tras 100 lineas tipo 'n' resetea trazanotif", func(t *testing.T) {
		a := New(nil)
		for i := 0; i < 101; i++ {
			a.logLocked("n", notifReq(fmt.Sprintf("uid-nflush-%d", i)), "")
		}
		if len(a.trazanotif) != 0 {
			t.Fatalf("tras el flush trazanotif deberia quedar vacio, len = %d", len(a.trazanotif))
		}
	})
}

func TestAdd(t *testing.T) {
	a := New(nil) // NewDataPago no toca la BD: db nil es suficiente

	t.Run("pago valido se agrega al mapa", func(t *testing.T) {
		ext := "uid-100-abc"
		if err := a.Add("10.1.1.1", reqPago(ext)); err != nil {
			t.Fatalf("Add error inesperado: %v", err)
		}
		p, ok := a.solPagos[ext]
		if !ok {
			t.Fatal("Add no inserto el pago en solPagos")
		}
		if p.Amount != 25.50 || p.Currency != "CUP" || p.Phone != "5355550000" ||
			p.Source != "30010" || p.ValidTime != "60" || p.ExternalId != ext {
			t.Fatalf("campos del pago incorrectos: %+v", p)
		}
		// Split("uid-100-abc", "-") → ["uid","100","abc"]; uid=tmp[0], idoperacion=tmp[1:]
		if p.uid != "uid" || p.idoperacion != "100-abc" {
			t.Fatalf("uid/idoperacion derivados incorrectos: uid=%q idoperacion=%q", p.uid, p.idoperacion)
		}
		if p.Vencido() {
			t.Fatal("pago recien agregado no deberia estar vencido")
		}
	})

	t.Run("Add del mismo external id reemplaza (no duplica)", func(t *testing.T) {
		ext := "uid-101-x"
		baseLen := len(a.solPagos) // subtest anterior ya agrego entradas
		if err := a.Add("10.1.1.1", reqPago(ext)); err != nil {
			t.Fatal(err)
		}
		if err := a.Add("10.1.1.1", reqPago(ext)); err != nil {
			t.Fatal(err)
		}
		if len(a.solPagos) != baseLen+1 {
			t.Fatalf("len(solPagos) = %d, quiere %d (clave unica, sin duplicar)", len(a.solPagos), baseLen+1)
		}
	})

	t.Run("ValidTime invalido devuelve error y no agrega", func(t *testing.T) {
		req := reqPago("uid-102-x")
		req.ValidTime = "no-es-numero"
		ext := req.ExternalId
		if err := a.Add("10.1.1.1", req); err == nil {
			t.Fatal("Add con ValidTime invalido deberia devolver error")
		}
		if _, ok := a.solPagos[ext]; ok {
			t.Fatal("Add con error no deberia dejar el pago en solPagos")
		}
	})

	t.Run("ExternalId sin guion no rompe (uid=completo, idoperacion vacio)", func(t *testing.T) {
		req := reqPago("singuion")
		if err := a.Add("10.1.1.1", req); err != nil {
			t.Fatalf("Add sin guion no deberia dar error: %v", err)
		}
		p := a.solPagos["singuion"]
		if p == nil {
			t.Fatal("pago sin guion no quedo en solPagos")
		}
		if p.uid != "singuion" || p.idoperacion != "" {
			t.Fatalf("derivacion sin guion incorrecta: uid=%q idoperacion=%q", p.uid, p.idoperacion)
		}
	})
}

func TestVencimiento(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(a *AdminPagos) string
		want       bool
	}{
		{
			"pago vigente -> false",
			func(a *AdminPagos) string {
				ext := "uid-v1"
				a.solPagos[ext] = pagoManual(ext, time.Hour)
				return ext
			},
			false,
		},
		{
			"pago vencido -> true",
			func(a *AdminPagos) string {
				ext := "uid-v2"
				a.solPagos[ext] = pagoManual(ext, -time.Second)
				return ext
			},
			true,
		},
		{
			"external id inexistente -> true (fallback documentado)",
			func(a *AdminPagos) string { return "uid-no-existe" },
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := New(nil)
			ext := tc.setup(a)
			if got := a.Vencimiento(ext); got != tc.want {
				t.Fatalf("Vencimiento(%q) = %v, quiere %v", ext, got, tc.want)
			}
		})
	}
}

func TestNotificaAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("pago en memoria coincide: actualiza y no guarda en notificaciones", func(t *testing.T) {
		a := New(nil)
		ext := "uid-n1"
		a.solPagos[ext] = pagoManual(ext, time.Hour)

		n := notifReq(ext)
		a.Notifica(ctx, n)

		p := a.solPagos[ext]
		if p.Phone != n.Phone || p.Status != n.Status || p.TmId != n.TmId ||
			p.BankId != n.BankId || p.Bank != n.Bank {
			t.Fatalf("pago no actualizado: %+v", p)
		}
		if !p.Notificado() {
			t.Fatal("pago deberia estar notificado")
		}
		if _, ok := a.notificaciones[ext]; ok {
			t.Fatal("pago notificado en memoria no deberia ir a notificaciones")
		}
	})

	t.Run("pago inexistente: queda en notificaciones y notifjson(nil)", func(t *testing.T) {
		a := New(nil)
		ext := "uid-n2"
		n := notifReq(ext)

		a.Notifica(ctx, n)

		got, ok := a.notificaciones[ext]
		if !ok || got != n {
			t.Fatalf("notificaciones[%q] = %#v (ok=%v), quiere el request", ext, got, ok)
		}
		// logLocked devuelve nil (any), asi que notifjson[ext] queda en nil y
		// NotificaJSON lo devuelve tal cual (no "{}").
		if got := a.NotificaJSON(ext); got != nil {
			t.Fatalf("NotificaJSON(%q) = %#v, quiere nil (logLocked devuelve nil)", ext, got)
		}
	})

	t.Run("clave del mapa distinta del ExternalId del pago: cae a notificaciones", func(t *testing.T) {
		// Caso defensivo: pago insertado a mano con clave != p.ExternalId.
		a := New(nil)
		p := pagoManual("OTRO-EXT", time.Hour)
		a.solPagos["clave-map"] = p

		n := notifReq("clave-map")
		a.Notifica(ctx, n)

		if _, ok := a.notificaciones["clave-map"]; !ok {
			t.Fatal("p.notifica=false deberia dejar el request en notificaciones")
		}
		if p.Notificado() {
			t.Fatal("el pago no deberia marcarse notificado (external id no coincide)")
		}
	})
}

func TestProcesarListaSinTrabajo(t *testing.T) {
	// Solo se prueban estados sin trabajo (no tocan la BD con db=nil):
	//  1) LOCK activo: retorna de inmediato.
	//  2) mapas vacios: cicla sin pagos vencidos ni notificaciones pendientes.
	// Con pagos vencidos/notificaciones el metodo llama a.db.* y se omite
	// (requiere PostgreSQL vivo).
	ctx := context.Background()

	t.Run("LOCK activo retorna sin tocar nada", func(t *testing.T) {
		a := New(nil)
		a.LOCK = true
		a.ProcesarLista(ctx)
		if !a.LOCK {
			t.Fatal("ProcesarLista con LOCK activo no deberia liberar el lock")
		}
	})

	t.Run("mapas vacios: adquiere y libera el lock sin errores", func(t *testing.T) {
		a := New(nil)
		a.ProcesarLista(ctx)
		// El defer libera el LOCK al terminar.
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.LOCK {
			t.Fatal("tras ProcesarLista el LOCK deberia quedar en false")
		}
	})
}

func TestAdminPagosConcurrente(t *testing.T) {
	// Ejercita los mutex (a.mu / p.mu) bajo -race. Sin DB: Add + lecturas.
	a := New(nil)
	const goroutines = 8
	const addsPorGoroutine = 2 // 16 total < 20: evita el disparo de ProcesarLista

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < addsPorGoroutine; i++ {
				ext := fmt.Sprintf("uid-c%d-%d", g, i)
				if err := a.Add("127.0.0.1", reqPago(ext)); err != nil {
					t.Errorf("Add(%s): %v", ext, err)
					continue
				}
				_ = a.Vencimiento(ext)
				_ = a.NotificaJSON(ext)
			}
		}(g)
	}
	wg.Wait()

	if got := len(a.solPagos); got != goroutines*addsPorGoroutine {
		t.Fatalf("len(solPagos) = %d, quiere %d", got, goroutines*addsPorGoroutine)
	}
	for g := 0; g < goroutines; g++ {
		for i := 0; i < addsPorGoroutine; i++ {
			ext := fmt.Sprintf("uid-c%d-%d", g, i)
			if p, ok := a.solPagos[ext]; !ok || p == nil {
				t.Fatalf("falta el pago concurrente %s", ext)
			}
		}
	}
}

// =============================================================================
// Metodos omitidos (requieren PostgreSQL/Redis vivos)
// =============================================================================

// Omitidos (documentacion, no ejecutable):
//
//   - DataPago.Guarda(ctx): llama a initAsync (ChkURL/ChkMsg con p.db) y luego
//     a p.db.InsertReturning. Con db=nil panic. Requiere PostgreSQL.
//
//   - DataPago.initAsync(ctx): ChkURL/ChkMsg con p.db y config.URLList/MsgList.
//     Requiere PostgreSQL (o caches precargados, ademas de que no hay forma de
//     observar el resultado sin Guarda).
//
//   - AdminPagos.NumOrden rama "no en memoria": a.db.Execute sobre
//     public.pagos. Requiere PostgreSQL.
//
//   - AdminPagos.ExistePago rama "no en memoria": a.db.Select sobre pagos.
//     Requiere PostgreSQL.
//
//   - AdminPagos.ProcesarLista con pagos vencidos/notificados: llama a
//     p.Guarda y a.db.Select/Execute. Requiere PostgreSQL.
//
//   - AdminPagos.Notifica con len(solPagos) > 20: dispara ProcesarLista.
//     Requiere PostgreSQL.