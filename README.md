# Warrant

Work queue and project context for AI agents. Agents claim tickets, get context, do work, and submit for review via MCP or REST.

## Getting started

You need a **GitHub OAuth App** so users (and agents) can sign in. Create one at [GitHub → Settings → Developer settings → OAuth Apps](https://github.com/settings/developers): set **Authorization callback URL** to `http://localhost:8080/auth/github/callback` for local dev (or your server’s `BASE_URL` + `/auth/github/callback`). Note the **Client ID** and generate a **Client Secret**.

Then:

1. **Copy env and set secrets:**
   ```bash
   cp .env.example .env
   ```
   Edit `.env`: set **GITHUB_CLIENT_ID** and **GITHUB_CLIENT_SECRET** from your GitHub app, and set **JWT_SECRET** to any long random string (used to sign tokens). Leave these empty only if you’re not using OAuth.

2. **Start Postgres and Redis, run migrations, start the server:**
   ```bash
   docker compose up -d postgres redis
   make migrate    # from host; DB on localhost:5433
   docker compose up -d server
   ```

3. **Check the server:** Open **http://localhost:8080**. `GET /healthz` should return 200. To use MCP in Cursor, add the server URL and sign in when prompted—see **docs/cursor-mcp.md**.

For full config, health checks, graceful shutdown, and migration workflow, see **docs/deployment.md**.

## License

Warrant is available under the **Business Source License 1.1 (BSL 1.1)**. You may use it for non-production purposes (development, testing, evaluation) without a commercial license. Production use requires a commercial license from the licensor until the **Change Date** (see [LICENSE](LICENSE)); after that date, the code is licensed under **GPL v2.0 or later**. For commercial licensing, contact the project maintainers.

**Run tests:** `make test` (runs `go test ./...`). No database or Redis required for unit/integration tests; some tests skip if `git` is not on PATH. After changing **api/openapi.yaml**, run **`make generate`** to regenerate the API client and server; CI runs codegen before tests.

## Environment reference

Every variable is listed in **.env.example** with comments. Required for a full run: **PORT**, **DATABASE_URL**, **REDIS_URL**, and (for OAuth/MCP URL auth) **GITHUB_CLIENT_ID**, **GITHUB_CLIENT_SECRET**, **JWT_SECRET**. Optional: **BASE_URL**, **AUTH_SUCCESS_REDIRECT_URL**, **LEASE_TTL_MINUTES**. See **docs/deployment.md** for details.

## API surface

- **REST** – Spec-driven: the server is generated from **api/openapi.yaml** (oapi-codegen). For humans and scripts: projects, tickets, queue, reviews, trace, auth. Use with curl or any HTTP client. Errors are JSON with `error`, `code`, `retriable` (see **docs/structured-errors.md**). Regenerate after spec changes with **`make generate`**.
- **MCP** – For agents (e.g. Cursor, Claude): same concepts as REST via tools (list_projects, claim_ticket, log_step, submit_ticket, etc.). Configure Cursor: **docs/cursor-mcp.md**. Agent flow and errors: in-app guide (resource `warrant://docs/agent-guide`) or **docs/interacting.md**.
- **Git notes** (optional) – Store agent decisions and traces in the repo via `refs/notes/warrant/*`. CLI: `warrant-git`; MCP: `warrant_add_git_note`, etc. Refs and schema: **docs/git-notes.md**. Design: **docs/git-integration-design.md**.

## SaaS readiness (when we host)

When hosting Warrant we will: read secrets (e.g. GITHUB_*, JWT_SECRET, DATABASE_URL, REDIS_URL) from a vault or provider; put TLS termination in front of the app; use managed Postgres and Redis; set **BASE_URL** to the public URL; register an OAuth app for the hosted domain. The same app and Docker Compose stack run locally and in a hosted environment; only env and infrastructure change. For security details (secrets, HTTPS, CORS, rate limiting), see **docs/deployment.md** → Security.
