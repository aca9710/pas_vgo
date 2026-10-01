// main.go — punto de entrada de la API REST en Go.
// Replica fiel de main.py (FastAPI): lifespan, middleware X-Process-Time,
// rutas y graceful shutdown.  Modulo: pasarela.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"pasarela/config"
	"pasarela/database"
	"pasarela/httpclient"
	"pasarela/pagosadmin"
	"pasarela/redisclient"
	"pasarela/routers"
	"pasarela/utils"
)

// =============================================================================
// Middleware: X-Process-Time
// =============================================================================

// responseWriter extiende http.ResponseWriter para capturar el primer
// Write/WriteHeader y setear el header X-Process-Time con la duracion de la
// peticion (equivalente al middleware http de FastAPI en main.py).
type responseWriter struct {
	http.ResponseWriter
	wroteHeader bool // true si ya se escribio la respuesta
	start       time.Time
}

// WriteHeader setea X-Process-Time en el primer llamado (y solo en ese).
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		elapsed := time.Since(rw.start).Seconds()
		rw.Header().Set("X-Process-Time", strconv.FormatFloat(elapsed, 'f', -1, 64))
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write setea X-Process-Time si aun no se escribio (como hace
// http.ResponseWriter: el primer Write implica WriteHeader(200)).
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap devuelve el ResponseWriter subyacente (para http.ResponseController).
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// middleware mide la duracion de cada peticion y la expone en el header
// X-Process-Time (replica @app.middleware("http") de main.py).
func middleware(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, start: time.Now()}
		mux.ServeHTTP(rw, r)
	})
}

// =============================================================================
// Ruta raiz
// =============================================================================

// rootHandler responde el JSON-encoded string "Pasarela de pagos v3.0"
// (FastAPI serializa str como JSON: body "Pasarela de pagos v3.0" con
// comillas y Content-Type application/json).
func rootHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`"Pasarela de pagos v3.0"`))
}

// =============================================================================
// MAIN
// =============================================================================

func main() {
	// 1) Cargar configuracion desde .env / variables de entorno.
	config.Load()

	// 2) Validar DSN de PostgreSQL.
	dsn := config.Cfg.DSN
	if dsn == "" {
		utils.VerError("DSN de PostgreSQL no configurado (variable DSN vacia)")
		os.Exit(1)
	}

	ctx := context.Background()

	// 3) Inicializar pool de PostgreSQL.
	if _, err := database.Init(ctx, dsn, config.Cfg.PgMinSize, config.Cfg.PgMaxSize); err != nil {
		utils.VerError("Error inicializando PostgreSQL: " + err.Error())
		os.Exit(1)
	}

	// 4) Inicializar cliente HTTP compartido (timeout 10s, como httpx).
	httpclient.Init(10 * time.Second)

	// 5) Inicializar cliente Redis.
	if _, err := redisclient.Init(config.Cfg.RedisDSN); err != nil {
		utils.VerError("Error inicializando Redis: " + err.Error())
		os.Exit(1)
	}

	// 6) Crear AdminPagos e inyectarlo en los routers.
	a := pagosadmin.New(database.Get())
	routers.SetAdmin(a)

	// 7) Goroutine procesadorListas: cada 30s procesa los pagos vencidos o
	// notificados (replica la tarea de background de main.py).
	//
	// Con un contexto cancelable + WaitGroup: antes el bucle usaba
	// context.Background() y no terminaba nunca, de modo que el shutdown
	// cerraba el pool de PostgreSQL mientras esta goroutine consultaba.
	ctx, cancelProcesador := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			a.ProcesarLista(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		}
	}()

	// 8) Registrar las rutas de la API (equivalente a include_router).
	mux := http.NewServeMux()
	routers.RegisterRoutes(mux)

	// 9) Ruta raiz: GET /{$} -> JSON "Pasarela de pagos v3.0"
	// (replica @app.get("/") que return "Pasarela de pagos v3.0").
	mux.HandleFunc("GET /{$}", rootHandler)

	// 10) Cualquier otra ruta: 404 Not Found.
	// El patron "/" captura todo lo que no matchea rutas mas especificas;
	// como GET /{$} es mas especifico, la raiz con GET usa rootHandler.
	mux.HandleFunc("/", http.NotFound)

	// 11) Crear servidor HTTP con el middleware de X-Process-Time.
	// Los timeouts no estan en el uvicorn del legacy: sin ellos el servidor
	// queda expuesto a slowloris (conexiones que nunca terminan de mandar
	// cabeceras) y a conexiones Keep-Alive que retienen un handler para siempre.
	// WriteTimeout > 60s porque /pago/ es sincrono y espera la notificacion de
	// ETECSA (cicloespera) hasta ValidTime segundos (<= 3600).
	addr := fmt.Sprintf("%s:%d", config.Cfg.APIHost, config.Cfg.APIPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      70 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 12) Canal para errores fatales del servidor.
	errCh := make(chan error, 1)
	fmt.Printf("\n API-REST (Pasarela de pagos)\n\n")
	fmt.Printf(" Servidor iniciado en: %s:%d\n\n Presione Ctrl+C para terminar\n\n", config.Cfg.APIHost, config.Cfg.APIPort)
	// 13) Ejecutar ListenAndServe en goroutine; ante un error -> VerError + exit.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 14) Esperar senial de interrupcion (SIGINT/SIGTERM) o error del servidor.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		// Graceful shutdown solicitado por senial.
	case err := <-errCh:
		utils.VerError("Error en el servidor HTTP: " + err.Error())
		os.Exit(1)
	}

	// 15) Shutdown ordenado con timeout de 10s.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		utils.VerError("Error durante el shutdown del servidor: " + err.Error())
		os.Exit(1)
	}

	// 15.1) Detener el procesador de listas y esperar a que salga ANTES de
	// cerrar el pool: si no, la ultima iteracion consulta una BD ya cerrada.
	cancelProcesador()
	wg.Wait()

	// 16) Cerrar recursos (replica el shutdown del lifespan de main.py:
	// close_httpx_client -> close_redis -> pool.close()).
	httpclient.Close()
	redisclient.Close()
	database.Close()
}
