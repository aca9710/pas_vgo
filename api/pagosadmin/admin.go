// Package pagosadmin replica routers/pagos_admin.py: gestion en memoria de
// las solicitudes de pago (sol_pagos), notificaciones y el procesamiento
// periodico de pagos vencidos/notificados.
//
// NOTA: se corrigen dos bugs latentes del codigo Python:
//  1. guarda() pasaba 14 parametros a 13 placeholders (asyncpg fallaba
//     siempre, el pago nunca se guardaba). Aqui se pasan los 13 correctos.
//  2. num_orden() llamaba .get('activo', False) sobre un objeto DataPago
//     (AttributeError). Aqui se asigna directamente pago.OrderId.
package pagosadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"pasarela/config"
	"pasarela/database"
	"pasarela/models"
	"pasarela/utils"
)

// =============================================================================
// DataPago — solicitud de pago en memoria
// =============================================================================

type DataPago struct {
	mu sync.Mutex

	vence       time.Time
	db          *database.Database
	notificado  bool
	initialized bool

	ip          string
	uid         string
	idoperacion string
	ExternalId  string
	Source      string
	Amount      float64
	Currency    string
	fecha       time.Time
	ValidTime   string
	idurl       int
	idmsg       int
	Phone       string
	Description string
	Status      string // string como en Python (inicia "0", luego Status de la notificacion)
	TmId        string
	BankId      string
	Bank        string
	OrderId     int64
	intentos    int // intentos de Guarda fallidos (ver ProcesarLista)
}

// sumaIntento incrementa y devuelve el numero de intentos de Guarda.
func (p *DataPago) sumaIntento() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.intentos++
	return p.intentos
}

// NewDataPago replica DataPago.__init__.
func NewDataPago(origen string, req *models.SolicitudPagoRequest, db *database.Database) (*DataPago, error) {
	vt, err := strconv.Atoi(req.ValidTime)
	if err != nil {
		return nil, fmt.Errorf("ValidTime invalido: %w", err)
	}

	tmp := strings.Split(req.ExternalId, "-")
	idoperacion := strings.Join(tmp[1:], "-")

	return &DataPago{
		vence:       time.Now().Add(time.Duration(vt+2) * time.Second),
		db:          db,
		notificado:  false,
		initialized: false,
		ip:          origen,
		uid:         tmp[0],
		idoperacion: idoperacion,
		ExternalId:  req.ExternalId,
		Source:      req.Source,
		Amount:      req.Amount,
		Currency:    req.Currency,
		fecha:       time.Now(),
		ValidTime:   req.ValidTime,
		Phone:       req.Phone,
		Description: req.Description,
		Status:      "0",
		TmId:        "",
		BankId:      "",
		Bank:        "",
	}, nil
}

// initAsync replica DataPago._init_async: inicializacion perezosa de
// idurl/idmsg. DataPago no tiene UrlResponse, por lo que chk_url recibe ""
// (que se convierte en "------").
func (p *DataPago) initAsync(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return nil
	}
	idurl, err := utils.ChkURL(ctx, "", config.URLList, p.db)
	if err != nil {
		return err
	}
	idmsg, err := utils.ChkMsg(ctx, "Pago en proceso", config.MsgList, p.db)
	if err != nil {
		return err
	}
	p.idurl = idurl
	p.idmsg = idmsg
	p.initialized = true
	return nil
}

// Vencido replica DataPago.vencido: now >= vence.
func (p *DataPago) Vencido() bool {
	return !time.Now().Before(p.vence)
}

// Notifica replica DataPago.notifica.
func (p *DataPago) Notifica(n *models.NotificacionRequest) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ExternalId == n.ExternalId {
		p.Phone = n.Phone
		p.Status = n.Status
		p.TmId = n.TmId
		p.BankId = n.BankId
		p.Bank = n.Bank
		p.notificado = true
		return true
	}
	return false
}

// Notificado replica DataPago.notificado().
func (p *DataPago) Notificado() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.notificado
}

// Guarda replica DataPago.guarda con la correccion del numero de parametros:
// 13 placeholders, 13 parametros (el Python pasaba 14, incluyendo idmsg que
// no pertenece al INSERT). Ademas se convierten a int los campos que en la
// base de datos son integer (estado, vigencia).
//
// DESVIACION deliberada del Python: se anaden msg y orderid al INSERT.
//   - orderid: el Python solo lo asignaba en memoria (num_orden) y jamas lo
//     persistia, por lo que pagos.orderid se quedaba siempre en 0 (y el UPDATE
//     de num_orden no encontraba la fila porque el pago aun estaba en memoria).
//   - msg: sin esto la columna se queda en su valor por defecto (-1).
func (p *DataPago) Guarda(ctx context.Context) error {
	if err := p.initAsync(ctx); err != nil {
		return err
	}

	p.mu.Lock()
	uid := p.uid
	idoperacion := p.idoperacion
	externalID := p.ExternalId
	source := p.Source
	amount := p.Amount
	currency := p.Currency
	validTime, _ := strconv.Atoi(p.ValidTime)
	idurl := p.idurl
	idmsg := p.idmsg
	phone := p.Phone
	estado, _ := strconv.Atoi(p.Status)
	tmID := p.TmId
	bankID := p.BankId
	bank := p.Bank
	orderID := p.OrderId
	p.mu.Unlock()

	consulta := `INSERT INTO pagos (uid, idoperacion, externalid, cliente, importe, moneda, fecha, vigencia, url,
	                               celular, estado, tmid, bankid, banco, msg, orderid)
	             VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`
	_, err := p.db.InsertReturning(ctx, consulta,
		uid, idoperacion, externalID, source, amount, currency,
		validTime, idurl, phone, estado, tmID, bankID, bank, idmsg, orderID)
	return err
}

// =============================================================================
// AdminPagos — gestion de solicitudes y notificaciones
// =============================================================================

// maxIntentosGuardado es el numero de reintentos de Guarda antes de descartar
// el pago (un fallo transitorio de BD no puede perder un pago, pero tampoco
// puede mantenerlo en memoria para siempre).
const maxIntentosGuardado = 5

// esErrorPermanente indica si un error de PostgreSQL es de violacion de datos
// (SQLSTATE clase 23: FK, unique, check, not-null...) y por tanto no mejora
// reintentando. Un error transitorio (conexion, serializacion, timeout) no lo
// es: se reintenta.
func esErrorPermanente(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return len(pgErr.Code) == 5 && pgErr.Code[0] == '2' && pgErr.Code[1] == '3'
	}
	return false
}

type AdminPagos struct {
	mu sync.Mutex

	db             *database.Database
	nsalvaPagos    string
	nsalvaNotif    string
	solPagos       map[string]*DataPago
	notificaciones map[string]*models.NotificacionRequest
	notifPr        map[string]*models.NotificacionRequest
	notifjson      map[string]any

	LOCK       bool
	trazanotif []string
	trazapagos []string
	vence      time.Time

	// onGuardado se invoca con el ExternalId de cada pago persistido
	// correctamente, para que la capa de routers libere los marcadores de
	// Redis (pasarela:listaid y pasarela:por_notificar) y el ExternalId pueda
	// reutilizarse. Se inyecta con SetOnGuardado: las claves viven en el
	// paquete routers y no se quiere acoplar pagosadmin a redisclient.
	onGuardado func(externalid string)
}

// SetOnGuardado registra el callback que se dispara tras persistir un pago.
// Si fn es nil, ProcesarLista omite la notificacion.
func (a *AdminPagos) SetOnGuardado(fn func(externalid string)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onGuardado = fn
}

// New crea un AdminPagos (equivalente a AdminPagos(db)).
func New(db *database.Database) *AdminPagos {
	return &AdminPagos{
		db:             db,
		nsalvaPagos:    "pagos",
		nsalvaNotif:    "not_pagos",
		solPagos:       make(map[string]*DataPago),
		notificaciones: make(map[string]*models.NotificacionRequest),
		notifPr:        make(map[string]*models.NotificacionRequest),
		notifjson:      make(map[string]any),
		LOCK:           false,
		vence:          time.Now(),
	}
}

// chkLock replica AdminPagos.chklock (asume mu tomado).
func (a *AdminPagos) chkLock() {
	if a.vence.Before(time.Now()) {
		a.LOCK = false
	}
}

// lock replica la intencion de AdminPagos.lock: adquirir (enable=true) o
// liberar (enable=false) el LOCK con vencimiento de 120s. En el Python el
// LOCK nunca se adquiria (vence se inicializaba en el pasado y nunca se
// actualizaba); aqui se implementa el comportamiento previsto.
func (a *AdminPagos) lock(enable bool) {
	now := time.Now()
	if a.vence.Before(now) {
		a.LOCK = false
	}
	if enable {
		a.LOCK = true
		a.vence = now.Add(120 * time.Second)
	} else {
		a.LOCK = false
	}
}

// NumOrden replica AdminPagos.num_orden con la correccion del bug de Python
// (.get('activo') sobre DataPago): si el pago esta en memoria se asigna
// OrderId; si no, se actualiza la tabla pagos.
func (a *AdminPagos) NumOrden(ctx context.Context, orderid int64, externalid string) {
	a.mu.Lock()
	_, inMem := a.solPagos[externalid]
	a.mu.Unlock()

	if inMem {
		a.mu.Lock()
		if p, ok := a.solPagos[externalid]; ok {
			p.mu.Lock()
			p.OrderId = orderid
			p.mu.Unlock()
		}
		a.mu.Unlock()
		return
	}
	_, _ = a.db.Execute(ctx, "UPDATE public.pagos SET orderid = $1 WHERE externalid = $2", orderid, externalid)
}

// NotificaJSON replica AdminPagos.notifica_json: devuelve el JSON guardado o
// "{}" si no existe.
func (a *AdminPagos) NotificaJSON(externalid string) any {
	a.mu.Lock()
	defer a.mu.Unlock()
	if v, ok := a.notifjson[externalid]; ok {
		return v
	}
	return "{}"
}

// ExistePago replica AdminPagos.existe_pago.
func (a *AdminPagos) ExistePago(ctx context.Context, externalid string) (map[string]any, error) {
	a.mu.Lock()
	_, inMem := a.solPagos[externalid]
	a.mu.Unlock()

	if inMem {
		return map[string]any{
			"transaction_id": externalid,
			"status":         "0",
			"message":        "Transacción idempotente",
		}, nil
	}

	rows, err := a.db.Select(ctx, "SELECT estado FROM pagos WHERE externalid = $1", externalid)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return map[string]any{
			"transaction_id": externalid,
			"status":         rows[0]["estado"],
			"message":        "Transacción idempotente",
		}, nil
	}
	return nil, nil
}

// logLocked replica AdminPagos.log (asume mu tomado). Devuelve nil siempre,
// igual que el Python (que no tiene return).
func (a *AdminPagos) logLocked(tipo string, modelo *models.NotificacionRequest, ip string) any {
	data, _ := json.Marshal(modelo)
	if tipo == "p" {
		linea := fmt.Sprintf(`"ip": "%s", "data": %s`, ip, string(data))
		a.trazapagos = append(a.trazapagos, linea)
		if len(a.trazapagos) > 100 {
			tmp := a.trazapagos
			a.trazapagos = nil
			utils.Data(a.nsalvaPagos, tmp)
		}
		return nil
	}
	a.trazanotif = append(a.trazanotif, string(data))
	if len(a.trazanotif) > 100 {
		tmp := a.trazanotif
		a.trazanotif = nil
		utils.Data(a.nsalvaNotif, tmp)
	}
	return nil
}

// maxNotifPendientes acota notificaciones / notifPr / notifjson. En el Python
// estos tres dicts crecen sin limite y nunca se limpian: una notificacion de
// un ExternalId inexistente se re-encola para siempre (procesar_lista la
// devuelve a notificaciones si no encuentra el pago), de modo que un cliente
// puede agotar la memoria del proceso solo llamando a /notificapagos/.
// Aqui se acotan a maxNotifPendientes entradas: se descarta la mas antigua
// inserta (Go no ordena mapas, asi que el rango da una clave cualquiera; peor
// caso se pierde una notificacion huerfana, que ya no se iba a aplicar nunca).
const maxNotifPendientes = 1000

// acotarNotifLocked recorta un mapa de notificaciones a maxNotifPendientes
// entradas (asume mu tomado).
func acotarNotifLocked[T any](nombre string, m map[string]T) {
	for len(m) >= maxNotifPendientes {
		for k := range m {
			delete(m, k)
			utils.VerError(fmt.Sprintf("pagosadmin: mapa %s lleno (%d entradas), notificacion descartada", nombre, maxNotifPendientes))
			break
		}
	}
}

// limpiarNotifLocked borra la notificacion ya aplicada de los mapas
// auxiliares (asume mu tomado).
func (a *AdminPagos) limpiarNotifLocked(ext string) {
	delete(a.notifPr, ext)
	delete(a.notifjson, ext)
}

// LimpiarNotif borra de los mapas de notificaciones lo asociado a externalid.
// Se invoca cuando el pago ya quedo persistido: a partir de ahi la
// notificacion en memoria ya no se va a leer ni a aplicar, y sin esta limpieza
// notifjson/notifPr crecian para siempre (el Python tampoco los limpia, pero
// ahi el proceso era de vida corta).
func (a *AdminPagos) LimpiarNotif(externalid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.limpiarNotifLocked(externalid)
}

// Notifica replica AdminPagos.notifica.
func (a *AdminPagos) Notifica(ctx context.Context, req *models.NotificacionRequest) {
	a.mu.Lock()
	p, ok := a.solPagos[req.ExternalId]
	notificado := false
	if ok {
		notificado = p.Notifica(req)
		if len(a.solPagos) > 20 {
			a.chkLock()
			a.mu.Unlock()
			a.ProcesarLista(ctx)
			a.mu.Lock()
		}
	}
	if !notificado {
		acotarNotifLocked("notificaciones", a.notificaciones)
		a.notificaciones[req.ExternalId] = req
		acotarNotifLocked("notifjson", a.notifjson)
		a.notifjson[req.ExternalId] = a.logLocked("n", req, "")
	}
	a.mu.Unlock()
}

// Add replica AdminPagos.add.
func (a *AdminPagos) Add(origen string, req *models.SolicitudPagoRequest) error {
	pago, err := NewDataPago(origen, req, a.db)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.solPagos[req.ExternalId] = pago
	if len(a.solPagos) > 20 {
		a.chkLock()
	}
	a.mu.Unlock()
	return nil
}

// Vencimiento replica AdminPagos.vencimiento.
func (a *AdminPagos) Vencimiento(externalid string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.solPagos[externalid]; ok {
		return p.Vencido()
	}
	return true
}

// ProcesarLista replica AdminPagos.procesar_lista: guarda los pagos
// vencidos/notificados y aplica las notificaciones pendientes a la BD.
func (a *AdminPagos) ProcesarLista(ctx context.Context) {
	a.mu.Lock()
	if a.LOCK {
		a.mu.Unlock()
		return
	}
	a.lock(true)
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.lock(false)
		a.mu.Unlock()
	}()

	// 1) Pagos vencidos o notificados -> guardar en BD
	a.mu.Lock()
	var vencidos []*DataPago
	for ext, p := range a.solPagos {
		if p.Vencido() || p.Notificado() {
			vencidos = append(vencidos, p)
			delete(a.solPagos, ext)
		}
	}
	onGuardado := a.onGuardado
	a.mu.Unlock()

	// Un fallo de Guarda NO puede perder el pago: se reencola para el proximo
	// ciclo en lugar de descartarlo (antes solo se escribia el error y el pago
	// se perdia de forma permanente ante cualquier fallo de BD).
	// Los errores PERMANENTES (violacion de FK, unique, check, not-null...) no
	// se reintentan: un INSERT que viola una FK volveria a fallar en cada ciclo
	// para siempre (el bug original de idurl=0 lo hacia endlessly) y el pago
	// ocuparia memoria + I/O de log indefinidamente.
	var fallidos []*DataPago
	for _, p := range vencidos {
		if err := p.Guarda(ctx); err != nil {
			permanente := esErrorPermanente(err)
			n := p.sumaIntento()
			utils.VerError(fmt.Sprintf("pagosadmin.Guarda: %s (intento %d, permanente=%v)", err.Error(), n, permanente))
			if !permanente && n < maxIntentosGuardado {
				fallidos = append(fallidos, p)
			}
			continue
		}
		// Persistido: el ciclo del pago termino, se liberan los marcadores.
		if onGuardado != nil {
			onGuardado(p.ExternalId)
		}
	}

	if len(fallidos) > 0 {
		a.mu.Lock()
		for _, p := range fallidos {
			// No pisa una entrada mas reciente del mismo ExternalId.
			if _, existe := a.solPagos[p.ExternalId]; !existe {
				a.solPagos[p.ExternalId] = p
			}
		}
		a.mu.Unlock()
	}

	// 2) Notificaciones pendientes -> actualizar pagos
	a.mu.Lock()
	pendientes := make([]*models.NotificacionRequest, 0, len(a.notificaciones))
	for ext, payload := range a.notificaciones {
		pendientes = append(pendientes, payload)
		delete(a.notificaciones, ext)
	}
	a.mu.Unlock()

	devolver := make(map[string]*models.NotificacionRequest)
	for _, payload := range pendientes {
		ext := payload.ExternalId

		rows, err := a.db.Select(ctx, "SELECT id FROM public.pagos WHERE externalid = $1", ext)
		if err == nil && len(rows) > 0 {
			idmsg, err := utils.ChkMsg(ctx, payload.Msg, config.MsgList, a.db)
			if err == nil {
				estado, _ := strconv.Atoi(payload.Status)
				_, _ = a.db.Execute(ctx,
					`UPDATE pagos SET celular = $1, estado = $2,
					  tmid = $3, bankid = $4, banco = $5,
					  msg = $6 WHERE externalid = $7`,
					payload.Phone, estado, payload.TmId, payload.BankId, payload.Bank, idmsg, ext)
			}
		} else {
			devolver[ext] = payload
		}

		a.mu.Lock()
		a.notifPr[ext] = payload
		acotarNotifLocked("notifPr", a.notifPr)
		// Si el pago existe en BD y se actualizo, la notificacion en memoria
		// (notifjson/notifPr) ya cumplio su funcion: se limpia para no filtrar.
		if _, reencolar := devolver[ext]; !reencolar {
			a.limpiarNotifLocked(ext)
		}
		a.mu.Unlock()
	}

	a.mu.Lock()
	for ext, payload := range devolver {
		acotarNotifLocked("notificaciones", a.notificaciones)
		a.notificaciones[ext] = payload
	}
	a.mu.Unlock()
}