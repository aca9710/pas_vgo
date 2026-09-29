// Package utils: tests unitarios de las utilidades puras.
//
// Funciones que tocan PostgreSQL/Redis/red (ChkURL, ChkMsg, IPValido,
// IPValidoTV) se omiten: requieren *database.Database y no se prueban sin
// infraestructura.
//
// Traza/VerError/Data escriben archivos bajo config.ProjectRoot() (=cwd
// durante `go test`); TestMain hace chdir a un directorio temporal para que
// esas escrituras no toquen el repo y ademas permite verificar los ficheros.
package utils

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pasarela/config"
)

// TestMain redirige el cwd a un dir temporal (Traza/VerError/Data escriben
// bajo config.ProjectRoot()) y lo restaura al terminar.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "utils-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "utils: MkdirTemp: %v\n", err)
		os.Exit(1)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "utils: Getwd: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintf(os.Stderr, "utils: Chdir: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.Chdir(oldWd)
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return wd
}

// withDefaultSemilla fuerza SemillaAuth vacia (ValidarUsuario cae al default
// "externalpayment") y restaura el valor previo al terminar.
func withDefaultSemilla(t *testing.T) {
	t.Helper()
	orig := config.Cfg.SemillaAuth
	config.Cfg.SemillaAuth = ""
	t.Cleanup(func() { config.Cfg.SemillaAuth = orig })
}

// =============================================================================
// IDAlfaGen
// =============================================================================

func TestIDAlfaGen(t *testing.T) {
	t.Run("longitud", func(t *testing.T) {
		// n = size-4 (clamp >= 0) aleatorios + "MMSS" (4) + " " (1).
		cases := []struct {
			size int
			want int
		}{
			{-3, 5},
			{0, 5},
			{3, 5},
			{4, 5},
			{5, 6},
			{10, 11},
			{20, 21},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("size=%d", tc.size), func(t *testing.T) {
				if got := IDAlfaGen(tc.size); len(got) != tc.want {
					t.Fatalf("IDAlfaGen(%d) longitud = %d, quiere %d (got %q)", tc.size, len(got), tc.want, got)
				}
			})
		}
	})

	t.Run("caracteres dentro del alfabeto", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			s := IDAlfaGen(20)
			if !strings.HasSuffix(s, " ") {
				t.Fatalf("IDAlfaGen(20) = %q, deberia terminar en espacio", s)
			}
			for _, c := range s[:len(s)-1] {
				if !strings.ContainsRune(alfaChars, c) {
					t.Fatalf("IDAlfaGen(20) = %q contiene %q fuera del alfabeto", s, c)
				}
			}
		}
	})

	t.Run("unicidad en 100 generaciones", func(t *testing.T) {
		seen := make(map[string]bool, 100)
		for i := 0; i < 100; i++ {
			id := IDAlfaGen(20)
			if seen[id] {
				t.Fatalf("colision de IDAlfaGen(20): %q", id)
			}
			seen[id] = true
		}
	})

	t.Run("size<=0 no rompe", func(t *testing.T) {
		for _, size := range []int{0, -1, -100} {
			s := IDAlfaGen(size)
			// n se clamp a 0 -> solo "MMSS " (5 chars), sin panico.
			if len(s) != 5 {
				t.Fatalf("IDAlfaGen(%d) = %q (longitud %d), quiere 5", size, s, len(s))
			}
		}
	})
}

// =============================================================================
// ObtenerOrigen (pura, no toca DB)
// =============================================================================

func TestObtenerOrigen(t *testing.T) {
	cases := []struct {
		name       string
		externalID string
		wantUID    string
		wantIDOp   string
		wantErr    bool
	}{
		{"separador simple", "abc-123", "abc", "123", false},
		{"varios guiones", "a-b-c", "a", "b-c", false},
		{"guion al final", "abc-", "abc", "", false},
		{"sin guion", "abc123", "", "", true},
		{"guion al inicio", "-abc", "", "", true},
		{"vacio", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uid, idOp, err := ObtenerOrigen(tc.externalID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ObtenerOrigen(%q) deberia dar error", tc.externalID)
				}
				return
			}
			if err != nil {
				t.Fatalf("ObtenerOrigen(%q) error inesperado: %v", tc.externalID, err)
			}
			if uid != tc.wantUID || idOp != tc.wantIDOp {
				t.Fatalf("ObtenerOrigen(%q) = (%q,%q), quiere (%q,%q)", tc.externalID, uid, idOp, tc.wantUID, tc.wantIDOp)
			}
		})
	}
}

// =============================================================================
// fechaAuth / passwordAuth / PreparaConexion / ValidarUsuario
// =============================================================================

func TestFechaAuth(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	cases := []struct {
		name       string
		now        time.Time
		wantDia    string
		wantMes    string
		wantAnio   string
	}{
		{"dia y mes con cero se recortan", time.Date(2026, time.September, 7, 10, 30, 0, 0, utc), "7", "9", "2026"},
		{"sin ceros queda igual", time.Date(2026, time.December, 30, 0, 0, 0, 0, utc), "30", "12", "2026"},
		{"dia con cero, mes sin cero", time.Date(2026, time.October, 5, 0, 0, 0, 0, utc), "5", "10", "2026"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dia, mes, anio := fechaAuth(tc.now)
			if dia != tc.wantDia || mes != tc.wantMes || anio != tc.wantAnio {
				t.Fatalf("fechaAuth(%v) = (%q,%q,%q), quiere (%q,%q,%q)",
					tc.now, dia, mes, anio, tc.wantDia, tc.wantMes, tc.wantAnio)
			}
		})
	}
}

// expectedPassword replica la concatenacion exacta de passwordAuth usando las
// mismas primitivas (crypto/sha512 + encoding/base64), sin hardcodear nada.
func expectedPassword(usuario, semilla, source string, now time.Time) string {
	dia, mes, anio := fechaAuth(now)
	raw := usuario + dia + mes + anio + semilla + source
	sum := sha512.Sum512([]byte(raw))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestPasswordAuth(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	// Fecha fija: 14/09/2026 -> fechaAuth = ("14","9","2026").
	now := time.Date(2026, time.September, 14, 23, 59, 0, 0, utc)

	t.Run("vector cimex/externalpayment/70014", func(t *testing.T) {
		dia, mes, anio := fechaAuth(now)
		if dia != "14" || mes != "9" || anio != "2026" {
			t.Fatalf("precondicion fechaAuth: (%q,%q,%q)", dia, mes, anio)
		}
		got := passwordAuth("cimex", "externalpayment", "70014", now)
		// Derivado con las mismas primitivas y concatenacion literal
		// usuario+dia+mes+anio+semilla+source.
		raw := "cimex" + dia + mes + anio + "externalpayment" + "70014"
		sum := sha512.Sum512([]byte(raw))
		want := base64.StdEncoding.EncodeToString(sum[:])
		if got != want {
			t.Fatalf("passwordAuth = %q, quiere %q", got, want)
		}
	})

	t.Run("equivale a base64(sha512(concatenacion))", func(t *testing.T) {
		got := passwordAuth("usuario1", "semilla2", "src3", now)
		if want := expectedPassword("usuario1", "semilla2", "src3", now); got != want {
			t.Fatalf("passwordAuth = %q, quiere %q", got, want)
		}
	})

	t.Run("cambia con el dia (formato sin ceros)", func(t *testing.T) {
		// 07/09/2026 -> ("7","9","2026") vs 14/09/2026 -> ("14","9","2026").
		now7 := time.Date(2026, time.September, 7, 0, 0, 0, 0, utc)
		if passwordAuth("cimex", "externalpayment", "70014", now) ==
			passwordAuth("cimex", "externalpayment", "70014", now7) {
			t.Fatal("el password deberia depender de la fecha")
		}
	})
}

func TestPreparaConexion(t *testing.T) {
	now := time.Now()

	t.Run("valores explicitos", func(t *testing.T) {
		h := PreparaConexion("semilla1", "usuario1", "fuente1", now)
		if h["username"] != "usuario1" || h["source"] != "fuente1" {
			t.Fatalf("headers = %v", h)
		}
		if h["password"] != expectedPassword("usuario1", "semilla1", "fuente1", now) {
			t.Fatalf("password no coincide con el esperado: %q", h["password"])
		}
		if h["Content-Type"] != "application/json" {
			t.Fatalf("Content-Type = %q, quiere application/json", h["Content-Type"])
		}
	})

	t.Run("valores por defecto", func(t *testing.T) {
		h := PreparaConexion("", "", "", now)
		if h["username"] != "cimex" || h["source"] != "30010" {
			t.Fatalf("defaults no aplicados: %v", h)
		}
		if h["password"] != expectedPassword("cimex", "externalpayment", "30010", now) {
			t.Fatalf("password default no coincide: %q", h["password"])
		}
	})
}

func TestValidarUsuario(t *testing.T) {
	now := time.Now()
	withDefaultSemilla(t)

	t.Run("headers correctos", func(t *testing.T) {
		headers := PreparaConexion("", "cimex", "70014", now)
		if !ValidarUsuario(headers, now) {
			t.Fatal("ValidarUsuario deberia aceptar headers de PreparaConexion")
		}
	})

	t.Run("password incorrecto", func(t *testing.T) {
		headers := PreparaConexion("", "cimex", "70014", now)
		headers["password"] = "incorrecto"
		if ValidarUsuario(headers, now) {
			t.Fatal("ValidarUsuario no deberia aceptar password incorrecto")
		}
	})

	missing := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"sin username", func(h map[string]string) { delete(h, "username") }},
		{"sin password", func(h map[string]string) { delete(h, "password") }},
		{"sin source", func(h map[string]string) { delete(h, "source") }},
	}
	for _, tc := range missing {
		t.Run(tc.name, func(t *testing.T) {
			headers := PreparaConexion("", "cimex", "70014", now)
			tc.mutate(headers)
			if ValidarUsuario(headers, now) {
				t.Fatalf("ValidarUsuario no deberia aceptar %s", tc.name)
			}
		})
	}

	t.Run("semilla custom en Cfg", func(t *testing.T) {
		orig := config.Cfg.SemillaAuth
		config.Cfg.SemillaAuth = "semilla-custom"
		t.Cleanup(func() { config.Cfg.SemillaAuth = orig })

		headers := PreparaConexion("semilla-custom", "cimex", "70014", now)
		if !ValidarUsuario(headers, now) {
			t.Fatal("con Cfg.SemillaAuth = semilla-custom deberia validar")
		}
		// PreparaConexion con semilla vacia usa "externalpayment", que ya no
		// coincide con la semilla de ValidarUsuario -> rechazado.
		headersDefault := PreparaConexion("", "cimex", "70014", now)
		if ValidarUsuario(headersDefault, now) {
			t.Fatal("con semilla distinta de la de Cfg deberia rechazar")
		}
	})
}

// =============================================================================
// txthora12
// =============================================================================

func TestTxthora12(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"manana", time.Date(2026, 9, 14, 9, 5, 7, 0, utc), "09:05:07 a.m."},
		{"tarde", time.Date(2026, 9, 14, 15, 5, 7, 0, utc), "03:05:07 p.m."},
		{"mediodia es p.m.", time.Date(2026, 9, 14, 12, 0, 0, 0, utc), "12:00:00 p.m."},
		{"medianoche es a.m.", time.Date(2026, 9, 14, 0, 0, 0, 0, utc), "12:00:00 a.m."},
		{"ultimo segundo a.m.", time.Date(2026, 9, 14, 11, 59, 59, 0, utc), "11:59:59 a.m."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := txthora12(tc.now); got != tc.want {
				t.Fatalf("txthora12(%v) = %q, quiere %q", tc.now, got, tc.want)
			}
		})
	}
}

// =============================================================================
// toInt (version utils: int/int64/float64/nil, el resto a 0)
// =============================================================================

func TestToInt(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want int
	}{
		{"int cero", 0, 0},
		{"int positivo", 42, 42},
		{"int negativo", -7, -7},
		{"int64", int64(42), 42},
		{"float64 entero", 3.0, 3},
		{"float64 trunca", 3.9, 3},
		{"float64 negativo trunca hacia cero", -3.9, -3},
		{"nil devuelve 0", nil, 0},
		{"string numerico no soportado", "0", 0},
		{"string invalido", "abc", 0},
		{"json.Number no soportado en utils", json.Number("42"), 0},
		{"[]byte no soportado", []byte("7"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toInt(tc.v); got != tc.want {
				t.Fatalf("toInt(%#v) = %d, quiere %d", tc.v, got, tc.want)
			}
		})
	}
}

// =============================================================================
// Traza / VerError / Data (escriben en el cwd temporal de TestMain)
// =============================================================================

func TestTraza(t *testing.T) {
	root := mustGetwd(t) // TestMain hizo chdir a un dir temporal
	Traza("texto de la traza", "ID-777")

	path := filepath.Join(root, "logs", fmt.Sprintf("traza_%s.log", time.Now().Format("20060102")))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se escribio %s: %v", path, err)
	}
	data := string(content)
	if !strings.Contains(data, "ID-777") || !strings.Contains(data, "texto de la traza") {
		t.Fatalf("contenido inesperado: %q", data)
	}
}

func TestVerError(t *testing.T) {
	root := mustGetwd(t)
	got := VerError("mensaje de prueba")
	if got != "mensaje de prueba" {
		t.Fatalf("VerError devolvio %q, quiere el texto de entrada", got)
	}

	path := filepath.Join(root, "log", fmt.Sprintf("error_%s.log", time.Now().Format("20060102")))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se escribio %s: %v", path, err)
	}
	if !strings.Contains(string(content), "mensaje de prueba") {
		t.Fatalf("contenido inesperado: %q", string(content))
	}
}

func TestData(t *testing.T) {
	root := mustGetwd(t)
	Data("prueba_test", []string{"linea1", "linea2"})

	path := filepath.Join(root, "data", fmt.Sprintf("prueba_test_%s.txt", time.Now().Format("20060102")))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se escribio %s: %v", path, err)
	}
	if want := "linea1\nlinea2\n"; string(content) != want {
		t.Fatalf("contenido = %q, quiere %q", string(content), want)
	}
}