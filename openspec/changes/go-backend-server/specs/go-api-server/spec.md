## ADDED Requirements

### Requirement: Static file serving
The Go server SHALL serve the built SPA from a configured static directory (`dist/`). All unmatched routes SHALL fall back to `index.html` for Vue Router SPA routing. Static assets (JS, CSS, images) SHALL be served with appropriate cache headers and `immutable` caching for content-hashed filenames.

#### Scenario: SPA fallback for unknown routes
- **WHEN** a browser requests `/any/spa/route`
- **THEN** the server SHALL return the contents of `index.html`

#### Scenario: Immutable caching for hashed assets
- **WHEN** a browser requests `/assets/index-abc123.js`
- **THEN** the server SHALL respond with `Cache-Control: public, max-age=31536000, immutable`

### Requirement: Immich API proxy
The Go server SHALL proxy requests from `/api/*` to the configured Immich backend. The server SHALL forward the original request method, headers, and body. The server SHALL NOT expose the Immich API key to the client — the key SHALL be attached server-side.

#### Scenario: Proxied API request
- **WHEN** the frontend sends `GET /api/users/me`
- **THEN** the server SHALL forward the request as `GET /api/users/me` to the configured Immich backend with `x-api-key` header
- **THEN** the server SHALL return the Immich response to the frontend

#### Scenario: Error response passthrough
- **WHEN** the Immich backend returns a 4xx or 5xx error
- **THEN** the server SHALL forward the error status and body to the frontend

### Requirement: Health check endpoint
The Go server SHALL expose `GET /api/health` that returns `{"status":"ok"}` with a 200 status. This SHALL NOT require authentication.

#### Scenario: Health check
- **WHEN** any client requests `GET /api/health`
- **THEN** the server SHALL respond with 200 and `{"status":"ok"}`

## ADDED Requirements

### Requirement: Session-based API key login
The server SHALL expose `POST /api/auth/login` accepting `{ "apiKey": "..." }` and optionally `{ "apiKey": "...", "serverUrl": "..." }`. It SHALL validate the key by calling `GET /api/users/me` on the Immich instance. On success, it SHALL return a session token. On failure, it SHALL return 401.

#### Scenario: Successful login
- **WHEN** a user posts valid `{ "apiKey": "valid-key" }` to `POST /api/auth/login`
- **THEN** the server SHALL validate the key against the configured Immich server
- **THEN** the server SHALL return `{ "token": "session-xxx", "userName": "Alice" }` with 200

#### Scenario: Failed login
- **WHEN** a user posts an invalid API key
- **THEN** the server SHALL return 401 with `{ "error": "Invalid API key" }`

### Requirement: Authenticated API proxy
The server SHALL require a valid session token for all `/api/*` requests except `/api/auth/login` and `/api/health`. The frontend SHALL send the token as `Authorization: Bearer <token>`. Invalid or expired tokens SHALL return 401.

#### Scenario: Authenticated proxy request
- **WHEN** a frontend sends `GET /api/users/me` with `Authorization: Bearer valid-token`
- **THEN** the server SHALL proxy the request to Immich with the stored API key

#### Scenario: Unauthenticated request rejected
- **WHEN** a request to `/api/albums` has no or invalid `Authorization` header
- **THEN** the server SHALL return 401

### Requirement: Session expiry
Sessions SHALL expire after 24 hours of inactivity. Each authenticated request SHALL reset the expiry (sliding expiration).

#### Scenario: Session refresh
- **WHEN** a valid session token is used in an API request
- **THEN** the session expiry SHALL be reset to 24 hours from that request

#### Scenario: Expired session
- **WHEN** a session token has not been used for 24+ hours
- **THEN** the server SHALL return 401 and require re-login
