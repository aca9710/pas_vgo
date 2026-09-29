package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// ListarTiendas muestra el listado paginado de tiendas.
// Equivalente a listar_tiendas() de tiendas.py.
func ListarTiendas(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: page y limit desde query string (defaults 1 y 20)
	page := 1
	limit := 20
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := (page - 1) * limit

	// Filtro por tiendas asignadas si el rol no es admin (uid IN placeholders)
	where := ""
	var args []any
	if admin.Rol != "admin" && len(admin.Tiendas) > 0 {
		placeholders := make([]string, len(admin.Tiendas))
		args = make([]any, len(admin.Tiendas))
		for i, tienda := range admin.Tiendas {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = tienda
		}
		where = fmt.Sprintf(" WHERE uid IN (%s)", strings.Join(placeholders, ","))
	}

	// Total de tiendas (para la paginación)
	total := int64(0)
	if admin.Rol == "admin" || len(admin.Tiendas) > 0 {
		if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM tiendas"+where, args...); err != nil {
			utils.LogError("listar_tiendas_total")
		} else {
			total = utils.ToInt64(val)
		}
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Lista de tiendas
	tiendas := []map[string]any{}
	if admin.Rol == "admin" || len(admin.Tiendas) > 0 {
		query := "SELECT * FROM tiendas" + where +
			fmt.Sprintf(" ORDER BY nombre LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		queryArgs := append(append([]any{}, args...), limit, offset)
		rows, err := db.Instance.Fetch(r.Context(), query, queryArgs...)
		if err != nil {
			utils.LogError("listar_tiendas")
		} else {
			tiendas = rows
		}
	}

	// Entidades activas para el select del formulario de alta
	entidades := []map[string]any{}
	entRows, err := db.Instance.Fetch(r.Context(),
		"SELECT id, nombre FROM entidad WHERE activo = TRUE ORDER BY nombre")
	if err != nil {
		utils.LogError("listar_tiendas_entidades")
	} else {
		entidades = entRows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/tiendas_list.html", map[string]any{
		"Title":      "Tiendas",
		"ActivePage": "tiendas",
		"Admin":      admin,
		"Tiendas":    tiendas,
		"Entidades":  entidades,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"Now":        time.Now(),
	})
}

// CrearTienda procesa el alta de una nueva tienda.
// Equivalente a crear_tienda() de tiendas.py.
func CrearTienda(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Campos del formulario
	uid := r.FormValue("uid")
	nombre := r.FormValue("nombre")
	produccion := r.FormValue("produccion")
	desarrollo := r.FormValue("desarrollo")
	activa := r.FormValue("activa") == "on" || r.FormValue("activa") == "true" || r.FormValue("activa") == "1"
	// entidad_id es opcional: nil -> NULL en BD
	var entidadID any
	if v := r.FormValue("entidad_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			entidadID = id
		}
	}

	_, err := db.Instance.Exec(r.Context(), `
		INSERT INTO tiendas (uid, nombre, produccion, desarrollo, activa, entidad_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uid, nombre, produccion, desarrollo, activa, entidadID)
	if err != nil {
		utils.LogError("crear_tienda")
		http.Redirect(w, r, "/admin/tiendas?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/tiendas", http.StatusSeeOther)
}

// EditarTiendaForm muestra el formulario de edición de una tienda.
// Equivalente a editar_tienda_form() de tiendas.py.
func EditarTiendaForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	uid := chi.URLParam(r, "uid")

	// Buscar la tienda por uid
	tienda, err := db.Instance.FetchOne(r.Context(), "SELECT * FROM tiendas WHERE uid = $1", uid)
	if err != nil {
		utils.LogError("editar_tienda_form_buscar")
		http.Error(w, "Error al buscar tienda", http.StatusInternalServerError)
		return
	}
	if tienda == nil {
		http.NotFound(w, r)
		return
	}

	// Entidades activas para el select
	entidades := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(),
		"SELECT id, nombre FROM entidad WHERE activo = TRUE ORDER BY nombre")
	if err != nil {
		utils.LogError("editar_tienda_form_entidades")
	} else {
		entidades = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/tiendas_edit.html", map[string]any{
		"Title":      "Editar Tienda",
		"ActivePage": "tiendas",
		"Admin":      admin,
		"Tienda":     tienda,
		"Entidades":  entidades,
		"Now":        time.Now(),
	})
}

// EditarTiendaPost procesa la actualización de una tienda.
// Equivalente a editar_tienda() de tiendas.py.
func EditarTiendaPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	uid := chi.URLParam(r, "uid")
	nombre := r.FormValue("nombre")
	produccion := r.FormValue("produccion")
	desarrollo := r.FormValue("desarrollo")
	activa := r.FormValue("activa") == "on" || r.FormValue("activa") == "true" || r.FormValue("activa") == "1"
	// entidad_id es opcional: nil -> NULL en BD
	var entidadID any
	if v := r.FormValue("entidad_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			entidadID = id
		}
	}

	_, err := db.Instance.Exec(r.Context(), `
		UPDATE tiendas
		SET nombre = $1, produccion = $2, desarrollo = $3, activa = $4,
		    entidad_id = $5, updated_at = CURRENT_TIMESTAMP
		WHERE uid = $6
	`, nombre, produccion, desarrollo, activa, entidadID, uid)
	if err != nil {
		utils.LogError("editar_tienda")
		http.Redirect(w, r, "/admin/tiendas/"+uid+"/editar?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/tiendas", http.StatusSeeOther)
}

// EliminarTienda elimina una tienda por uid.
// Equivalente a eliminar_tienda() de tiendas.py.
func EliminarTienda(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	uid := chi.URLParam(r, "uid")
	if _, err := db.Instance.Exec(r.Context(), "DELETE FROM tiendas WHERE uid = $1", uid); err != nil {
		utils.LogError("eliminar_tienda")
	}
	http.Redirect(w, r, "/admin/tiendas", http.StatusSeeOther)
}

// ListarTPV muestra el listado paginado de TPVs.
// Equivalente a listar_tpv() de tiendas.py.
func ListarTPV(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: page y limit desde query string (defaults 1 y 20)
	page := 1
	limit := 20
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := (page - 1) * limit

	// Total de TPVs (para la paginación)
	total := int64(0)
	if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM tpv"); err != nil {
		utils.LogError("listar_tpv_total")
	} else {
		total = utils.ToInt64(val)
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Lista de TPVs con el nombre de la tienda asociada
	tpvList := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT t.*, ti.nombre as tienda_nombre
		FROM tpv t
		LEFT JOIN tiendas ti ON t.uid = ti.uid
		ORDER BY t.id
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		utils.LogError("listar_tpv")
	} else {
		tpvList = rows
	}

	// Tiendas para el select del formulario de alta
	tiendas := []map[string]any{}
	rows, err = db.Instance.Fetch(r.Context(), "SELECT uid, nombre FROM tiendas ORDER BY nombre")
	if err != nil {
		utils.LogError("listar_tpv_tiendas")
	} else {
		tiendas = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/tpv_list.html", map[string]any{
		"Title":      "TPV",
		"ActivePage": "tpv",
		"Admin":      admin,
		"TPVs":       tpvList,
		"Tiendas":    tiendas,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"Now":        time.Now(),
	})
}

// CrearTPV procesa el alta de un nuevo TPV.
// Equivalente a crear_tpv() de tiendas.py.
func CrearTPV(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Campos del formulario
	uid := r.FormValue("uid")
	dirip := r.FormValue("dirip")
	identoptima := r.FormValue("identoptima")
	nombre := r.FormValue("nombre")
	activo := r.FormValue("activo") == "on" || r.FormValue("activo") == "true" || r.FormValue("activo") == "1"

	_, err := db.Instance.Exec(r.Context(), `
		INSERT INTO tpv (uid, dirip, identoptima, nombre, activo)
		VALUES ($1, $2, $3, $4, $5)
	`, uid, dirip, identoptima, nombre, activo)
	if err != nil {
		utils.LogError("crear_tpv")
		http.Redirect(w, r, "/admin/tpv?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/tpv", http.StatusSeeOther)
}

// EditarTPVForm muestra el formulario de edición de un TPV.
// Equivalente a editar_tpv_form() de tiendas.py.
func EditarTPVForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tpvIDStr := chi.URLParam(r, "tpvID")
	tpvID, err := strconv.ParseInt(tpvIDStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Buscar el TPV por id
	tpv, err := db.Instance.FetchOne(r.Context(), "SELECT * FROM tpv WHERE id = $1", tpvID)
	if err != nil {
		utils.LogError("editar_tpv_form_buscar")
		http.Error(w, "Error al buscar TPV", http.StatusInternalServerError)
		return
	}
	if tpv == nil {
		http.NotFound(w, r)
		return
	}

	// Tiendas para el select
	tiendas := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), "SELECT uid, nombre FROM tiendas ORDER BY nombre")
	if err != nil {
		utils.LogError("editar_tpv_form_tiendas")
	} else {
		tiendas = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/tpv_edit.html", map[string]any{
		"Title":      "Editar TPV",
		"ActivePage": "tpv",
		"Admin":      admin,
		"TPV":        tpv,
		"Tiendas":    tiendas,
		"Now":        time.Now(),
	})
}

// EditarTPVPost procesa la actualización de un TPV.
// Equivalente a editar_tpv_post() de tiendas.py.
func EditarTPVPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tpvIDStr := chi.URLParam(r, "tpvID")
	tpvID, err := strconv.ParseInt(tpvIDStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	uid := r.FormValue("uid")
	dirip := r.FormValue("dirip")
	identoptima := r.FormValue("identoptima")
	nombre := r.FormValue("nombre")
	activo := r.FormValue("activo") == "on" || r.FormValue("activo") == "true" || r.FormValue("activo") == "1"

	_, err = db.Instance.Exec(r.Context(), `
		UPDATE tpv
		SET uid = $1, dirip = $2, identoptima = $3, nombre = $4, activo = $5
		WHERE id = $6
	`, uid, dirip, identoptima, nombre, activo, tpvID)
	if err != nil {
		utils.LogError("editar_tpv")
		http.Redirect(w, r, "/admin/tpv/"+tpvIDStr+"/editar?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/tpv", http.StatusSeeOther)
}

// EliminarTPV elimina un TPV por id.
// Equivalente a eliminar_tpv() de tiendas.py.
func EliminarTPV(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tpvID, err := strconv.ParseInt(chi.URLParam(r, "tpvID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if _, err := db.Instance.Exec(r.Context(), "DELETE FROM tpv WHERE id = $1", tpvID); err != nil {
		utils.LogError("eliminar_tpv")
	}
	http.Redirect(w, r, "/admin/tpv", http.StatusSeeOther)
}

// ReportesTienda muestra el panel de reportes para la tienda logueada.
// Equivalente a reportes_tienda() de tiendas.py.
func ReportesTienda(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Obtener la primera tienda disponible
	tienda, err := db.Instance.FetchOne(r.Context(), "SELECT * FROM tiendas LIMIT 1")
	if err != nil {
		utils.LogError("reportes_tienda")
		views.Render(w, r, "layouts/admin.html", "pages/admin/tiendas_list.html", map[string]any{
			"Title":      "Reportes",
			"ActivePage": "reportes",
			"Admin":      admin,
			"Error":      "Error al cargar reportes",
			"Now":        time.Now(),
		})
		return
	}

	if tienda == nil {
		// Si no hay tiendas, mostrar error
		views.Render(w, r, "layouts/admin.html", "pages/admin/tiendas_list.html", map[string]any{
			"Title":      "Reportes",
			"ActivePage": "reportes",
			"Admin":      admin,
			"Error":      "No hay tiendas disponibles",
			"Now":        time.Now(),
		})
		return
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/tiendas_list.html", map[string]any{
		"Title":      "Reportes",
		"ActivePage": "reportes",
		"Admin":      admin,
		"Tienda":     tienda,
		"Now":        time.Now(),
	})
}