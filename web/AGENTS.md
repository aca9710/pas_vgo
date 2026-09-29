# AGENTS - Pasarela Web

## Run
```bash
go run .
# or: go build -o pasarela . && ./pasarela
```

## Stack
- Go 1.22+ / chi router
- pgx v5 + pgxpool (PostgreSQL async)
- html/template (Go templates)
- HTMX + Tailwind CSS (CDN)

## DB Config
Config via env vars (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`) - defaults en `internal/db/postgres.go` (postgres/1234/pasarela).

## Project Structure
- `main.go` - entrypoint + rutas
- `internal/handlers/` - endpoints (dashboard, pagos, tiendas, entidades, usuarios, notificaciones, portal, estadisticas, auth)
- `internal/models/` - structs
- `internal/db/` - conexión + migraciones
- `internal/middleware/` - auth (RequireAdmin, RequireCliente)
- `internal/session/` - sesiones
- `internal/utils/` - LogError, HashPassword, conversiones
- `internal/views/` - render de templates
- `templates/` - Go templates (layouts/, pages/, partials/)
- `static/` - JS/CSS

## Dev Notes
- Los templates se parsean al arrancar (`views.Init()`); errores de sintaxis abortan el inicio
- El layout invoca la página activa vía `{{.PageContent}}` (pre-renderizada en `views.Render`)
- `ActivePage` es la clave corta para resaltar el link del sidebar
- HTMX: las peticiones con header `HX-Request` devuelven solo el fragmento de la página

## Coding Standards

### Tratamiento de Excepciones (OBLIGATORIO)
**ES OBLIGATORIO** usar la función `ver_error()` para todo tratamiento de excepciones en routers, database y cualquier código del proyecto.

Reglas:
1. Toda función que realice operaciones de BD debe verificar el error devuelto
2. En caso de error debe llamarse a `utils.LogError("nombre_funcion")` para registrar el error
3. El mensaje de error debe ser conciso y descriptivo: `utils.LogError("crear_entidad_post")`

Ejemplo correcto:
```go
_, err := db.Instance.Exec(r.Context(), "INSERT INTO entidad ...")
if err != nil {
    utils.LogError("crear_entidad_post")
    http.Redirect(w, r, "/admin/entidades?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
    return
}
```

Ejemplo incorrecto (NO USAR):
```go
// Sin verificar error - INCORRECTO
db.Instance.Exec(r.Context(), "INSERT INTO entidad ...")

// Solo ignorar - INCORRECTO
_, _ = db.Instance.Exec(r.Context(), "INSERT INTO entidad ...")
```

Los errores se guardan en `log/YYYY-MM-DD.log` para facilitar el debugging.

---

# Enhanced Planning Workflow

This project uses an enhanced planning workflow built around `enhance-plan`.

## Default Workflow

- Use `enhance-plan` for non-trivial features, cross-module changes, architecture decisions, and work that benefits from explicit review before implementation.
- After a feature plan has been reviewed and explicitly approved, switch to `enhance-build` for execution. OpenCode's built-in code mode also works but loads more context.
- Keep planning focused on one feature at a time.

## Planning Artifacts

Active feature plans live under `plan/active/<feature>/` and should include:

- `plan.json` - structured plan state and todo metadata
- `plan.md` - human-readable plan draft
- `.plan-original.md` - preserved baseline draft
- `handoff.md` - minimal execution-facing handoff for build mode

Completed or inactive feature plans should move to `plan/archive/<feature>/` when appropriate.

## Build Context Policy

- After a plan is approved, switch to the `enhance-build` agent for execution.
- `enhance-build` reads ONLY `handoff.md` and `plan.json` at startup — no broad codebase exploration.
- It executes one batch per conversation and prompts for commit checkpoints between batches.
- This results in significantly lower token usage compared to OpenCode's built-in code mode.
- If using the built-in code mode instead, prefer the current feature's `handoff.md` as the primary execution context.
- Read `plan.json` or `plan.md` only when additional detail is needed.
- Avoid loading broad background context when the current feature handoff is sufficient.

## Build Context Hygiene

Long conversations accumulate token overhead from tool outputs and modified file contents injected into context. To keep build sessions efficient:

- Execute todos in small batches as defined in the `Execution Batching` section of `handoff.md`.
- After completing a batch, prompt the user to commit and push changes — **including the `plan/` directory** — then start a new conversation for the next batch.
- Planning artifacts (`plan.json`, `plan.md`, etc.) are part of the working tree and will be injected into context by OpenCode if left uncommitted. Committing them reduces noise in subsequent conversations.
- Starting a new conversation resets accumulated context (tool outputs, modified file contents), keeping each batch session lean.
- Do not attempt to implement all todos in a single conversation when the feature involves many files or complex changes.

## Planning Write Policy

- `enhance-plan` may create or update `AGENTS.md`, `.opencode/README.md`, and files under `plan/` while staying in planning mode.
- `enhance-plan` must not modify implementation files such as application source, build config, release config, or dependency manifests.
- Treat planning writes as workflow state management, not as implementation work.

## Feature Discipline

- Keep one active feature context at a time.
- If returning to a previous feature, resume its existing plan instead of creating a duplicate.
- If multiple approaches are viable, include a dedicated `Option Paths` section before execution approval.

## Legacy Docs

This project previously used `openspec/` for planning. Legacy docs are preserved in `plan/archive/openspec-legacy/` for traceability. New planning work should use the `plan/` structure.

---

## Documentation Commands

- `/doc` or `/doc full` - Genera documentación completa (técnica + usuario)
- `/doc tech` - Genera solo documentación técnica
- `/doc user` - Genera solo manual de usuario

La documentación se genera en:
- `doc/tecnica.md` - Documentación técnica
- `doc/usuario.md` - Manual de usuario
- `doc/README.md` - Índice