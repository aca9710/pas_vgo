// Package routers replica los endpoints de pagos.py (FastAPI): solicitud de
// pago sincronica (/pago/), asincronica (/pago_a/), cancelacion (/cancelar/) y
// notificacion sincronica (/notificapagos/), junto con los helpers Redis
// (pasarela:listaid / pasarela:por_notificar) y el cicloespera.
package routers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"pasarela/config"
	"pasarela/database"
	"pasarela/models"
	"pasarela/redisclient"
	"pasarela/utils"
)

// =============================================================================
// Estados de pago (replica de pagos.py)
// =============================================================================

const (
	NO_ENVIADA  = 50
	ACEPTADA    = 51
	NO_ACEPTADA = 52
	NOTIFICADA  = 53
	VENCIDA     = 54
	NO_EXISTE   = 55
)

// =============================================================================
// Helpers Redis (replica de pagos.py: KEY_LISTAID / KEY_POR_NOTIFICAR)
// =============================================================================

const (
	keyListaID      = "pasarela:listaid"
	keyPorNotificar = "pasarela:por_notificar"
)

// addListaID anade ext al SET pasarela:listaid. Devuelve true si el id YA
// existia (duplicado) y false si se anadio ahora.
//
// NOTA de fidelidad: el Python usa add_listaid() -> sadd()==1 (true=anadido) y
// el handler trata "if not add_listaid" como duplicado. Aqui, con la semantica
// invertida pedida por el contrato Go (SIsMember primero, true=duplicado), los
// call sites usan "if addListaID(...)" directamente: el comportamiento neto
// frente al cliente es identico (duplicado -> Estado 14).
// Errores Redis: se registran con utils.VerError y se continua (no fatal).
func addListaID(ctx context.Context, ext string) bool {
	r := redisclient.Get()

	esMiembro, err := r.SIsMember(ctx, keyListaID, ext).Result()
	if err != nil {
		utils.VerError("addListaID SIsMember: " + err.Error())
		// No se pudo consultar: intentar anadir igualmente y continuar.
		if err := r.SAdd(ctx, keyListaID, ext).Err(); err != nil {
			utils.VerError("addListaID SAdd: " + err.Error())
		}
		return false
	}
	if esMiembro {
		return true // duplicado
	}
	if err := r.SAdd(ctx, keyListaID, ext).Err(); err != nil {
		utils.VerError("addListaID SAdd: " + err.Error())
	}
	return false
}

// containsListaID replica contains_listaid: SIsMember sobre pasarela:listaid.
func containsListaID(ctx context.Context, ext string) bool {
	r := redisclient.Get()
	esMiembro, err := r.SIsMember(ctx, keyListaID, ext).Result()
	if err != nil {
		utils.VerError("containsListaID: " + err.Error())
		return false
	}
	return esMiembro
}

// removeListaID replica remove_listaid: SRem sobre pasarela:listaid.
func removeListaID(ctx context.Context, ext string) {
	r := redisclient.Get()
	if err := r.SRem(ctx, keyListaID, ext).Err(); err != nil {
		utils.VerError("removeListaID: " + err.Error())
	}
}

// addPorNotificar replica add_por_notificar: SAdd sobre pasarela:por_notificar.
func addPorNotificar(ctx context.Context, ext string) {
	r := redisclient.Get()
	if err := r.SAdd(ctx, keyPorNotificar, ext).Err(); err != nil {
		utils.VerError("addPorNotificar: " + err.Error())
	}
}

// removePorNotificar replica remove_por_notificar: SRem sobre
// pasarela:por_notificar.
func removePorNotificar(ctx context.Context, ext string) {
	r := redisclient.Get()
	if err := r.SRem(ctx, keyPorNotificar, ext).Err(); err != nil {
		utils.VerError("removePorNotificar: " + err.Error())
	}
}

// containsPorNotificar replica contains_por_notificar: SIsMember sobre
// pasarela:por_notificar. Si Redis falla se devuelve true (sigue pendiente)
// para que cicloespera no corte la espera por un error transitorio.
func containsPorNotificar(ctx context.Context, ext string) bool {
	r := redisclient.Get()
	esMiembro, err := r.SIsMember(ctx, keyPorNotificar, ext).Result()
	if err != nil {
		utils.VerError("containsPorNotificar: " + err.Error())
		return true
	}
	return esMiembro
}

// =============================================================================
// continuar / cicloespera (replica de pagos.py)
// =============================================================================

// continuar replica pagos.continuar: true mientras quede tiempo hasta tlimite.
func continuar(tlimite time.Time) bool {
	return time.Until(tlimite) > 0
}

// cicloespera replica pagos.cicloespera: espera hasta que el externalid
// desaparezca de pasarela:por_notificar (-> NOTIFICADA) o se agote la ventana
// de segundos (-> queda el estado inicial, igual que el Python). Si al inicio
// ya no queda tiempo -> VENCIDA.
//
// NOTA de fidelidad: el Python NO consulta Vencimiento aqui: la espera es
// puramente temporal ((tlimite - now) > 0). El ticker de 500ms + select con
// ctx.Done() reproduce el asyncio.sleep(0.5) y evita bloquear el goroutine si
// el cliente se desconecta.
func cicloespera(ctx context.Context, externalid string, segundos float64, estado int) int {
	tlimite := time.Now().Add(time.Duration(segundos * float64(time.Second)))
	resultado := estado

	if !continuar(tlimite) {
		return VENCIDA
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Replica: si ya no esta en por_notificar -> NOTIFICADA (aunque
			// haya expirado el tiempo: la notificacion gana).
			if !containsPorNotificar(ctx, externalid) {
				resultado = NOTIFICADA
				return resultado
			}
			// Sigue pendiente: repetir solo si queda tiempo (el Python hace
			// ciclo = continuar(tlimite)). Si expiro, queda el estado inicial.
			if !continuar(tlimite) {
				return resultado
			}
		case <-ctx.Done():
			// El cliente se desconecto: devolver el estado actual.
			return resultado
		}
	}
}

// =============================================================================
// Construccion del pago_dict y envio a ETECSA (replica de pagos.py)
// =============================================================================

// buildPagoDict replica data.model_dump() de pagos.py con sus ajustes:
// Phone='0123456789' si viene vacio y UrlResponse SIEMPRE igual a
// settings.notif_pago (config.Cfg.NotifPago).
func buildPagoDict(req *models.SolicitudPagoRequest) map[string]any {
	d := map[string]any{
		"Description": req.Description,
		"Phone":       req.Phone,
		"Currency":    req.Currency,
		"Amount":      req.Amount,
		"ExternalId":  req.ExternalId,
		"ValidTime":   req.ValidTime,
		"Source":      req.Source,
	}
	if req.Phone == "" {
		d["Phone"] = "0123456789"
	}
	// Replica exacta del Python: pago_dict["UrlResponse"] = URL_NOTIF
	// (sobrescribe SIEMPRE, no solo si el request no trae UrlResponse).
	d["UrlResponse"] = config.Cfg.NotifPago
	return d
}

// postPagoEtecsa replica el envio a ETECSA de pagos.py: body
// json.dumps({"request": pago_dict}) y headers de prepara_conexion con source
// por defecto '00000'. Timeout de 60s (context.WithTimeout).
func postPagoEtecsa(ctx context.Context, pagoDict map[string]any) (map[string]any, error) {
	encodedParms, err := json.Marshal(map[string]any{"request": pagoDict})
	if err != nil {
		return nil, err
	}

	// Replica: prepara_conexion(source=pago_dict.get('Source', '00000')).
	source := toString(pagoDict["Source"])
	if source == "" {
		source = "00000"
	}
	encab := utils.PreparaConexion(config.Cfg.SemillaAuth, "", source, time.Now())

	ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return postJSON(ctxTimeout, config.Cfg.OrdenPago, string(encodedParms), encab)
}

// =============================================================================
// PagoHandler — POST /pago/
// Replica pagos.pago (solicitud de pago sincronica, espera el cicloespera)
// =============================================================================

// PagoHandler procesa una solicitud de pago sincronica. Replica el orden
// exacto de pagos.py: add_listaid (duplicado -> 14) ANTES de verify_auth.
func PagoHandler(w http.ResponseWriter, r *http.Request) {
	var req models.SolicitudPagoRequest
	// Replica pydantic: los 8 campos de SolicitudPagoRequest son requeridos
	// (ver doc/endpoint_ref.md y models/modelos.py).
	if !decodeJSON(w, r, &req, []string{"Amount", "Phone", "Currency", "Description", "ExternalId", "Source", "ValidTime", "UrlResponse"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	externalid := req.ExternalId

	// Replica: if not await add_listaid(externalid): Estado 14 (fuera del try).
	// addListaID devuelve true si el id YA existia (duplicado).
	if addListaID(r.Context(), externalid) {
		writeJSON(w, http.StatusOK, models.SolicitudPagoResponse{
			ExternalId: externalid,
			Estado:     14,
			Msg:        config.DICRES[14],
		})
		return
	}

	// Replica: await verify_auth(request) (fuera del try -> 401 directo).
	if !verifyAuth(r) {
		writeAuthError(w)
		return
	}

	idTraza := utils.IDAlfaGen(10)
	utils.Traza("Solicitud de pago", idTraza)

	resp := procesaPago(r, &req, externalid, idTraza)
	writeJSON(w, http.StatusOK, resp)
}

// procesaPago replica el try/except de pagos.pago: cualquier fallo inesperado
// (panic) se traduce en SolicitudPagoResponse{Estado:500, Msg:str(e)} igual
// que el except del Python.
func procesaPago(r *http.Request, req *models.SolicitudPagoRequest, externalid, idTraza string) (resp models.SolicitudPagoResponse) {
	defer func() {
		if rec := recover(); rec != nil {
			utils.VerError(fmt.Sprintf("panic en pago: %v", rec))
			resp = models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: fmt.Sprintf("%v", rec)}
		}
	}()

	ctx := r.Context()

	// Replica: vigencia = int(data.ValidTime) (ValidTime ya validado por
	// Validate(); si fallara, el except del Python devolveria 500).
	vigencia, _ := strconv.Atoi(req.ValidTime)

	// Replica: uid, id_operacion = utils.obtener_origen(...); si lanza -> except -> 500.
	if _, _, err := utils.ObtenerOrigen(externalid); err != nil {
		utils.VerError(err.Error())
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: err.Error()}
	}

	ip := clientIP(r)
	valido, err := utils.IPValido(ctx, ip, false, database.Get())
	if err != nil {
		// ipvalido lanza -> except del Python -> Estado 500.
		utils.VerError(err.Error())
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: err.Error()}
	}
	if !valido {
		// Replica: if not await ipvalido(...) -> Estado 25.
		// NOTA: el Python devuelve aqui ExternalId="" (default del modelo) y
		// con un campo 'Solicitud' que pydantic descarta; el contrato Go fija
		// ExternalId en la respuesta.
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 25, Msg: config.DICRES[25]}
	}

	// Replica: admpagos.add(ip, data) (en el sincronico se llama ANTES del POST).
	admpagos := getAdmin()
	if err := admpagos.Add(ip, req); err != nil {
		utils.VerError(err.Error())
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: err.Error()}
	}

	// Replica: pago_dict = data.model_dump() + ajustes, luego el POST a ETECSA.
	pagoDict := buildPagoDict(req)
	utils.Traza(fmt.Sprintf("Enviando solicitud de pago a %s", config.Cfg.OrdenPago), idTraza)

	respData, err := postPagoEtecsa(ctx, pagoDict)
	if err != nil {
		// El except del Python devuelve {ExternalId, 500, str(e)}.
		utils.VerError(err.Error())
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: err.Error()}
	}
	utils.Traza("Conexion terminada", idTraza)

	// Replica: if response_data.get('PayOrderResult', {}).get('Success'):
	payOrderRaw, ok := respData["PayOrderResult"]
	if !ok {
		// Sin PayOrderResult -> el .get del Python da falsy -> NO_ACEPTADA.
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: NO_ACEPTADA, Msg: config.DICRES[NO_ACEPTADA]}
	}
	payOrder, ok := payOrderRaw.(map[string]any)
	if !ok {
		// PayOrderResult no es un objeto JSON: el Python lanza AttributeError
		// al hacer .get('Success') -> except -> Estado 500.
		utils.VerError("PayOrderResult no es un objeto JSON")
		return models.SolicitudPagoResponse{ExternalId: externalid, Estado: 500, Msg: "Respuesta inesperada ETECSA"}
	}

	// NOTA de fidelidad: el Python usa truthiness en .get('Success'); aqui se
	// asume bool (igual que PayOrderResult.Success: bool del modelo pydantic).
	success, _ := payOrder["Success"].(bool)
	if success {
		// Replica: orderid = response_data['PayOrderResult']['OrderId']
		// (defensivo: si falta la clave -> 0, nunca panic).
		orderID := toInt64(payOrder["OrderId"])
		utils.Traza(fmt.Sprintf("Solicitud de pago aceptada por etecsa OrderId = %d", orderID), idTraza)
		admpagos.NumOrden(ctx, orderID, externalid)

		// Replica: estado = ACEPTADA; estado = await cicloespera(externalid, int(vigencia), estado)
		estado := ACEPTADA
		estado = cicloespera(ctx, externalid, float64(vigencia), estado)

		msg := config.DICRES[estado]
		if estado == ACEPTADA {
			return models.SolicitudPagoResponse{
				ExternalId:   externalid,
				Estado:       estado,
				Msg:          msg,
				Notificacion: admpagos.NotificaJSON(externalid),
			}
		}
		return models.SolicitudPagoResponse{
			ExternalId: externalid,
			Estado:     estado,
			Msg:        msg,
		}
	}

	// Replica: else -> estado = NO_ACEPTADA.
	return models.SolicitudPagoResponse{ExternalId: externalid, Estado: NO_ACEPTADA, Msg: config.DICRES[NO_ACEPTADA]}
}

// =============================================================================
// PagoAHandler — POST /pago_a/
// Replica pagos.pago_async (solicitud de pago asincronica, retorna inmediato)
// =============================================================================

// PagoAHandler procesa una solicitud de pago asincronica. Replica el orden de
// pagos.py: contains_listaid (duplicado -> 14) -> verify_auth -> traza ->
// add_listaid -> try.
func PagoAHandler(w http.ResponseWriter, r *http.Request) {
	var req models.SolicitudPagoRequest
	// Replica pydantic: los 8 campos de SolicitudPagoRequest son requeridos
	// (ver doc/endpoint_ref.md y models/modelos.py).
	if !decodeJSON(w, r, &req, []string{"Amount", "Phone", "Currency", "Description", "ExternalId", "Source", "ValidTime", "UrlResponse"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	externalid := req.ExternalId

	// Replica: if await contains_listaid(externalid): Estado 14 (no anade).
	if containsListaID(r.Context(), externalid) {
		writeJSON(w, http.StatusOK, models.SolicitudPagoAsyncResponse{
			ExternalId: externalid,
			Estado:     14,
			Msg:        config.DICRES[14],
		})
		return
	}

	if !verifyAuth(r) {
		writeAuthError(w)
		return
	}

	idTraza := utils.IDAlfaGen(10)
	utils.Traza("Solicitud de pago async", idTraza)

	// Replica: await add_listaid(externalid) (el retorno se ignora, igual que
	// el Python).
	addListaID(r.Context(), externalid)

	resp := procesaPagoA(r, &req, externalid, idTraza)
	writeJSON(w, http.StatusOK, resp)
}

// procesaPagoA replica el try/except de pagos.pago_async. A diferencia del
// sincronico, cualquier fallo hace remove_listaid y responde Estado=10 (no 500)
// con Msg=str(e), igual que el except del Python.
func procesaPagoA(r *http.Request, req *models.SolicitudPagoRequest, externalid, idTraza string) (resp models.SolicitudPagoAsyncResponse) {
	ctx := r.Context()

	defer func() {
		if rec := recover(); rec != nil {
			utils.VerError(fmt.Sprintf("panic en pago async: %v", rec))
			removeListaID(ctx, externalid)
			resp = models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 10, Msg: fmt.Sprintf("%v", rec)}
		}
	}()

	// Replica: ip = request.client.host; if not await ipvalido(...):
	// remove_listaid + Estado 25.
	ip := clientIP(r)
	valido, err := utils.IPValido(ctx, ip, false, database.Get())
	if err != nil {
		// ipvalido lanza -> except del Python -> remove + Estado 10.
		utils.VerError(err.Error())
		removeListaID(ctx, externalid)
		return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 10, Msg: err.Error()}
	}
	if !valido {
		removeListaID(ctx, externalid)
		return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 25, Msg: config.DICRES[25]}
	}

	// Replica: pago_dict + POST a ETECSA.
	pagoDict := buildPagoDict(req)
	utils.Traza(fmt.Sprintf("Enviando pago async a %s", config.Cfg.OrdenPago), idTraza)

	respData, err := postPagoEtecsa(ctx, pagoDict)
	if err != nil {
		// except del Python: ver_error + remove_listaid + Estado 10.
		utils.VerError(err.Error())
		removeListaID(ctx, externalid)
		return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 10, Msg: err.Error()}
	}
	utils.Traza("Respuesta ETECSA recibida", idTraza)

	// Replica: if response_data.get('PayOrderResult', {}).get('Success'):
	payOrderRaw, ok := respData["PayOrderResult"]
	if !ok {
		// Sin PayOrderResult -> falsy -> NO_ACEPTADA.
		return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: NO_ACEPTADA, Msg: config.DICRES[NO_ACEPTADA]}
	}
	payOrder, ok := payOrderRaw.(map[string]any)
	if !ok {
		// PayOrderResult no es un objeto JSON -> AttributeError en el Python
		// -> except -> remove + Estado 10.
		utils.VerError("PayOrderResult no es un objeto JSON")
		removeListaID(ctx, externalid)
		return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 10, Msg: "Respuesta inesperada ETECSA"}
	}

	success, _ := payOrder["Success"].(bool)
	if success {
		// Replica: orderid defensivo; admpagos.add(ip, data) y num_orden se
		// llaman SOLO en el caso de exito (igual que en pago_async).
		orderID := toInt64(payOrder["OrderId"])

		admpagos := getAdmin()
		if err := admpagos.Add(ip, req); err != nil {
			utils.VerError(err.Error())
			removeListaID(ctx, externalid)
			return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: 10, Msg: err.Error()}
		}
		admpagos.NumOrden(ctx, orderID, externalid)

		estado := ACEPTADA
		msg := config.DICRES[estado]
		orderIDInt := int(orderID)
		vt := req.ValidTime
		return models.SolicitudPagoAsyncResponse{
			ExternalId: externalid,
			Estado:     estado,
			Msg:        msg,
			OrderId:    &orderIDInt,
			ValidTime:  &vt,
		}
	}

	// Replica: else -> NO_ACEPTADA.
	return models.SolicitudPagoAsyncResponse{ExternalId: externalid, Estado: NO_ACEPTADA, Msg: config.DICRES[NO_ACEPTADA]}
}

// =============================================================================
// CancelarHandler — POST /cancelar/
// Replica pagos.cancelar
// =============================================================================

// CancelarHandler cancela una solicitud de pago.
//
// NOTA de fidelidad: el flujo REAL del Python es un unico UPDATE directo a la
// tabla pago_en_proceso (no consulta sol_pagos ni devuelve Estado 19). Se
// replica la query exacta, incluido el detalle de que el Python pasa
// data.Phone como uid y data.ExternalId como idoperacion. Si el UPDATE falla,
// la excepcion se propaga como 500 de FastAPI -> writeError500.
func CancelarHandler(w http.ResponseWriter, r *http.Request) {
	var req models.CancelarRequest
	if !decodeJSON(w, r, &req, []string{"Phone", "ExternalId"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	// Replica: await verify_auth(request) (lo primero dentro del handler).
	if !verifyAuth(r) {
		writeAuthError(w)
		return
	}

	// Replica exacta de la query parameterizada de pagos.py.
	consulta := "UPDATE pago_en_proceso SET estado = 11, tmid = -1, bankid = -1, msg = -1 WHERE uid = $1 AND idoperacion = $2"
	if _, err := database.Get().Execute(r.Context(), consulta, req.Phone, req.ExternalId); err != nil {
		utils.VerError(err.Error())
		writeError500(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, models.CancelarResponse{Estado: 11, Error: nil})
}

// =============================================================================
// NotificacionHandler — POST /notificapagos/
// Replica pagos.notificacion (notificacion sincronica de estado del pago)
// =============================================================================

// NotificacionHandler procesa la notificacion sincronica de un pago. Replica
// la estructura de dos try del Python: el primer bloque (obtener_origen,
// existe_pago, notifica, remove_por_notificar) con fallo -> {Success:true,
// Msg:str(e), Status:"500"} y el segundo bloque que decide la respuesta segun
// existe/vencimiento.
func NotificacionHandler(w http.ResponseWriter, r *http.Request) {
	var req models.NotificacionRequest
	// El pydantic declara los 8 campos requeridos, pero el contrato Go (y los
	// tests) mandan Source/Msg/Bank como opcionales.
	if !decodeJSON(w, r, &req, []string{"ExternalId", "Phone", "TmId", "BankId", "Status"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	resp := procesaNotificacion(r, &req)
	writeJSON(w, http.StatusOK, resp)
}

// procesaNotificacion replica el primer try del Python (incluido su except
// -> {Success:true, Resultmsg:str(e), Status:"500"}) y el segundo bloque de
// decision (existe/vencimiento). El except del segundo try del Python devuelve
// {Success:true, Resultmsg:DICRES[18], Status:"18"} — en Go ese bloque solo
// toca memoria (Vencimiento), por lo que ese camino no es alcanzable.
func procesaNotificacion(r *http.Request, req *models.NotificacionRequest) (resp models.NotificacionResponse) {
	defer func() {
		if rec := recover(); rec != nil {
			utils.VerError(fmt.Sprintf("panic en notificacion: %v", rec))
			resp = models.NotificacionResponse{Success: true, Resultmsg: fmt.Sprintf("%v", rec), Status: "500"}
		}
	}()

	idTraza := utils.IDAlfaGen(10)
	utils.Traza("Llega notificacion de pago sincronica", idTraza)

	ctx := r.Context()
	admpagos := getAdmin()

	// Replica: uid, id_operacion = utils.obtener_origen(...); si lanza ->
	// except -> {Success:true, Resultmsg:str(e), Status:"500"}.
	if _, _, err := utils.ObtenerOrigen(req.ExternalId); err != nil {
		utils.VerError(err.Error())
		return models.NotificacionResponse{Success: true, Resultmsg: err.Error(), Status: "500"}
	}

	// Replica: existe = await admpagos.existe_pago(externalid).
	existe, err := admpagos.ExistePago(ctx, req.ExternalId)
	if err != nil {
		utils.VerError(err.Error())
		return models.NotificacionResponse{Success: true, Resultmsg: err.Error(), Status: "500"}
	}

	// Replica exacta del Python: notifica() y remove_por_notificar() se llaman
	// SIEMPRE (incluso si el pago no existe), dentro del primer try.
	admpagos.Notifica(ctx, req)
	removePorNotificar(ctx, req.ExternalId)

	// Segundo bloque: decidir la respuesta segun existe/vencimiento.
	if existe != nil {
		if admpagos.Vencimiento(req.ExternalId) {
			return models.NotificacionResponse{
				Success:   false,
				Resultmsg: config.DICRES[VENCIDA],
				Status:    strconv.Itoa(VENCIDA),
			}
		}
		return models.NotificacionResponse{
			Success:   true,
			Resultmsg: config.DICRES[18],
			Status:    "18",
		}
	}

	return models.NotificacionResponse{
		Success:   false,
		Resultmsg: config.DICRES[NO_EXISTE],
		Status:    strconv.Itoa(NO_EXISTE),
	}
}