package handlers

import (
	"net/http"
	"time"

	"webgo/internal/db"
	"webgo/internal/session"
	"webgo/internal/utils"
	"webgo/internal/views"
)

func GetOutput(w http.ResponseWriter, r *http.Request) {
	
	views.RenderRaw(w, "layouts/login.html", map[string]any{
		"Title": "Login Admin",
		"Error": r.URL.Query().Get("error"),
	})
}

// LoginPage muestra el formulario de login admin.
func LoginPage(w http.ResponseWriter, r *http.Request) {
	views.RenderRaw(w, "layouts/login.html", map[string]any{
		"Title": "Login Admin",
		"Error": r.URL.Query().Get("error"),
	})
}

// LoginPost procesa el login admin (equivalente a login_post de Python).
func LoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	passwordHash := utils.HashPassword(password)

	usuario, err := db.Instance.FetchOne(r.Context(), `
		SELECT id, username, rol, activo
		FROM admin_usuario
		WHERE username = $1 AND password_hash = $2 AND activo = TRUE
	`, username, passwordHash)
	if err != nil {
		utils.LogError("login_post")
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}
	if usuario == nil {
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}

	usuarioID := utils.ToInt64(usuario["id"])
	sessionID, err := session.CreateAdminSession(r.Context(), usuarioID, 24*time.Hour)
	if err != nil {
		utils.LogError("login_post_create_session")
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// Logout cierra la sesión admin.
func Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil {
		_ = session.DeleteAdminSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}