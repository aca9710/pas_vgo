package middleware

import (
	"context"
	"net/http"

	"webgo/internal/db"
	"webgo/internal/models"
	"webgo/internal/utils"
)

type ctxKey string

const (
	adminKey   ctxKey = "admin"
	clienteKey ctxKey = "cliente"
)

// RequireAdmin exige sesión de admin válida; redirige a /login si no.
// Equivalente a get_current_admin + redirect de Python.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin := loadAdmin(r)
		if admin == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), adminKey, admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loadAdmin carga el admin desde la cookie admin_session (con tiendas asignadas).
func loadAdmin(r *http.Request) *models.SesionAdmin {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return nil
	}

	sesion, err := db.CollectOne[models.SesionAdmin](r.Context(), `
		SELECT s.id, s.usuario_id, s.created_at, s.expires_at,
		       u.username, u.rol, u.activo
		FROM admin_sesion s
		JOIN admin_usuario u ON s.usuario_id = u.id
		WHERE s.id = $1 AND s.expires_at > CURRENT_TIMESTAMP
	`, cookie.Value)
	if err != nil {
		utils.LogError("get_current_admin_sesion")
		return nil
	}
	if sesion == nil || !sesion.Activo {
		return nil
	}

	// Tiendas asignadas al usuario
	rows, err := db.Instance.Fetch(r.Context(),
		"SELECT tienda_uid FROM admin_usuario_tienda WHERE usuario_id = $1", sesion.UsuarioID)
	if err != nil {
		utils.LogError("get_current_admin_tiendas")
		sesion.Tiendas = []string{}
	} else {
		sesion.Tiendas = make([]string, 0, len(rows))
		for _, row := range rows {
			sesion.Tiendas = append(sesion.Tiendas, utils.ToString(row["tienda_uid"]))
		}
	}
	return sesion
}

// AdminFromContext obtiene el admin autenticado del contexto de la petición.
func AdminFromContext(r *http.Request) *models.SesionAdmin {
	admin, _ := r.Context().Value(adminKey).(*models.SesionAdmin)
	return admin
}

// RequireAdminRol exige sesión de admin con un rol específico (ej: "admin").
func RequireAdminRol(rol string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := AdminFromContext(r)
			if admin == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if admin.Rol != rol {
				http.Error(w, "Sin permisos", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCliente exige sesión de cliente válida; redirige a /portal/login si no.
func RequireCliente(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cliente := loadCliente(r)
		if cliente == nil {
			http.Redirect(w, r, "/portal/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), clienteKey, cliente)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loadCliente carga el cliente desde la cookie sesion_cliente.
func loadCliente(r *http.Request) *models.SesionCliente {
	cookie, err := r.Cookie("sesion_cliente")
	if err != nil {
		return nil
	}

	sesion, err := db.CollectOne[models.SesionCliente](r.Context(), `
		SELECT s.id, s.cliente_id, s.created_at, s.expires_at,
		       c.nombre as cliente_nombre, c.email as cliente_email
		FROM sesion_cliente s
		JOIN cliente c ON s.cliente_id = c.id
		WHERE s.id = $1 AND s.expires_at > NOW() AND c.activo = TRUE
	`, cookie.Value)
	if err != nil {
		utils.LogError("get_session_cliente")
		return nil
	}
	return sesion
}

// ClienteFromContext obtiene el cliente autenticado del contexto de la petición.
func ClienteFromContext(r *http.Request) *models.SesionCliente {
	cliente, _ := r.Context().Value(clienteKey).(*models.SesionCliente)
	return cliente
}