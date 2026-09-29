// Package routers replica los endpoints de estado.py: verificacion del
// estado de ordenes de pago (ETECSA y local) y devoluciones.
package routers

import (
	"fmt"
	"net/http"
	"time"

	"pasarela/config"
	"pasarela/database"
	"pasarela/models"
	"pasarela/utils"
)

// dicMsg replica el acceso con fallback del Python:
// dicerror.get(estado, "Estado no conocido"). Un acceso directo al mapa en Go
// devolveria "" para claves ausentes, lo que difiere del comportamiento de
// estado.py en todos los endpoints de estado.
func dicMsg(dicerror map[int]string, estado int) string {
	if m, ok := dicerror[estado]; ok {
		return m
	}
	return "Estado no conocido"
}

// =============================================================================
// EstadoOrdenHandler — GET /estadoordenpago/{externalid}/{source}/
// Replica estado.py :: estado_orden
// =============================================================================

// EstadoOrdenHandler verifica el estado de la solicitud de pago en ETECSA.
func EstadoOrdenHandler(w http.ResponseWriter, r *http.Request) {
	externalid := r.PathValue("externalid")
	source := r.PathValue("source")

	// Mapeo de estados ETECSA -> locales (replica exacta de estado.py)
	estados := map[int]int{0: 0, 1: 1, 2: 0, 3: 3, 4: 3, 5: 5, 6: 5, 7: 5, 8: 5, -7: 7, -8: 8}
	idTraza := utils.IDAlfaGen(10)

	// Construccion de la URL de ETECSA (replica: f"{settings.estadoorden}{externalid}/{source}/")
	direccionEtecsa := fmt.Sprintf("%s%s/%s/", config.Cfg.EstadoOrden, externalid, source)

	// Headers de autenticacion para ETECSA
	encab := utils.PreparaConexion(config.Cfg.SemillaAuth, "", source, time.Now())

	// Llamada GET a ETECSA
	respData, err := getJSON(r.Context(), direccionEtecsa, encab)
	if err != nil {
		utils.VerError(err.Error())
		// Replica del except de estado.py: Estado=-7 y response=str(e)
		strErr := err.Error()
		writeJSON(w, http.StatusOK, models.EstadoOrdenResponse{
			Estado:   -7,
			MsgAPI:   fmt.Sprintf("7: %s", config.DICRES[7]),
			ETECSA:   nil,
			Response: &strErr,
		})
		return
	}

	// Buscar 'GetStatusOrderResult' en la respuesta (replica: if 'GetStatusOrderResult' in response_data)
	if getStatus, ok := respData["GetStatusOrderResult"]; ok {
		if getResult, ok := getStatus.(map[string]any); ok {
			// Extraer Status del resultado (default 0 si no existe)
			estado := 0
			if st, ok := getResult["Status"]; ok {
				estado = toInt(st)
			}

			utils.Traza(fmt.Sprintf("etecsa dice = %v", respData), idTraza)

			// MsgAPI: "{mapped}: {dicerror.get(mapped, 'Estado no conocido')}" —
			// usa el valor MAPEADO en el diccionario con fallback (replica de
			// estado.py: estados.get(estado, estado) + dicerror.get(...)).
			mapped, ok := estados[estado]
			if !ok {
				mapped = estado // estados.get(estado, estado)
			}
			dicerror := utils.GetErrorList()
			msgAPI := fmt.Sprintf("%d: %s", mapped, dicMsg(dicerror, mapped))

			writeJSON(w, http.StatusOK, models.EstadoOrdenResponse{
				ETECSA: getResult,
				MsgAPI: msgAPI,
				Estado: estado,
			})
			return
		}
	}

	// Key 'GetStatusOrderResult' no encontrada o tipo inesperado
	strResp := fmt.Sprintf("%v", respData)
	writeJSON(w, http.StatusOK, models.EstadoOrdenResponse{
		Estado:   -7,
		MsgAPI:   fmt.Sprintf("7: %s", config.DICRES[7]),
		Response: &strResp,
	})
}

// =============================================================================
// EstadoOrdenLocalHandler — GET /estadoordenpagolocal/{externalid}/{source}/
// Replica estado.py :: estadoordenpagolocal
// =============================================================================

// EstadoOrdenLocalHandler verifica el estado local de la solicitud de pago
// consultando las tablas pagos y pago_en_proceso.
func EstadoOrdenLocalHandler(w http.ResponseWriter, r *http.Request) {
	externalid := r.PathValue("externalid")
	_ = r.PathValue("source") // source no se usa en queries, solo en path

	idTraza := utils.IDAlfaGen(10)
	utils.Traza(fmt.Sprintf("Chequeo estado pago, ExternalID= %s", externalid), idTraza)

	// Mapeo de estados locales (replica exacta de estado.py)
	convierte := map[int]int{30: 1, 0: 2, 1: 3, 31: 4, 32: 5, 33: 6, 34: 7, 35: 8, 36: 9}

	// Obtener uid e id_operacion del externalid (replica: uid, id_operacion = obtener_origen(externalid))
	uid, idOperacion, err := utils.ObtenerOrigen(externalid)
	if err != nil {
		utils.VerError(err.Error())
		writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
			Estado: 19,
			Msg:    fmt.Sprintf("19: %s", config.DICRES[19]),
			Local:  map[string]any{},
		})
		return
	}

	ctx := r.Context()

	// Query 1: buscar en tabla pagos (replica exacta de estado.py)
	consulta := `SELECT id, idoperacion, importe, cliente, celular, moneda, uid, estado,
		orderid, tmid, bankid, msg, url, banco
		FROM pagos WHERE uid = $1 AND idoperacion = $2`
	rows, err := database.Get().Select(ctx, consulta, uid, idOperacion)
	if err != nil {
		utils.VerError(err.Error())
		writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
			Estado: 19,
			Msg:    fmt.Sprintf("19: %s", config.DICRES[19]),
			Local:  map[string]any{},
		})
		return
	}

	if len(rows) > 0 {
		// Pago encontrado en tabla pagos
		p := rows[0]

		// Mapear estado: convierte.get(p['estado'], p['estado']) si no es None, si no -1
		estadoRaw := p["estado"]
		hestad := -1
		if estadoRaw != nil {
			estadoInt := toInt(estadoRaw)
			if mapped, ok := convierte[estadoInt]; ok {
				hestad = mapped
			} else {
				hestad = estadoInt
			}
		}

		// Construir respuesta Local (replica exacta de estado.py, pagos)
		local := map[string]any{
			"OrderId":    orEmpty(p["orderid"]),
			"Status":     hestad,
			"Resultmsg":  orEmptyStr(p["msg"]),
			"Success":    hestad == 3,
			"TmId":       orEmpty(p["tmid"]),
			"ExternalId": externalid,
			"BankId":     orEmpty(p["bankid"]),
			"Bank":       orZero(p["banco"]),
			"Phone":      orEmpty(p["celular"]),
		}

		// Estado: raw del DB si no es None, si no -1 (replica: p['estado'] if p.get('estado') is not None else -1)
		estado := -1
		if estadoRaw != nil {
			estado = toInt(estadoRaw)
		}

		dicerror := utils.GetErrorList()
		writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
			Msg:    fmt.Sprintf("%d: %s", estado, dicMsg(dicerror, estado)),
			Local:  local,
			Estado: estado,
		})
		return
	}

	// Query 2: buscar en tabla pago_en_proceso (replica: segundo query de estado.py)
	consulta = `SELECT id, idoperacion, importe, cliente, celular, moneda, uid, estado,
		orderid, tmid, bankid, msg, url, banco
		FROM pago_en_proceso WHERE uid = $1 AND idoperacion = $2`
	rows, err = database.Get().Select(ctx, consulta, uid, idOperacion)
	if err != nil {
		utils.VerError(err.Error())
		writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
			Estado: 19,
			Msg:    fmt.Sprintf("19: %s", config.DICRES[19]),
			Local:  map[string]any{},
		})
		return
	}

	if len(rows) > 0 {
		// Pago encontrado en tabla pago_en_proceso
		p := rows[0]

		// Mapear estado (mismo algoritmo que pagos)
		estadoRaw := p["estado"]
		hestad := -1
		if estadoRaw != nil {
			estadoInt := toInt(estadoRaw)
			if mapped, ok := convierte[estadoInt]; ok {
				hestad = mapped
			} else {
				hestad = estadoInt
			}
		}

		// Construir respuesta Local (replica exacta de estado.py, pago_en_proceso)
		// Nota: Bank = -1 si banco es None (diferente de pagos donde usa 0)
		local := map[string]any{
			"OrderId":    orEmpty(p["orderid"]),
			"Status":     hestad,
			"Resultmsg":  orEmptyStr(p["msg"]),
			"Success":    hestad == 3,
			"TmId":       orEmpty(p["tmid"]),
			"ExternalId": externalid,
			"BankId":     orEmpty(p["bankid"]),
			"Bank":       orMinusOne(p["banco"]),
			"Phone":      orEmpty(p["celular"]),
		}

		estado := -1
		if estadoRaw != nil {
			estado = toInt(estadoRaw)
		}

		dicerror := utils.GetErrorList()
		writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
			Msg:    fmt.Sprintf("%d: %s", estado, dicMsg(dicerror, estado)),
			Local:  local,
			Estado: estado,
		})
		return
	}

	// No encontrado en ninguna tabla (replica: else final de estado.py)
	local := map[string]any{
		"OrderId":    -1,
		"Status":     -1,
		"Resultmsg":  "No encontrado",
		"Success":    false,
		"TmId":       -1,
		"ExternalId": externalid,
		"BankId":     "",
		"Bank":       0,
		"Phone":      "",
	}
	estado := -1

	utils.Traza(fmt.Sprintf("No encontrado: %s", externalid), idTraza)

	dicerror := utils.GetErrorList()
	writeJSON(w, http.StatusOK, models.EstadoOrdenLocalResponse{
		Msg:    fmt.Sprintf("%d: %s", estado, dicMsg(dicerror, estado)),
		Local:  local,
		Estado: estado,
	})
}

// =============================================================================
// EstadoDevolucionHandler — GET /estadodevolucion/{externalid}/{source}/{tmid}/
// Replica estado.py :: estado_devolucion
// =============================================================================

// EstadoDevolucionHandler verifica el estado de la solicitud de devolucion en ETECSA.
func EstadoDevolucionHandler(w http.ResponseWriter, r *http.Request) {
	externalid := r.PathValue("externalid")
	source := r.PathValue("source")
	tmid := r.PathValue("tmid")

	// Construccion de la URL de ETECSA (replica: f"{settings.estadodevol}{externalid}/{source}/{tmid}/")
	direccionEtecsa := fmt.Sprintf("%s%s/%s/%s/", config.Cfg.EstadoDevol, externalid, source, tmid)

	// Headers de autenticacion para ETECSA
	encab := utils.PreparaConexion(config.Cfg.SemillaAuth, "", source, time.Now())

	// Llamada GET a ETECSA
	respData, err := getJSON(r.Context(), direccionEtecsa, encab)
	if err != nil {
		utils.VerError(err.Error())
		writeJSON(w, http.StatusOK, models.EstadoDevolucionResponse{
			Estado: 7,
			Msg:    config.DICRES[7],
			ETECSA: nil,
		})
		return
	}

	// Buscar 'getStatusRefundOrderResult' en la respuesta (replica exacta de estado.py)
	if refundResult, ok := respData["getStatusRefundOrderResult"]; ok {
		if refundMap, ok := refundResult.(map[string]any); ok {
			// Extraer Success (default False si no existe — replica: .get('Success', False))
			success := false
			if s, ok := refundMap["Success"]; ok {
				if sb, ok := s.(bool); ok {
					success = sb
				}
			}

			estado := 26
			if !success {
				estado = 27
			}

			writeJSON(w, http.StatusOK, models.EstadoDevolucionResponse{
				ETECSA: refundMap,
				Msg:    fmt.Sprintf("%d: %s", estado, config.DICRES[estado]),
				Estado: estado,
			})
			return
		}
	}

	// Key 'getStatusRefundOrderResult' no encontrada o tipo inesperado
	writeJSON(w, http.StatusOK, models.EstadoDevolucionResponse{
		Estado: 7,
		Msg:    config.DICRES[7],
		ETECSA: nil,
	})
}
