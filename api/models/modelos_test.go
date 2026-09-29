package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// findFieldError busca un ValidationErrorItem cuyo campo sea (Loc[1] == field).
func findFieldError(t *testing.T, verr *ValidationError, field string) (ValidationErrorItem, bool) {
	t.Helper()
	if verr == nil {
		return ValidationErrorItem{}, false
	}
	for _, item := range verr.Detail {
		if len(item.Loc) > 1 && item.Loc[1] == field {
			return item, true
		}
	}
	return ValidationErrorItem{}, false
}

// =============================================================================
// SolicitudPagoRequest.Validate
//
// Nota: la validacion real replica field_validator de pydantic (valor/formato),
// NO produce errores "Field required" tipo pydantic para campos vacios. Sobre un
// request vacio falla por Amount (mayor que 0), ExternalId (formato UID-ID),
// Currency (moneda valida) y ValidTime (entero).
// =============================================================================

func TestSolicitudPagoRequestValidate(t *testing.T) {
	base := SolicitudPagoRequest{
		Amount:      100.00,
		Phone:       "5355550000", // 10 caracteres: limite superior exacto
		Currency:    "CUP",
		ExternalId:  "caja-123",
		ValidTime:   "300",
		Description: "pago de prueba",
		Source:      "caja1",
		UrlResponse: "http://127.0.0.1:5082/notificar",
	}

	t.Run("request valido", func(t *testing.T) {
		if err := base.Validate(); err != nil {
			t.Fatalf("request valido no deberia fallar: %v", err)
		}
	})

	t.Run("request vacio falla por valor/formato", func(t *testing.T) {
		var req SolicitudPagoRequest
		err := req.Validate()
		if err == nil {
			t.Fatal("request vacio deberia fallar")
		}
		// Campos que la validacion real comprueba (por valor, no por presencia):
		want := []string{"Amount", "ExternalId", "Currency", "ValidTime"}
		if len(err.Detail) != len(want) {
			t.Fatalf("esperaba %d errores, obtuve %d: %+v", len(want), len(err.Detail), err.Detail)
		}
		for _, field := range want {
			item, ok := findFieldError(t, err, field)
			if !ok {
				t.Errorf("falta error para %s en %+v", field, err.Detail)
				continue
			}
			if item.Type == "missing" {
				t.Errorf("%s deberia ser error de valor/formato, no 'missing': %+v", field, item)
			}
		}
	})

	t.Run("errores por campo", func(t *testing.T) {
		cases := []struct {
			name    string
			mutate  func(*SolicitudPagoRequest)
			field   string
			wantMsg string
		}{
			{
				name:    "Amount cero",
				mutate:  func(r *SolicitudPagoRequest) { r.Amount = 0 },
				field:   "Amount",
				wantMsg: "greater than 0",
			},
			{
				name:    "Amount negativo",
				mutate:  func(r *SolicitudPagoRequest) { r.Amount = -5 },
				field:   "Amount",
				wantMsg: "greater than 0",
			},
			{
				name:    "Phone mayor a 10 caracteres",
				mutate:  func(r *SolicitudPagoRequest) { r.Phone = "53555500000" },
				field:   "Phone",
				wantMsg: "at most 10 characters",
			},
			{
				name:    "ExternalId sin guion",
				mutate:  func(r *SolicitudPagoRequest) { r.ExternalId = "caja123" },
				field:   "ExternalId",
				wantMsg: "UID-IDOPERACION",
			},
			{
				name:    "ExternalId con guion al inicio",
				mutate:  func(r *SolicitudPagoRequest) { r.ExternalId = "-abc" },
				field:   "ExternalId",
				wantMsg: "UID-IDOPERACION",
			},
			{
				name:    "Currency moneda no valida",
				mutate:  func(r *SolicitudPagoRequest) { r.Currency = "MXN" },
				field:   "Currency",
				wantMsg: "Moneda no válida",
			},
			{
				name:    "ValidTime con formato fecha ISO",
				mutate:  func(r *SolicitudPagoRequest) { r.ValidTime = "20260101T000000" },
				field:   "ValidTime",
				wantMsg: "entero",
			},
			{
				name:    "ValidTime no entero",
				mutate:  func(r *SolicitudPagoRequest) { r.ValidTime = "treinta" },
				field:   "ValidTime",
				wantMsg: "entero",
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				req := base
				c.mutate(&req)
				err := req.Validate()
				if err == nil {
					t.Fatalf("esperaba error en %s", c.field)
				}
				item, ok := findFieldError(t, err, c.field)
				if !ok {
					t.Fatalf("no se encontro error para %s en %+v", c.field, err.Detail)
				}
				if item.Type != "value_error" && item.Type != "greater_than" && item.Type != "string_too_long" {
					t.Errorf("tipo inesperado %q para %s", item.Type, c.field)
				}
				if !strings.Contains(item.Msg, c.wantMsg) {
					t.Errorf("msg %q no contiene %q", item.Msg, c.wantMsg)
				}
			})
		}
	})

	t.Run("Currency se normaliza a mayusculas", func(t *testing.T) {
		req := base
		req.Currency = "usd"
		if err := req.Validate(); err != nil {
			t.Fatalf("'usd' deberia ser valido: %v", err)
		}
		if req.Currency != "USD" {
			t.Errorf("Currency deberia normalizarse a USD, obtuve %q", req.Currency)
		}
	})
}

// =============================================================================
// CancelarRequest.Validate — solo valida formato de ExternalId (el struct
// solo tiene Phone y ExternalId; no existe campo Source).
// =============================================================================

func TestCancelarRequestValidate(t *testing.T) {
	t.Run("request vacio", func(t *testing.T) {
		var req CancelarRequest
		err := req.Validate()
		if err == nil {
			t.Fatal("request vacio deberia fallar")
		}
		if _, ok := findFieldError(t, err, "ExternalId"); !ok {
			t.Errorf("esperaba error de ExternalId: %+v", err.Detail)
		}
	})

	t.Run("request valido", func(t *testing.T) {
		req := CancelarRequest{Phone: "5355550000", ExternalId: "caja-77"}
		if err := req.Validate(); err != nil {
			t.Fatalf("request valido no deberia fallar: %v", err)
		}
	})

	t.Run("ExternalId sin guion", func(t *testing.T) {
		req := CancelarRequest{Phone: "5355550000", ExternalId: "caja77"}
		err := req.Validate()
		if err == nil {
			t.Fatal("ExternalId sin guion deberia fallar")
		}
		item, ok := findFieldError(t, err, "ExternalId")
		if !ok || !strings.Contains(item.Msg, "UID-IDOPERACION") {
			t.Errorf("error de ExternalId esperado: %+v", err.Detail)
		}
	})
}

// =============================================================================
// NotificacionRequest.Validate — solo valida formato de ExternalId.
// =============================================================================

func TestNotificacionRequestValidate(t *testing.T) {
	t.Run("request vacio", func(t *testing.T) {
		var req NotificacionRequest
		err := req.Validate()
		if err == nil {
			t.Fatal("request vacio deberia fallar")
		}
		if _, ok := findFieldError(t, err, "ExternalId"); !ok {
			t.Errorf("esperaba error de ExternalId: %+v", err.Detail)
		}
	})

	t.Run("request valido", func(t *testing.T) {
		req := NotificacionRequest{
			Phone: "5355550000", ExternalId: "caja-77", TmId: "123",
			BankId: "1", Status: "1", Source: "caja1", Msg: "ok", Bank: "BANDEC",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("request valido no deberia fallar: %v", err)
		}
	})
}

// =============================================================================
// DevolucionRequest.Validate — valida RefundID (formato) y UrlResponse (no vacia,
// con trim). Importe e IdTercero son opcionales y NO se validan.
// =============================================================================

func TestDevolucionRequestValidate(t *testing.T) {
	t.Run("request vacio falla por RefundID y UrlResponse", func(t *testing.T) {
		var req DevolucionRequest
		err := req.Validate()
		if err == nil {
			t.Fatal("request vacio deberia fallar")
		}
		for _, field := range []string{"RefundID", "UrlResponse"} {
			if _, ok := findFieldError(t, err, field); !ok {
				t.Errorf("falta error para %s: %+v", field, err.Detail)
			}
		}
	})

	t.Run("request valido", func(t *testing.T) {
		req := DevolucionRequest{
			RefundID: "caja-77", Source: "web", Code: "0",
			UrlResponse: "http://127.0.0.1:5082/notificar", Bank: "BANDEC",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("request valido no deberia fallar: %v", err)
		}
	})

	t.Run("Importe e IdTercero opcionales", func(t *testing.T) {
		req := DevolucionRequest{
			RefundID: "caja-77", Source: "web", Code: "0",
			UrlResponse: "http://127.0.0.1:5082/notificar", Bank: "BANDEC",
			Importe: 12.50, IdTercero: "tercero-1",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("campos opcionales no deberian afectar la validacion: %v", err)
		}
	})

	t.Run("UrlResponse solo espacios falla", func(t *testing.T) {
		req := DevolucionRequest{
			RefundID: "caja-77", UrlResponse: "   ",
		}
		err := req.Validate()
		if err == nil {
			t.Fatal("UrlResponse de solo espacios deberia fallar")
		}
		if _, ok := findFieldError(t, err, "UrlResponse"); !ok {
			t.Errorf("esperaba error de UrlResponse: %+v", err.Detail)
		}
	})

	t.Run("UrlResponse se recorta (trim)", func(t *testing.T) {
		req := DevolucionRequest{
			RefundID: "caja-77", UrlResponse: "  http://127.0.0.1:5082/notificar  ",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("UrlResponse con espacios alrededor deberia ser valida: %v", err)
		}
		if req.UrlResponse != "http://127.0.0.1:5082/notificar" {
			t.Errorf("UrlResponse deberia quedar recortada, obtuve %q", req.UrlResponse)
		}
	})
}

// =============================================================================
// NotificacionDevolRequest.Validate — solo valida formato de RefundID.
// =============================================================================

func TestNotificacionDevolRequestValidate(t *testing.T) {
	t.Run("request vacio", func(t *testing.T) {
		var req NotificacionDevolRequest
		err := req.Validate()
		if err == nil {
			t.Fatal("request vacio deberia fallar")
		}
		item, ok := findFieldError(t, err, "RefundID")
		if !ok || !strings.Contains(item.Msg, "UID-IDOPERACION") {
			t.Errorf("esperaba error de RefundID: %+v", err.Detail)
		}
	})

	t.Run("request valido", func(t *testing.T) {
		req := NotificacionDevolRequest{
			RefundID: "caja-77", ReferenceRefund: "r1", ReferenceRefundTM: "r2",
			Success: true, Resultmsg: "ok", Status: "1",
			ExternalID: "caja-77", BankId: "1", TmId: "123",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("request valido no deberia fallar: %v", err)
		}
	})
}

// =============================================================================
// ToJSON y json tags
// =============================================================================

func TestToJSONJSONTags(t *testing.T) {
	t.Run("SolicitudPagoRequest usa sus json tags exactos", func(t *testing.T) {
		req := SolicitudPagoRequest{
			Amount: 100.00, Phone: "5355550000", Currency: "CUP",
			Description: "pago", ExternalId: "caja-123", Source: "caja1",
			ValidTime: "300", UrlResponse: "http://x",
		}
		out := ToJSON(req)
		if !json.Valid([]byte(out)) {
			t.Fatalf("ToJSON no produjo JSON valido: %q", out)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, key := range []string{"Amount", "Phone", "Currency", "Description", "ExternalId", "Source", "ValidTime", "UrlResponse"} {
			if _, ok := m[key]; !ok {
				t.Errorf("falta la clave json %q en %q", key, out)
			}
		}
		if _, ok := m["external_id"]; ok {
			t.Errorf("no deberia existir 'external_id'; ExternalId usa su json tag exacto: %q", out)
		}
	})

	t.Run("DevolucionRequest: importe e id_tercero en minuscula", func(t *testing.T) {
		req := DevolucionRequest{
			RefundID: "caja-9", Source: "web", Code: "0",
			UrlResponse: "http://x", Bank: "BANDEC",
			Importe: 12.5, IdTercero: "tercero-1",
		}
		out := ToJSON(req)
		if !json.Valid([]byte(out)) {
			t.Fatalf("ToJSON no produjo JSON valido: %q", out)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, key := range []string{"RefundID", "Source", "Code", "UrlResponse", "Bank", "importe", "id_tercero"} {
			if _, ok := m[key]; !ok {
				t.Errorf("falta la clave json %q en %q", key, out)
			}
		}
		if _, ok := m["Importe"]; ok {
			t.Error("Importe no deberia serializarse como 'Importe' sino como 'importe'")
		}
		if _, ok := m["IdTercero"]; ok {
			t.Error("IdTercero no deberia serializarse como 'IdTercero' sino como 'id_tercero'")
		}
	})

	t.Run("round trip DevolucionRequest", func(t *testing.T) {
		original := DevolucionRequest{
			RefundID: "caja-9", Source: "web", Code: "0",
			UrlResponse: "http://x", Bank: "BANDEC",
			Importe: 12.5, IdTercero: "tercero-1",
		}
		var got DevolucionRequest
		if err := json.Unmarshal([]byte(ToJSON(original)), &got); err != nil {
			t.Fatalf("round trip unmarshal: %v", err)
		}
		if got != original {
			t.Errorf("round trip no coincide: %+v != %+v", got, original)
		}
	})
}

func TestValidationErrorSerialization(t *testing.T) {
	var req SolicitudPagoRequest
	err := req.Validate()
	if err == nil {
		t.Fatal("request vacio deberia fallar")
	}
	out := ToJSON(err)
	if !json.Valid([]byte(out)) {
		t.Fatalf("ToJSON(ValidationError) no es JSON valido: %q", out)
	}
	var parsed map[string]any
	if uerr := json.Unmarshal([]byte(out), &parsed); uerr != nil {
		t.Fatalf("no se puede parsear: %v", uerr)
	}
	detail, ok := parsed["detail"]
	if !ok {
		t.Fatalf("el JSON del ValidationError deberia tener clave 'detail': %q", out)
	}
	items, ok := detail.([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("detail deberia ser un arreglo no vacio: %q", out)
	}
	// Formato pydantic: cada item tiene loc, msg, type.
	if first, ok := items[0].(map[string]any); ok {
		for _, k := range []string{"loc", "msg", "type"} {
			if _, ok := first[k]; !ok {
				t.Errorf("falta clave %q en el item de detalle: %q", k, out)
			}
		}
	}
}