# Specification: modernize-infrastructure

## Purpose

Eliminate SQL injection (51+ f-string queries), replace sync DB/HTTP blocking the event loop, add Redis for state persistence, consolidate 3 concurrent patterns into single async model, remove dead code.

## Domains

| # | Domain | Type | Summary |
|---|--------|------|---------|
| 1 | Database Layer | Full | psycopg2 sync → asyncpg async; all queries parameterized |
| 2 | State Management | Full | In-memory dicts/locks → Redis hashes/keys |
| 3 | HTTP Client | Full | sync `requests` → `httpx.AsyncClient` |
| 4 | Concurrency | Full | Eliminate 8× `threading.Lock`; unify on async |
| 5 | Dead Code | Full | Remove `database_psycopg3.py`, unused imports, legacy code |

---

## 1. Database Layer

### DB-1: Async Driver
The system MUST replace `psycopg2` (`ThreadedConnectionPool`) with `asyncpg` (native async pool).
- **Scenario**: App starts → `asyncpg.Pool` created with configurable `min_size`/`max_size`
- **Scenario**: `close_all()` called → pool drains and closes cleanly

### DB-2: Parameterized Queries
All SQL MUST use `$1`, `$2` placeholders; zero f-string interpolation in SQL.
- **Scenario**: Cancel endpoint runs `UPDATE ... WHERE uid = $1 AND idoperacion = $2` with params, not `f'...{Phone}...'`
- **Scenario**: Malicious `ExternalId="'; DROP TABLE pagos; --"` → safely escaped, not executed as SQL
- **Acceptance**: `grep -r "f['\"]\(SELECT\|INSERT\|UPDATE\|DELETE\)" --include="*.py" . | grep -v venv` → zero matches

### DB-3: Async CRUD
All `Database` methods MUST be `async def`: `select`, `select_one`, `insert`, `update`, `delete`, `insert_many`, `execute`.

### DB-4: Legacy API Removed
`GetTableList`, `GetTableFields`, `execquery`, `commit`, `rollback` MUST be removed.

---

## 2. State Management

### SM-1: Redis as State Backend
The system MUST use `redis.asyncio` to replace: `sol_pagos` dict, `ThreadSafeDict` (`msg_list`, `url_list`, `urlvalido`), global sets `_listaid_set`, `_por_notificar_set`.
- **Scenario**: Payment stored in Redis → API restarted → payment rehydrated on startup
- **Scenario**: `.env` provides `REDIS_DSN` (default `redis://localhost:6379/0`)
- **Scenario**: Redis unreachable → app logs warning, falls back to in-memory dicts (no crash)

### SM-2: DataPago → Redis Hash
`DataPago` stored as Redis hash `pasarela:pago:{externalid}` with TTL `ValidTime + 600s`.
- **Scenario**: `add()` called → `HSET pasarela:pago:U123-TEST-001` with all fields + `EXPIRE`

### SM-3: Global Sets via Redis
`_listaid_set` / `_por_notificar_set` replaced by Redis SET (`SADD`, `SREM`, `SISMEMBER`).
- **Scenario**: Duplicate `SADD` returns 0 → caller gets `Estado=14`

---

## 3. HTTP Client

### HC-1: Async HTTP Calls
All external calls to ETECSA MUST use `httpx.AsyncClient` instead of `requests`.
- **Scenario**: `/pago/` submits via `await client.post(settings.ordenpago, ...)` with 10s timeout
- **Scenario**: `/estadoordenpago/` queries via `await client.get(settings.estadoorden + ...)`

### HC-2: Shared Client Instance
Single `httpx.AsyncClient` reused across app; closed on shutdown via `lifespan`.

---

## 4. Concurrency

### C-1: Zero Manual Locks
All 8 `threading.Lock` instances removed. Concurrency from async event loop only.
- **Acceptance**: `grep -r "threading\.Lock" database.py config.py routers/*.py` → zero matches

### C-2: Remove ThreadSafeDB
`class ThreadSafeDB` in `pagos.py` removed (subsumed by Redis).

### C-3: Background Thread Replaced
Daemon thread `procesador_listas` → `asyncio.Task` (not `threading.Thread`).

---

## 5. Dead Code

### DC-1: Remove database_psycopg3.py
File MUST be deleted from repository.

### DC-2: Remove Unused Imports
- `import psycopg2` → zero matches (excl. `venv/`)
- `import requests` → zero matches (excl. `venv/`)

---

## Constraints

| # | Detail |
|---|--------|
| LC-1 | asyncpg v0.29+ with `create_pool` |
| LC-2 | `redis.asyncio` v5+, `hiredis` parser |
| LC-3 | httpx v0.25+, shared `AsyncClient` via `lifespan` |
| LC-4 | No Pydantic model changes; no new endpoints; version stays `3.0.0` |
| LC-5 | Each phase → standalone deployable commit |

## Acceptance Criteria

| Criteria | Verification |
|----------|-------------|
| Zero SQL injection | `grep f'["](SELECT\|INSERT\|UPDATE\|DELETE)` in `.py` = 0 (excl. venv) |
| Zero psycopg2 in app | `grep "import psycopg2"` in app code = 0 |
| Zero requests in app | `grep "import requests\|requests\.\(post\|get\)"` in app code = 0 |
| Zero threading.Lock | `grep "threading\.Lock"` in `database.py config.py routers/*.py` = 0 |
| Zero database_psycopg3.py | File does not exist |
| Tests pass | `locust` + `mtest.sh` pass without changes |
| Redis persistence | Payment survives API restart