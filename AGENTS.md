# Immich Swipe (Repo-Notizen für Agenten)

## Kurzüberblick
- Single-Page-App (Vue 3 + TypeScript + Tailwind) zum Durchsehen von Immich-Fotos: rechts = behalten, links = (in den Papierkorb) löschen.
- Go-Backend (`server/main.go`): statische SPA + Session-Auth + Reverse-Proxy zu Immich.
- State-Management über Pinia (`src/stores/*`), Routing über `vue-router` (`src/router/index.ts`).

## Quickstart (lokal)
- Voraussetzungen: Node.js (Docker nutzt `node:20-alpine`), npm, Go für Backend-Tests.
- Install: `npm install`
- Dev-Server: `npm run dev` (Vite, Port `5173`, `host: true`)
- Backend: `cd server && go run .` (Default `:8080`)
- Build: `npm run build`
- Preview: `npm run preview`
- Typecheck: `npm run type-check`

## Konfiguration (.env / Login-Flow)
- Runtime-Env (Go-Backend, siehe `env.example` / `README.md`):
  - `IMMICH_SERVER_URL` (Default-Immich-URL)
  - `IMMICH_API_KEY_<N>_NAME` / `IMMICH_API_KEY_<N>_KEY` (optional; Auto-Login / User-Picker)
  - Legacy-Fallback: `IMMICH_USER_<N>_NAME` / `IMMICH_USER_<N>_API_KEY`
- Verhalten:
  - 1 Env-User: Auto-Login
  - >1 Env-User: User-Auswahl (`/select-user`); Link „Sign in with Immich account“ → `/login`
  - keine Env-Keys: Login (`/login`) mit Tabs **Immich account** (email/password) oder **API key**
- Login-API `POST /api/auth/login` Body-Varianten (mutually exclusive):
  - `{ "userName" }` → Env-API-Key-Session
  - `{ "apiKey", "serverUrl?" }` → manuelle API-Key-Session
  - `{ "email", "password", "serverUrl?" }` → Immich Password-Login → Access-Token-Session
- Session-Modi (server-side only):
  - `apiKey`: Proxy setzt `x-api-key`
  - `accessToken`: Proxy setzt `Authorization: Bearer <immich-access-token>`
  - Browser-`Authorization` (Swipe-Session) wird vor Upstream immer gestrippt
- Wichtige lokale Storage Keys:
  - Auth: `immich-swipe-session` (sessionStorage: Swipe-Token + userName + serverUrl; **keine** Immich-Secrets)
  - UI: `immich-swipe-theme`, `immich-swipe-skip-videos`
  - Stats: `immich-swipe-stats:<server>:<user>` (keep/delete Counter)
  - Review-Cache: `immich-swipe-reviewed:<server>:<user>` (bereits gesehene IDs + keep/delete)
- Credential-Login braucht Immich Password-Login enabled; OAuth/SSO out of scope.

## API/Proxy
- Frontend ruft nur das Go-Backend unter `/api/...` auf mit `Authorization: Bearer <swipe-session>`.
- `src/composables/useImmich.ts` → `apiRequest()` nutzt relative `/api` + `authStore.authHeader`.
- Proxy-Director: strip client auth headers, dann mode-spezifische Immich-Credentials anhängen.
- Logout: `POST /api/auth/logout` löscht Swipe-Session; bei Access-Token-Mode best-effort Immich logout.

## Immich API (Erkenntnisse / relevante Endpoints)
- Proxied Requests: je nach Session `x-api-key` **oder** Immich Bearer (nie beides mit Swipe-Token).
- Auth login: `POST /auth/login` `{ email, password }` → `accessToken`, `name`, `userEmail`, `userId`
- Connection-Check: `GET /users/me`
- Random Asset: `GET /assets/random?count=<n>`
- Chronologisch: `POST /search/metadata` (Body u.a. `take`, `size`, `skip`, `order`, `assetType`)
- Albums:
  - `GET /albums`
  - Asset in Album: `PUT /albums/<albumId>/assets` mit Body `{ "ids": ["<assetId>"] }`
- Papierkorb:
  - Löschen (Trash): `DELETE /assets` mit Body `{ "ids": ["<assetId>"], "force": false }`
  - Restore: `POST /trash/restore/assets` mit Body `{ "ids": ["<assetId>"] }`
- Favoriten:
  - Toggle/Set: `PUT /assets/<assetId>` mit Body `{ "isFavorite": true|false }` (Antwort wird in der App nicht benötigt; `currentAsset.isFavorite` wird lokal aktualisiert)
  - Optional (Bulk): `PUT /assets` mit Body `{ "ids": ["..."], "isFavorite": true|false }`
- Asset Media:
  - Thumbnail: `GET /assets/<assetId>/thumbnail?size=preview|thumbnail`
  - Original: `GET /assets/<assetId>/original`

## Docker/Deployment
- `docker-compose.yml` baut das Image und veröffentlicht Port `2293:80`.
- Die `.env` Werte werden als **Build-Args** in den Build gebacken (siehe `Dockerfile` + `docker-compose.yml`).
  - Änderung der `.env` in Production erfordert Rebuild/Recreate des Containers.
- Runtime-Server ist Nginx (`nginx:alpine`) und serviert `dist/` + `nginx.conf`.
- CI/CD: `.github/workflows/publish-ghcr.yml` baut & pushed ein generisches Image nach GHCR (`ghcr.io/<owner>/<repo>`) bei Push auf `main` und Tags `v*` (keine Build-Args/Keys im Workflow → Konfiguration erfolgt dann per manuellem Login/`localStorage`, Auto-Login nur via Custom Build).

## Code-Map (wichtigste Stellen)
- Routing/Auth:
  - `src/router/index.ts` (Guard: Redirects je nach Login/Env-Konfig, autoLoginBlocked)
  - `src/stores/auth.ts` (sessionStorage, loginWithUser/loginManual/loginWithCredentials)
  - `src/views/LoginView.vue` (Account- vs API-Key-Tabs)
  - `server/main.go` (Sessions, Login, Proxy, Logout)
- Immich-Integration:
  - `src/composables/useImmich.ts` (Random Asset inkl. Skip-Videos Filter, Delete/Restore, Undo zeigt gelöschtes Asset wieder, Preload)
  - `src/types/immich.ts` (API-Typen)
- UI/Interaktion:
  - `src/views/HomeView.vue` (Hauptscreen, Keyboard: ←/→ Keep/Delete, ↑ oder Ctrl/⌘+Z = Undo)
  - `src/components/SwipeCard.vue` (lädt Thumbnail/Video-Original als Blob mit Headern; Videos als `<video autoplay loop controls>`; Overlay-Button öffnet Asset-Detail in Immich `/photos/<id>`)
  - `src/components/ActionButtons.vue` (Undo-Button; Keep/Delete Buttons nur Desktop)
  - `src/composables/useSwipe.ts` (Touch+Mouse Swipe-Erkennung)
  - `src/stores/ui.ts` + `src/components/LoadingOverlay.vue` + `src/components/ToastNotification.vue`
  - `src/style.css` (`overflow: hidden`, `viewport-fit` via `100dvh`, Safe-Area Utilities)

## Konventionen für Änderungen
- TypeScript ist `strict` + `noUnusedLocals/noUnusedParameters` aktiv (`tsconfig.json`): saubere Imports/Variablen, sonst Build bricht.
- Beim Hinzufügen neuer `VITE_*` Variablen: `src/vite-env.d.ts`, `env.example` und ggf. `README.md` synchron halten.
- Neue Immich-Calls bevorzugt in `src/composables/useImmich.ts` ergänzen und intern `apiRequest()` nutzen (Fehlerhandling/Headers konsistent halten).
