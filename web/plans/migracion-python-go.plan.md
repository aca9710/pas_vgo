# Plan: Migracion Python -> Go (Pasarela Web)

## Contexto

Migrar la aplicacion de administracion de pasarela de pagos de **FastAPI + Jinja2 + Bootstrap 5 + jQuery** a **Go + html/template + HTMX + Tailwind CSS**. Solo la parte web (admin + portal cliente). La API de pagos con ETECSA queda fuera de alcance.

Ubicacion: nueva carpeta `webgo/` dentro del proyecto actual.

---

## Estructura del Proyecto Go

```
webgo/
  main.go                    # Entry point, server config, DB init
  go.mod / go.sum            # Dependencias

  internal/
    db/
      postgres.go            # Pool de conexiones pgx
      migrations.go          # Creacion de tablas + admin inicial

    handlers/
      auth.go                # Login/Logout admin
      dashboard.go           # Dashboard principal admin
      pagos.go               # Listar/editar pagos + devoluciones admin
      tiendas.go             # CRUD tiendas + TPV
      entidades.go           # CRUD entidades
      usuarios.go            # CRUD usuarios admin
      estadisticas.go        # Estadisticas + PDF
      notificaciones.go      # Listar notificaciones
      portal.go              # Portal cliente (login, pagos, devoluciones)

    middleware/
      auth.go                # Middleware de sesion admin/cliente
      logging.go             # Request logging

    models/
      models.go              # Structs para DB rows (no ORM)

    pdf/
      reporte.go             # Generacion PDF con gofpdf

    utils/
      log.go                 # Sistema de logs (equivalente a ver_error)
      hash.go                # SHA256 hashing
      session.go             # Session management (DB-backed)

  templates/
    layouts/
      admin.html             # Layout base admin (sidebar + content area)
      portal.html            # Layout base portal cliente
      login.html             # Login standalone
    partials/
      sidebar.html           # Sidebar navegacion
      pagination.html        # Componente paginacion
      status_badge.html      # Badge de estado
      search_form.html       # Formulario de busqueda/filtros
    pages/
      admin/
        dashboard.html       # Dashboard
        pagos_list.html      # Listar pagos
        pagos_edit.html      # Editar pago
        tiendas_list.html    # Listar tiendas
        tiendas_edit.html    # Editar tienda
        tpv_list.html        # Listar TPV
        tpv_edit.html        # Editar TPV
        entidades_list.html  # Listar entidades
        entidades_create.html
        entidades_edit.html
        usuarios_list.html   # Listar usuarios
        usuarios_create.html
        usuarios_edit.html
        notificaciones.html  # Notif. pago + devolucion
        devoluciones_list.html
        estadisticas.html    # Dashboard estadisticas
      portal/
        login.html           # Login cliente
        dashboard.html       # Portal dashboard
        pagos_list.html      # Pagos del cliente
        pago_detail.html     # Detalle pago
        devoluciones_list.html
        devolucion_detail.html
    components/
      tables/
        pagos_table.html     # Tabla reutilizable de pagos
        tiendas_table.html
        tpv_table.html
      cards/
        stat_card.html       # Tarjeta estadistica
      charts/
        doughnut_chart.html  # Chart.js integration

  static/
    css/
      output.css             # Tailwind CSS compilado
    js/
      htmx.min.js            # HTMX
      chart.min.js           # Chart.js (si necesario)

  sql/
    datos_prueba.sql         # Datos de prueba (adaptado)
```

---

## Dependencias Go

```
github.com/jackc/pgx/v5          # PostgreSQL driver (async, pool nativo)
github.com/go-chi/chi/v5         # Router HTTP
github.com/jung-kurt/gofpdf      # Generacion PDF
```

**NOTA sobre Plantillas**: Se usa `html/template` (stdlib) que es el mas probado en Go. HTMX reemplaza jQuery para toda interaccion dinamica. Tailwind CSS via CDN o build local.

---

## Mapeo de Endpoints

### Auth (sin prefijo)

| Metodo | Ruta Go | Descripcion | HTMX |
|--------|---------|-------------|------|
| GET | `/login` | Pagina login admin | No |
| POST | `/login` | Procesar login admin | No (redirect) |
| GET | `/logout` | Cerrar sesion admin | No (redirect) |

### Admin (`/admin`)

| Metodo | Ruta Go | Descripcion | HTMX Target |
|--------|---------|-------------|-------------|
| GET | `/admin/` | Dashboard principal | hx-target="#content" |
| GET | `/admin/pagos` | Listar pagos con filtros | hx-target="#content" |
| GET | `/admin/pagos/proceso` | Pagos en proceso | hx-target="#content" |
| GET | `/admin/pagos/{id}/editar` | Form editar pago | hx-target="#content" |
| POST | `/admin/pagos/{id}/editar` | Actualizar pago | hx-target="#content" |
| GET | `/admin/pagos/{id}` | Detalle pago (JSON) | hx-target="#content" |
| GET | `/admin/devoluciones` | Listar devoluciones | hx-target="#content" |
| GET | `/admin/devoluciones/proceso` | Devoluciones en proceso | hx-target="#content" |
| GET | `/admin/tiendas` | Listar tiendas | hx-target="#content" |
| POST | `/admin/tiendas` | Crear tienda | hx-target="#content" |
| GET | `/admin/tiendas/{uid}/editar` | Form editar tienda | hx-target="#content" |
| POST | `/admin/tiendas/{uid}/editar` | Actualizar tienda | hx-target="#content" |
| DELETE | `/admin/tiendas/{uid}` | Eliminar tienda | hx-target="#tiendas-table" |
| GET | `/admin/tpv` | Listar TPV | hx-target="#content" |
| POST | `/admin/tpv` | Crear TPV | hx-target="#content" |
| GET | `/admin/tpv/{id}/editar` | Form editar TPV | hx-target="#content" |
| POST | `/admin/tpv/{id}/editar` | Actualizar TPV | hx-target="#content" |
| DELETE | `/admin/tpv/{id}` | Eliminar TPV | hx-target="#tpv-table" |
| GET | `/admin/entidades` | Listar entidades | hx-target="#content" |
| GET | `/admin/entidades/crear` | Form crear entidad | hx-target="#content" |
| POST | `/admin/entidades/crear` | Crear entidad | hx-target="#content" |
| GET | `/admin/entidades/{id}/editar` | Form editar entidad | hx-target="#content" |
| POST | `/admin/entidades/{id}/editar` | Actualizar entidad | hx-target="#content" |
| GET | `/admin/entidades/{id}/eliminar` | Eliminar entidad | hx-target="#content" |
| GET | `/admin/usuarios` | Listar usuarios | hx-target="#content" |
| GET | `/admin/usuarios/crear` | Form crear usuario | hx-target="#content" |
| POST | `/admin/usuarios/crear` | Crear usuario | hx-target="#content" |
| GET | `/admin/usuarios/{id}/editar` | Form editar usuario | hx-target="#content" |
| POST | `/admin/usuarios/{id}/editar` | Actualizar usuario | hx-target="#content" |
| GET | `/admin/usuarios/{id}/eliminar` | Eliminar usuario | hx-target="#content" |
| GET | `/admin/notificaciones` | Notif. pago | hx-target="#content" |
| GET | `/admin/notificaciones/devolucion` | Notif. devolucion | hx-target="#content" |
| GET | `/admin/estadisticas` | Estadisticas | hx-target="#content" |
| GET | `/admin/estadisticas/reporte_pdf` | PDF reporte | No (download) |

### Portal Cliente (`/portal`)

| Metodo | Ruta Go | Descripcion | HTMX Target |
|--------|---------|-------------|-------------|
| GET | `/portal/login` | Login cliente | No |
| POST | `/portal/login` | Procesar login | No (redirect) |
| POST | `/portal/logout` | Cerrar sesion | No (redirect) |
| GET | `/portal/` | Dashboard cliente | No |
| GET | `/portal/pagos` | Pagos del cliente | hx-target="#content" |
| GET | `/portal/pagos/{id}` | Detalle pago | hx-target="#content" |
| GET | `/portal/devoluciones` | Devoluciones del cliente | hx-target="#content" |
| GET | `/portal/devoluciones/{id}` | Detalle devolucion | hx-target="#content" |

### API JSON (entidades)

| Metodo | Ruta Go | Descripcion |
|--------|---------|-------------|
| GET | `/admin/api/entidades` | Listar entidades JSON |
| GET | `/admin/api/entidades/{id}` | Obtener entidad JSON |
| POST | `/admin/api/entidades` | Crear entidad JSON |
| PUT | `/admin/api/entidades/{id}` | Actualizar entidad JSON |
| DELETE | `/admin/api/entidades/{id}` | Eliminar entidad JSON |

---

## Diseno UI: HTMX + Tailwind CSS

### Patron de Navegacion SPA con HTMX

El sidebar usa HTMX para cargar contenido en el area principal. Cada link del sidebar tiene:

```html
<a href="/admin/pagos"
   hx-get="/admin/pagos"
   hx-target="#content"
   hx-push-url="true"
   class="nav-link">
  Pagos
</a>
```

### Layout Admin

```html
<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Admin Pagos</title>
    <script src="/static/js/htmx.min.js"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-slate-50">
    <div class="flex min-h-screen">
        <!-- Sidebar fijo -->
        <aside class="w-60 bg-white border-r border-slate-200 fixed h-full">
            {{template "partials/sidebar.html" .}}
        </aside>

        <!-- Contenido principal -->
        <main class="ml-60 flex-1 p-6" id="content">
            {{template .ActivePage .}}
        </main>
    </div>
</body>
</html>
```

### Componentes HTMX Reutilizables

**Tablas con carga incremental:**
```html
<table hx-get="/admin/tiendas?page=2"
       hx-trigger="revealed"
       hx-swap="innerHTML">
```

**Formularios con HTMX:**
```html
<form hx-post="/admin/tiendas"
      hx-target="#content"
      hx-push-url="true"
      hx-indicator="#spinner">
```

**Eliminacion con confirmacion HTMX:**
```html
<button hx-delete="/admin/tiendas/TI001"
        hx-confirm="Eliminar esta tienda?"
        hx-target="#tiendas-table"
        hx-swap="outerHTML">
  Eliminar
</button>
```

---

## Sistema de Diseno (Mejorado: Moderno, Minimalista e Intuitivo)

### Design Tokens (tailwind.config / CSS vars)

```css
:root {
  /* Paleta refinada sobre la identidad coral/slate/cream existente */
  --primary:        #4F46E5;   /* Indigo-600 - acento principal */
  --primary-light:  #6366F1;   /* Indigo-500 */
  --primary-dark:   #4338CA;   /* Indigo-700 */
  --success:        #10B981;
  --warning:        #F59E0B;
  --danger:         #EF4444;
  --info:           #0EA5E9;

  --surface:        #FFFFFF;
  --bg:             #F8FAFC;   /* slate-50 */
  --sidebar:        #0F172A;   /* slate-900 sidebar dark */
  --text:           #0F172A;
  --text-muted:     #64748B;

  --radius-sm: 0.375rem;  --radius-md: 0.75rem;  --radius-lg: 1rem;
  --shadow-card: 0 1px 3px 0 rgba(0,0,0,0.08);
  --shadow-pop: 0 10px 30px -5px rgba(0,0,0,0.15);
}

/* Tipografia: Inter (body) + Space Grotesk (display) - mismas Google Fonts */
```

### Principios de Diseno

1. **Jerarquia clara**: sidebar oscuro fijo + area de contenido claro; cada seccion con header descriptivo.
2. **Densidad controlada**: tablas compactas (text-sm, py-2) con hover sutil; maxima 6-7 columnas visibles.
3. **Estados siempre visibles**: badges de color semantico (verde/rojo/ambar/azul) para estados de pago.
4. **Micro-interacciones HTMX**: indicadores de carga (`htmx-indicator`), transiciones `hx-swap="outerHTML transition:true"`, feedback de errores inline.
5. **Respuesta inmediata**: eliminaciones con `hx-confirm`; formularios con boton deshabilitado + spinner mientras POST.
6. **Mobile-first**: sidebar colapsable (`hx-on:click` toggle), tablas con scroll horizontal, grid responsivo en cards.
7. **Empty states disenados**: icono + texto + accion sugerida cuando no hay datos.
8. **Consistencia**: componentes reutilizables por partials (`pagination`, `stat_card`, `status_badge`, `table`).

### Layout Admin Redisenado

```
+--------------------------------------------------------------+
| Sidebar (w-64, slate-900, fija)  | Topbar (fecha + usuario)  |
|  Logo "Pasarela"                 +---------------------------+
|  - Dashboard                     |  #content (htmx target)   |
|  - Pagos                         |                           |
|  - En Proceso                    |  Contenido dinamico       |
|  - Devoluciones                  |                           |
|  - Dev. Proceso                  |                           |
|  - Tiendas                       |                           |
|  - TPV                           |                           |
|  - Entidades  [admin]            |                           |
|  - Usuarios   [admin]            |                           |
|  - Notif. Pago/Dev.              |                           |
|  - Reportes                      |                           |
|  ----                            |                           |
|  - Cerrar Sesion                 |                           |
+--------------------------------------------------------------+
```

### Componentes Clave (Tailwind + HTMX)

| Componente | Descripcion |
|-----------|-------------|
| `StatCard` | Fondo blanco, icono en acento, valor grande `text-2xl font-bold`, label uppercase `text-xs` |
| `DataTable` | `overflow-x-auto`, thead `bg-slate-50 text-xs uppercase text-slate-500`, filas `hover:bg-slate-50`, zebra `odd:bg-white even:bg-slate-50/50` |
| `StatusBadge` | `inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold` con colores por estado (51=emerald, 52=rose, 53=sky, 54=amber, 15=slate, 11=slate) |
| `Pagination` | Partial reutilizable con links HTMX `hx-get` + `hx-target="#content"` + `hx-push-url` |
| `Modal` | Reemplazar modales Bootstrap por `<dialog>` nativo abierto con `hx-on:click` o overlay simple con HTMX |
| `PageHeader` | Titulo `text-xl font-bold` + subtitulo `text-sm text-slate-500` + botones de accion a la derecha |
| `EmptyState` | `flex flex-col items-center justify-center py-16 text-center` con icono, mensaje y boton "Crear primero" |
| `SearchBar` | Formulario con `hx-get` + `hx-target="#table-body"` + `hx-trigger="input changed delay:500ms, submit"` para busqueda en vivo |
| `Toast` | Notificacion `hx-swap-oob` para confirmaciones/errores sin recargar |

---

## Mapeo de Conversion de Templates (Jinja2 -> Go + HTMX + Tailwind)

Conversion de los **25 templates** del proyecto. Cada template se convierte de Bootstrap/jQuery a Tailwind + HTMX. Los que solo son fragments (listas/tablas) ya sirven como respuesta directa para `hx-target`.

### Layouts (3)

| Original | Nuevo (webgo) | Cambios |
|----------|--------------|---------|
| `templates/base.html` | `templates/layouts/admin.html` | Bootstrap -> Tailwind. jQuery `loadContent()` -> navegacion `hx-get` en sidebar con `hx-target="#content"` y `hx-push-url`. Eliminar sistemas de modales Bootstrap. |
| `templates/cliente/base.html` | `templates/layouts/portal.html` | Navbar Bootstrap -> header minimalista Tailwind. Logout con form HTMX o link POST. |
| `templates/admin/login.html` | `templates/layouts/login.html` | Rediseno completo: fondo degradado slate-900, card centrada `max-w-sm`, logo + subtitle, input estandarizados Tailwind, error inline. Form normal (POST redirect). |

### Partials (4)

| Original | Nuevo (webgo) | Cambios |
|----------|--------------|---------|
| `templates/partials/sidebar.html` | `templates/partials/sidebar.html` | Links con `hx-get` + `hx-target="#content"` + `hx-push-url="true"`. Item activo con `bg-white/10 text-white`. Agrupar secciones con labels. Responsive: colapsable en mobile con toggle HTMX. |
| (nuevo) | `templates/partials/pagination.html` | Extraer paginacion repetida (aparece en 5 templates) a un partial: parametros `base_url`, `page`, `total_pages`, `query_string`. Links con `hx-get` + `hx-target="#content"`. |
| (nuevo) | `templates/partials/status_badge.html` | Mapeo estado -> clase Tailwind. `{{template "partials/status_badge.html" .Estado}}`. |
| (nuevo) | `templates/partials/stat_card.html` | Card de estadistica con label, valor e icono. |

### Paginas Admin (14)

| Original | Nuevo (webgo) | Cambios |
|----------|--------------|---------|
| `templates/index.html` | `templates/pages/admin/dashboard.html` | Grid `grid-cols-1 md:grid-cols-2 lg:grid-cols-4` con 4 StatCards. Tabla ultimos pagos con StatusBadge. Doughnut Chart.js cargado desde partial. Fecha en JS `Intl.DateTimeFormat`. |
| `templates/pagos/listar.html` | `templates/pages/admin/pagos_list.html` | Card filtros -> SearchBar con HTMX `hx-get="/admin/pagos"` `hx-target="#pagos-tbody"` `hx-trigger="input changed delay:500ms"`. Paginacion con partial `pagination`. Fila clickeable `hx-get` a detalle. |
| `templates/pagos/editar.html` | `templates/pages/admin/pagos_edit.html` | Form con `hx-post` + `hx-target="#content"`. Selects/inputs con clases Tailwind estandar. Info del pago en grid 2 cols. Toast de exito con `hx-swap-oob`. |
| `templates/devoluciones/listar.html` | `templates/pages/admin/devoluciones_list.html` | Reusar DataTable + pagination partial. Base URL segun `active_page` (devoluciones / proceso). |
| `templates/tiendas/listar.html` | `templates/pages/admin/tiendas_list.html` | Eliminar modal Bootstrap -> formulario inline con `hx-post` que se oculta/muestra con HTMX (`hx-swap` de un `<div id="create-form">`). Eliminar `eliminarTienda()` JS/fetch -> `hx-delete` + `hx-confirm`. Pagination partial. |
| `templates/tiendas/editar.html` | `templates/pages/admin/tiendas_edit.html` | Form HTMX + info tienda en grid. Agregar select entidad (ya existe en router). |
| `templates/tiendas/tpv.html` | `templates/pages/admin/tpv_list.html` | CRUD integrado: boton nuevo muestra form via `hx-get="/admin/tpv/nuevo"` swap en zona. Editar con `hx-get="/admin/tpv/{id}/editar"` swap de modal/dialog `<dialog>`. Eliminar `eliminarTPV()` -> `hx-delete` + `hx-confirm`. |
| (falta en original) | `templates/pages/admin/tpv_edit.html` | El router referencia `tiendas/tpv_editar.html` que NO existe -> crear el template nuevo. |
| `templates/admin/entidades/listar.html` | `templates/pages/admin/entidades_list.html` | DataTable + badges (admin/nombre/tiendas/usuarios). Alert error -> toast OOB. Acciones editar/eliminar con HTMX. |
| `templates/admin/entidades/crear.html` | `templates/pages/admin/entidades_create.html` | Form centrado `max-w-lg mx-auto` card blanca. Select admin con optgroups. Error validation inline. |
| `templates/admin/entidades/editar.html` | `templates/pages/admin/entidades_edit.html` | Igual a create + checkbox activo como toggle Tailwind `peer-checked` o switch. |
| `templates/admin/usuarios/listar.html` | `templates/pages/admin/usuarios_list.html` | DataTable con badges rol (admin=rose, user=slate), tiendas como chips. Pagination partial. |
| `templates/admin/usuarios/crear.html` | `templates/pages/admin/usuarios_create.html` | Form HTMX. Grid 2 cols para rol/activo. Checkboxes tiendas en grid 2 cols con chips. |
| `templates/admin/usuarios/editar.html` | `templates/pages/admin/usuarios_edit.html` | Igual a create + password opcional (placeholder "Dejar vacio para no cambiar"). |
| `templates/notificaciones/listar.html` | `templates/pages/admin/notificaciones_list.html` | DataTable con badge tipo (pago=sky, devolucion=violet). Reutilizada para ambos endpoints (pago/devolucion) con titulo dinamico. |
| (nuevo) | `templates/pages/admin/notificaciones_pago.html` | Variante con columnas de `notificacion_pago` (phone, external_id, tmid, bankid, status). |
| (nuevo) | `templates/pages/admin/notificaciones_devolucion.html` | Variante con columnas de `notificacion_devolucion` (refund_id, success, resultmsg). |
| `templates/estadisticas/dashboard.html` | `templates/pages/admin/estadisticas.html` | Filtros (anio/tipo) con `hx-get` + `hx-target` del bloque graficos. Selects con `hx-trigger="change"`. Boton PDF como `<a download>`. 3 canvases Chart.js en grid. Inyectar datos como `<script type="application/json">` para evitar `|safe`. |

### Portal Cliente (6)

| Original | Nuevo (webgo) | Cambios |
|----------|--------------|---------|
| `templates/cliente/login.html` | `templates/pages/portal/login.html` | Reusar layout login: card centrada, logo "Portal Cliente", inputs Tailwind, error inline. |
| `templates/cliente/portal.html` | `templates/pages/portal/dashboard.html` | Saludo personal (`Bienvenido, {nombre}`), grid 2 cols con cards "Pagos Recientes" y "Devoluciones Recientes" + link "Ver todos". Resumen KPI (total pagos, total devuelto) arriba. |
| `templates/cliente/pagos.html` | `templates/pages/portal/pagos_list.html` | DataTable + StatusBadge. Boton volver outline. EmptyState de pagos. |
| `templates/cliente/pago_detalle.html` | `templates/pages/portal/pago_detail.html` | Detalle en 2 columnas (dato: valor). Importe destacado `text-3xl font-bold`. URL/OrderID/TMID en bloque "Informacion adicional". |
| `templates/cliente/devoluciones.html` | `templates/pages/portal/devoluciones_list.html` | Igual que pagos_list pero con columnas de devolucion. |
| `templates/cliente/devolucion_detalle.html` | `templates/pages/portal/devolucion_detail.html` | Igual que pago_detail. |

### PDF Reporte (1)

| Original | Cambios |
|----------|---------|
| `templates/estadisticas/reporte.html` + `static/css/reporte.css` | NO se convierte a HTML: el PDF se genera directamente con `gofpdf` en `internal/pdf/reporte.go` (mismo layout: header con titulo/fecha, resumen 3 cards, tabla por tipo con totales). Se elimina WeasyPrint. |

---

## Bases de Datos (Reutilizar)

Se conecta a la misma BD PostgreSQL existente. Las tablas ya estan creadas por el Python. Solo se necesita `init_admin_inicial` al arrancar (crear usuario admin si no existe).

**Tablas existentes** (NO cambiar):
- `cliente`, `sesion_cliente`
- `admin_usuario`, `admin_usuario_tienda`, `admin_sesion`
- `entidad`, `tiendas`, `tpv`
- `pagos`, `devolucion`, `url`, `msg`
- `notificacion_pago`, `notificacion_devolucion`

**Configuracion via `.env`** (mismo formato que Python):
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=1234
DB_NAME=pasarela
```

---

## Modulo PDF (migracion de WeasyPrint)

Usar `gofpdf` para generar PDFs directamente en Go. Templates HTML para PDF se reemplazan por llamadas a gofpdf:

```go
func GenerarReportePDF(tipo string, datos []ReporteRow, ...) ([]byte, error) {
    pdf := gofpdf.New("P", "mm", "A4", "")
    // ... agregar contenido tabla, totales, headers
    return buf.Bytes(), nil
}
```

---

## Implementacion por Fases

### Fase 1: Infraestructura Base
1. Crear `webgo/` con `go mod init`
2. Implementar `internal/db/postgres.go` (pool pgx, helpers Execute/Fetch/FetchOne/FetchVal)
3. Implementar `internal/db/migrations.go` (crear tablas si no existen + admin inicial)
4. Implementar `internal/models/models.go` (todos los structs Go equivalentes a Pydantic)
5. Implementar `internal/utils/log.go` (sistema de logs equivalente a ver_error)
6. Implementar `internal/utils/hash.go` (SHA256)
7. Implementar `internal/utils/session.go` (session management DB-backed)
8. Crear `main.go` con Chi router + lifespan

### Fase 2: Layout + Auth
9. Crear `templates/layouts/admin.html` con Tailwind + HTMX
10. Crear `templates/layouts/portal.html`
11. Crear `templates/layouts/login.html`
12. Crear `templates/partials/sidebar.html` (con HTMX nav)
13. Crear `templates/partials/pagination.html`
14. Crear `templates/partials/status_badge.html`
15. Implementar `internal/handlers/auth.go` (login admin, logout)
16. Implementar `internal/middleware/auth.go` (session checking)

### Fase 3: Admin CRUD
17. Dashboard (`internal/handlers/dashboard.go` + template)
18. Pagos (`internal/handlers/pagos.go` + templates listar/editar)
19. Tiendas (`internal/handlers/tiendas.go` + templates listar/editar)
20. TPV (`internal/handlers/tiendas.go` + templates tpv_list/tpv_edit)
21. Entidades (`internal/handlers/entidades.go` + templates listar/crear/editar + API JSON)
22. Usuarios (`internal/handlers/usuarios.go` + templates listar/crear/editar)
23. Notificaciones (`internal/handlers/notificaciones.go` + template)
24. Devoluciones (`internal/handlers/pagos.go` + template listar)

### Fase 4: Portal Cliente
25. Login cliente
26. Dashboard cliente
27. Pagos cliente (listar + detalle)
28. Devoluciones cliente (listar + detalle)

### Fase 5: Estadisticas + PDF
29. Dashboard estadisticas (Chart.js + Tailwind)
30. Generacion PDF con gofpdf
31. Reporte por meses/dias/tiendas

### Fase 6: Pulido
32. Indicadores de carga HTMX (htmx-indicator)
33. Manejo de errores amigable
34. Datos de prueba SQL adaptados
35. Documentacion de credenciales de prueba

---

## Credenciales de Prueba (se mantienen)

| Usuario | Contrasena | Rol | Tiendas |
|---------|-----------|-----|---------|
| admin | admin | admin | TODAS |
| operador1 | user1 | user | TI001, TI002 |
| operador2 | user2 | user | TI003, TI004 |
| supervisor | user3 | user | TI005 |

---

## Convenciones Go del Proyecto

- **Error handling**: toda funcion con DB usa `defer rows.Close()` y retorna error
- **Logging**: `utils.LogError("contexto")` escribe a `log/error_YYYYMMDD.log`
- **Templates**: naming `{{define "pages/admin/pagos_list.html"}}` para identificacion unica
- **Routing**: Chi mux con middleware de sesion por grupo `/admin` y `/portal`
- **No ORM**: queries SQL directas via pgx (mantiene compatibilidad con la BD existente)
- **HTMX**: todas las respuestas HTML son fragments parciales cuando se usa hx-target