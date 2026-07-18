## ADDED Requirements

### Requirement: Session credential modes
The Go server session store SHALL support at least two credential modes for upstream Immich authentication:

1. **API key mode** — session holds an Immich API key used as `x-api-key` on proxied requests
2. **Access token mode** — session holds an Immich user access token used as `Authorization: Bearer <accessToken>` on proxied requests

A session SHALL be created in exactly one mode. Swipe session tokens issued to the browser remain opaque and independent of the Immich credential.

#### Scenario: API-key session created from manual or env login
- **WHEN** login succeeds via `userName` or `apiKey`
- **THEN** the created session SHALL be in API key mode
- **THEN** the session SHALL store the Immich API key server-side only

#### Scenario: Access-token session created from credential login
- **WHEN** login succeeds via email and password
- **THEN** the created session SHALL be in access token mode
- **THEN** the session SHALL store the Immich access token server-side only

### Requirement: Proxy attaches mode-appropriate Immich auth
For authenticated proxied requests to Immich, the server SHALL strip browser-originated Immich/auth headers (`Authorization`, `x-api-key`, `x-immich-user-token`, `x-immich-session-token`, `x-immich-share-key`) and then attach credentials from the Swipe session:

- API key mode → set `x-api-key` to the session API key and do not send a Bearer token to Immich
- Access token mode → set `Authorization: Bearer <immich-access-token>` and do not send `x-api-key`

#### Scenario: API-key session proxy
- **WHEN** a valid API-key-mode session proxies a request to Immich
- **THEN** the upstream request SHALL include `x-api-key` with the session key
- **THEN** the upstream request SHALL NOT include the browser's Swipe session Bearer token

#### Scenario: Access-token session proxy
- **WHEN** a valid access-token-mode session proxies a request to Immich
- **THEN** the upstream request SHALL include `Authorization: Bearer` with the Immich access token
- **THEN** the upstream request SHALL NOT include `x-api-key`
- **THEN** the upstream Bearer value SHALL be the Immich access token, not the Swipe session token

#### Scenario: Browser Authorization never forwarded as-is
- **WHEN** the browser sends `Authorization: Bearer <swipe-session-token>` to the Swipe API proxy
- **THEN** the server SHALL NOT forward that exact header value to Immich

### Requirement: Credential login does not expose Immich tokens to clients
The login and config HTTP APIs SHALL NOT return Immich API keys or Immich access tokens to the client. Only the opaque Swipe session token and non-secret profile fields (user name, server URL) may be returned on successful login.

#### Scenario: Login response omits secrets
- **WHEN** any login mode succeeds
- **THEN** the JSON response SHALL include `token`, `userName`, and `serverUrl` as applicable
- **THEN** the JSON response SHALL NOT include `apiKey`, `accessToken`, or `password`

## MODIFIED Requirements

### Requirement: Multiple user support
The server SHALL support multiple people authenticating to the same deployment through any combination of:

- env-configured users (`IMMICH_API_KEY_N_*` / legacy `IMMICH_USER_N_*`) selected by `userName`
- manual API-key login
- Immich email/password credential login

Env-configured multi-user selection remains available when multiple env users are configured. Credential login SHALL allow additional Immich accounts that are not preconfigured in env vars.

#### Scenario: Multi-user env login
- **WHEN** a user posts `{ "userName": "Alice" }` to `POST /api/auth/login` and Alice is configured
- **THEN** the server SHALL look up Alice's API key from env config
- **THEN** the server SHALL validate it against Immich and return a session token

#### Scenario: Non-env Immich user via credentials
- **WHEN** Bob is not listed in env API keys and posts valid Immich email/password for the configured (or provided) server
- **THEN** the server SHALL create an access-token-mode session for Bob
- **THEN** Bob SHALL be able to use proxied Immich APIs as himself

## REMOVED Requirements

<!-- None. API-key and env-user requirements remain. -->
