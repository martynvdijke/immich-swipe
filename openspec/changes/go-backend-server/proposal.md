## Why

The app currently exposes Immich API keys in the browser (baked into the JS bundle or stored in `localStorage`) and relies on a complex CORS/reverse-proxy setup with Nginx that includes an open-proxy SSRF risk via `X-Target-Host`. A Go HTTP server backend would keep credentials server-side, eliminate all CORS issues, and simplify the deployment — all while improving security.

## What Changes

- **BREAKING**: Replace Nginx runtime with a Go HTTP server
- **BREAKING**: API requests now go through the Go backend, not directly to Immich from the browser
- **BREAKING**: Credential flow changes — API keys stored server-side, not in browser
- **NEW**: Session-based auth — browser gets an http-only cookie/token instead of raw API key
- **NEW**: Runtime credential config — env vars read at startup, no rebuild required for `.env` changes
- **REMOVED**: `nginx.conf` and Nginx runtime stage from Dockerfile
- **REMOVED**: All `VITE_*` build-arg plumbing from Dockerfile and docker-compose.yml
- **REMOVED**: `X-Target-Host` dynamic proxy (SSRF risk)
- **MODIFIED**: Frontend auth store — uses backend session instead of storing API keys
- **MODIFIED**: API request helper — calls backend proxy instead of direct Immich API

## Capabilities

### New Capabilities

- `go-api-server`: Go HTTP server serving static files + proxying Immich API requests
- `server-side-auth`: Session-based auth where backend validates and stores Immich credentials

### Modified Capabilities

- `immich-v3-api`: API request flow changes from direct browser→Immich to browser→Go→Immich; existing endpoints unchanged, just proxied

## Impact

- **NEW**: `cmd/` or `server/` directory with Go source code, `go.mod`, `go.sum`
- **REMOVED**: `nginx.conf`
- **MODIFIED**: `Dockerfile` (3-stage: Node build → Go build → Alpine), `docker-compose.yml` (runtime env vars instead of build args)
- **MODIFIED**: `src/composables/useImmich.ts` — API requests route through backend
- **MODIFIED**: `src/stores/auth.ts` — login calls backend, stores session not API key
- **MODIFIED**: `src/types/immich.ts` — possible new types for session/auth responses
- **MODIFIED**: `.env.example` — new env vars for Go backend config
- No changes to UI components, swipe logic, or stores besides auth
