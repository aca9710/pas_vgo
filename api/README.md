# Pasarela de Pagos API (Go)

API REST de pasarela de pagos para Transfermóvil (ETECSA). Conversión a Go de la
implementación original en Python (FastAPI), preservada como referencia en
[`python_legacy/`](python_legacy/).

La API gestiona solicitudes de pago (síncronas y asíncronas), cancelaciones,
devoluciones y notificaciones de estado, integrando con los servicios externos
de ETECSA (`ordenpago`, `devolucion`) y persistiendo en PostgreSQL con Redis
como cola de seguimiento.

---

## Características

- **Pagos síncronos** (`/pago/`): espera la notificación de ETECSA hasta el
  vencimiento del `ValidTime`.
- **Pagos asíncronos** (`/pago_a/`): retorna inmediatamente; el estado se
  procesa en segundo plano.
- **Cancelación** de solicitudes en proceso (`/cancelar/`).
- **Devoluciones** con validación de importe (`/devolucion/`).
- **Notificaciones** de pago y devolución (callbacks de ETECSA).
- **Consulta de estado** en ETECSA y local.
- **Procesamiento periódico**: goroutine que cada 30 s guarda pagos
  vencidos/notificados y aplica notificaciones pendientes.
- **Autenticación** por headers (`username`, `password` SHA512+Base64, `source`).
- **Fidelidad con pydantic**: validación de campos requeridos y errores 422
  estilo FastAPI.

---

## Stack tecnológico

| Componente | Tecnología |
|------------|------------|
| Lenguaje | Go 1.26 (`net/http` ServeMux con patrones 1.22+) |
| Base de datos | PostgreSQL (pgx v5, pool `pgxpool`) |
| Cache / cola | Redis (go-redis v9) |
| Configuración | `.env` vía `godotenv` |
| Cliente HTTP | `net/http` compartido (timeout 10 s) |

---

## Requisitos

- **Go 1.26+**
- **PostgreSQL** en ejecución (DSN en `.env` o defaults en `config/config.go`)
- **Redis** en ejecución (`REDIS_DSN` en `.env` o defaults en `config/config.go`)
- **Servicios ETECSA** alcanzables (endpoints `ordenpago`, `devolucion`)

---

## Dependencias imprescindibles (PostgreSQL y Redis)

> ⚠️ **La API NO arranca sin PostgreSQL y Redis.** En `main.go` se inicializan en
> este orden: primero el pool de PostgreSQL, luego el cliente Redis. Si cualquiera
> no está disponible, el proceso aborta con `os.Exit(1)` y escribe el error en
> `log/error_YYYYMMDD.log`. El error típico si falta Redis es:
>
> ```
> Error inicializando Redis: dial tcp 127.0.0.1:6379: connect: connection refused
> ```
>
> Ese stack trace en el log **no es un panic del programa**: es la salida normal
> de `utils.VerError` (equivale al traceback de Python). El arranque se detiene
> porque Redis es una dependencia obligatoria.

Los valores que usa la API están en `.env`:

```dotenv
DSN=host=127.0.0.1 dbname=pasarela user=postgres password=1234 port=5432
REDIS_DSN=redis://localhost:6379/0
```

### 🔴 Levantar PostgreSQL

1. **Instalar** (Debian/Ubuntu, si no está):

   ```bash
   sudo apt update && sudo apt install -y postgresql postgresql-contrib
   ```

2. **Iniciar el servicio** y habilitarlo al arranque:

   ```bash
   sudo systemctl enable --now postgresql
   ```

3. **Verificar** que acepta conexiones (debe responder con `accepting connections`):

   ```bash
   pg_isready
   ```

4. **Crear la base de datos y el usuario** que espera el `.env`
   (`dbname=pasarela user=postgres password=1234`):

   ```bash
   sudo -u postgres psql <<'SQL'
   ALTER USER postgres WITH PASSWORD '1234';
   CREATE DATABASE pasarela OWNER postgres;
   SQL
   ```

   > Alternativa en Docker (si prefieres no instalar en el host):

   ```bash
   docker run -d --name pasarela-pg -p 5432:5432 \
     -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=1234 -e POSTGRES_DB=pasarela \
     postgres:16
   ```

### 🔴 Levantar Redis

1. **Instalar** (Debian/Ubuntu, si no está):

   ```bash
   sudo apt update && sudo apt install -y redis-server
   ```

2. **Iniciar el servicio** y habilitarlo al arranque:

   ```bash
   sudo systemctl enable --now redis-server
   ```

3. **Verificar** que responde a `PING` (debe responder `PONG`):

   ```bash
   redis-cli ping
   ```

   > Alternativa en Docker:

   ```bash
   docker run -d --name pasarela-redis -p 6379:6379 redis:7
   ```

### ✅ Checklist antes de ejecutar

```bash
pg_isready                              # PostgreSQL: accepting connections
redis-cli ping                          # Redis: PONG
go run .                                # API arranca en :5081
```

---

## Puesta en marcha

```bash
# 1. Configurar el entorno (editar .env con DSN, REDIS_DSN, credenciales...)
#    Ver sección "Configuración" más abajo.

# 2. Ejecutar
go run .

# 3. O construir y ejecutar el binario
go build -o pasarela .
./pasarela
```

La API escucha por defecto en el puerto **5081** (`API_PORT` en `.env`).

---

## Configuración

Las variables de entorno se leen de `.env` (cwd) y luego `../.env`. Si no están
definidas, se usan los valores por defecto de `config/config.go`.

| Variable | Descripción |
|----------|-------------|
| `API_HOST` / `API_PORT` | Host y puerto del servidor HTTP |
| `DSN` | Cadena de conexión PostgreSQL |
| `REDIS_DSN` | Cadena de conexión Redis |
| `PG_MIN_SIZE` / `PG_MAX_SIZE` | Tamaños mínimo/máximo del pool pgx |
| `SEMILLA_AUTH` | Semilla para generar los headers de autenticación hacia ETECSA |
| `PASARELA_URL` | URL base de la pasarela |
| `ORDENPAGO` | Endpoint externo de órdenes de pago (ETECSA) |
| `DEVOLUCION` | Endpoint externo de devoluciones (ETECSA) |
| `NOTIF_PAGO` | URL de notificación de pago (sobrescribe `UrlResponse`) |
| `DEBUG` | Modo debug |

---

## Arquitectura

```
main.go            Punto de entrada: config, Init de DB/Redis/HTTP, rutas,
                   middleware X-Process-Time, goroutine de procesamiento (30 s)
routers/           Handlers HTTP: pagos.go, devoluciones.go, estado.go, helpers.go
database/          Acceso PostgreSQL (pgxpool): Select, Execute, InsertReturning...
models/            Modelos de datos + validación estilo pydantic
pagosadmin/        Gestión en memoria de pagos + procesamiento periódico
utils/             IDs, auth (SHA512 base64), logging, validación de IP
redisclient/       Cliente Redis global (listaid / por_notificar)
httpclient/        Cliente HTTP compartido para llamadas a ETECSA
config/            Carga de configuración desde .env
doc/               Documentación de endpoints y reportes
test/              Scripts de prueba (mtest.sh)
python_legacy/     Implementación Python original (referencia)
```

### Flujo de un pago síncrono

1. Verificar `ExternalId` duplicado en Redis (`pasarela:listaid`).
2. Verificar autenticación (headers).
3. Validar IP del cliente.
4. Enviar solicitud a ETECSA (`ORDENPAGO`).
5. Esperar notificación (ciclo de 500 ms hasta `ValidTime`).
6. Responder con estado y notificación.

---

## Endpoints

| Path | Método | Descripción |
|------|--------|-------------|
| `/pago/` | POST | Solicitud de pago síncrono |
| `/pago_a/` | POST | Solicitud de pago asíncrono |
| `/cancelar/` | POST | Cancelar solicitud de pago |
| `/notificapagos/` | POST | Notificación de estado de pago (callback) |
| `/devolucion/` | POST | Solicitud de devolución |
| `/notificaciondevol/` | POST | Notificación de devolución (callback) |
| `/estadoordenpago/{externalid}/{source}/` | GET | Consultar estado en ETECSA |
| `/estadoordenpagolocal/{externalid}/{source}/` | GET | Consultar estado local |
| `/estadodevolucion/{externalid}/{source}/{tmid}/` | GET | Consultar estado de devolución |

> **Nota:** cada endpoint se sirve con y sin slash final (`/pago/` y `/pago`).

La referencia completa de cada endpoint (URL, método, entrada campo por campo,
headers, descripción y salida) está en [`doc/endpoint_ref.md`](doc/endpoint_ref.md).

### Autenticación

Todos los endpoints requieren headers HTTP, excepto los callbacks
(`/notificapagos/`, `/notificaciondevol/`) y las consultas de estado:

| Header | Descripción |
|--------|-------------|
| `username` | Usuario registrado |
| `password` | Contraseña en SHA512 codificada en Base64 |
| `source` | Código de origen registrado |

```python
import hashlib, base64
password = "mi_contraseña"
b64 = base64.b64encode(hashlib.sha512(password.encode()).digest()).decode()
```

---

## Testing

```bash
# Suite completa de tests unitarios
go test ./...

# Con detector de carreras (data races)
go test -race ./...

# Con reporte de cobertura
go test -cover ./...

# Prueba de pago contra la API en ejecución (requiere API en :5081)
./test/mtest.sh [total] [concurrentes]

# Simulador de Transfermóvil (Python, referencia)
(cd python_legacy/test && python sim_tfm.py)
```

Los tests cubren funciones puras de todos los paquetes (sin requerir
PostgreSQL/Redis/red): validación de modelos, normalización de tipos pgx,
helpers HTTP, gestión en memoria de pagos, configuración y cliente HTTP.

---

## Documentación

| Documento | Contenido |
|-----------|-----------|
| [`doc/endpoint_ref.md`](doc/endpoint_ref.md) | Referencia completa de endpoints |
| [`doc/sdd-verify-report.md`](doc/sdd-verify-report.md) | Reporte de verificación de la conversión Python→Go |

---

## Notas

- La conversión corrige bugs latentes del código Python: `guarda()` pasaba 14
  parámetros a 13 placeholders, `num_orden()` llamaba `.get('activo')` sobre un
  objeto `DataPago`, y el `LOCK` nunca se adquiría. Ver
  [`doc/sdd-verify-report.md`](doc/sdd-verify-report.md).
- El contrato Go trata `Source`, `Msg` y `Bank` de `/notificapagos/` como
  opcionales (desviación documentada del pydantic).
