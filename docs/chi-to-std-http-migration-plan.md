# Migration: Chi → Go net/http with spec-generated server

## Goal

Replace the Chi router with the Go standard library HTTP server, using a **server generated from the OpenAPI spec** (oapi-codegen). The result will be feature-complete (all current REST behavior preserved), spec-driven, and thoroughly tested.

## Current state

- **Router**: Chi (`github.com/go-chi/chi/v5`) in `api/rest/server.go`; middleware: RequestID, RealIP, Logger, Recoverer, optional Auth.
- **Handlers**: Each handler has `Register(r chi.Router)` and uses `chi.URLParam(r, "...")` for path params. Mounted in `cmd/server/main.go`.
- **OpenAPI**: `api/openapi.yaml` exists and largely matches the REST surface. `api/oapi-codegen.yaml` currently generates **client** only (no server).
- **Routes not in spec**: Git notes (`/orgs/{orgID}/projects/{projectID}/git-notes/...`), POST `/tickets/{ticketID}/trace` (log step), optional `/tickets/{ticketID}/transitions`. Auth (OAuth), MCP, and `/metrics` are outside the API spec by design.

## Approach

1. **Spec as source of truth**  
   - Update the OpenAPI spec so it includes every REST endpoint we want generated (including git-notes, POST trace, and optionally transitions).  
   - Keep auth, MCP, and `/metrics` as non-spec routes mounted alongside the generated API.

2. **Generate server with oapi-codegen**  
   - Add **server** generation (StrictServerInterface or equivalent) from the spec.  
   - Keep or drop client generation as needed.  
   - Generated code: server interface + router that dispatches to our implementation.

3. **Implement the generated server interface**  
   - One or more handler types that satisfy the generated interface.  
   - Delegate to existing business logic (services); adapt request/response types to/from generated types (or reuse generated types in services where it makes sense).

4. **Std net/http only**  
   - No Chi. Use the generated router as the main API handler.  
   - Middleware: reimplement RequestID, RealIP, Logger, Recoverer, and Auth as `func(http.Handler) http.Handler` and wrap the generated handler.  
   - Mount healthz, metrics, auth, and MCP on a single `http.ServeMux` (or a thin wrapper) that also mounts the generated API.

5. **Feature parity**  
   - All current REST behavior preserved: same status codes, error bodies (StructuredError), and response shapes.  
   - Git notes, trace (GET + POST), queue, reviews, escalations, orgs, projects, tickets — all covered by spec and generated server.

6. **Testing**  
   - Update all existing REST tests to use the new server (no `chi.NewRouter()`; use the generated router + our impl + test middleware for auth context).  
   - Add/expand tests so the migration is thoroughly tested (tenancy, 4xx/5xx, success paths).  
   - Optional: add a contract test that verifies responses against the OpenAPI spec.

## Phases (tickets)

1. **Spec completeness** – Add missing paths/operations to OpenAPI: git-notes (get commit notes, get log), POST `/tickets/{ticketID}/trace`, and optionally PATCH/POST transitions. Ensure path params and request/response schemas match current behavior. Lint spec (e.g. spectral or openapi-lint).
2. **oapi-codegen server** – Add server generation to the codegen config; generate server interface and router into a new package (e.g. `api/rest/spec` or `api/generated`). Ensure build passes.
3. **Middleware (std)** – Implement RequestID, RealIP, Logger, Recoverer, and Auth as `func(http.Handler) http.Handler` in a new middleware package or under `api/rest`, with no Chi dependency.
4. **Server implementation** – Implement the generated server interface by delegating to existing handlers/services; adapt URL params and bodies from generated types. Wire in auth context (e.g. agent ID) from middleware.
5. **Wire main router** – In `api/rest` (or equivalent), build the final handler: middleware chain → (healthz, metrics, auth routes, MCP, generated API). Return `http.Handler`; remove Chi from `server.go` and `cmd/server/main.go`.
6. **Remove Chi** – Remove all Chi imports and `Register(chi.Router)` patterns; delete Chi from `go.mod`. Fix any remaining references.
7. **REST tests** – Update all REST tests to use the new router and middleware (inject agent ID for auth tests). Ensure existing tests pass and add any missing cases for new or edge behavior.
8. **Docs and CI** – Update README/docs to mention spec-driven server and `make generate` if needed. Ensure CI runs codegen and tests.

## Success criteria

- No Chi dependency; all REST served via generated server + std net/http.
- OpenAPI spec is complete and lint-clean; server is generated from it.
- All current REST endpoints behave the same (status codes, errors, bodies).
- Existing REST tests pass; migration is thoroughly tested (tenancy, errors, success paths).

## Risks / notes

- **Path style**: Spec uses flat paths (e.g. `/projects/{projectID}`). Current code uses the same. Org-scoped routes (`/orgs/{orgID}/projects`) are in the spec; ensure generated paths match what we mount.
- **Error format**: Generated code may expect a specific error shape; ensure we still return StructuredError (and any required fields) so clients are unchanged.
- **Optional routes**: Auth, MCP, metrics stay outside the spec and are mounted separately so we don’t over-complicate the spec with one-off endpoints.
