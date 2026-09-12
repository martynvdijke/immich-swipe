# Immich Swipe (Repo Notes for Agents)

## Overview
- Single-page app (Vue 3 + TypeScript + Tailwind) for reviewing Immich photos: right = keep, left = delete (to trash).
- Go backend (`server/main.go`): static SPA + session auth + reverse proxy to Immich.
- State management via Pinia (`src/stores/*`), routing via `vue-router` (`src/router/index.ts`).

## Quickstart (local)
- Prerequisites: Node.js (Docker uses `node:20-alpine`), npm, Go for backend tests.
- Install: `npm install`
- Dev server: `npm run dev` (Vite, port `5173`, `host: true`)
- Backend: `cd server && go run .` (default `:8080`)
- Build: `npm run build`
- Preview: `npm run preview`
- Typecheck: `npm run type-check`

## Configuration (.env / Login Flow)
- Runtime env (Go backend, see `env.example` / `README.md`):
  - `IMMICH_SERVER_URL` (default Immich URL)
  - `IMMICH_API_KEY_<N>_NAME` / `IMMICH_API_KEY_<N>_KEY` (optional; auto-migrated into local accounts at startup — NOT a login path anymore)
  - Legacy fallback: `IMMICH_USER_<N>_NAME` / `IMMICH_USER_<N>_API_KEY`
  - `IMMICH_SESSIONS_DB` (optional): path to a SQLite file; persists swipe sessions (token + API key/access token) **and local account passwords (PBKDF2-hashed)** across server restarts. Empty = in-memory only (login required after every restart). File contains Immich credentials in plain text → treat like secrets. See `server/main.go` `SessionStore` (write-through, startup restore + expired purge, cleanup also deletes DB rows) and `server/accounts.go` `AccountStore`.
  - `SWIPE_PUBLIC_URL` (optional): external base URL for the OAuth callback `redirectUri` (`/api/auth/oauth/callback`); empty = derived from the request (with `X-Forwarded-Proto`/`X-Forwarded-Host`).
- Local swipe accounts (`server/accounts.go`): anyone logged in can change their account password in Settings → `POST /api/auth/account` (apiKey sessions only; accessToken sessions → 400 `unsupported_mode`; password ≥8 chars; changing requires `currentPassword`) and set/replace the account's Immich API key → `POST /api/auth/account/apikey` (apiKey sessions only; 401 `invalid_api_key` if Immich rejects it). `SetPassword` preserves the existing API key when passed an empty one; `SetAPIKey(serverURL,userName,apiKey)` updates it. `NewAccountStore(db, envUsers, defaultServerURL)` auto-migrates env users (INSERT ... ON CONFLICT DO UPDATE SET api_key — NEVER overwrites passwords that were set). Hashing: `pbkdf2$<iter>$<saltHex>$<keyHex>` (600000 iterations, 16B salt, SHA-256, constant-time comparison).
- Behavior:
  - No active session → **always** `/login` (deliberately NO auto-login, even with exactly 1 env user); the login page has no env-user picker
  - `/select-user` no longer exists (route redirects to `/`; unauthenticated visitors are caught by the guard on `/login`)
  - Login page tabs: **Sign in** (userName/password, default) and **Create account** (userName + password ≥8; the Immich API key is optional here and can be added later in Settings); **SSO button** (only when Immich has OAuth enabled — `GET /api/auth/config` returns `oauthEnabled`/`oauthButtonText` from Immich `GET /public/config`)
  - After a successful login/create, if the account has no Immich API key (`needsApiKey`) the app toasts and routes to `/settings`; API-key mode requests return HTTP 428 `api_key_required` until the key is set
- Login API `POST /api/auth/login` body variants (only these two are accepted):
  - `{ "userName", "password", "serverUrl?" }` → local account sign-in (401 codes: `unknown_user` / `password_not_set` / `invalid_password`; does NOT contact Immich or validate the bound API key)
  - `{ "userName", "password", "create": true, "apiKey?", "serverUrl?" }` → account creation (400 `weak_password` below 8 chars; 400 `account_exists` when the name already has a password; 401 `invalid_api_key` for a provided key Immich rejects, and for claiming a password-less migrated account with a non-matching key; a blank `apiKey` creates the account with no key)
  - Any other shape (email/password, apiKey-only, userName-only) → 400 unsupported login method
  - SSO login (Immich-native, no IdP contact from swipe): `POST /api/auth/oauth/start { serverUrl? }` → `{ url, state }` (backend calls Immich `POST /api/oauth/authorize` with state + PKCE-S256; 400 `oauth_not_enabled` when Immich OAuth is off) → browser full-redirect to the IdP URL → IdP calls `GET /api/auth/oauth/callback?code&state` (must be whitelisted as redirect URI in the IdP) → backend calls Immich `POST /api/oauth/callback { url, state, codeVerifier }` + `GET /users/me` validation + `CreateAccessToken` → 302 `/login?oauthCode=<one-time-code>` → `POST /api/auth/oauth/finish { code }` → `{ token, userName, serverUrl, mode: "accessToken" }` (400 `invalid_code` for unknown/used/expired). Pending state (10 min) + handoff codes (5 min, single-use) are in-memory in `Server.oauthPending`/`oauthCodes`.
  - Account sign-in/create success responses: `{ token, userName, serverUrl, mode: "apiKey", hasApiKey }`; error responses optionally `code`
- Session modes (server-side only):
  - `apiKey`: proxy sets `x-api-key`
  - `accessToken`: proxy sets `Authorization: Bearer <immich-access-token>`
  - Browser `Authorization` (swipe session) is always stripped before upstream
- Important local storage keys:
  - Auth: `immich-swipe-sessions` (localStorage: array `{token, userName, serverUrl}` of all logged-in people; **no** Immich secrets) + `immich-swipe-active-session` (active person, key `serverUrl|userName`); legacy `immich-swipe-session` (sessionStorage) is migrated on first load
  - UI: `immich-swipe-theme`, `immich-swipe-skip-videos`
  - Stats: `immich-swipe-stats:<server>:<user>` (keep/delete counters)
  - Review cache: `immich-swipe-reviewed:<server>:<user>` (already seen IDs + keep/delete)
  - Preferences: `immich-swipe-preferences:<server>:<user>` (ordering, album hotkeys, scope, person)
- **Multi-person sessions**: multiple people can be logged in simultaneously; header switcher (user badge) changes the active one; "Add person" → `/login` without losing other sessions; logout removes only that one person and falls back to the next; 401 removes only the dead session (`removeActiveSession`). All per-user stores (ui/preferences/reviewed/observability) hang off `authStore.immichServerUrl`/`currentUserName` (computed from the active session) and reload on switch.
- Credential login needs Immich password login enabled; SSO sessions are `accessToken` sessions (proxy/logout/multi-person unchanged, `POST /api/auth/account` and `POST /api/auth/account/apikey` → `unsupported_mode`, since the API key is only meaningful in API-key mode). Account passwords are purely local (swipe's own auth, no Immich contact during password checks).

## API/Proxy
- Frontend only calls the Go backend under `/api/...` with `Authorization: Bearer <swipe-session>`.
- `src/composables/useImmich.ts` → `apiRequest()` uses relative `/api` + `authStore.authHeader`.
- Proxy director: strip client auth headers, then attach mode-specific Immich credentials.
- Logout: `POST /api/auth/logout` deletes the swipe session; best-effort Immich logout in access-token mode.

## Immich API (Findings / Relevant Endpoints)
- Proxied requests: per session `x-api-key` **or** Immich Bearer (never both with a swipe token).
- OAuth/SSO (all under the `/api` prefix, public, no auth needed):
  - Availability: `GET /public/config` → `oauth { enabled, buttonText }`
  - Start: `POST /oauth/authorize` `{ redirectUri, state?, codeChallenge? }` → `{ url }` (+ `immich_oauth_state`/`immich_oauth_code_verifier` cookies; state/verifier alternatively in the callback body)
  - Finish: `POST /oauth/callback` `{ url, state?, codeVerifier? }` → `LoginResponseDto` (like password login)
- Connection check: `GET /users/me`
- Random asset: `GET /assets/random?count=<n>`
- Chronological: `POST /search/metadata` (body incl. `take`, `size`, `skip`, `order`, `assetType`)
- Albums:
  - `GET /albums`
  - Asset into album: `PUT /albums/<albumId>/assets` with body `{ "ids": ["<assetId>"] }`
- Trash:
  - Delete (trash): `DELETE /assets` with body `{ "ids": ["<assetId>"], "force": false }`
  - Restore: `POST /trash/restore/assets` with body `{ "ids": ["<assetId>"] }`
- Favorites:
  - Toggle/set: `PUT /assets/<assetId>` with body `{ "isFavorite": true|false }` (response not needed by the app; `currentAsset.isFavorite` is updated locally)
  - Optional (bulk): `PUT /assets` with body `{ "ids": ["..."], "isFavorite": true|false }`
- Asset media:
  - Thumbnail: `GET /assets/<assetId>/thumbnail?size=preview|thumbnail`
  - Original: `GET /assets/<assetId>/original`

## Docker/Deployment
- `docker-compose.yml` builds the image and publishes port `2293:80`.
- The `.env` values are baked into the build as **build args** (see `Dockerfile` + `docker-compose.yml`).
  - Changing `.env` in production requires rebuilding/recreating the container.
- Runtime server is Nginx (`nginx:alpine`) serving `dist/` + `nginx.conf`.
- CI/CD: `.github/workflows/publish-ghcr.yml` builds & pushes a generic image to GHCR (`ghcr.io/<owner>/<repo>`) on pushes to `main` and tags `v*` (no build args/keys in the workflow → configuration then happens via manual login/`localStorage`, auto-login only via custom build).

## Code Map (Key Locations)
- Routing/Auth:
  - `src/router/index.ts` (guard: restore last session on reload, no auto-login — unauthenticated → always `/login`; `/select-user` redirects to `/`; `/login` is reachable while logged in = add-person flow)
  - `src/stores/auth.ts` (multi-session registry in localStorage, `switchTo`/`restoreLastActive`/`logout`/`logoutSession`/`removeActiveSession`, `loginWithAccount`/`loginWithAccountCreate`/`setApiKey`/`startOAuthLogin`/`loginWithOAuthCode`/`setAccountPassword`; `sessionToken`/`currentUserName`/`immichServerUrl`/`activeSessionMode`/`oauthEnabled`/`oauthButtonText` from the active session / config)
  - `src/views/LoginView.vue` (tabs: Sign in vs Create account, optional Immich API key on create + SSO button when `oauthEnabled` + `oauthCode`/`oauthError` query handling; no env-user picker)
  - `src/components/AppHeader.vue` (person switcher dropdown: list of all sessions, active highlight, sign out per person, add person)
  - `server/main.go` (sessions, login, proxy, logout, OAuth SSO: `oauthStartHandler`/`oauthCallbackHandler`/`oauthFinishHandler` + `oauthPending`/`oauthCodes`) + `server/accounts.go` (AccountStore, password hashing, env user migration)
  - Tests: `server/oauth_test.go` (fake Immich for the OAuth flow), `tests/helpers/seedAuth.ts` (`seedAuthSession`/`seedAuthSessions` — MUST run before the first `useAuthStore()`)
- Immich integration:
  - `src/composables/useImmich.ts` (random asset incl. skip-videos filter, delete/restore, undo re-shows the deleted asset, preload)
  - `src/types/immich.ts` (API types)
- UI/Interaction:
  - `src/views/HomeView.vue` (main screen, keyboard: ←/→ keep/delete, ↑ or Ctrl/⌘+Z = undo)
  - `src/components/SwipeCard.vue` (loads thumbnail/video original as blob with headers; videos as `<video autoplay loop controls>`; overlay button opens the asset detail in Immich `/photos/<id>`)
  - `src/components/ActionButtons.vue` (undo button; keep/delete buttons desktop only)
  - `src/composables/useSwipe.ts` (touch+mouse swipe detection)
  - `src/stores/ui.ts` + `src/components/LoadingOverlay.vue` + `src/components/ToastNotification.vue`
  - `src/style.css` (`overflow: hidden`, `viewport-fit` via `100dvh`, safe-area utilities)

## Conventions for Changes
- TypeScript is `strict` + `noUnusedLocals/noUnusedParameters` (`tsconfig.json`): clean imports/variables, otherwise the build breaks.
- When adding new `VITE_*` variables: keep `src/vite-env.d.ts`, `env.example` and possibly `README.md` in sync.
- Prefer adding new Immich calls in `src/composables/useImmich.ts` and use `apiRequest()` internally (keeps error handling/headers consistent).
