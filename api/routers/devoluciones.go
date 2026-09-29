// Package routers replica los endpoints de devoluciones.py: solicitudes
// de devolucion asincronica y notificaciones de devolucion a ETECSA.
package routers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pasarela/config"
	"pasarela/database"
	"pasarela/models"
	"pasarela/utils"
)

// =============================================================================
// enviaDevolucion — envio de solicitud de devolucion a ETECSA
// =============================================================================

// enviaDevolucion replica devoluciones.envia_devolucion: construye el body
// {"request": devol_dict}, sobrescribe SIEMPRE UrlResponse con
// config.Cfg.URLResDevol y hace POST a config.Cfg.Devolucion con headers de
// autenticacion. Devuelve (respuesta_json, 1) o ({'Error': ...}, 7).
func enviaDevolucion(ctx context.Context, req *models.DevolucionRequest) (map[string]any, int) {
	// Replica data.model_dump(): serializar el request a map.
	devolDict, err := json.Marshal(req)
	if err != nil {
		utils.VerError(err.Error())
		return map[string]any{"Error": "Error de conexion"}, 7
	}
	var devolMap map[string]any
	if err := json.Unmarshal(devolDict, &devolMap); err != nil {
		utils.VerError(err.Error())
		return map[string]any{"Error": "Error de conexion"}, 7
	}

	// Replica: enviar["request"]["UrlResponse"] = settings.url_res_devol
	enviar := map[string]any{"request": devolMap}
	enviar["request"].(map[string]any)["UrlResponse"] = config.Cfg.URLResDevol

	encodedParms, err := json.Marshal(enviar)
	if err != nil {
		utils.VerError(err.Error())
		return map[string]any{"Error": "Error de conexion"}, 7
	}

	// Replica: prepara_conexion(source=devolucion.get('Source', '30010'))
	source := req.Source
	if source == "" {
		source = "30010"
	}
	encab := utils.PreparaConexion(config.Cfg.SemillaAuth, "", source, time.Now())

	respuesta, err := postJSON(ctx, config.Cfg.Devolucion, string(encodedParms), encab)
	if err != nil {
		utils.VerError(err.Error())
		return map[string]any{"Error": "Error de conexion"}, 7
	}
	return respuesta, 1
}

// =============================================================================
// DevolucionHandler — POST /devolucion/
// Replica devoluciones.devolucion (FastAPI)
// =============================================================================

// DevolucionHandler procesa solicitudes de devolucion asincronicas.
// Replica el flujo: validacion del body -> verify_auth -> try con el equivalente
// del except (Estado=10) para cualquier error inesperado.
func DevolucionHandler(w http.ResponseWriter, r *http.Request) {
	idTraza := utils.IDAlfaGen(10)
	utils.Traza("---- Devolucion ----", idTraza)

	// Pydantic valida el body antes de entrar al handler; verify_auth es lo
	// primero dentro del handler (replica el orden exacto del Python).
	var req models.DevolucionRequest
	// Replica pydantic: DevolucionRequest requiere RefundID, Source, Code,
	// UrlResponse y Bank; importe e id_tercero son opcionales (default 0 / "")
	// (ver doc/endpoint_ref.md y models/modelos.py).
	if !decodeJSON(w, r, &req, []string{"RefundID", "Source", "Code", "UrlResponse", "Bank"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	// Replica: await verify_auth(request)
	if !verifyAuth(r) {
		writeAuthError(w)
		return
	}

	resp := procesaDevolucion(r, &req, idTraza)
	writeJSON(w, http.StatusOK, resp)
}

// procesaDevolucion replica el cuerpo del try de devolucion(). Cualquier panic
// inesperado se traduce en DevolucionResponse(Estado=10) igual que el except
// del Python.
func procesaDevolucion(r *http.Request, req *models.DevolucionRequest, idTraza string) (resp models.DevolucionResponse) {
	defer func() {
		if rec := recover(); rec != nil {
			utils.VerError(fmt.Sprintf("panic en devolucion: %v", rec))
			resp = models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}
	}()

	ctx := r.Context()
	db := database.Get()

	// Replica: importeadev = data.importe if data.importe else 0
	importeAdev := req.Importe
	// Replica: idpago = data.id_tercero if data.id_tercero else ''
	idpago := req.IdTercero

	// =========================================================================
	// Validar importe (replica exacta del Python)
	// =========================================================================
	if idpago != "" {
		// Suma de devoluciones previas en devolucion_proceso
		valDevuelto, err := db.SelectVal(ctx,
			"SELECT COALESCE(SUM(importe),0) FROM devolucion_proceso WHERE idpago = $1", idpago)
		if err != nil {
			// El except del Python devuelve Estado=10
			utils.VerError(err.Error())
			return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}
		devuelto := importeAdev + toFloat64(valDevuelto)

		// Suma de devoluciones previas en devolucion
		valDevuelto2, err := db.SelectVal(ctx,
			"SELECT COALESCE(SUM(importe),0) FROM devolucion WHERE idpago = $1", idpago)
		if err != nil {
			utils.VerError(err.Error())
			return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}
		devuelto += toFloat64(valDevuelto2)

		// obtener_origen(idpago): si falla, el except devuelve Estado=10
		uidPago, idOperacionPago, errOrigen := utils.ObtenerOrigen(idpago)
		if errOrigen != nil {
			utils.VerError(errOrigen.Error())
			return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}

		// Importe pagado en la tabla pagos (solo si tiene bankid)
		impago, err := db.Select(ctx,
			"SELECT importe, bankid FROM pagos WHERE uid = $1 AND idoperacion = $2",
			uidPago, idOperacionPago)
		if err != nil {
			utils.VerError(err.Error())
			return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}
		var pagado float64
		if len(impago) > 0 && impago[0] != nil {
			if _, ok := impago[0]["bankid"]; ok {
				pagado = toFloat64(impago[0]["importe"])
			}
		}

		// Importe pagado en la tabla pago_en_proceso (se suma si tiene bankid)
		impago2, err := db.Select(ctx,
			"SELECT importe, bankid FROM pago_en_proceso WHERE uid = $1 AND idoperacion = $2",
			uidPago, idOperacionPago)
		if err != nil {
			utils.VerError(err.Error())
			return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
		}
		if len(impago2) > 0 && impago2[0] != nil {
			if _, ok := impago2[0]["bankid"]; ok {
				pagado += toFloat64(impago2[0]["importe"])
			}
		}

		// Si lo devuelto (incluyendo esta devolucion) excede lo pagado -> 37
		if pagado < devuelto {
			return models.DevolucionResponse{Estado: 37, Msg: config.DICRES[37]}
		}
	}

	// =========================================================================
	// Obtener uid/id_operacion del RefundID (dentro del try del Python)
	// =========================================================================
	uid, idOperacion, errOrigen := utils.ObtenerOrigen(req.RefundID)
	if errOrigen != nil {
		// obtener_origen lanza excepcion -> except -> Estado=10
		utils.VerError(errOrigen.Error())
		return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
	}

	// =========================================================================
	// Validar IP del origen (replica: request.client.host)
	// =========================================================================
	ip := clientIP(r)
	valido, err := utils.IPValidoTV(ctx, uid, ip, db)
	if err != nil {
		utils.VerError(err.Error())
		return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
	}
	if !valido {
		return models.DevolucionResponse{Estado: 25, Msg: config.DICRES[25]}
	}

	// =========================================================================
	// Validar/insertar URL de respuesta (replica: select + insert_returning)
	// Si hay error -> except del Python -> Estado=10
	// =========================================================================
	idurl, err := utils.ChkURL(ctx, req.UrlResponse, config.URLList, db)
	if err != nil {
		utils.VerError(err.Error())
		return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
	}

	// =========================================================================
	// INSERT en devolucion_proceso (replica exacta de la query del Python)
	// =========================================================================
	_, err = db.Execute(ctx,
		`INSERT INTO devolucion_proceso(uid, idoperacion, cliente, url, banco, fecha, estado, importe, idpago)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8)`,
		uid, idOperacion, req.Source, idurl, req.Bank, time.Now(), importeAdev, idpago)
	if err != nil {
		utils.VerError(err.Error())
		return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
	}

	// =========================================================================
	// Enviar devolucion a ETECSA
	// =========================================================================
	utils.Traza(fmt.Sprintf("Api intenta conectarse a: %s", config.Cfg.Devolucion), idTraza)
	respuesta, error := enviaDevolucion(ctx, req)

	// =========================================================================
	// Procesar respuesta (replica exacta de la logica de estados)
	// =========================================================================
	estado := 0
	if error == 1 {
		rprRaw, exists := respuesta["RefundPayResult"]
		if exists {
			rpr, ok := rprRaw.(map[string]any)
			if !ok {
				// Python: .get() sobre un no-dict lanza AttributeError -> except -> 10
				utils.VerError("RefundPayResult no es un objeto JSON")
				return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
			}

			success, _ := rpr["Success"].(bool)
			if success {
				estado = 23
			} else {
				estado = 24
			}

			// Buscar/insertar msg con el Resultmsg de ETECSA
			msg, _ := rpr["Resultmsg"].(string)
			idmsg, errChkMsg := utils.ChkMsg(ctx, msg, config.MsgList, db)
			if errChkMsg != nil {
				utils.VerError(errChkMsg.Error())
				return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
			}

			// RefundID_Order: si viene como string se usa tal cual, si no "-1"
			refundIDOrder := "-1"
			if v, ok := rpr["RefundID_Order"]; ok {
				if s, okS := v.(string); okS {
					refundIDOrder = s
				} else {
					refundIDOrder = "-1"
				}
			}

			// Actualizar devolucion_proceso con el resultado de ETECSA
			_, errUpd := db.Execute(ctx,
				"UPDATE devolucion_proceso SET estado = $1, numorden = $2, msg = $3 WHERE uid = $4 AND idoperacion = $5",
				estado, refundIDOrder, idmsg, uid, idOperacion)
			if errUpd != nil {
				utils.VerError(errUpd.Error())
				return models.DevolucionResponse{Estado: 10, Msg: config.DICRES[10]}
			}
		} else {
			// No hay RefundPayResult -> estado 17
			estado = 17
		}
	} else {
		// error != 1 (p.ej. error de conexion) -> estado = error
		estado = error
	}

	// Replica: Msg=get_error_list().get(estado, 'Error desconocido')
	msgResp, okMsg := config.DICRES[estado]
	if !okMsg {
		msgResp = "Error desconocido"
	}
	// Replica: ETECSA=respuesta.get('RefundPayResult', {})
	eteca := map[string]any{}
	if rprRaw, ok := respuesta["RefundPayResult"]; ok {
		if rpr, okMap := rprRaw.(map[string]any); okMap {
			eteca = rpr
		}
	}

	return models.DevolucionResponse{
		Estado: estado,
		Msg:    msgResp,
		ETECSA: eteca,
	}
}

// =============================================================================
// NotificacionDevolHandler — POST /notificaciondevol/
// Replica devoluciones.notificaciondevol (FastAPI)
// =============================================================================

// NotificacionDevolHandler procesa notificaciones de estado de devolucion.
// El Python no declara response_model: devuelve un dict crudo, por eso aqui se
// responde con map[string]any (misma estructura JSON).
func NotificacionDevolHandler(w http.ResponseWriter, r *http.Request) {
	idTraza := utils.IDAlfaGen(10)
	utils.Traza("Llega una notificacion de devolucion", idTraza)

	var req models.NotificacionDevolRequest
	if !decodeJSON(w, r, &req, []string{"RefundID", "ReferenceRefund", "ReferenceRefundTM",
		"Success", "Resultmsg", "Status", "ExternalID", "BankId", "TmId"}) {
		return
	}
	if ve := req.Validate(); ve != nil {
		writeValidationError(w, ve)
		return
	}

	resp := procesaNotificacionDevol(r, &req)
	writeJSON(w, http.StatusOK, resp)
}

// procesaNotificacionDevol replica el try/except principal de notificaciondevol:
// cualquier error fuera del bloque interno de notificacion -> el except externo
// -> {'Success': False, 'Resultmsg': 'Error interno', 'Status': '10'}.
func procesaNotificacionDevol(r *http.Request, req *models.NotificacionDevolRequest) (resp map[string]any) {
	defer func() {
		if rec := recover(); rec != nil {
			utils.VerError(fmt.Sprintf("panic en notificaciondevol: %v", rec))
			resp = map[string]any{
				"Success":   false,
				"Resultmsg": "Error interno",
				"Status":    "10",
			}
		}
	}()

	ctx := r.Context()
	db := database.Get()

	// Replica: referencia = data.RefundID; uid, id_operacion = obtener_origen(...)
	uid, idOperacion, errOrigen := utils.ObtenerOrigen(req.RefundID)
	if errOrigen != nil {
		// obtener_origen lanza excepcion -> except externo
		utils.VerError(errOrigen.Error())
		return map[string]any{
			"Success":   false,
			"Resultmsg": "Error interno",
			"Status":    "10",
		}
	}

	// =========================================================================
	// Consultar estado actual de la devolucion en proceso
	// =========================================================================
	verEstado, err := db.Select(ctx,
		"SELECT estado, id FROM devolucion_proceso WHERE uid = $1 AND idoperacion = $2",
		uid, idOperacion)
	if err != nil {
		utils.VerError(err.Error())
		return map[string]any{
			"Success":   false,
			"Resultmsg": "Error interno",
			"Status":    "10",
		}
	}

	// Replica del bloque: if verestado and verestado[0]: (ejecuta solo si hay)
	if len(verEstado) > 0 && verEstado[0] != nil {
		// id_reg = verestado[0]['id'] if verestado[0].get('id') else None
		var idReg any
		if v, ok := verEstado[0]["id"]; ok && v != nil {
			if toInt64(v) != 0 {
				idReg = v
			}
		}

		// Replica: mensaje = data.Resultmsg.replace("'", " ")
		mensaje := strings.ReplaceAll(req.Resultmsg, "'", " ")

		// Buscar/insertar msg (idmsg = ... else -1)
		idmsg, errChkMsg := utils.ChkMsg(ctx, mensaje, config.MsgList, db)
		if errChkMsg != nil {
			// El except externo del Python captura el error de la query
			utils.VerError(errChkMsg.Error())
			return map[string]any{
				"Success":   false,
				"Resultmsg": "Error interno",
				"Status":    "10",
			}
		}

		// Replica: convierte = {1:30, 2:0, 3:1, 4:31, 5:32, 6:33, 7:34, 8:35, 9:36}
		convierte := map[int]int{1: 30, 2: 0, 3: 1, 4: 31, 5: 32, 6: 33, 7: 34, 8: 35, 9: 36}
		statusInt := 0
		if isDigit(req.Status) {
			statusInt, _ = strconv.Atoi(req.Status)
		}
		convertido, okConv := convierte[statusInt]
		if !okConv {
			convertido = statusInt
		}

		// Replica: query dinamica con "AND id = $N" solo si existe id_reg
		consulta := `UPDATE devolucion_proceso SET enlacepago = $1, estado = $2,
                     tmid = $3, bancoid = $4, msg = $5
                     WHERE uid = $6 AND idoperacion = $7`
		params := []any{
			req.ExternalID, convertido, req.ReferenceRefundTM,
			req.ReferenceRefund, idmsg, uid, idOperacion,
		}
		if idReg != nil {
			consulta += fmt.Sprintf(" AND id = $%d", len(params)+1)
			params = append(params, idReg)
		}

		_, errUpd := db.Execute(ctx, consulta, params...)
		if errUpd != nil {
			utils.VerError(errUpd.Error())
			return map[string]any{
				"Success":   false,
				"Resultmsg": "Error interno",
				"Status":    "10",
			}
		}
	}

	// =========================================================================
	// Obtener URL a notificar
	// =========================================================================
	// 1) Intentar de devolucion_proceso
	urlRows, err := db.Select(ctx,
		"SELECT url FROM devolucion_proceso WHERE uid = $1 AND idoperacion = $2",
		uid, idOperacion)
	if err != nil {
		utils.VerError(err.Error())
		return map[string]any{
			"Success":   false,
			"Resultmsg": "Error interno",
			"Status":    "10",
		}
	}
	url := ""
	if len(urlRows) > 0 && urlRows[0] != nil {
		url = toString(urlRows[0]["url"])
	}

	// 2) Si no, intentar de devolucion
	if url == "" {
		urlRows2, err := db.Select(ctx,
			"SELECT url FROM devolucion WHERE uid = $1 AND idoperacion = $2",
			uid, idOperacion)
		if err != nil {
			utils.VerError(err.Error())
			return map[string]any{
				"Success":   false,
				"Resultmsg": "Error interno",
				"Status":    "10",
			}
		}
		if len(urlRows2) > 0 && urlRows2[0] != nil {
			url = toString(urlRows2[0]["url"])
		}
	}

	// 3) Resolver la URL real: SELECT url FROM url WHERE id = $1
	if url != "" {
		urlRow, err := db.SelectOne(ctx, "SELECT url FROM url WHERE id = $1", url)
		if err != nil {
			utils.VerError(err.Error())
			return map[string]any{
				"Success":   false,
				"Resultmsg": "Error interno",
				"Status":    "10",
			}
		}
		if urlRow != nil {
			url = toString(urlRow["url"])
		} else {
			url = ""
		}
	}

	// =========================================================================
	// Enviar notificacion al origen (replica del bloque con try interno)
	// =========================================================================
	if url == "" {
		// Python: return {'Success': True}
		return map[string]any{"Success": true}
	}

	// Bloque interno del Python: cualquier error aqui -> {'Success': True}
	responseData, errPost := func() (map[string]any, error) {
		// Replica data.model_dump()
		notificaDict := map[string]any{
			"RefundID":          req.RefundID,
			"ReferenceRefund":   req.ReferenceRefund,
			"ReferenceRefundTM": req.ReferenceRefundTM,
			"Success":           req.Success,
			"Resultmsg":         req.Resultmsg,
			"Status":            req.Status,
			"ExternalID":        req.ExternalID,
			"BankId":            req.BankId,
			"TmId":              req.TmId,
		}
		encodedParms, err := json.Marshal(notificaDict)
		if err != nil {
			return nil, err
		}

		// Headers: prepara_conexion() + encab['tipo'] = '2'
		encab := utils.PreparaConexion(config.Cfg.SemillaAuth, "", "", time.Now())
		encab["tipo"] = "2"

		resp, err := postJSON(ctx, url, string(encodedParms), encab)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}()
	if errPost != nil {
		// Replica: except interno -> return {'Success': True}
		utils.VerError(errPost.Error())
		return map[string]any{"Success": true}
	}

	// Replica: if response_data.get('Success'): DELETE ...
	success, _ := responseData["Success"].(bool)
	if success {
		_, errDel := db.Execute(ctx,
			"DELETE FROM devolucion_proceso WHERE uid = $1 AND idoperacion = $2",
			uid, idOperacion)
		if errDel != nil {
			// El DELETE esta dentro del try interno: error -> {'Success': True}
			utils.VerError(errDel.Error())
			return map[string]any{"Success": true}
		}
	}

	// Replica: return response_data
	return responseData
}

// =============================================================================
// Helpers locales
// =============================================================================

// isDigit verifica si un string contiene solo digitos (replica str.isdigit()).
func isDigit(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}