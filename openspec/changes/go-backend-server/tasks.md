## 1. Go Server Scaffolding

- [x] 1.1 Initialize Go module at `server/` with `go.mod`
- [x] 1.2 Create `server/main.go` with HTTP server, graceful shutdown, and `SIGTERM` handling
- [x] 1.3 Implement static file serving for `dist/` with SPA fallback to `index.html`
- [x] 1.4 Add `Cache-Control: immutable` for content-hashed static assets

## 2. Immich API Proxy

- [x] 2.1 Implement `httputil.ReverseProxy` for `/api/*` → configured Immich backend
- [x] 2.2 Add server-side `x-api-key` header injection (key not exposed to client)
- [x] 2.3 Implement request/header forwarding (method, body, query params, Content-Type)
- [x] 2.4 Add error passthrough (Immich error status + body forwarded to client)

## 3. Server-Side Auth

- [x] 3.1 Implement in-memory session store (`map[string]session` with RW mutex)
- [x] 3.2 Implement `POST /api/auth/login` — validates key via `GET /users/me`, returns session token
- [x] 3.3 Implement `Authorization: Bearer` middleware that validates session tokens
- [x] 3.4 Implement sliding session expiry (24h inactivity timeout)
- [x] 3.5 Support env-var configured users (`IMMICH_API_KEY_1_NAME`/`_KEY`) for multi-user login
- [x] 3.6 Support manual login mode (no env var users → accepts `serverUrl` + `apiKey` in login body)
- [x] 3.7 Add `GET /api/auth/config` endpoint for frontend to discover available users

## 4. Health Check

- [x] 4.1 Implement `GET /api/health` returning `{"status":"ok"}` (no auth required)

## 5. Frontend Auth Changes

- [x] 5.1 Update `auth.ts` store: replace `localStorage` config with login call to backend
- [x] 5.2 Update `auth.ts` store: store session token in `sessionStorage`, not API key
- [x] 5.3 Update `apiRequest` helper: prepend backend URL, add `Authorization: Bearer` header, remove `x-api-key`
- [x] 5.4 Remove direct Immich URL handling from auth store (backend handles proxy target)
- [x] 5.5 Handle 401 responses (redirect to login on session expiry)
- [x] 5.6 Implement logout (clear session token, call `POST /api/auth/logout`)
- [x] 5.7 Update router guard for new session-based auth flow
- [x] 5.8 Update LoginView for manual login via backend
- [x] 5.9 Update UserSelectView for env-user login via backend
- [x] 5.10 Update AppHeader logout to use new auth store
- [x] 5.11 Update SwipeCard thumbnail/image loading for same-origin proxy
- [x] 5.12 Update preferences/reviewed/ui stores for renamed `immichServerUrl`

## 6. Docker & Config

- [x] 6.1 Update `Dockerfile` to 3-stage build (Node build → Go build → Alpine runtime)
- [x] 6.2 Remove `nginx.conf`, Nginx runtime stage, and all `VITE_*` build args from Dockerfile
- [x] 6.3 Update `docker-compose.yml` to pass runtime env vars (`IMMICH_SERVER_URL`, `IMMICH_API_KEY_*`) instead of build args
- [x] 6.4 Update `env.example` with new Go server env vars
- [ ] 6.5 Update `README.md` if needed for new config flow

## 7. Cleanup & Verification

- [x] 7.1 Remove `nginx.conf`
- [x] 7.2 Remove unused types from `src/types/immich.ts` (`ImmichConfig`, `EnvUser`, `EnvConfig`, `DeleteAssetsRequest/Response`, `AddAssetsToAlbumRequest`)
- [x] 7.3 Run `npm run type-check` and fix TypeScript errors
- [x] 7.4 Run `npm run build` and verify frontend builds
- [x] 7.5 Build Go binary and verify it compiles
- [ ] 7.6 Update test files for new auth store API
