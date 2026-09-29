// Package httpclient: tests unitarios del cliente HTTP global.
//
// No depende de PostgreSQL/Redis/red externa: usa httptest (servidor local)
// para validar que Init configura un transporte funcional.
//
// Comportamiento documentado de Close(): ademas de cerrar las conexiones
// idle del transporte, pone _client = nil, por lo que Get() posterior hace
// panic (mismo mensaje que si nunca se hubiera llamado Init).
package httpclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Init
// =============================================================================

func TestInit(t *testing.T) {
	cases := []struct {
		name    string
		timeout time.Duration
	}{
		{"cero (sin timeout)", 0},
		{"un segundo", time.Second},
		{"diez segundos", 10 * time.Second},
		{"minuto", time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Init(tc.timeout)
			if c == nil {
				t.Fatal("Init devolvio nil")
			}
			if c.Timeout != tc.timeout {
				t.Fatalf("Timeout = %v, quiere %v", c.Timeout, tc.timeout)
			}
			if c.Transport == nil {
				t.Fatal("Init deberia configurar un Transport")
			}
			if _, ok := c.Transport.(*http.Transport); !ok {
				t.Fatalf("Transport = %T, quiere *http.Transport", c.Transport)
			}
			// Init tambien fija el cliente global: Get() == mismo puntero.
			if got := Get(); got != c {
				t.Fatal("Get() deberia devolver el mismo puntero que Init")
			}
		})
	}
}

// =============================================================================
// Get
// =============================================================================

func TestGet(t *testing.T) {
	t.Run("devuelve el cliente global tras Init", func(t *testing.T) {
		c := Init(5 * time.Second)
		if Get() != c {
			t.Fatal("Get() deberia devolver el cliente de Init")
		}
		if Get() == nil {
			t.Fatal("Get() no deberia devolver nil tras Init")
		}
	})

	t.Run("panic si no fue inicializado", func(t *testing.T) {
		Close() // garantiza estado no inicializado
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("Get() sin Init deberia hacer panic")
			}
			if !strings.Contains(fmt.Sprint(r), "http client not initialized") {
				t.Fatalf("mensaje de panic = %v, quiere 'http client not initialized...'", r)
			}
		}()
		Get()
	})
}

// =============================================================================
// Close
// =============================================================================

func TestClose(t *testing.T) {
	t.Run("no panic con cliente inicializado", func(t *testing.T) {
		Init(5 * time.Second)
		Close() // no deberia paniquear
	})

	t.Run("no panic con cliente nil (doble Close)", func(t *testing.T) {
		Close()
		Close()
	})

	t.Run("tras Close, Get() hace panic: comportamiento documentado", func(t *testing.T) {
		Init(5 * time.Second)
		Close()
		defer func() {
			if recover() == nil {
				t.Fatal("Get() tras Close() deberia hacer panic (Close deja _client = nil)")
			}
		}()
		Get()
	})
}

// =============================================================================
// Requests a servidor local (httptest) — valida que Init configuro bien el
// transporte
// =============================================================================

func TestClientRequestsLocal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "hola desde etecsa simulada")
		case r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"recibido":`+string(body)+`}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)

	t.Run("GET: 200 y body leido", func(t *testing.T) {
		c := Init(5 * time.Second)

		resp, err := c.Get(server.URL)
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, quiere 200", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("leer body: %v", err)
		}
		if got := string(body); got != "hola desde etecsa simulada" {
			t.Fatalf("body = %q, quiere %q", got, "hola desde etecsa simulada")
		}
	})

	t.Run("POST con JSON: 201 y eco del body", func(t *testing.T) {
		c := Init(5 * time.Second)

		resp, err := c.Post(server.URL+"/ordenpago", "application/json",
			strings.NewReader(`{"externalid":"uid-1"}`))
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("StatusCode = %d, quiere 201", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("leer body: %v", err)
		}
		if got, want := string(body), `{"recibido":{"externalid":"uid-1"}}`; got != want {
			t.Fatalf("body = %q, quiere %q", got, want)
		}
	})
}

// =============================================================================
// Timeout aplicado — el Timeout de Init rige el request completo
// =============================================================================

func TestClientTimeout(t *testing.T) {
	// Servidor lento (500ms) vs timeout corto (50ms): el request debe fallar
	// por timeout, demostrando que Init aplico el Timeout real al transporte.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, "tarde")
	}))
	t.Cleanup(server.Close)

	c := Init(50 * time.Millisecond)

	_, err := c.Get(server.URL)
	if err == nil {
		t.Fatal("el request contra un servidor lento deberia fallar por timeout")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("error deberia ser de tipo timeout, obtuve %T: %v", err, err)
	}
}