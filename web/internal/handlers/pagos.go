package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/models"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// ListarPagos muestra el listado de pagos con filtros y paginación.
// Equivalente a listar_pagos() de pagos_admin.py.
func ListarPagos(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	listarPagos(w, r, admin, "", "pagos", 20)
}

// ListarPagosProceso muestra los pagos en proceso (estado=15 - En Proceso).
// Equivalente a listar_pagos_proceso() de pagos_admin.py.
func ListarPagosProceso(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// Pagos en proceso: estado fijo 15, sin filtros de búsqueda
	listarPagos(w, r, admin, "15", "pagos_proceso", 100)
}

// listarPagos renderiza el listado de pagos. Si estadoFijo no es vacío,
// fuerza ese estado e ignora los filtros de búsqueda (modo "proceso").
func listarPagos(w http.ResponseWriter, r *http.Request, admin *models.SesionAdmin, estadoFijo, activePage string, defLimit int) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", defLimit)
	offset := (page - 1) * limit

	// Construir condiciones dinámicas con placeholders $1, $2, ...
	conditions := []string{}
	args := []any{}
	queryParams := []string{}

	if estadoFijo != "" {
		// Modo "proceso": estado fijo, sin filtros de búsqueda
		if n, err := strconv.Atoi(estadoFijo); err == nil {
			args = append(args, n)
		} else {
			args = append(args, estadoFijo)
		}
		conditions = append(conditions, fmt.Sprintf("estado = $%d", len(args)))
	} else {
		q := r.URL.Query()
		idoperacion := q.Get("idoperacion")
		cliente := q.Get("cliente")
		estado := q.Get("estado")
		fechaDesde := q.Get("fecha_desde")
		fechaHasta := q.Get("fecha_hasta")

		if idoperacion != "" {
			args = append(args, idoperacion)
			conditions = append(conditions, fmt.Sprintf("idoperacion = $%d", len(args)))
			queryParams = append(queryParams, "idoperacion="+idoperacion)
		}
		if cliente != "" {
			args = append(args, cliente)
			conditions = append(conditions, fmt.Sprintf("cliente = $%d", len(args)))
			queryParams = append(queryParams, "cliente="+cliente)
		}
		if estado != "" {
			if n, err := strconv.Atoi(estado); err == nil {
				args = append(args, n)
			} else {
				args = append(args, estado)
			}
			conditions = append(conditions, fmt.Sprintf("estado = $%d", len(args)))
			queryParams = append(queryParams, "estado="+estado)
		}
		if fechaDesde != "" {
			args = append(args, fechaDesde)
			conditions = append(conditions, fmt.Sprintf("fecha >= $%d", len(args)))
			queryParams = append(queryParams, "fecha_desde="+fechaDesde)
		}
		if fechaHasta != "" {
			args = append(args, fechaHasta)
			conditions = append(conditions, fmt.Sprintf("fecha <= $%d", len(args)))
			queryParams = append(queryParams, "fecha_hasta="+fechaHasta)
		}
	}

	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	// Filtro por tiendas asignadas (solo usuarios no admin con tiendas)
	tiendaFilterCount := ""
	tiendaFilterPagos := ""
	if admin.Rol != "admin" && len(admin.Tiendas) > 0 {
		placeholders := make([]string, len(admin.Tiendas))
		for i, tienda := range admin.Tiendas {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
			args = append(args, tienda)
		}
		inClause := strings.Join(placeholders, ",")
		tiendaFilterCount = fmt.Sprintf(" AND uid IN (%s)", inClause)
		tiendaFilterPagos = fmt.Sprintf(" AND p.uid IN (%s)", inClause)
	}

	// Total de pagos (para paginación)
	total := int64(0)
	if val, err := db.Instance.FetchVal(r.Context(),
		fmt.Sprintf("SELECT COUNT(*) FROM pagos WHERE %s%s", where, tiendaFilterCount), args...); err != nil {
		utils.LogError("listar_pagos_count")
	} else {
		total = utils.ToInt64(val)
	}

	// Página de pagos con el texto de la URL asociada
	pagos := []map[string]any{}
	selectArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.Instance.Fetch(r.Context(), fmt.Sprintf(`
		SELECT p.*, u.url as url_texto
		FROM pagos p
		LEFT JOIN url u ON p.url = u.id
		WHERE %s%s
		ORDER BY p.fecha DESC
		LIMIT $%d OFFSET $%d`, where, tiendaFilterPagos, len(args)+1, len(args)+2), selectArgs...)
	if err != nil {
		utils.LogError("listar_pagos_query")
	} else {
		pagos = rows
	}

	totalPages := (int(total) + limit - 1) / limit

	queryString := ""
	if len(queryParams) > 0 {
		queryString = "&" + strings.Join(queryParams, "&")
	}

	filters := map[string]any{
		"idoperacion": r.URL.Query().Get("idoperacion"),
		"cliente":     r.URL.Query().Get("cliente"),
		"estado":      r.URL.Query().Get("estado"),
		"fecha_desde": r.URL.Query().Get("fecha_desde"),
		"fecha_hasta": r.URL.Query().Get("fecha_hasta"),
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/pagos_list.html", map[string]any{
		"Title":       "Gestión de Pagos",
		"ActivePage":  activePage,
		"Admin":       admin,
		"Pagos":       pagos,
		"Total":       total,
		"Page":        page,
		"TotalPages":  totalPages,
		"QueryString": queryString,
		"Filters":     filters,
		"Now":         time.Now(),
	})
}

// DetallePago muestra el detalle de un pago.
// Equivalente a detalle_pago() de pagos_admin.py.
func DetallePago(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pagoID := chi.URLParam(r, "pagoID")
	pagoIDN, err := strconv.ParseInt(pagoID, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pago, err := db.Instance.FetchOne(r.Context(), `
		SELECT p.*, u.url as url_texto
		FROM pagos p
		LEFT JOIN url u ON p.url = u.id
		WHERE p.id = $1
	`, pagoIDN)
	if err != nil {
		utils.LogError("detalle_pago")
		http.Error(w, "Error al consultar el pago", http.StatusInternalServerError)
		return
	}
	if pago == nil {
		http.NotFound(w, r)
		return
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/pagos_edit.html", map[string]any{
		"Title":      "Detalle de Pago",
		"ActivePage": "pagos",
		"Admin":      admin,
		"Pago":       pago,
		"Now":        time.Now(),
	})
}

// EditarPagoForm muestra el formulario para editar un pago.
// Equivalente a editar_pago_form() de pagos_admin.py.
func EditarPagoForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pagoID := chi.URLParam(r, "pagoID")
	pagoIDN, err := strconv.ParseInt(pagoID, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pago, err := db.Instance.FetchOne(r.Context(), `
		SELECT p.*, u.url as url_texto
		FROM pagos p
		LEFT JOIN url u ON p.url = u.id
		WHERE p.id = $1
	`, pagoIDN)
	if err != nil {
		utils.LogError("editar_pago_form_buscar")
		http.Error(w, "Error al consultar el pago", http.StatusInternalServerError)
		return
	}
	if pago == nil {
		http.NotFound(w, r)
		return
	}

	// Listas de URLs y mensajes disponibles para el formulario
	urls := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), "SELECT id, url FROM url ORDER BY id")
	if err != nil {
		utils.LogError("editar_pago_form_urls")
	} else {
		urls = rows
	}

	msgs := []map[string]any{}
	rows, err = db.Instance.Fetch(r.Context(), "SELECT id, texto FROM msg ORDER BY id")
	if err != nil {
		utils.LogError("editar_pago_form_mensajes")
	} else {
		msgs = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/pagos_edit.html", map[string]any{
		"Title":      "Editar Pago",
		"ActivePage": "pagos",
		"Admin":      admin,
		"Pago":       pago,
		"Urls":       urls,
		"Msgs":       msgs,
		"Now":        time.Now(),
	})
}

// EditarPagoPost actualiza los datos de un pago.
// Equivalente a editar_pago_post() de pagos_admin.py.
func EditarPagoPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pagoID := chi.URLParam(r, "pagoID")
	pagoIDN, err := strconv.ParseInt(pagoID, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Estado es obligatorio en el formulario
	estado, err := strconv.Atoi(r.FormValue("estado"))
	if err != nil {
		http.Redirect(w, r, "/admin/pagos?error=1", http.StatusSeeOther)
		return
	}

	_, err = db.Instance.Exec(r.Context(), `
		UPDATE pagos
		SET estado = $1, orderid = $2, msg = $3, tmid = $4, bankid = $5, url = $6
		WHERE id = $7
	`, estado, nullableInt(r.FormValue("orderid")), nullableInt(r.FormValue("msg")),
		nullableStr(r.FormValue("tmid")), nullableStr(r.FormValue("bankid")),
		nullableInt(r.FormValue("url")), pagoIDN)
	if err != nil {
		utils.LogError("editar_pago_post")
		http.Redirect(w, r, "/admin/pagos?error=1", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/pagos", http.StatusSeeOther)
}

// ListarDevoluciones muestra el listado de devoluciones con paginación.
// Equivalente a listar_devoluciones() de pagos_admin.py.
func ListarDevoluciones(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	offset := (page - 1) * limit

	// Filtro por tiendas asignadas (solo usuarios no admin con tiendas)
	args := []any{}
	tiendaFilter := ""
	if admin.Rol != "admin" && len(admin.Tiendas) > 0 {
		placeholders := make([]string, len(admin.Tiendas))
		for i, tienda := range admin.Tiendas {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, tienda)
		}
		tiendaFilter = fmt.Sprintf(" AND uid IN (%s)", strings.Join(placeholders, ","))
	}

	where := "1=1" + tiendaFilter

	// Total de devoluciones (para paginación)
	total := int64(0)
	if val, err := db.Instance.FetchVal(r.Context(),
		fmt.Sprintf("SELECT COUNT(*) FROM devolucion WHERE %s", where), args...); err != nil {
		utils.LogError("listar_devoluciones_count")
	} else {
		total = utils.ToInt64(val)
	}

	// Listado de devoluciones
	devoluciones := []map[string]any{}
	selectArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.Instance.Fetch(r.Context(), fmt.Sprintf(`
		SELECT id, idoperacion, uid, cliente, importe, estado, fecha
		FROM devolucion
		WHERE %s
		ORDER BY fecha DESC
		LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2), selectArgs...)
	if err != nil {
		utils.LogError("listar_devoluciones")
	} else {
		devoluciones = rows
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/devoluciones_list.html", map[string]any{
		"Title":        "Listado de Devoluciones",
		"ActivePage":   "devoluciones",
		"Admin":        admin,
		"Devoluciones": devoluciones,
		"Total":        total,
		"Page":         page,
		"TotalPages":   totalPages,
		"Now":          time.Now(),
	})
}

// ListarDevolucionesProceso muestra las devoluciones en proceso
// (estado=50 - No Enviada, sobre la tabla pagos).
// Equivalente a listar_devoluciones_proceso() de pagos_admin.py.
func ListarDevolucionesProceso(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	offset := (page - 1) * limit

	// Filtro por tiendas asignadas (solo usuarios no admin con tiendas)
	args := []any{50}
	tiendaFilter := ""
	if admin.Rol != "admin" && len(admin.Tiendas) > 0 {
		placeholders := make([]string, len(admin.Tiendas))
		for i, tienda := range admin.Tiendas {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
			args = append(args, tienda)
		}
		tiendaFilter = fmt.Sprintf(" AND uid IN (%s)", strings.Join(placeholders, ","))
	}

	// Total de devoluciones en proceso (para paginación)
	total := int64(0)
	if val, err := db.Instance.FetchVal(r.Context(),
		fmt.Sprintf("SELECT COUNT(*) FROM pagos WHERE estado = $1%s", tiendaFilter), args...); err != nil {
		utils.LogError("listar_devoluciones_proceso_count")
	} else {
		total = utils.ToInt64(val)
	}

	// Listado de devoluciones en proceso (estado=50 sobre la tabla pagos)
	devoluciones := []map[string]any{}
	selectArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.Instance.Fetch(r.Context(), fmt.Sprintf(`
		SELECT id, idoperacion, uid, cliente, importe, estado, fecha
		FROM pagos
		WHERE estado = $1%s
		ORDER BY fecha DESC
		LIMIT $%d OFFSET $%d`, tiendaFilter, len(args)+1, len(args)+2), selectArgs...)
	if err != nil {
		utils.LogError("listar_devoluciones_proceso")
	} else {
		devoluciones = rows
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/devoluciones_list.html", map[string]any{
		"Title":        "Devoluciones en Proceso",
		"ActivePage":   "devoluciones_proceso",
		"Admin":        admin,
		"Devoluciones": devoluciones,
		"Total":        total,
		"Page":         page,
		"TotalPages":   totalPages,
		"Now":          time.Now(),
	})
}

// queryInt lee un parámetro de query como entero con valor por defecto.
func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// nullableInt convierte un valor de formulario a int64 o nil (para columnas NULL).
func nullableInt(s string) any {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return n
}

// nullableStr convierte un valor de formulario a string o nil (para columnas NULL).
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}