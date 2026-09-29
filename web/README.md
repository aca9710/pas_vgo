# Pasarela Web

Aplicación web de administración de pagos y devoluciones para una pasarela de pagos. Incluye un panel de administración y un portal para clientes.

## Stack

- **Go 1.22+** con router [chi](https://github.com/go-chi/chi)
- **pgx v5** + pgxpool (PostgreSQL asíncrono)
- **html/template** (templates Go)
- **HTMX** + **Tailwind CSS** (CDN) + **Chart.js** (gráficos)

## Requisitos

- Go 1.22 o superior
- PostgreSQL (base de datos `pasarela`)

## Configuración de la base de datos

La conexión se configura mediante variables de entorno (con defaults en `internal/db/postgres.go`):

| Variable      | Default    |
| ------------- | ---------- |
| `DB_HOST`     | `localhost` |
| `DB_PORT`     | `5432`     |
| `DB_USER`     | `postgres` |
| `DB_PASSWORD` | `1234`     |
| `DB_NAME`     | `pasarela` |

Al arrancar, la aplicación crea automáticamente las tablas necesarias (migraciones) y el usuario admin inicial si no existe.

## Ejecución

```bash
go run .
# o compilar:
go build -o pasarela . && ./pasarela
```

El servidor escucha en `http://0.0.0.0:8000` (configurable con `PORT`).

### Credenciales iniciales

- **Admin**: `admin` / `admin` (panel en `/login`)

## Estructura del proyecto

```
main.go                    # Entrypoint + rutas
internal/
  handlers/                # Endpoints (dashboard, pagos, tiendas, entidades,
                           #   usuarios, notificaciones, portal, estadisticas, auth)
  models/                  # Structs
  db/                      # Conexión + migraciones
  middleware/              # Auth (RequireAdmin, RequireCliente)
  session/                 # Sesiones
  utils/                   # LogError, HashPassword, conversiones
  views/                   # Render de templates
templates/
  layouts/                 # Layouts (admin, portal, login)
  pages/                   # Páginas (admin/, portal/)
  partials/                # Sidebar, paginación, badges, stat cards
static/                    # JS/CSS (htmx)
log/                       # Logs de errores (YYYY-MM-DD.log)
```

## Funcionalidades

### Panel de administración (`/admin`)

- **Dashboard**: resumen de pagos (totales, exitosos, monto, en proceso) + distribución por estado
- **Pagos**: listado con filtros (operación, cliente, estado, fechas), detalle y edición de estado
- **Devoluciones**: listado y seguimiento
- **Tiendas y TPV**: alta, edición y baja de tiendas y terminales de pago virtual
- **Entidades** (solo rol admin): gestión de entidades + API JSON
- **Usuarios** (solo rol admin): gestión de usuarios del panel con asignación de tiendas
- **Notificaciones**: historial de notificaciones de pago y devolución
- **Reportes**: estadísticas por mes/día/tienda con exportación a PDF

### Portal cliente (`/portal`)

- Login de clientes
- Consulta de pagos y devoluciones con detalle

## Notas de desarrollo

- Los templates se parsean al arrancar (`views.Init()`); errores de sintaxis abortan el inicio
- El layout invoca la página activa vía `{{.PageContent}}` (pre-renderizada en `views.Render`)
- `ActivePage` es la clave corta para resaltar el link del sidebar
- HTMX: las peticiones con header `HX-Request` devuelven solo el fragmento de la página
- Los errores se registran en `log/YYYY-MM-DD.log` mediante `utils.LogError`

## Documentación

- `doc/tecnica.md` — Documentación técnica
- `doc/usuario.md` — Manual de usuario