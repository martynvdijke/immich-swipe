## Context

The Go backend migration introduced a reverse proxy that forwards browser requests to Immich. The browser authenticates to the Go backend with `Authorization: Bearer <session-token>`. The Go proxy's `Director` adds `x-api-key` from the session but does **not** strip the incoming `Authorization` header. Immich's `AuthService.validate()` (confirmed from `immich-app/immich` source) checks auth sources in this priority order:

1. shared link key
2. shared link slug
3. **session token** — sourced from `x-immich-user-token`, `x-immich-session-token`, query `sessionKey`, **`Authorization: Bearer`**, or cookie
4. **api key** — `x-api-key`

Because the Bearer session token (an opaque hex string issued by the Go backend) is checked before `x-api-key`, Immich tries to look it up as an Immich session token, fails, and returns 401 — never reaching the valid `x-api-key`. Every proxied API call therefore 401s, the frontend logs out, the router auto-logs-in again, and the loop repeats.

## Goals / Non-Goals

**Goals:**
- Stop the login loop by ensuring Immich only sees the `x-api-key` header, never the browser's Bearer session token.
- Harden the frontend so that a genuine 401 cannot cause an infinite auto-login retry loop.
- Keep the fix minimal and low-risk — no auth model changes, no new endpoints.

**Non-Goals:**
- Replacing the in-memory session store with a persistent/JWT-based scheme.
- Changing Immich's auth behavior (out of our control).
- Adding refresh-token or sliding-renewal logic for the Go session.
- Touching the swipe/review flow, stores, or UI components.

## Decisions

### Decision 1: Strip `Authorization` header in the Go proxy Director

The `proxyHandler`'s `Director` currently sets `x-api-key` but leaves the incoming `Authorization: Bearer <session-token>` header intact, which `httputil.ReverseProxy` copies through to Immich.

**Change:** In the `Director`, explicitly delete `req.Header.Del("Authorization")` after setting `x-api-key`.

**Alternatives considered:**
- *Strip on the client (don't send `Authorization` to proxy):* Rejected — the Go `authMiddleware` requires the Bearer token to authenticate the browser→backend hop. The header must be present on the browser→Go leg and absent on the Go→Immich leg. Only the proxy Director can remove it at the right boundary.
- *Switch the browser auth to a cookie instead of Bearer:* Rejected as over-engineering for a bug fix; would require CSRF handling and changes to the auth store, middleware, and logout.

**Why this is correct:** Immich's `validate()` only needs `x-api-key` for API-key auth. Removing `Authorization` guarantees Immich falls through to the `apiKey` branch. This is a one-line, surgical fix at the trust boundary.

### Decision 2: Router-based 401 redirect with loop guard

Currently `apiRequest` does `window.location.href = '/login'` on 401, causing a full reload. The router guard then re-runs, sees no session, and (for single-env-user config) immediately auto-logs-in again — re-entering the same broken state.

**Change:**
- Replace `window.location.href = '/login'` with a router push to `/login`.
- Add a `consecutiveLoginFailures` counter (or `autoLoginBlocked` flag) on the auth store. The guard increments it on each failed `loginWithUser`/`loginManual` auto-login and stops auto-logging-in once it reaches a threshold (1 failure is enough to break the loop; the user can still manually retry). The flag resets on a successful manual login or explicit logout.

**Alternatives considered:**
- *Only fix the proxy (Decision 1) and leave the frontend:* Tempting, since the proxy fix alone breaks the loop. But a genuine future 401 (expired session, server restart losing in-memory sessions) would still cause a reload→auto-login→fail→reload loop for single-env-user deployments. The loop guard is cheap defense-in-depth.
- *Redirect to a dedicated "session expired" page:* Rejected — adds a route and UX surface for a rare case; `/login` with the failure flag suppressed is sufficient.

### Decision 3: Keep `apiRequest` sending `Authorization` to the Go backend

The browser must still authenticate to the Go backend's `authMiddleware`. So `apiRequest` continues to attach `Authorization: Bearer <session-token>` for browser→Go calls. The stripping happens only in the Go→Immich hop (Decision 1). No change to `apiRequest`'s header construction is needed for the proxy fix; only its 401 *handler* changes (Decision 2).

## Risks / Trade-offs

- **[Risk] Other upstream auth headers leak similarly** → The Director currently only sets `x-api-key`. We should also consider deleting any `x-immich-user-token` / `x-immich-session-token` headers from the inbound request before forwarding, to prevent the same class of bug. Mitigation: delete all `x-immich-*` and `Authorization` headers in the Director, then set only `x-api-key`.
- **[Risk] Loop guard blocks a legitimate transient failure** → If the Immich server is briefly unreachable during auto-login, the guard would stop retrying and strand the user on `/login`. Mitigation: the threshold is 1, and the user can manually click "Connect" / select a user, which resets the flag on success. Auto-login is a convenience, not a guarantee.
- **[Risk] `window.location.href` removal changes reload semantics** → Some callers may rely on a full reload to clear in-memory state after logout. Mitigation: `authStore.logout()` already clears session state and `sessionStorage`; the router navigation is sufficient. In-memory caches (albums, reviewed) are keyed by user/server and reset via the existing `watch` in `useImmich`.
- **[Trade-off] In-memory session store is still volatile** → A Go server restart logs everyone out (all sessions lost → 401). This is accepted; the loop guard prevents the resulting loop. Persistent sessions are a Non-Goal here.