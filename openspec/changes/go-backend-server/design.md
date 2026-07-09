## Context

The app currently uses Nginx as a combined static file server and reverse proxy. The browser holds API keys (in JS bundle or `localStorage`) and calls Immich directly through the Nginx proxy. This architecture requires:

- Build-time injection of `VITE_*` env vars into the frontend bundle
- Complex CORS configuration on the Nginx proxy
- An `X-Target-Host` header for dynamic proxy targets (open-proxy SSRF risk)
- Full Docker rebuild for any config change

A Go backend consolidates serving + proxying + credential management into a single binary.

## Goals / Non-Goals

**Goals:**
- Replace Nginx with a Go HTTP server that serves static files and proxies API requests
- Keep Immich API keys server-side (never in browser JS)
- Eliminate CORS configuration entirely
- Support runtime config changes without rebuild
- Maintain all existing functionality (random, chronological, albums, keep/delete/favorite/undo)
- 3-stage Docker build (Node build → Go build → scratch/alpine runtime)

**Non-Goals:**
- Adding new features beyond the backend migration
- Rewriting the frontend
- Supporting Immich OAuth/SSO login flows (future concern)
- Horizontal scaling or high availability
- Changing the swipe/review logic

## Decisions

### Decision 1: Go stdlib `net/http` over Gin/Chi
**Choice**: Use Go 1.22+ `net/http` with the new `ServeMux` patterns (`GET /api/...`, `POST /api/...`).
**Rationale**: The server has ~4 routes: static files, login, API proxy, health check. A framework adds dependency weight and learning curve for minimal benefit. `net/http` 1.22+ can handle path parameters with `{id}` syntax.

### Decision 2: Session token via Authorization header over http-only cookie
**Choice**: Backend issues a session token that the browser sends as `Authorization: Bearer <token>`. Token stored in `sessionStorage` (tab-scoped).
**Rationale**: Avoids CSRF considerations entirely. Simpler to implement than http-only cookies. `sessionStorage` is cleared on tab close, which is acceptable for a swipe-session tool. The alternative (http-only cookie) is slightly more secure but requires CSRF middleware.

### Decision 3: Single in-memory session store over database/Redis
**Choice**: Store sessions in a `map[string]session` in Go memory. Session expires after 24h of inactivity.
**Rationale**: Single-user or small-team tool. No need for persistent session storage — if the server restarts, users re-login (same as current behavior on page refresh). Avoids adding Redis or SQLite dependency.

### Decision 4: Config via env vars with `.env` auto-load
**Choice**: Server reads `IMMICH_SERVER_URL` and `IMMICH_API_KEY_*` env vars at startup. Uses `github.com/joho/godotenv` to optionally load from `.env`.
**Rationale**: Same `.env` file pattern users already have. No rebuild needed — just restart the container. The `VITE_*` prefix is dropped since these are server-side now.

### Decision 5: Full proxy replace `X-Target-Host` with config-only targets
**Choice**: Remove the `X-Target-Host` dynamic proxy header entirely. The Immich target is configured via env vars only.
**Rationale**: Eliminates the SSRF/open-proxy vulnerability. The app only needs one Immich instance — dynamic targets add complexity without real benefit.

### Decision 6: Go module in project root as `server/` package
**Choice**: Go source at `server/main.go` with `server/go.mod`. Not a monorepo — the Go module is separate from the Node frontend.
**Rationale**: Clean separation. The frontend build output (`dist/`) is embedded or copied into the Go binary. Docker multi-stage build handles the two build systems independently.

## Risks / Trade-offs

- **[Risk] Session expiry during use**: User gets 401 mid-swipe → Mitigation: backend returns 401 on proxy, frontend auto-redirects to login; session refreshes on each request (sliding expiry).
- **[Risk] Go build in CI fails**: New language dependency for the build pipeline → Mitigation: `golang:alpine` is a standard image; simple `go build` with no external dependencies.
- **[Trade-off] More code to maintain**: Nginx config is ~60 lines of declarative config. Go server will be ~200-300 lines of imperative code. The trade-off is acceptable for the security and flexibility gains.
- **[Risk] Breaking change for existing deployments**: Users with custom Nginx configs or direct-API setups will need to migrate → Mitigation: docker-compose.yml changes are straightforward (env vars replace build args).
