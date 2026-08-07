## ADDED Requirements

### Requirement: Proxy strips browser auth headers before forwarding to Immich
The Go reverse proxy SHALL remove all browser-originated authentication headers from the outbound request to Immich before adding the server-side `x-api-key`. Specifically, the proxy Director SHALL delete the `Authorization`, `x-immich-user-token`, `x-immich-session-token`, and `x-immich-share-key` request headers. The proxy SHALL then set `x-api-key` from the authenticated session. The upstream Immich server SHALL only ever see the `x-api-key` header for proxied API requests.

#### Scenario: Browser sends Bearer session token to proxy
- **WHEN** the browser sends a request to `/api/<endpoint>` with `Authorization: Bearer <go-session-token>`
- **THEN** the Go proxy SHALL authenticate the session via `authMiddleware`
- **THEN** the forwarded request to Immich SHALL NOT contain the `Authorization` header
- **THEN** the forwarded request SHALL contain `x-api-key: <immich-api-key>` set from the session
- **THEN** Immich SHALL receive a valid `x-api-key` and no competing session token

#### Scenario: Browser sends no Authorization header
- **WHEN** the browser sends a request to `/api/<endpoint>` without an `Authorization` header
- **THEN** the Go `authMiddleware` SHALL reject it with 401 (missing authorization)
- **THEN** no request SHALL be forwarded to Immich

#### Scenario: Browser sends x-immich-user-token header
- **WHEN** the browser sends a request with an `x-immich-user-token` header (e.g. via a browser extension or cookie leak)
- **THEN** the forwarded request to Immich SHALL NOT contain `x-immich-user-token`
- **THEN** only `x-api-key` SHALL be present for upstream auth