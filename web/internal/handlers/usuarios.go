package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// usuarioIDFromURL extrae y convierte el parámetro "usuarioID" de la URL a int64.
// Devuelve 0 si el parámetro falta o no es numérico.
func usuarioIDFromURL(r *http.Request) int64 {
	id := int64(0)
	if v := chi.URLParam(r, "usuarioID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			id = n
		}
	}
	return id
}

// ListarUsuarios muestra la lista paginada de usuarios admin.
// Equivalente a listar_usuarios() de admin_usuarios.py.
func ListarUsuarios(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Paginación: página actual (default 1) y 20 usuarios por página
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	limit := 20
	offset := (page - 1) * limit

	// Total de usuarios para calcular el número de páginas
	total := int64(0)
	if val, err := db.Instance.FetchVal(r.Context(), "SELECT COUNT(*) FROM admin_usuario"); err != nil {
		utils.LogError("listar_usuarios_total")
	} else {
		total = utils.ToInt64(val)
	}

	// Lista de usuarios con sus tiendas asignadas
	usuarios := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT id, username, rol, activo, created_at
		FROM admin_usuario ORDER BY id
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		utils.LogError("listar_usuarios")
	} else {
		usuarios = rows
		for _, u := range usuarios {
			tiendas, err := db.Instance.Fetch(r.Context(), `
				SELECT t.nombre, t.uid
				FROM admin_usuario_tienda ut
				JOIN tiendas t ON ut.tienda_uid = t.uid
				WHERE ut.usuario_id = $1
			`, u["id"])
			if err != nil {
				utils.LogError("listar_usuarios_tiendas")
				u["tiendas"] = []map[string]any{}
			} else {
				u["tiendas"] = tiendas
			}
		}
	}

	// Total de páginas (mínimo 1)
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/usuarios_list.html", map[string]any{
		"Title":      "Usuarios",
		"ActivePage": "usuarios",
		"Admin":      admin,
		"Usuarios":   usuarios,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"Now":        time.Now(),
	})
}

// CrearUsuarioForm muestra el formulario para crear un usuario admin.
// Equivalente a crear_usuario_form() de admin_usuarios.py.
func CrearUsuarioForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Tiendas disponibles para asignar
	tiendas := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), "SELECT uid, nombre FROM tiendas ORDER BY nombre")
	if err != nil {
		utils.LogError("crear_usuario_form_tiendas")
	} else {
		tiendas = rows
	}

	// Entidades activas
	entidades := []map[string]any{}
	rows, err = db.Instance.Fetch(r.Context(), "SELECT id, nombre FROM entidad WHERE activo = TRUE ORDER BY nombre")
	if err != nil {
		utils.LogError("crear_usuario_form_entidades")
	} else {
		entidades = rows
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/usuarios_create.html", map[string]any{
		"Title":      "Nuevo Usuario",
		"ActivePage": "usuarios",
		"Admin":      admin,
		"Tiendas":    tiendas,
		"Entidades":  entidades,
		"Now":        time.Now(),
	})
}

// CrearUsuarioPost procesa la creación de un usuario admin.
// Equivalente a crear_usuario_post() de admin_usuarios.py.
func CrearUsuarioPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		utils.LogError("crear_usuario_post_parse")
		http.Redirect(w, r, "/admin/usuarios/crear?error=1", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	rol := r.FormValue("rol")
	if rol == "" {
		rol = "user"
	}
	// Checkbox: presente con valor "on"/"true" cuando está marcado
	activo := r.FormValue("activo") == "on" || r.FormValue("activo") == "true"
	tiendas := r.Form["tiendas"]

	// entidad_id opcional: nil si vacío o no numérico
	var entidadID any
	if v := r.FormValue("entidad_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			entidadID = id
		}
	}

	// Insertar el usuario y obtener su id
	val, err := db.Instance.FetchVal(r.Context(), `
		INSERT INTO admin_usuario (username, password_hash, rol, activo, entidad_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, username, utils.HashPassword(password), rol, activo, entidadID)
	if err != nil {
		utils.LogError("crear_usuario_post")
		q := url.Values{}
		q.Set("error", err.Error())
		http.Redirect(w, r, "/admin/usuarios/crear?"+q.Encode(), http.StatusSeeOther)
		return
	}
	usuarioID := utils.ToInt64(val)

	// Asignar las tiendas seleccionadas al usuario
	for _, tiendaUID := range tiendas {
		if _, err := db.Instance.Exec(r.Context(), `
			INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
			VALUES ($1, $2)
		`, usuarioID, tiendaUID); err != nil {
			utils.LogError("crear_usuario_tienda")
		}
	}

	http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
}

// EditarUsuarioForm muestra el formulario para editar un usuario admin.
// Equivalente a editar_usuario_form() de admin_usuarios.py.
func EditarUsuarioForm(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	usuarioID := usuarioIDFromURL(r)

	// Buscar el usuario a editar
	usuario, err := db.Instance.FetchOne(r.Context(), `
		SELECT id, username, rol, activo, entidad_id
		FROM admin_usuario WHERE id = $1
	`, usuarioID)
	if err != nil {
		utils.LogError("editar_usuario_form_buscar")
		http.NotFound(w, r)
		return
	}
	if usuario == nil {
		http.NotFound(w, r)
		return
	}

	// Tiendas disponibles para asignar
	tiendas := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), "SELECT uid, nombre FROM tiendas ORDER BY nombre")
	if err != nil {
		utils.LogError("editar_usuario_form_tiendas")
	} else {
		tiendas = rows
	}

	// Entidades activas
	entidades := []map[string]any{}
	rows, err = db.Instance.Fetch(r.Context(), "SELECT id, nombre FROM entidad WHERE activo = TRUE ORDER BY nombre")
	if err != nil {
		utils.LogError("editar_usuario_form_entidades")
	} else {
		entidades = rows
	}

	// Tiendas ya asignadas al usuario
	tiendasAsignadas := []string{}
	rows, err = db.Instance.Fetch(r.Context(), `
		SELECT tienda_uid FROM admin_usuario_tienda WHERE usuario_id = $1
	`, usuarioID)
	if err != nil {
		utils.LogError("editar_usuario_form_tiendas_asignadas")
	} else {
		for _, row := range rows {
			tiendasAsignadas = append(tiendasAsignadas, utils.ToString(row["tienda_uid"]))
		}
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/usuarios_edit.html", map[string]any{
		"Title":            "Editar Usuario",
		"ActivePage":       "usuarios",
		"Admin":            admin,
		"Usuario":          usuario,
		"Tiendas":          tiendas,
		"Entidades":        entidades,
		"TiendasAsignadas": tiendasAsignadas,
		"Now":              time.Now(),
	})
}

// EditarUsuarioPost procesa la edición de un usuario admin.
// Equivalente a editar_usuario_post() de admin_usuarios.py.
func EditarUsuarioPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	usuarioID := usuarioIDFromURL(r)

	if err := r.ParseForm(); err != nil {
		utils.LogError("editar_usuario_post_parse")
		http.Redirect(w, r, "/admin/usuarios?error=1", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	rol := r.FormValue("rol")
	if rol == "" {
		rol = "user"
	}
	activo := r.FormValue("activo") == "on" || r.FormValue("activo") == "true"
	tiendas := r.Form["tiendas"]

	// entidad_id opcional: nil si vacío o no numérico
	var entidadID any
	if v := r.FormValue("entidad_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			entidadID = id
		}
	}

	// Actualizar datos del usuario (con o sin cambio de contraseña)
	if password != "" {
		if _, err := db.Instance.Exec(r.Context(), `
			UPDATE admin_usuario
			SET username = $1, password_hash = $2, rol = $3, activo = $4, entidad_id = $5
			WHERE id = $6
		`, username, utils.HashPassword(password), rol, activo, entidadID, usuarioID); err != nil {
			utils.LogError("editar_usuario_update_con_pass")
		}
	} else {
		if _, err := db.Instance.Exec(r.Context(), `
			UPDATE admin_usuario
			SET username = $1, rol = $2, activo = $3, entidad_id = $4
			WHERE id = $5
		`, username, rol, activo, entidadID, usuarioID); err != nil {
			utils.LogError("editar_usuario_update_sin_pass")
		}
	}

	// Reemplazar tiendas asignadas: borrar todas y re-insertar las seleccionadas
	if _, err := db.Instance.Exec(r.Context(), "DELETE FROM admin_usuario_tienda WHERE usuario_id = $1", usuarioID); err != nil {
		utils.LogError("editar_usuario_delete_tiendas")
	}
	for _, tiendaUID := range tiendas {
		if _, err := db.Instance.Exec(r.Context(), `
			INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
			VALUES ($1, $2)
		`, usuarioID, tiendaUID); err != nil {
			utils.LogError("editar_usuario_insert_tienda")
		}
	}

	http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
}

// EliminarUsuario elimina un usuario admin.
// Equivalente a eliminar_usuario() de admin_usuarios.py.
func EliminarUsuario(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	usuarioID := usuarioIDFromURL(r)

	// Evitar que un admin se elimine a sí mismo
	if usuarioID == admin.UsuarioID {
		http.Error(w, "No puedes eliminarte a ti mismo", http.StatusBadRequest)
		return
	}

	if _, err := db.Instance.Exec(r.Context(), "DELETE FROM admin_usuario WHERE id = $1", usuarioID); err != nil {
		utils.LogError("eliminar_usuario")
	}

	http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
}