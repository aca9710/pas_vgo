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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
	phone := p.Phone
	estado, _ := strconv.Atoi(p.Status)
	tmID := p.TmId
	bankID := p.BankId
	bank := p.Bank
	p.mu.Unlock()

	consulta := `INSERT INTO pagos (uid, idoperacion, externalid, cliente, importe, moneda, fecha, vigencia, url,
	                               celular, estado, tmid, bankid, banco)
	             VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7, $8, $9, $10, $11, $12, $13) RETURNING id`
	_, err := p.db.InsertReturning(ctx, consulta,
		uid, idoperacion, externalID, source, amount, currency,
		validTime, idurl, phone, estado, tmID, bankID, bank)
	return err
}

// =============================================================================
// AdminPagos — gestion de solicitudes y notificaciones
// =============================================================================

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
		a.notificaciones[req.ExternalId] = req
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
	a.mu.Unlock()

	for _, p := range vencidos {
		if err := p.Guarda(ctx); err != nil {
			utils.VerError("pagosadmin.Guarda: " + err.Error())
		}
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
		a.mu.Unlock()
	}

	a.mu.Lock()
	for ext, payload := range devolver {
		a.notificaciones[ext] = payload
	}
	a.mu.Unlock()
}