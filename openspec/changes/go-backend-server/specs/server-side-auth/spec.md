## ADDED Requirements

### Requirement: Server-side credential storage
The Go server SHALL read Immich credentials from environment variables at startup:
- `IMMICH_SERVER_URL` — the base URL of the Immich instance
- `IMMICH_API_KEY_1_NAME` / `IMMICH_API_KEY_1_KEY` — user credentials (numbered)
- `IMMICH_SESSION_SECRET` — optional secret for session signing

The server SHALL NOT expose these credentials to any client request. The credentials SHALL only be used server-side for authenticating API proxy requests.

#### Scenario: Credentials loaded from env
- **WHEN** the server starts with `IMMICH_SERVER_URL` and `IMMICH_API_KEY_1_*` set
- **THEN** the server SHALL make the configured users available for login
- **THEN** the server SHALL NOT serve these values to any client endpoint

#### Scenario: No credentials configured
- **WHEN** the server starts without any `IMMICH_API_KEY_*` env vars
- **THEN** the login endpoint SHALL accept dynamic `serverUrl` and `apiKey` in the request body (manual login mode, same as current `localStorage` flow but proxied)

### Requirement: Multiple user support
The server SHALL support multiple configured users (via `IMMICH_API_KEY_1_*`, `IMMICH_API_KEY_2_*`, etc.). The login endpoint SHALL accept a `userName` field to select which configured user to authenticate as.

#### Scenario: Multi-user login
- **WHEN** a user posts `{ "userName": "Alice" }` to `POST /api/auth/login`
- **THEN** the server SHALL look up Alice's API key from env config
- **THEN** the server SHALL validate it against Immich and return a session token

### Requirement: No credentials in browser
The frontend SHALL NOT store or transmit raw Immich API keys. All API requests SHALL be authenticated via the session token obtained from login.

#### Scenario: Frontend uses session token
- **WHEN** the frontend makes any API request
- **THEN** the request SHALL include `Authorization: Bearer <session-token>`
- **THEN** the request SHALL NOT include `x-api-key`

## REMOVED Requirements

### Requirement: Browser-side API key storage
**Reason**: API key storage moved to Go server
**Migration**: Remove `localStorage` config storage for API keys; use session token from backend login

### Requirement: VITE_* build-time env injection
**Reason**: Credentials no longer need to be baked into frontend build
**Migration**: Remove `VITE_USER_*` build args from Dockerfile; server reads `IMMICH_API_KEY_*` at runtime
