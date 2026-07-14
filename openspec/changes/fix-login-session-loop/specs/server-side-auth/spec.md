## MODIFIED Requirements

### Requirement: No credentials in browser
The frontend SHALL NOT store or transmit raw Immich API keys. All API requests SHALL be authenticated via the session token obtained from login. The frontend SHALL handle a 401 response from the backend by clearing the session and navigating to the login route via the router (not a full page reload), and SHALL prevent an infinite auto-login loop when the backend repeatedly rejects sessions.

#### Scenario: Frontend uses session token
- **WHEN** the frontend makes any API request
- **THEN** the request SHALL include `Authorization: Bearer <session-token>`
- **THEN** the request SHALL NOT include `x-api-key`

#### Scenario: Backend returns 401 on an API request
- **WHEN** the backend responds with 401 to an authenticated API request
- **THEN** the frontend SHALL clear the session token from the auth store and `sessionStorage`
- **THEN** the frontend SHALL navigate to `/login` via the Vue router (not `window.location.href`)
- **THEN** the frontend SHALL set an `autoLoginBlocked` flag so the router guard does not immediately re-attempt auto-login

#### Scenario: Auto-login fails once
- **WHEN** the router guard attempts an auto-login (single env user) and `loginWithUser` returns false
- **THEN** the guard SHALL set `autoLoginBlocked = true`
- **THEN** the guard SHALL navigate to `/login` (manual login) instead of retrying
- **THEN** subsequent guard invocations SHALL NOT re-attempt auto-login while the flag is set

#### Scenario: Manual login succeeds after a blocked auto-login
- **WHEN** the user completes a manual login (or user selection) successfully
- **THEN** the auth store SHALL reset `autoLoginBlocked = false`
- **THEN** the user SHALL be navigated to `/`

#### Scenario: Explicit logout resets loop guard
- **WHEN** the user explicitly logs out
- **THEN** the auth store SHALL reset `autoLoginBlocked = false`
- **THEN** the next navigation SHALL allow auto-login to be attempted again