## Context

Immich Swipe already has a Go backend that:

1. Accepts login via env-configured user name or manual API key
2. Issues an opaque app session token (`Authorization: Bearer <session>`)
3. Proxies `/api/*` to Immich with server-side `x-api-key`, stripping the browser Bearer header so Immich never sees the app session token

The frontend stores only the app session in `sessionStorage` and never holds Immich API keys after the Go migration. Multi-person support today means either multiple env API keys (user picker) or each person pasting their own API key.

Immich natively supports password login:

- `POST {immich}/api/auth/login` with `{ "email", "password" }` → `LoginResponseDto` including `accessToken`, `userId`, `userEmail`, `name`, …
- Authenticated API calls accept `Authorization: Bearer <accessToken>` (user session token), in addition to `x-api-key`

Password login can be disabled on the Immich server (`passwordLogin.enabled`). OAuth/SSO is a separate Immich flow and is out of scope for this change.

## Goals / Non-Goals

**Goals:**

- Let any Immich user sign into Immich Swipe with email + password + server URL
- Keep env-user and manual API-key login fully working
- Keep Immich secrets (API keys and Immich access tokens) server-side only
- Correctly proxy upstream auth for both credential types without reintroducing the Bearer/session-token collision bug
- Isolate per-user client state by Immich identity (name/email) as today
- Clear, multi-person UX: one shared deployment, each person logs in as themselves

**Non-Goals:**

- Immich OAuth / OIDC / SSO browser redirect flows
- PIN elevation, change-password, or admin user management inside Swipe
- Persistent session store (Redis/DB); in-memory sessions remain
- Sharing one Immich session across browser tabs beyond current `sessionStorage` behavior
- Removing API-key login or env multi-user picker
- Multi-Immich-server admin console beyond the existing per-login `serverUrl`

## Decisions

### Decision 1: Server-mediated Immich password login (not browser → Immich direct)

**Choice**: Browser posts `{ email, password, serverUrl? }` to Swipe `POST /api/auth/login`. Go server calls Immich `POST /api/auth/login`, stores the returned `accessToken` in the session, returns only the opaque Swipe session token to the browser.

**Rationale**: Matches the existing security model (no Immich secrets in the browser). Avoids CORS against Immich for login. Centralizes validation and session creation.

**Alternatives considered**:

- *Browser calls Immich login directly*: Rejected — puts `accessToken` in the browser, reintroduces dual-auth complexity, and fights the Go proxy design.
- *Exchange password for a long-lived API key via Immich admin APIs*: Rejected — requires elevated permissions and is not how Immich end-user login works.

### Decision 2: Dual credential types on the session

**Choice**: Extend `Session` with an auth mode:

```text
AuthModeAPIKey | AuthModeAccessToken
```

Fields conceptually:

- `UserName`, `ServerURL`, `ExpiresAt` (existing)
- `APIKey` (set for API-key sessions; empty otherwise)
- `AccessToken` (set for credential sessions; empty otherwise)
- Optional: `UserEmail`, `UserID` for display/scoping

**Rationale**: Proxy behavior differs by mode. Keeping both fields explicit avoids overloading `APIKey` with a Bearer token.

**Alternatives considered**:

- *Always mint an Immich API key after password login*: Not generally available to normal users.
- *Single `UpstreamToken` + type enum only*: Equivalent; named fields are clearer in Go.

### Decision 3: Proxy director attaches the right Immich auth

**Choice**:

1. Always delete browser-originated auth headers before upstream:
   - `Authorization`
   - `x-immich-user-token`
   - `x-immich-session-token`
   - `x-immich-share-key`
   - `x-api-key` (client must not supply)
2. Then set:
   - API-key session → `x-api-key: <session.APIKey>`
   - Access-token session → `Authorization: Bearer <session.AccessToken>`

**Rationale**: Preserves the fix from `fix-login-session-loop` (never forward Swipe session Bearer to Immich) while supporting Immich user tokens.

**Alternatives considered**:

- *Forward Swipe Bearer and hope Immich ignores it*: Already proven broken.
- *Use only cookies Immich sets*: Cookie jar across reverse proxy is fragile and couples to Immich's cookie names/domains.

### Decision 4: Login request discrimination

**Choice**: `POST /api/auth/login` body variants (mutually exclusive priority):

1. `{ "userName": "..." }` → env API key (existing)
2. `{ "apiKey": "...", "serverUrl?": "..." }` → manual API key (existing)
3. `{ "email": "...", "password": "...", "serverUrl?": "..." }` → Immich credential login (new)

If multiple are present, prefer in the order above and return 400 if the combination is ambiguous (e.g. `userName` + `email`). `serverUrl` falls back to `IMMICH_SERVER_URL` when omitted.

**Rationale**: Backward compatible; simple branching; no new route required.

**Alternatives considered**:

- *Separate `/api/auth/login/credentials`*: Cleaner REST, more frontend surface; unnecessary for this app size.
- *Query `?method=`*: Worse than body fields for POST.

### Decision 5: Validation after Immich login

**Choice**: On successful Immich login response, optionally call `GET /api/users/me` with the new Bearer token to confirm the token works and to normalize display name. Prefer Immich `name` (fallback `userEmail`) as `userName` for session + client storage keys.

**Rationale**: Mirrors API-key validation path; ensures proxy auth works before returning success to the UI.

### Decision 6: Logout behavior

**Choice**:

- Always delete the Swipe session (existing).
- Best-effort: if session is access-token mode, call Immich `POST /api/auth/logout` with that Bearer token (if Immich exposes it and accepts it). Failures must not block local logout.
- API-key sessions: no Immich logout call (API keys are not sessions).

**Rationale**: Avoids leaving dangling Immich sessions when possible without making logout fragile.

### Decision 7: Frontend login UX

**Choice**: Single `LoginView` with a mode toggle or tabs:

- **Immich account** (default when no env users force another path): server URL, email, password
- **API key**: existing server URL + API key form

Env multi-user `/select-user` remains for preconfigured keys. Header logout/switch-user behavior unchanged in spirit: clear session and return to login or user select based on env config.

**Rationale**: One page, clear choice; multi-person households use Immich account mode without admin pre-provisioning keys.

**Alternatives considered**:

- *Separate routes `/login/api-key` and `/login/account`*: Extra routing for little gain.
- *Replace API-key UI entirely*: Rejected per product constraint (keep API-key flow).

### Decision 8: Password never logged; minimal error detail

**Choice**: Never log email/password bodies. Map Immich 401 to generic "Invalid email or password". Map Immich "password login disabled" to a distinct user-visible error when detectable. Network failures → "cannot reach Immich server".

**Rationale**: Security and UX clarity.

## Risks / Trade-offs

- **[Risk] Immich password login disabled** → Show clear error; API-key path still works.
- **[Risk] Immich access tokens expire while Swipe session is still valid** → Upstream 401; existing frontend 401 handler clears Swipe session and sends user to login (with `autoLoginBlocked` loop guard). Optionally later: detect upstream 401 and invalidate session earlier.
- **[Risk] Reintroduce Authorization header collision** → Strict director order: strip all client auth headers, then set only the session-appropriate Immich credential. Add a regression test or documented invariant.
- **[Risk] Server URL open-proxy abuse for credential login** → Same class as manual API-key `serverUrl` today. Mitigation (optional follow-up): allowlist / require `IMMICH_SERVER_URL` only. Document that public deployments should pin server URL via env.
- **[Risk] Credentials in transit** → Require HTTPS in production (deployment concern); passwords only sent to Swipe backend over TLS, then Swipe→Immich over TLS.
- **[Trade-off] In-memory Immich access tokens** → Server restart forces re-login (same as today). Acceptable for this app.
- **[Trade-off] No OAuth** → Users on OAuth-only Immich instances still need API keys until a future change.

## Migration Plan

1. Deploy backend that accepts credential login and dual-mode proxy (backward compatible).
2. Deploy frontend with login mode toggle.
3. No env var migration required; existing `IMMICH_API_KEY_*` configs keep working.
4. Rollback: previous image still accepts only API-key login; no data migration.

## Open Questions

- Should credential login be hidden when env users are configured, or always available as an alternate path? **Recommendation**: always available on `/login` (e.g. link from user select: "Sign in with Immich account") so households can mix env keys and personal logins.
- Exact Immich logout endpoint behavior across versions — implement best-effort and tolerate failure.
- Whether to pin `serverUrl` to `IMMICH_SERVER_URL` when set (disallow override) for locked-down deploys — defer unless needed; current manual login already allows override.
