# AGENTS.md

## Run the API

```bash
go run .
# Or build & run:
go build -o pasarela .
./pasarela
```

Default port: **5081** (configured in `config/config.go` / `.env` via `API_PORT`).

## Test

```bash
# Go unit/integration tests
go test ./...

# Single payment test (uses curl against running API)
./test/mtest.sh

# ETECSA simulator (Python legacy helper, run from python_legacy)
(cd python_legacy/test && python sim_tfm.py)
```

- `carpeta de test (Go):` archivos `*_test.go` junto a cada paquete
- `simulador de transfermovil:` `python_legacy/test/sim_tfm.py`

## Dependencies

- **PostgreSQL**: Must be running (DSN in `.env` or defaults in `config/config.go`)
- **Redis**: Must be running (REDIS_DSN in `.env` or defaults in `config/config.go`)
- **ETECSA services**: External endpoints must be reachable
- **API running**: `test/mtest.sh` requires API running on port 5081

## Config

Environment variables in `.env` file. Falls back to hardcoded defaults in `config/config.go` if not provided. `config.Load()` reads `.env` (cwd) then `../.env`.

## Architecture

- `main.go` - Go app entry point (net/http ServeMux, Go 1.22+ patterns)
- `routers/` - API endpoints: `pagos.go`, `devoluciones.go`, `estado.go`, `helpers.go`
- `database/` - PostgreSQL access (pgx pool)
- `models/modelos.go` - Data models + pydantic-style validation
- `pagosadmin/` - In-memory payment management + periodic processing
- `utils/` - ids, auth (SHA512 base64), logging, ip validation
- `redisclient/` - Redis client (listaid / por_notificar sets)
- `httpclient/` - HTTP client for ETECSA calls

## Notes

- Uses `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, `github.com/joho/godotenv`
- All endpoints require authentication headers: `username`, `password` (SHA512 base64), `source`
- Background goroutine in `main.go` processes payment lists every 30 seconds
- External payment service endpoints (ETECSA): `ordenpago`, `devolucion` configured in `config/config.go`
- The original Python implementation is preserved under `python_legacy/` (reference only)

## Especificaciones endpoint
   Cada endpoint debe estar explicado en un documento llamado doc/endpoint_ref.md
   que debe tener las siguientes secciones por cada uno titulo, url, metodo, entrada (cada campo de entrada explicado), header, descripcion, salida(cada campo explicado)

## Forma de probar este API
   En la carpeta /home/arturo/proyectos/2026/pasarela/go/api/stress hay un programa probarapi_tpv312.py que sirve para enviar pagos a este api, en la carpeta /home/arturo/proyectos/2026/pasarela/go/api/sim_transf esta el programa sim312.py que sirve como nodo de destino a las conexiones al puerto 15001 iniciadas por este api   
   
   Este es el flujo
   probarapi_tpv312.py -> api(pasarela) -> sim312.py


## API Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/pago/` | POST | Main payment request |
| `/cancelar/` | POST | Cancel pending payment |
| `/notificapagos/` | POST | Payment status notification callback |
| `/devolucion/` | POST | Submit refund request |
| `/notificaciondevol/` | POST | Refund notification callback |
| `/estadoordenpago/{externalid}/{source}/` | GET | Query payment status from ETECSA |
| `/estadoordenpagolocal/{externalid}/{source}/` | GET | Query local payment status |
| `/estadodevolucion/{externalid}/{source}/{tmid}/` | GET | Query refund status |