package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/session"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// PortalLoginPage muestra el formulario de login del portal cliente.
// Equivalente a login_page de cliente.py.
func PortalLoginPage(w http.ResponseWriter, r *http.Request) {
	views.RenderRaw(w, "pages/portal/login.html", map[string]any{
		"Title": "Portal Cliente",
		"Error": r.URL.Query().Get("error"),
	})
}

// PortalLoginPost procesa el login del portal cliente.
// Equivalente a login_post de cliente.py.
func PortalLoginPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	// Buscar cliente activo por email
	cliente, err := db.Instance.FetchOne(r.Context(), `
		SELECT * FROM cliente
		WHERE email = $1 AND activo = TRUE
	`, email)
	if err != nil {
		utils.LogError("portal_login_post")
		views.RenderRaw(w, "pages/portal/login.html", map[string]any{
			"Title": "Portal Cliente",
			"Error": "Error al iniciar sesión",
		})
		return
	}

	// Verificar credenciales
	if cliente == nil || utils.HashPassword(password) != utils.ToString(cliente["password_hash"]) {
		views.RenderRaw(w, "pages/portal/login.html", map[string]any{
			"Title": "Portal Cliente",
			"Error": "Email o contraseña incorrectos",
		})
		return
	}

	// Crear sesión de cliente con expiración de 7 días
	clienteID := utils.ToInt64(cliente["id"])
	sessionID, err := session.CreateClienteSession(r.Context(), clienteID, 7*24*time.Hour)
	if err != nil {
		utils.LogError("portal_login_post_create_session")
		views.RenderRaw(w, "pages/portal/login.html", map[string]any{
			"Title": "Portal Cliente",
			"Error": "Error al iniciar sesión",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sesion_cliente",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   604800,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/portal/", http.StatusSeeOther)
}

// PortalLogout cierra la sesión del portal cliente.
// Equivalente a logout de cliente.py.
func PortalLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("sesion_cliente"); err == nil {
		if err := session.DeleteClienteSession(r.Context(), cookie.Value); err != nil {
			utils.LogError("portal_logout")
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "sesion_cliente",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
}

// PortalDashboard muestra el dashboard del portal cliente con pagos y
// devoluciones recientes. Equivalente a portal_dashboard de cliente.py.
func PortalDashboard(w http.ResponseWriter, r *http.Request) {
	cliente := middleware.ClienteFromContext(r)
	if cliente == nil {
		http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
		return
	}

	// Pagos recientes del cliente
	pagos := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT * FROM pagos
		WHERE cliente = $1
		ORDER BY fecha DESC
		LIMIT 20
	`, cliente.ClienteEmail)
	if err != nil {
		utils.LogError("portal_dashboard_pagos")
	} else {
		pagos = rows
	}

	// Devoluciones recientes del cliente
	devoluciones := []map[string]any{}
	rows, err = db.Instance.Fetch(r.Context(), `
		SELECT * FROM devolucion
		WHERE cliente = $1
		ORDER BY fecha DESC
		LIMIT 20
	`, cliente.ClienteEmail)
	if err != nil {
		utils.LogError("portal_dashboard_devoluciones")
	} else {
		devoluciones = rows
	}

	views.Render(w, r, "layouts/portal.html", "pages/portal/dashboard.html", map[string]any{
		"Title":        "Mi Portal",
		"ActivePage":   "dashboard",
		"Cliente":      cliente,
		"Pagos":        pagos,
		"Devoluciones": devoluciones,
		"Now":          time.Now(),
	})
}

// PortalPagos lista los pagos del cliente.
// Equivalente a lista_pagos de cliente.py.
func PortalPagos(w http.ResponseWriter, r *http.Request) {
	cliente := middleware.ClienteFromContext(r)
	if cliente == nil {
		http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
		return
	}

	pagos := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT * FROM pagos
		WHERE cliente = $1
		ORDER BY fecha DESC
		LIMIT 50
	`, cliente.ClienteEmail)
	if err != nil {
		utils.LogError("portal_lista_pagos")
	} else {
		pagos = rows
	}

	views.Render(w, r, "layouts/portal.html", "pages/portal/pagos_list.html", map[string]any{
		"Title":      "Mis Pagos",
		"ActivePage": "pagos",
		"Cliente":    cliente,
		"Pagos":      pagos,
		"Now":        time.Now(),
	})
}

// PortalPagoDetalle muestra el detalle de un pago del cliente.
// Equivalente a detalle_pago de cliente.py.
func PortalPagoDetalle(w http.ResponseWriter, r *http.Request) {
	cliente := middleware.ClienteFromContext(r)
	if cliente == nil {
		http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
		return
	}

	pagoID, err := strconv.ParseInt(chi.URLParam(r, "pagoID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pago, err := db.Instance.FetchOne(r.Context(), `
		SELECT p.*, u.url as url_texto
		FROM pagos p
		LEFT JOIN url u ON p.url = u.id
		WHERE p.id = $1
	`, pagoID)
	if err != nil {
		utils.LogError("portal_detalle_pago")
		http.Error(w, "Error al cargar pago", http.StatusInternalServerError)
		return
	}

	// Verificar que el pago pertenece al cliente
	if pago == nil || utils.ToString(pago["cliente"]) != cliente.ClienteEmail {
		http.NotFound(w, r)
		return
	}

	views.Render(w, r, "layouts/portal.html", "pages/portal/pago_detail.html", map[string]any{
		"Title":      "Detalle de Pago",
		"ActivePage": "pagos",
		"Cliente":    cliente,
		"Pago":       pago,
		"Now":        time.Now(),
	})
}

// PortalDevoluciones lista las devoluciones del cliente.
// Equivalente a lista_devoluciones de cliente.py.
func PortalDevoluciones(w http.ResponseWriter, r *http.Request) {
	cliente := middleware.ClienteFromContext(r)
	if cliente == nil {
		http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
		return
	}

	devoluciones := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT * FROM devolucion
		WHERE cliente = $1
		ORDER BY fecha DESC
		LIMIT 50
	`, cliente.ClienteEmail)
	if err != nil {
		utils.LogError("portal_lista_devoluciones")
	} else {
		devoluciones = rows
	}

	views.Render(w, r, "layouts/portal.html", "pages/portal/devoluciones_list.html", map[string]any{
		"Title":        "Mis Devoluciones",
		"ActivePage":   "devoluciones",
		"Cliente":      cliente,
		"Devoluciones": devoluciones,
		"Now":          time.Now(),
	})
}

// PortalDevolucionDetalle muestra el detalle de una devolución del cliente.
// Equivalente a detalle_devolucion de cliente.py.
func PortalDevolucionDetalle(w http.ResponseWriter, r *http.Request) {
	cliente := middleware.ClienteFromContext(r)
	if cliente == nil {
		http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
		return
	}

	devolucionID, err := strconv.ParseInt(chi.URLParam(r, "devolucionID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	devolucion, err := db.Instance.FetchOne(r.Context(), `
		SELECT * FROM devolucion
		WHERE id = $1
	`, devolucionID)
	if err != nil {
		utils.LogError("portal_detalle_devolucion")
		http.Error(w, "Error al cargar devolución", http.StatusInternalServerError)
		return
	}

	// Verificar que la devolución pertenece al cliente
	if devolucion == nil || utils.ToString(devolucion["cliente"]) != cliente.ClienteEmail {
		http.NotFound(w, r)
		return
	}

	views.Render(w, r, "layouts/portal.html", "pages/portal/devolucion_detail.html", map[string]any{
		"Title":      "Detalle de Devolución",
		"ActivePage": "devoluciones",
		"Cliente":    cliente,
		"Devolucion": devolucion,
		"Now":        time.Now(),
	})
}