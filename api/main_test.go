// Package main: tests unitarios de los componentes HTTP puros.
//
// Solo se prueban funciones que NO requieren PostgreSQL/Redis/red:
//   - rootHandler: GET / -> JSON "Pasarela de pagos v3.0".
//   - responseWriter: WriteHeader/Write/Unwrap y el header X-Process-Time.
//   - middleware: envuelve un mux y mide la duracion de la peticion.
//
// Se omite main() completo (requiere PostgreSQL + Redis + ETECSA).
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	rootHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiere 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, quiere application/json", ct)
	}
	body := rec.Body.String()
	if body != `"Pasarela de pagos v3.0"` {
		t.Fatalf("body = %q, quiere %q", body, `"Pasarela de pagos v3.0"`)
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("WriteHeader setea X-Process-Time una sola vez", func(t *testing.T) {
		inner := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: inner, start: timeNow()}

		rw.WriteHeader(http.StatusOK)
		rw.WriteHeader(http.StatusInternalServerError) // segundo llamado: no-op por contrato

		// El recorder conserva el primer code (200); el segundo WriteHeader
		// no tiene efecto (contrato de http.ResponseWriter).
		if inner.Code != http.StatusOK {
			t.Fatalf("el segundo WriteHeader no deberia cambiar el code, code = %d", inner.Code)
		}
		pt := inner.Header().Get("X-Process-Time")
		if pt == "" {
			t.Fatal("X-Process-Time deberia estar seteado tras el primer WriteHeader")
		}
		if _, err := strconv.ParseFloat(pt, 64); err != nil {
			t.Fatalf("X-Process-Time = %q no es un float valido: %v", pt, err)
		}
	})

	t.Run("Write implica WriteHeader(200) y setea el header", func(t *testing.T) {
		inner := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: inner, start: timeNow()}

		n, err := rw.Write([]byte("hola"))
		if err != nil || n != 4 {
			t.Fatalf("Write = (%d, %v), quiere (4, nil)", n, err)
		}
		if inner.Code != http.StatusOK {
			t.Fatalf("Write sin WriteHeader previo deberia implicar 200, code = %d", inner.Code)
		}
		if inner.Header().Get("X-Process-Time") == "" {
			t.Fatal("X-Process-Time deberia estar seteado tras Write")
		}
		if inner.Body.String() != "hola" {
			t.Fatalf("body = %q, quiere %q", inner.Body.String(), "hola")
		}
	})

	t.Run("Unwrap devuelve el ResponseWriter subyacente", func(t *testing.T) {
		inner := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: inner, start: timeNow()}

		if rw.Unwrap() != http.ResponseWriter(inner) {
			t.Fatal("Unwrap deberia devolver el writer subyacente")
		}
	})
}

func TestMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	h := middleware(mux)

	t.Run("agrega X-Process-Time a respuestas normales", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, quiere 200", rec.Code)
		}
		if rec.Body.String() != "ok" {
			t.Fatalf("body = %q, quiere %q", rec.Body.String(), "ok")
		}
		if rec.Header().Get("X-Process-Time") == "" {
			t.Fatal("middleware deberia setear X-Process-Time")
		}
	})

	t.Run("respuestas con status explicito tambien llevan el header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, quiere 201", rec.Code)
		}
		if rec.Header().Get("X-Process-Time") == "" {
			t.Fatal("middleware deberia setear X-Process-Time en status explicitos")
		}
	})

	t.Run("404 de rutas no registradas tambien llevan el header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/no-existe", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, quiere 404", rec.Code)
		}
		if rec.Header().Get("X-Process-Time") == "" {
			t.Fatal("middleware deberia setear X-Process-Time en 404")
		}
	})

	t.Run("el body de la raiz es el JSON de rootHandler", func(t *testing.T) {
		// Registra la raiz como en main() y verifica el flujo completo.
		mux2 := http.NewServeMux()
		mux2.HandleFunc("GET /{$}", rootHandler)
		h2 := middleware(mux2)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		h2.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, quiere 200", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "Pasarela de pagos v3.0") {
			t.Fatalf("body = %q, deberia contener el texto de la raiz", string(body))
		}
		if rec.Header().Get("X-Process-Time") == "" {
			t.Fatal("middleware deberia setear X-Process-Time en la raiz")
		}
	})
}

// timeNow es un helper para fijar start en el pasado reciente (evita
// dependencias de reloj en las aserciones).
func timeNow() time.Time {
	return time.Now().Add(-time.Millisecond)
}