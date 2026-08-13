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
  - `IMMICH_SESSIONS_DB` (optional): Pfad zu einer SQLite-Datei; persistiert Swipe-Sessions (Token + API-Key/Access-Token) **und lokale Account-Passwörter (PBKDF2-gehasht)** über Server-Neustarts hinweg. Leer = nur In-Memory (Login nach jedem Neustart nötig). Datei enthält Immich-Credentials im Klartext → wie Secrets behandeln. Siehe `server/main.go` `SessionStore` (write-through, Startup-Restore + Expired-Purge, Cleanup löscht auch DB-Zeilen) und `server/accounts.go` `AccountStore`.
- Lokale Swipe-Accounts (`server/accounts.go`): jeder Eingeloggte kann in den Settings (Account password) ein Passwort setzen → `POST /api/auth/account` (nur apiKey-Sessions; accessToken-Sessions → 400 `unsupported_mode`; Passwort ≥8 Zeichen; Änderung verlangt `currentPassword`). `NewAccountStore(db, envUsers, defaultServerURL)` migriert Env-User automatisch (INSERT ... ON CONFLICT DO UPDATE SET api_key — überschreibt NIE gesetzte Passwörter). Hashing: `pbkdf2$<iter>$<saltHex>$<keyHex>` (600000 Iterationen, 16B Salt, SHA-256, constant-time Vergleich).
- Verhalten:
  - 1 Env-User: Auto-Login
  - >1 Env-User: User-Auswahl (`/select-user`); Link „Sign in with Immich account“ → `/login`
  - keine Env-Keys: Login (`/login`) mit Tabs **Swipe account** (userName/password), **Immich account** (email/password) oder **API key**
  - Hat ein Env-User ein Account-Passwort gesetzt: Auto-Login/User-Picker leitet zu `/login` um und befüllt den Swipe-Tab vor (gesteuert über `authStore.pendingPasswordUser`, NICHT über URL-Query — vue-router 5.2.0 wirft Query bei Redirects auf denselben Pfad weg)
- Login-API `POST /api/auth/login` Body-Varianten (mutually exclusive):
  - `{ "userName" }` → Env-API-Key-Session (401 `password_required`, wenn der Account ein Passwort hat)
  - `{ "userName", "password", "serverUrl?" }` → lokaler Account-Login (401-Codes: `unknown_user` / `password_not_set` / `invalid_password`; nutzt die gebundene Immich-API-Key)
  - `{ "apiKey", "serverUrl?" }` → manuelle API-Key-Session
  - `{ "email", "password", "serverUrl?" }` → Immich Password-Login → Access-Token-Session
  - Alle Erfolgsantworten enthalten `mode` (`apiKey` | `accessToken`); Fehlerantworten optional `code`
- Session-Modi (server-side only):
  - `apiKey`: Proxy setzt `x-api-key`
  - `accessToken`: Proxy setzt `Authorization: Bearer <immich-access-token>`
  - Browser-`Authorization` (Swipe-Session) wird vor Upstream immer gestrippt
- Wichtige lokale Storage Keys:
  - Auth: `immich-swipe-sessions` (localStorage: Array `{token, userName, serverUrl}` aller eingeloggten Personen; **keine** Immich-Secrets) + `immich-swipe-active-session` (Aktive Person, Key `serverUrl|userName`); Legacy `immich-swipe-session` (sessionStorage) wird beim ersten Laden migriert
  - UI: `immich-swipe-theme`, `immich-swipe-skip-videos`
  - Stats: `immich-swipe-stats:<server>:<user>` (keep/delete Counter)
  - Review-Cache: `immich-swipe-reviewed:<server>:<user>` (bereits gesehene IDs + keep/delete)
  - Preferences: `immich-swipe-preferences:<server>:<user>` (Reihenfolge, Album-Hotkeys, Scope, Person)
- **Multi-Person-Sessions**: mehrere Personen können gleichzeitig eingeloggt sein; Header-Switcher (User-Badge) wechselt aktiv; „Add person“ → `/login` ohne andere Sessions zu verlieren; Logout entfernt nur die eine Person und fällt auf die nächste zurück; 401 entfernt nur die tote Session (`removeActiveSession`). Alle pro-User-Stores (ui/preferences/reviewed/observability) hängen an `authStore.immichServerUrl`/`currentUserName` (Computed aus aktiver Session) und laden beim Wechsel neu.
- Credential-Login braucht Immich Password-Login enabled; OAuth/SSO out of scope. Account-Passwörter sind rein lokal (Swipe-eigene Auth, kein Immich-Kontakt beim Passwort-Check).

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
  - `src/router/index.ts` (Guard: Restore letzte Session bei Reload, Redirects je nach Login/Env-Konfig, autoLoginBlocked; `/login` ist auch eingeloggt erreichbar = Add-Person-Flow)
  - `src/stores/auth.ts` (Multi-Session-Registry in localStorage, `switchTo`/`restoreLastActive`/`logout`/`logoutSession`/`removeActiveSession`, `loginWithUser`/`loginManual`/`loginWithCredentials`/`loginWithAccount`/`setAccountPassword`; `sessionToken`/`currentUserName`/`immichServerUrl`/`activeSessionMode`/`pendingPasswordUser` aus aktiver Session)
  - `src/views/LoginView.vue` (Swipe-Account- vs Immich-Account- vs API-Key-Tabs)
  - `src/components/AppHeader.vue` (Person-Switcher-Dropdown: Liste aller Sessions, aktive Markierung, Sign out pro Person, Add person)
  - `server/main.go` (Sessions, Login, Proxy, Logout) + `server/accounts.go` (AccountStore, Passwort-Hashing, Env-User-Migration)
  - Tests: `tests/helpers/seedAuth.ts` (`seedAuthSession`/`seedAuthSessions` — MUSS vor erstem `useAuthStore()` laufen)
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
