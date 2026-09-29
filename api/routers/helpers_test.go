// Package routers: tests unitarios de los helpers HTTP puros y de
// conversion. No se prueban doRequest/postJSON/getJSON (requieren red) ni
// los handlers (requieren DB/Redis).
package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"pasarela/config"
	"pasarela/models"
	"pasarela/utils"
)

// withDefaultSemilla fuerza SemillaAuth vacia (verifyAuth cae al default
// "externalpayment") y restaura el valor previo al terminar.
func withDefaultSemilla(t *testing.T) {
	t.Helper()
	orig := config.Cfg.SemillaAuth
	config.Cfg.SemillaAuth = ""
	t.Cleanup(func() { config.Cfg.SemillaAuth = orig })
}

// =============================================================================
// writeJSON / writeAuthError / writeValidationError / writeError500
// =============================================================================

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]any{"ok": true, "n": 3})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiere 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, quiere application/json", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body no es JSON valido: %v (%q)", err, rec.Body.String())
	}
	if got["ok"] != true || got["n"] != float64(3) {
		t.Fatalf("body = %v", got)
	}
}

func TestWriteAuthError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAuthError(rec)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, quiere 401", rec.Code)
	}
	// El mapa se serializa con claves ordenadas: {"estado":8,"msg":...}.
	want := `{"detail":{"estado":8,"msg":"Acceso denegado, Usuario, Password o Source no válido"}}` + "\n"
	if rec.Body.String() != want {
		t.Fatalf("body = %q, quiere %q", rec.Body.String(), want)
	}

	var got struct {
		Detail struct {
			Estado int    `json:"estado"`
			Msg    string `json:"msg"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body no es JSON: %v", err)
	}
	if got.Detail.Estado != 8 || got.Detail.Msg != "Acceso denegado, Usuario, Password o Source no válido" {
		t.Fatalf("detail = %+v", got.Detail)
	}
}

func TestWriteValidationError(t *testing.T) {
	ve := &models.ValidationError{Detail: []models.ValidationErrorItem{
		{Loc: []string{"body", "Amount"}, Msg: "Field required", Type: "missing"},
		{Loc: []string{"body", "Phone"}, Msg: "String should have at most 10 characters", Type: "string_too_long"},
	}}
	rec := httptest.NewRecorder()
	writeValidationError(rec, ve)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quiere 422", rec.Code)
	}
	var got models.ValidationError
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body no es JSON: %v (%q)", err, rec.Body.String())
	}
	if len(got.Detail) != 2 {
		t.Fatalf("detail tiene %d items, quiere 2", len(got.Detail))
	}
	if !reflect.DeepEqual(got.Detail[0].Loc, []string{"body", "Amount"}) ||
		got.Detail[0].Msg != "Field required" || got.Detail[0].Type != "missing" {
		t.Fatalf("detail[0] = %+v", got.Detail[0])
	}
}

func TestWriteError500(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError500(rec, "boom")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiere 500", rec.Code)
	}
	var got struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body no es JSON: %v", err)
	}
	if got.Detail != "boom" {
		t.Fatalf("detail = %q, quiere boom", got.Detail)
	}
}

// =============================================================================
// decodeJSON
// =============================================================================

// demoRequest: struct con json tags para probar decodeJSON.
type demoRequest struct {
	Nombre string  `json:"Nombre"`
	Monto  float64 `json:"Monto"`
}

// assertDetail compara el body de la respuesta con la lista de errores esperada.
func assertDetail(t *testing.T, rec *httptest.ResponseRecorder, want []models.ValidationErrorItem) {
	t.Helper()
	var got struct {
		Detail []models.ValidationErrorItem `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body no es JSON: %v (%q)", err, rec.Body.String())
	}
	if !reflect.DeepEqual(got.Detail, want) {
		t.Fatalf("detail = %+v, quiere %+v", got.Detail, want)
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Run("JSON valido sin required: true y llena v", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", strings.NewReader(`{"Nombre":"x","Monto":1.5}`))
		var v demoRequest
		if !decodeJSON(rec, req, &v, nil) {
			t.Fatalf("decodeJSON devolvio false, body: %s", rec.Body.String())
		}
		if v.Nombre != "x" || v.Monto != 1.5 {
			t.Fatalf("v = %+v", v)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("no deberia escribir respuesta, status = %d", rec.Code)
		}
	})

	t.Run("required presente valida", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", strings.NewReader(`{"Nombre":"x"}`))
		var v demoRequest
		if !decodeJSON(rec, req, &v, []string{"Nombre"}) {
			t.Fatalf("deberia validar, body: %s", rec.Body.String())
		}
	})

	t.Run("JSON invalido: false y 422 json_invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", strings.NewReader(`{esto no es json`))
		var v demoRequest
		if decodeJSON(rec, req, &v, nil) {
			t.Fatal("decodeJSON deberia devolver false")
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, quiere 422", rec.Code)
		}
		assertDetail(t, rec, []models.ValidationErrorItem{
			{Loc: []string{"body"}, Msg: "There was an error parsing the body", Type: "json_invalid"},
		})
	})

	t.Run("body vacio: false y 422 json_invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", nil)
		var v demoRequest
		if decodeJSON(rec, req, &v, nil) {
			t.Fatal("decodeJSON deberia devolver false")
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, quiere 422", rec.Code)
		}
	})

	t.Run("required faltante: false y 422 missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", strings.NewReader(`{"Nombre":"x"}`))
		var v demoRequest
		if decodeJSON(rec, req, &v, []string{"Monto"}) {
			t.Fatal("decodeJSON deberia devolver false")
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, quiere 422", rec.Code)
		}
		assertDetail(t, rec, []models.ValidationErrorItem{
			{Loc: []string{"body", "Monto"}, Msg: "Field required", Type: "missing"},
		})
	})

	t.Run("error de tipo: false y 422 type_error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/pago/", strings.NewReader(`{"Nombre":"x","Monto":"texto"}`))
		var v demoRequest
		if decodeJSON(rec, req, &v, nil) {
			t.Fatal("decodeJSON deberia devolver false")
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, quiere 422", rec.Code)
		}
		assertDetail(t, rec, []models.ValidationErrorItem{
			{Loc: []string{"body", "Monto"}, Msg: "Input should be a valid float", Type: "type_error"},
		})
	})
}

// =============================================================================
// verifyAuth
// =============================================================================

func TestVerifyAuth(t *testing.T) {
	withDefaultSemilla(t)

	t.Run("headers correctos", func(t *testing.T) {
		headers := utils.PreparaConexion("", "cimex", "70014", time.Now())
		req := httptest.NewRequest(http.MethodPost, "/pago/", nil)
		for k, v := range headers {
			if k == "Content-Type" {
				continue
			}
			req.Header.Set(k, v)
		}
		if !verifyAuth(req) {
			t.Fatal("verifyAuth deberia autenticar con headers de PreparaConexion")
		}
	})

	t.Run("sin headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/pago/", nil)
		if verifyAuth(req) {
			t.Fatal("verifyAuth no deberia autenticar sin headers")
		}
	})

	t.Run("password malo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/pago/", nil)
		req.Header.Set("username", "cimex")
		req.Header.Set("password", "malo")
		req.Header.Set("source", "70014")
		if verifyAuth(req) {
			t.Fatal("verifyAuth no deberia autenticar con password malo")
		}
	})
}

// =============================================================================
// clientIP
// =============================================================================

func TestClientIP(t *testing.T) {
	t.Run("con puerto devuelve la IP del peer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		if got := clientIP(req); got != "1.2.3.4" {
			t.Fatalf("clientIP = %q, quiere 1.2.3.4", got)
		}
	})

	t.Run("sin puerto devuelve el valor tal cual", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4"
		if got := clientIP(req); got != "1.2.3.4" {
			t.Fatalf("clientIP = %q, quiere 1.2.3.4", got)
		}
	})

	t.Run("IPv6", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "[::1]:8080"
		if got := clientIP(req); got != "::1" {
			t.Fatalf("clientIP = %q, quiere ::1", got)
		}
	})

	t.Run("X-Forwarded-For se ignora", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		req.Header.Set("X-Forwarded-For", "9.9.9.9")
		if got := clientIP(req); got != "1.2.3.4" {
			t.Fatalf("clientIP = %q, quiere la IP del peer 1.2.3.4 (XFF ignorado)", got)
		}
	})
}

// =============================================================================
// Conversiones (version routers, con json.Number)
// =============================================================================

func TestConversiones(t *testing.T) {
	num := json.Number("42")
	fnum := json.Number("2.5")
	badNum := json.Number("abc")

	t.Run("toInt", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want int
		}{
			{"int", 7, 7},
			{"int64", int64(7), 7},
			{"float64 entero", 7.0, 7},
			{"float64 trunca", 3.9, 3},
			{"float64 negativo trunca hacia cero", -3.9, -3},
			{"json.Number entero", num, 42},
			{"json.Number invalido", badNum, 0},
			{"json.Number decimal no soportado", json.Number("12.5"), 0},
			{"nil", nil, 0},
			{"string no soportado", "7", 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := toInt(tc.v); got != tc.want {
					t.Fatalf("toInt(%#v) = %d, quiere %d", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("toInt64", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want int64
		}{
			{"int", 7, 7},
			{"int64", int64(7), 7},
			{"float64 trunca", 3.9, 3},
			{"json.Number entero", num, 42},
			{"json.Number invalido", badNum, 0},
			{"nil", nil, 0},
			{"string no soportado", "7", 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := toInt64(tc.v); got != tc.want {
					t.Fatalf("toInt64(%#v) = %d, quiere %d", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("toFloat64", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want float64
		}{
			{"float64", 1.5, 1.5},
			{"int", 3, 3.0},
			{"int64", int64(3), 3.0},
			{"json.Number decimal", fnum, 2.5},
			{"json.Number invalido", badNum, 0.0},
			{"nil", nil, 0.0},
			{"string no soportado", "1.5", 0.0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := toFloat64(tc.v); got != tc.want {
					t.Fatalf("toFloat64(%#v) = %v, quiere %v", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("toString", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want string
		}{
			{"string", "hola", "hola"},
			{"nil", nil, ""},
			{"float64", 1.5, "1.5"},
			{"float64 entero via %v", 1.0, "1"},
			{"int64", int64(42), "42"},
			{"int", 42, "42"},
			{"json.Number via %v", num, "42"},
			{"bool via %v", true, "true"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := toString(tc.v); got != tc.want {
					t.Fatalf("toString(%#v) = %q, quiere %q", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("orEmpty", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want any
		}{
			{"nil devuelve string vacio", nil, ""},
			{"string pasa igual", "x", "x"},
			{"float64 pasa igual", 1.5, 1.5},
			{"int cero no es nil", 0, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := orEmpty(tc.v); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("orEmpty(%#v) = %#v, quiere %#v", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("orEmptyStr", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want any
		}{
			{"nil devuelve string vacio", nil, ""},
			{"string pasa igual", "x", "x"},
			{"float64 se formatea", 1.5, "1.5"},
			{"int64 se formatea", int64(3), "3"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := orEmptyStr(tc.v); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("orEmptyStr(%#v) = %#v, quiere %#v", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("orZero", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want any
		}{
			{"nil devuelve 0", nil, 0},
			{"string pasa igual", "x", "x"},
			{"int cero no es nil", 0, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := orZero(tc.v); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("orZero(%#v) = %#v, quiere %#v", tc.v, got, tc.want)
				}
			})
		}
	})

	t.Run("orMinusOne", func(t *testing.T) {
		cases := []struct {
			name string
			v    any
			want any
		}{
			{"nil devuelve -1", nil, -1},
			{"string pasa igual", "x", "x"},
			{"int cero no es nil", 0, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := orMinusOne(tc.v); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("orMinusOne(%#v) = %#v, quiere %#v", tc.v, got, tc.want)
				}
			})
		}
	})
}