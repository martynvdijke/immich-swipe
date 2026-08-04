## Why

Today Immich Swipe only authenticates via preconfigured env API keys or a manually pasted API key. Household members who already have Immich accounts must create and share API keys instead of signing in with the credentials they already use. Adding Immich email/password login lets any person on the Immich instance authenticate as themselves while keeping the existing API-key flows for private/single-user deployments.

## What Changes

- Add an Immich **credential login** path (email + password + server URL) that authenticates against Immich's own login API.
- Extend the Go backend session model so a session can be backed by either an Immich **API key** or an Immich **access token** from password login.
- Update the reverse proxy to attach the correct upstream auth header per session type (`x-api-key` vs Immich `Authorization: Bearer <accessToken>`), without leaking the app's own session token to Immich.
- Update the login UI so users can choose **API key** or **Immich account** login; multiple people can each log in with their own Immich account on the same deployment.
- Keep existing env-user selection and manual API-key login working side by side (no removal).
- Scope per-user local data (reviewed cache, preferences/stats) by the authenticated Immich identity (name/email), same as today.

## Capabilities

### New Capabilities
- `immich-credential-login`: Users can sign in with Immich email/password (server-mediated), receive an app session, and use the app as that Immich user.

### Modified Capabilities
- `server-side-auth`: Sessions may be established via Immich credentials in addition to env/manual API keys; proxy auth attachment depends on session credential type. (Delta against the capability introduced in `go-backend-server`; no archived root spec yet.)

## Impact

- **Backend**: `server/main.go` — login handler, session struct, credential validation, proxy director, optional Immich logout on app logout.
- **Frontend auth**: `src/stores/auth.ts`, `src/views/LoginView.vue`, possibly `src/router/index.ts` / `src/components/AppHeader.vue`.
- **API surface**: `POST /api/auth/login` accepts `{ email, password, serverUrl? }` in addition to existing `{ userName }` and `{ apiKey, serverUrl? }`.
- **Immich upstream**: `POST /api/auth/login` (LoginCredentialDto), `GET /api/users/me` with Bearer token; password login must be enabled on the Immich server.
- **Docs**: `README.md`, `env.example` notes that credential login does not require env API keys.
- **Tests**: auth store / login-related unit tests; backend login path coverage if present.
- **Non-breaking**: existing env multi-user and manual API-key flows remain supported.
