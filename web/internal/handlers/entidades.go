package handlers

import (
	"encoding/json"
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

// entidadCreateRequest es el cuerpo JSON para crear una entidad (API).
type entidadCreateRequest struct {
	Nombre  string `json:"nombre"`
	Llave   string `json:"llave"`
	AdminID *int64 `json:"admin_id"`
}

// entidadUpdateRequest es el cuerpo JSON para actualizar una entidad (API).
// Todos los campos son opcionales (punteros) para permitir actualización parcial.
type entidadUpdateRequest struct {
	Nombre  *string `json:"nombre"`
	Llave   *string `json:"llave"`
	AdminID *int64  `json:"admin_id"`
	Activo  *bool   `json:"activo"`
}

// escribirErrorEntidadJSON escribe una respuesta de error en JSON.
func escribirErrorEntidadJSON(w http.ResponseWriter, status int, mensaje string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": mensaje})
}

// ==================== VISTAS HTML ====================

// ListarEntidades muestra el listado paginado de entidades.
// Equivalente a listar_entidades() de entidades.py.
func ListarEntidades(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: page y limit desde query params (defaults 1 y 20)
	page := 1
	limit := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	offset := (page - 1) * limit

	entidades := []map[string]any{}
	total := int64(0)

	// Total de entidades para la paginación
	if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM entidad"); err != nil {
		utils.LogError("listar_entidades_total")
	} else {
		total = utils.ToInt64(val)
	}

	// Listado con JOIN a admin_usuario y subconsultas de tiendas/usuarios
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT e.id, e.nombre, e.llave, e.admin_id, e.activo, e.created_at,
		       u.username as admin_username,
		       (SELECT COUNT(*) FROM tiendas WHERE entidad_id = e.id) as num_tiendas,
		       (SELECT COUNT(*) FROM admin_usuario WHERE entidad_id = e.id) as num_usuarios
		FROM entidad e
		LEFT JOIN admin_usuario u ON e.admin_id = u.id
		ORDER BY e.nombre
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		utils.LogError("listar_entidades")
	} else {
		entidades = rows
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/entidades_list.html", map[string]any{
		"Title":      "Entidades",
		"ActivePage": "entidades",
		"Admin":      admin,
		"Entidades":  entidades,
		"Total":      int(total),
		"Page":       page,
		"TotalPages": totalPages,
		"Now":        time.Now(),
	})
}

// CrearEntidadForm muestra el formulario para crear una entidad.
// Equivalente a crear_entidad_form() de entidades.py.
func CrearEntidadForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Usuarios activos disponibles para ser admin de la entidad
	usuarios := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT id, username FROM admin_usuario
		WHERE activo = TRUE ORDER BY username
	`)
	if err != nil {
		utils.LogError("crear_entidad_form_usuarios")
	} else {
		usuarios = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/entidades_create.html", map[string]any{
		"Title":      "Crear Entidad",
		"ActivePage": "entidades",
		"Admin":      admin,
		"Usuarios":   usuarios,
		"Now":        time.Now(),
	})
}

// CrearEntidadPost procesa el alta de una entidad desde el formulario.
// Equivalente a crear_entidad_post() de entidades.py.
func CrearEntidadPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	nombre := r.FormValue("nombre")
	llave := r.FormValue("llave")

	// admin_id es opcional: si viene vacío o no numérico se guarda NULL
	var adminID any
	if v := r.FormValue("admin_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			adminID = n
		}
	}

	_, err := db.Instance.Exec(r.Context(), `
		INSERT INTO entidad (nombre, llave, admin_id)
		VALUES ($1, $2, $3)
	`, nombre, llave, adminID)
	if err != nil {
		utils.LogError("crear_entidad_post")
		http.Redirect(w, r, "/admin/entidades/crear?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/entidades", http.StatusSeeOther)
}

// EditarEntidadForm muestra el formulario para editar una entidad.
// Equivalente a editar_entidad_form() de entidades.py.
func EditarEntidadForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Buscar la entidad por id
	entidad, err := db.Instance.FetchOne(r.Context(), `
		SELECT id, nombre, llave, admin_id, activo
		FROM entidad WHERE id = $1
	`, entidadID)
	if err != nil {
		utils.LogError("editar_entidad_form_buscar")
		http.Error(w, "Error al buscar entidad", http.StatusInternalServerError)
		return
	}
	if entidad == nil {
		http.NotFound(w, r)
		return
	}

	// Usuarios activos disponibles para ser admin de la entidad
	usuarios := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT id, username FROM admin_usuario
		WHERE activo = TRUE ORDER BY username
	`)
	if err != nil {
		utils.LogError("editar_entidad_form_usuarios")
	} else {
		usuarios = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/entidades_edit.html", map[string]any{
		"Title":      "Editar Entidad",
		"ActivePage": "entidades",
		"Admin":      admin,
		"Entidad":    entidad,
		"Usuarios":   usuarios,
		"Now":        time.Now(),
	})
}

// EditarEntidadPost procesa la actualización de una entidad desde el formulario.
// Equivalente a editar_entidad_post() de entidades.py.
func EditarEntidadPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	nombre := r.FormValue("nombre")
	llave := r.FormValue("llave")
	activo := r.FormValue("activo") == "on"

	// admin_id es opcional: si viene vacío o no numérico se guarda NULL
	var adminID any
	if v := r.FormValue("admin_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			adminID = n
		}
	}

	_, err = db.Instance.Exec(r.Context(), `
		UPDATE entidad SET nombre = $1, llave = $2, admin_id = $3, activo = $4, updated_at = $5
		WHERE id = $6
	`, nombre, llave, adminID, activo, time.Now(), entidadID)
	if err != nil {
		utils.LogError("editar_entidad_post")
		http.Redirect(w, r, fmt.Sprintf("/admin/entidades/%d/editar?error=%s", entidadID, url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/entidades", http.StatusSeeOther)
}

// EliminarEntidad elimina una entidad (verificando que no tenga tiendas asociadas).
// Equivalente a eliminar_entidad() de entidades.py.
func EliminarEntidad(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Verificar si hay tiendas asociadas antes de eliminar
	val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM tiendas WHERE entidad_id = $1", entidadID)
	if err != nil {
		utils.LogError("eliminar_entidad_tiendas")
	} else if utils.ToInt64(val) > 0 {
		http.Redirect(w, r, "/admin/entidades?error="+url.QueryEscape("No se puede eliminar: hay tiendas asociadas"), http.StatusSeeOther)
		return
	}

	_, err = db.Instance.Exec(r.Context(), "DELETE FROM entidad WHERE id = $1", entidadID)
	if err != nil {
		utils.LogError("eliminar_entidad")
	}

	http.Redirect(w, r, "/admin/entidades", http.StatusSeeOther)
}

// ==================== API JSON ====================

// APIListarEntidades devuelve todas las entidades en JSON.
// Equivalente a api_listar_entidades() de entidades.py.
func APIListarEntidades(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT e.id, e.nombre, e.llave, e.admin_id, e.activo, e.created_at,
		       u.username as admin_username
		FROM entidad e
		LEFT JOIN admin_usuario u ON e.admin_id = u.id
		ORDER BY e.nombre
	`)
	if err != nil {
		utils.LogError("api_listar_entidades")
		rows = []map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

// APIObtenerEntidad devuelve una entidad con sus tiendas y usuarios en JSON.
// Equivalente a api_obtener_entidad() de entidades.py.
func APIObtenerEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		escribirErrorEntidadJSON(w, http.StatusNotFound, "Entidad no encontrada")
		return
	}

	entidad, err := db.Instance.FetchOne(r.Context(), `
		SELECT e.id, e.nombre, e.llave, e.admin_id, e.activo, e.created_at,
		       u.username as admin_username
		FROM entidad e
		LEFT JOIN admin_usuario u ON e.admin_id = u.id
		WHERE e.id = $1
	`, entidadID)
	if err != nil {
		utils.LogError("api_obtener_entidad")
		escribirErrorEntidadJSON(w, http.StatusInternalServerError, "Error al obtener entidad")
		return
	}
	if entidad == nil {
		escribirErrorEntidadJSON(w, http.StatusNotFound, "Entidad no encontrada")
		return
	}

	// Tiendas de la entidad
	tiendas, err := db.Instance.Fetch(r.Context(), `
		SELECT uid, nombre, activa
		FROM tiendas WHERE entidad_id = $1
		ORDER BY nombre
	`, entidadID)
	if err != nil {
		utils.LogError("api_obtener_entidad_tiendas")
		tiendas = []map[string]any{}
	}

	// Usuarios de la entidad
	usuarios, err := db.Instance.Fetch(r.Context(), `
		SELECT id, username, rol, activo
		FROM admin_usuario WHERE entidad_id = $1
		ORDER BY username
	`, entidadID)
	if err != nil {
		utils.LogError("api_obtener_entidad_usuarios")
		usuarios = []map[string]any{}
	}

	entidad["tiendas"] = tiendas
	entidad["usuarios"] = usuarios

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entidad)
}

// APICrearEntidad crea una entidad desde JSON y devuelve la entidad creada.
// Equivalente a api_crear_entidad() de entidades.py.
func APICrearEntidad(w http.ResponseWriter, r *http.Request) {
	var req entidadCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escribirErrorEntidadJSON(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	result, err := db.Instance.FetchOne(r.Context(), `
		INSERT INTO entidad (nombre, llave, admin_id)
		VALUES ($1, $2, $3)
		RETURNING id, nombre, llave, admin_id, activo, created_at
	`, req.Nombre, req.Llave, req.AdminID)
	if err != nil {
		utils.LogError("api_crear_entidad")
		escribirErrorEntidadJSON(w, http.StatusInternalServerError, "Error al crear entidad")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// APIActualizarEntidad actualiza parcialmente una entidad desde JSON.
// Equivalente a api_actualizar_entidad() de entidades.py.
func APIActualizarEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		escribirErrorEntidadJSON(w, http.StatusNotFound, "Entidad no encontrada")
		return
	}

	var req entidadUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escribirErrorEntidadJSON(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	// Construir dinámicamente la consulta con los campos presentes
	campos := []string{}
	valores := []any{}
	idx := 1

	if req.Nombre != nil {
		campos = append(campos, fmt.Sprintf("nombre = $%d", idx))
		valores = append(valores, *req.Nombre)
		idx++
	}
	if req.Llave != nil {
		campos = append(campos, fmt.Sprintf("llave = $%d", idx))
		valores = append(valores, *req.Llave)
		idx++
	}
	if req.AdminID != nil {
		campos = append(campos, fmt.Sprintf("admin_id = $%d", idx))
		valores = append(valores, *req.AdminID)
		idx++
	}
	if req.Activo != nil {
		campos = append(campos, fmt.Sprintf("activo = $%d", idx))
		valores = append(valores, *req.Activo)
		idx++
	}

	if len(campos) == 0 {
		escribirErrorEntidadJSON(w, http.StatusBadRequest, "No hay campos para actualizar")
		return
	}

	campos = append(campos, fmt.Sprintf("updated_at = $%d", idx))
	valores = append(valores, time.Now())
	idx++

	valores = append(valores, entidadID)

	query := fmt.Sprintf(`
		UPDATE entidad SET %s
		WHERE id = $%d
		RETURNING id, nombre, llave, admin_id, activo, created_at
	`, strings.Join(campos, ", "), idx)

	result, err := db.Instance.FetchOne(r.Context(), query, valores...)
	if err != nil {
		utils.LogError("api_actualizar_entidad")
		escribirErrorEntidadJSON(w, http.StatusInternalServerError, "Error al actualizar entidad")
		return
	}
	if result == nil {
		escribirErrorEntidadJSON(w, http.StatusNotFound, "Entidad no encontrada")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// APIEliminarEntidad elimina una entidad (verificando que no tenga tiendas).
// Equivalente a api_eliminar_entidad() de entidades.py.
func APIEliminarEntidad(w http.ResponseWriter, r *http.Request) {
	entidadID, err := strconv.ParseInt(chi.URLParam(r, "entidadID"), 10, 64)
	if err != nil {
		escribirErrorEntidadJSON(w, http.StatusNotFound, "Entidad no encontrada")
		return
	}

	// Verificar si hay tiendas asociadas antes de eliminar
	val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM tiendas WHERE entidad_id = $1", entidadID)
	if err != nil {
		utils.LogError("api_eliminar_entidad_tiendas")
		escribirErrorEntidadJSON(w, http.StatusInternalServerError, "Error al eliminar entidad")
		return
	}
	if utils.ToInt64(val) > 0 {
		escribirErrorEntidadJSON(w, http.StatusBadRequest, "No se puede eliminar: hay tiendas asociadas")
		return
	}

	_, err = db.Instance.Exec(r.Context(), "DELETE FROM entidad WHERE id = $1", entidadID)
	if err != nil {
		utils.LogError("api_eliminar_entidad")
		escribirErrorEntidadJSON(w, http.StatusInternalServerError, "Error al eliminar entidad")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}