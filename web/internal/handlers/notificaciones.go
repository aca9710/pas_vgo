package handlers

import (
	"net/http"
	"strconv"
	"time"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// ListarNotificaciones muestra las notificaciones de pago con paginación.
// Equivalente a listar_notificaciones() de pagos_admin.py.
func ListarNotificaciones(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: page desde query string (default 1), limit fijo de 20
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	limit := 20
	offset := (page - 1) * limit

	notificaciones := []map[string]any{}
	total := 0

	// Total de notificaciones de pago
	if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM notificacion_pago"); err != nil {
		utils.LogError("listar_notificaciones")
	} else {
		total = utils.ToInt(val)
	}

	// Página actual de notificaciones
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT * FROM notificacion_pago
		ORDER BY fecha DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		utils.LogError("listar_notificaciones")
	} else {
		notificaciones = rows
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/notificaciones_list.html", map[string]any{
		"Title":          "Notificaciones de Pago",
		"ActivePage":     "notificaciones",
		"Admin":          admin,
		"Notificaciones": notificaciones,
		"Tipo":           "pago",
		"Total":          total,
		"Page":           page,
		"TotalPages":     totalPages,
		"Now":            time.Now(),
	})
}

// ListarNotificacionesDevolucion muestra las notificaciones de devolución con paginación.
// Equivalente a listar_notificaciones_devolucion() de pagos_admin.py.
func ListarNotificacionesDevolucion(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: page desde query string (default 1), limit fijo de 20
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	limit := 20
	offset := (page - 1) * limit

	notificaciones := []map[string]any{}
	total := 0

	// Total de notificaciones de devolución
	if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM notificacion_devolucion"); err != nil {
		utils.LogError("listar_notificaciones_devolucion")
	} else {
		total = utils.ToInt(val)
	}

	// Página actual de notificaciones de devolución
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT * FROM notificacion_devolucion
		ORDER BY fecha DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		utils.LogError("listar_notificaciones_devolucion")
	} else {
		notificaciones = rows
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/notificaciones_list.html", map[string]any{
		"Title":          "Notificaciones de Devolución",
		"ActivePage":     "notificaciones_devolucion",
		"Admin":          admin,
		"Notificaciones": notificaciones,
		"Tipo":           "devolucion",
		"Total":          total,
		"Page":           page,
		"TotalPages":     totalPages,
		"Now":            time.Now(),
	})
}
