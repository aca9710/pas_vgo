// Package models replica models/modelos.py: modelos de request/response con
// validacion equivalente a los validators de pydantic.
package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// =============================================================================
// Validacion estilo pydantic (para replicar errores 422 de FastAPI)
// =============================================================================

// ValidationErrorItem replica un elemento de detail[] de FastAPI.
type ValidationErrorItem struct {
	Loc  []string `json:"loc"`
	Msg  string   `json:"msg"`
	Type string   `json:"type"`
}

// ValidationError agrupa los errores de validacion de un request.
type ValidationError struct {
	Detail []ValidationErrorItem `json:"detail"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %d field(s)", len(e.Detail))
}

func fieldRequired(field string) ValidationErrorItem {
	return ValidationErrorItem{Loc: []string{"body", field}, Msg: "Field required", Type: "missing"}
}

func valueError(field, msg string) ValidationErrorItem {
	return ValidationErrorItem{Loc: []string{"body", field}, Msg: "Value error, " + msg, Type: "value_error"}
}

// =============================================================================
// REQUEST MODELS
// =============================================================================

// SolicitudPagoRequest — Solicitud de pago sincronico (PCPOS)
type SolicitudPagoRequest struct {
	Amount      float64 `json:"Amount"`
	Phone       string  `json:"Phone"`
	Currency    string  `json:"Currency"`
	Description string  `json:"Description"`
	ExternalId  string  `json:"ExternalId"`
	Source      string  `json:"Source"`
	ValidTime   string  `json:"ValidTime"`
	UrlResponse string  `json:"UrlResponse"`
}

// Validate replica los field_validator de pydantic.
func (r *SolicitudPagoRequest) Validate() *ValidationError {
	var errs []ValidationErrorItem

	if r.Amount <= 0 {
		errs = append(errs, ValidationErrorItem{Loc: []string{"body", "Amount"}, Msg: "Input should be greater than 0", Type: "greater_than"})
	}
	if len(r.Phone) > 10 {
		errs = append(errs, ValidationErrorItem{Loc: []string{"body", "Phone"}, Msg: "String should have at most 10 characters", Type: "string_too_long"})
	}
	if strings.Index(r.ExternalId, "-") <= 0 {
		errs = append(errs, valueError("ExternalId", "Formato inválido, debe ser UID-IDOPERACION"))
	}
	up := strings.ToUpper(r.Currency)
	if up != "CUP" && up != "CUC" && up != "USD" && up != "EUR" {
		errs = append(errs, valueError("Currency", "Moneda no válida"))
	} else {
		r.Currency = up
	}
	if _, err := strconv.Atoi(r.ValidTime); err != nil {
		errs = append(errs, valueError("ValidTime", "ValidTime debe ser un número entero"))
	}

	if len(errs) > 0 {
		return &ValidationError{Detail: errs}
	}
	return nil
}

// CancelarRequest — Cancelar solicitud de pago
type CancelarRequest struct {
	Phone      string `json:"Phone"`
	ExternalId string `json:"ExternalId"`
}

func (r *CancelarRequest) Validate() *ValidationError {
	var errs []ValidationErrorItem
	if strings.Index(r.ExternalId, "-") <= 0 {
		errs = append(errs, valueError("ExternalId", "Formato inválido, debe ser UID-IDOPERACION"))
	}
	if len(errs) > 0 {
		return &ValidationError{Detail: errs}
	}
	return nil
}

// NotificacionRequest — Notificacion sincronica de estado del pago
type NotificacionRequest struct {
	Phone      string `json:"Phone"`
	ExternalId string `json:"ExternalId"`
	TmId       string `json:"TmId"`
	BankId     string `json:"BankId"`
	Status     string `json:"Status"`
	Source     string `json:"Source"`
	Msg        string `json:"Msg"`
	Bank       string `json:"Bank"`
}

func (r *NotificacionRequest) Validate() *ValidationError {
	var errs []ValidationErrorItem
	if strings.Index(r.ExternalId, "-") <= 0 {
		errs = append(errs, valueError("ExternalId", "Formato inválido, debe ser UID-IDOPERACION"))
	}
	if len(errs) > 0 {
		return &ValidationError{Detail: errs}
	}
	return nil
}

// NotificacionDevolRequest — Notificacion de devolucion
type NotificacionDevolRequest struct {
	RefundID          string `json:"RefundID"`
	ReferenceRefund   string `json:"ReferenceRefund"`
	ReferenceRefundTM string `json:"ReferenceRefundTM"`
	Success           bool   `json:"Success"`
	Resultmsg         string `json:"Resultmsg"`
	Status            string `json:"Status"`
	ExternalID        string `json:"ExternalID"`
	BankId            string `json:"BankId"`
	TmId              string `json:"TmId"`
}

func (r *NotificacionDevolRequest) Validate() *ValidationError {
	var errs []ValidationErrorItem
	if strings.Index(r.RefundID, "-") <= 0 {
		errs = append(errs, valueError("RefundID", "Formato inválido, debe ser UID-IDOPERACION"))
	}
	if len(errs) > 0 {
		return &ValidationError{Detail: errs}
	}
	return nil
}

// DevolucionRequest — Solicitud de devolucion
type DevolucionRequest struct {
	RefundID    string  `json:"RefundID"`
	Source      string  `json:"Source"`
	Code        string  `json:"Code"`
	UrlResponse string  `json:"UrlResponse"`
	Bank        string  `json:"Bank"`
	Importe     float64 `json:"importe"`
	IdTercero   string  `json:"id_tercero"`
}

func (r *DevolucionRequest) Validate() *ValidationError {
	var errs []ValidationErrorItem
	if strings.Index(r.RefundID, "-") <= 0 {
		errs = append(errs, valueError("RefundID", "Formato inválido, debe ser UID-IDOPERACION"))
	}
	if strings.TrimSpace(r.UrlResponse) == "" {
		errs = append(errs, valueError("UrlResponse", "UrlResponse no puede estar vacío"))
	} else {
		r.UrlResponse = strings.TrimSpace(r.UrlResponse)
	}
	if len(errs) > 0 {
		return &ValidationError{Detail: errs}
	}
	return nil
}

// =============================================================================
// RESPONSE MODELS
// =============================================================================

// SolicitudPagoResponse — Respuesta de solicitud de pago sincronico
type SolicitudPagoResponse struct {
	ExternalId   string `json:"ExternalId"`
	Estado       int    `json:"Estado"`
	Msg          string `json:"Msg"`
	Notificacion any    `json:"Notificacion"` // nil -> null (como Optional[Dict])
}

// SolicitudPagoAsyncResponse — Respuesta de solicitud de pago asincronico
type SolicitudPagoAsyncResponse struct {
	ExternalId string  `json:"ExternalId"`
	Estado     int     `json:"Estado"`
	Msg        string  `json:"Msg"`
	OrderId    *int    `json:"OrderId"`    // nil -> null
	ValidTime  *string `json:"ValidTime"`  // nil -> null
}

// NotificacionResponse — Respuesta de notificacion
type NotificacionResponse struct {
	Success   bool   `json:"Success"`
	Resultmsg string `json:"Resultmsg"`
	Status    string `json:"Status"`
}

// CancelarResponse — Respuesta de cancelacion
type CancelarResponse struct {
	Estado int    `json:"Estado"`
	Error  *string `json:"Error"` // nil -> null
}

// DevolucionResponse — Respuesta de devolucion
type DevolucionResponse struct {
	Estado int            `json:"Estado"`
	Msg    string         `json:"Msg"`
	ETECSA map[string]any `json:"ETECSA"`
}

// EstadoOrdenResponse — Respuesta de estado de orden de pago
type EstadoOrdenResponse struct {
	ETECSA  map[string]any `json:"ETECSA"`
	MsgAPI  string         `json:"MsgAPI"`
	Estado  int            `json:"Estado"`
	Response *string       `json:"response"` // nil -> null
}

// EstadoOrdenLocalResponse — Respuesta de estado local de orden de pago
type EstadoOrdenLocalResponse struct {
	Msg    string         `json:"Msg"`
	Local  map[string]any `json:"Local"`
	Estado int            `json:"Estado"`
}

// EstadoDevolucionResponse — Respuesta de estado de devolucion
type EstadoDevolucionResponse struct {
	ETECSA map[string]any `json:"ETECSA"`
	Msg    string         `json:"Msg"`
	Estado int            `json:"Estado"`
}

// =============================================================================
// Helpers
// =============================================================================

// ToJSON serializa el modelo a JSON (para debug/tests).
func ToJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}