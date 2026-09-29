package main

import (
	"context"
	"log"
	"net/http"


	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"webgo/internal/db"
	"webgo/internal/handlers"
	"webgo/internal/middleware"
	"webgo/internal/views"
)

func main() {
	// 1. Conexión a BD
	if err := db.Connect(); err != nil {
		log.Fatalf("Error conectando a BD: %v", err)
	}
	defer db.Instance.Close()

	// 2. Migraciones (crear tablas + admin inicial)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("Error en migraciones: %v", err)
	}

	// 3. Templates
	if err := views.Init(); err != nil {
		log.Fatalf("Error cargando templates: %v", err)
	}

	// 4. Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// Archivos estáticos
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Root -> login
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	// Auth admin
	
	r.Get("/login", handlers.LoginPage)
	r.Post("/login", handlers.LoginPost)
	r.Get("/logout", handlers.Logout)

	// ==================== ADMIN ====================
	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.RequireAdmin)

		r.Get("/", handlers.Dashboard)

		// Pagos
		r.Get("/pagos", handlers.ListarPagos)
		r.Get("/pagos/proceso", handlers.ListarPagosProceso)
		r.Get("/pagos/{pagoID}/editar", handlers.EditarPagoForm)
		r.Post("/pagos/{pagoID}/editar", handlers.EditarPagoPost)
		r.Get("/pagos/{pagoID}", handlers.DetallePago)

		// Devoluciones
		r.Get("/devoluciones", handlers.ListarDevoluciones)
		r.Get("/devoluciones/proceso", handlers.ListarDevolucionesProceso)

		// Notificaciones
		r.Get("/notificaciones", handlers.ListarNotificaciones)
		r.Get("/notificaciones/devolucion", handlers.ListarNotificacionesDevolucion)

		// Tiendas
		r.Get("/tiendas", handlers.ListarTiendas)
		r.Post("/tiendas", handlers.CrearTienda)
		r.Get("/tiendas/{uid}/editar", handlers.EditarTiendaForm)
		r.Post("/tiendas/{uid}/editar", handlers.EditarTiendaPost)
		r.Delete("/tiendas/{uid}", handlers.EliminarTienda)

		// TPV
		r.Get("/tpv", handlers.ListarTPV)
		r.Post("/tpv", handlers.CrearTPV)
		r.Get("/tpv/{tpvID}/editar", handlers.EditarTPVForm)
		r.Post("/tpv/{tpvID}/editar", handlers.EditarTPVPost)
		r.Delete("/tpv/{tpvID}", handlers.EliminarTPV)

		// Reportes
		r.Get("/reportes", handlers.ReportesTienda)

		// Entidades (solo admin)
		r.Route("/entidades", func(r chi.Router) {
			r.Use(middleware.RequireAdminRol("admin"))
			r.Get("/", handlers.ListarEntidades)
			r.Get("/crear", handlers.CrearEntidadForm)
			r.Post("/crear", handlers.CrearEntidadPost)
			r.Get("/{entidadID}/editar", handlers.EditarEntidadForm)
			r.Post("/{entidadID}/editar", handlers.EditarEntidadPost)
			r.Get("/{entidadID}/eliminar", handlers.EliminarEntidad)
		})

		// API JSON entidades (solo admin)
		r.Route("/api/entidades", func(r chi.Router) {
			r.Use(middleware.RequireAdminRol("admin"))
			r.Get("/", handlers.APIListarEntidades)
			r.Get("/{entidadID}", handlers.APIObtenerEntidad)
			r.Post("/", handlers.APICrearEntidad)
			r.Put("/{entidadID}", handlers.APIActualizarEntidad)
			r.Delete("/{entidadID}", handlers.APIEliminarEntidad)
		})

		// Usuarios (solo admin)
		r.Route("/usuarios", func(r chi.Router) {
			r.Use(middleware.RequireAdminRol("admin"))
			r.Get("/", handlers.ListarUsuarios)
			r.Get("/crear", handlers.CrearUsuarioForm)
			r.Post("/crear", handlers.CrearUsuarioPost)
			r.Get("/{usuarioID}/editar", handlers.EditarUsuarioForm)
			r.Post("/{usuarioID}/editar", handlers.EditarUsuarioPost)
			r.Get("/{usuarioID}/eliminar", handlers.EliminarUsuario)
		})

		// Estadísticas
		r.Get("/estadisticas", handlers.Estadisticas)
		r.Get("/estadisticas/reporte_pdf", handlers.ReportePDF)
	})

	// ==================== PORTAL CLIENTE ====================
	r.Route("/portal", func(r chi.Router) {
		r.Get("/login", handlers.PortalLoginPage)
		r.Post("/login", handlers.PortalLoginPost)
		r.Post("/logout", handlers.PortalLogout)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireCliente)
			r.Get("/", handlers.PortalDashboard)
			r.Get("/pagos", handlers.PortalPagos)
			r.Get("/pagos/{pagoID}", handlers.PortalPagoDetalle)
			r.Get("/devoluciones", handlers.PortalDevoluciones)
			r.Get("/devoluciones/{devolucionID}", handlers.PortalDevolucionDetalle)
		})
	})

	// 5. Servidor
	port :=  "8060"
	
	log.Printf("🚀 Servidor en http://0.0.0.0:%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, r))
}