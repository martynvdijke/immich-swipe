## ADDED Requirements

### Requirement: Immich email/password login via backend
The system SHALL allow a user to authenticate to Immich Swipe by submitting Immich account credentials (email and password) and an Immich server URL to the Swipe backend. The backend SHALL call Immich `POST /api/auth/login` with those credentials, create a Swipe session on success, and return a Swipe session token. The browser SHALL NOT retain the Immich password or Immich access token after login completes.

#### Scenario: Successful credential login
- **WHEN** a user posts valid `{ "email", "password", "serverUrl" }` to `POST /api/auth/login`
- **THEN** the backend SHALL authenticate against Immich using password login
- **THEN** the backend SHALL return HTTP 200 with `{ "token", "userName", "serverUrl" }`
- **THEN** `userName` SHALL be the Immich user's display name when available, otherwise their email
- **THEN** the response body SHALL NOT include the Immich access token or password

#### Scenario: Invalid credentials
- **WHEN** a user posts incorrect email or password to `POST /api/auth/login`
- **THEN** the backend SHALL return HTTP 401
- **THEN** the backend SHALL NOT create a Swipe session

#### Scenario: Password login disabled on Immich
- **WHEN** Immich rejects login because password login is disabled
- **THEN** the backend SHALL return an error indicating password login is unavailable
- **THEN** the backend SHALL NOT create a Swipe session

#### Scenario: Server URL fallback
- **WHEN** a user posts `{ "email", "password" }` without `serverUrl` and `IMMICH_SERVER_URL` is configured
- **THEN** the backend SHALL use `IMMICH_SERVER_URL` as the Immich base URL

#### Scenario: Missing server URL
- **WHEN** a user posts credential login without `serverUrl` and no default server URL is configured
- **THEN** the backend SHALL return HTTP 400

### Requirement: Credential login coexists with API-key login
The system SHALL continue to support env-user login (`userName`) and manual API-key login (`apiKey` + optional `serverUrl`) unchanged. Credential login SHALL be an additional accepted body shape on the same login endpoint (or an equivalent documented endpoint).

#### Scenario: Manual API-key login still works
- **WHEN** a user posts valid `{ "apiKey", "serverUrl" }` to `POST /api/auth/login`
- **THEN** the backend SHALL create a session using API-key validation as before

#### Scenario: Env-user login still works
- **WHEN** a user posts a configured `{ "userName" }` to `POST /api/auth/login`
- **THEN** the backend SHALL create a session from the env API key as before

#### Scenario: Ambiguous login body rejected
- **WHEN** a login request includes both `email` and `apiKey`, or both `email` and `userName`
- **THEN** the backend SHALL return HTTP 400
- **THEN** the backend SHALL NOT create a session

### Requirement: Multi-person credential sessions
The system SHALL allow different people to log in sequentially on the same deployment, each receiving a session scoped to their own Immich identity. Per-user client state keys that already incorporate user identity SHALL use the authenticated Immich name (or email fallback).

#### Scenario: Second person logs in after logout
- **WHEN** user A logs out and user B completes credential login with a different Immich account
- **THEN** the app SHALL operate under user B's Immich identity
- **THEN** reviewed-cache and preference keys SHALL resolve under user B's identity, not user A's

#### Scenario: Concurrent browser profiles
- **WHEN** two browser profiles each complete credential login as different Immich users against the same Swipe deployment
- **THEN** each profile SHALL hold its own Swipe session token
- **THEN** each profile's proxied Immich calls SHALL use that profile's Immich credentials

### Requirement: Login UI for Immich account
The frontend SHALL provide a login UI path where the user enters Immich server URL, email, and password, submits to the backend login API, stores only the returned Swipe session token, and navigates to the main app on success.

#### Scenario: User signs in with Immich account from UI
- **WHEN** the user submits valid Immich account fields on the login screen
- **THEN** the frontend SHALL call backend login with email and password
- **THEN** on success the frontend SHALL store the session token in session storage
- **THEN** the frontend SHALL navigate to the home route

#### Scenario: User can still choose API-key login
- **WHEN** the user is on the login screen
- **THEN** the UI SHALL offer API-key login in addition to Immich account login

#### Scenario: Credential login failure shown in UI
- **WHEN** backend credential login fails
- **THEN** the UI SHALL show an error message
- **THEN** the user SHALL remain on the login screen without a session

### Requirement: No Immich secrets in the browser for credential login
After credential login, the frontend SHALL authenticate to the Swipe backend only with `Authorization: Bearer <swipe-session-token>`. The frontend SHALL NOT store Immich passwords or Immich access tokens in `localStorage`, `sessionStorage`, cookies it controls, or application state beyond the transient login form inputs.

#### Scenario: Session storage contents after credential login
- **WHEN** credential login succeeds
- **THEN** session storage SHALL contain the Swipe session token and display metadata (user name, server URL)
- **THEN** session storage SHALL NOT contain the password or Immich access token
