## Why

After the Go backend migration, the app enters an infinite loop between logging in and "session expired". The root cause: the Go reverse proxy forwards the browser's `Authorization: Bearer <session-token>` header to Immich alongside the server-side `x-api-key`. Immich's auth guard checks the `Authorization: Bearer` header **before** `x-api-key`, treats the opaque session token as an invalid Immich user token, and returns 401 — even though the `x-api-key` is valid. The frontend then logs out, the router auto-logs-in again, and the cycle repeats on every API call.

## What Changes

- **FIX**: The Go reverse proxy SHALL strip the `Authorization` header from proxied requests before forwarding to Immich. Immich only needs `x-api-key` (set server-side); the browser's Bearer session token is for the Go backend only.
- **FIX**: The frontend `apiRequest` helper SHALL NOT send the `Authorization: Bearer` header to non-proxy endpoints that don't require it (defense in depth).
- **HARDENING**: The router guard SHALL detect a failed auto-login and stop retrying (break the loop) by falling back to the manual login page instead of re-attempting indefinitely.
- **HARDENING**: The `apiRequest` 401 handler SHALL use router navigation instead of `window.location.href` to avoid full reloads and respect the guard's loop-prevention logic.

## Capabilities

### New Capabilities

- `proxy-header-hygiene`: The Go reverse proxy must sanitize forwarded headers so that browser-only auth artifacts (Bearer session token) are not leaked upstream to Immich, which would confuse Immich's auth guard.

### Modified Capabilities

- `server-side-auth`: The 401-handling and auto-login flow must be hardened to prevent infinite loops when the backend rejects a session (expired, invalid, or upstream 401 misinterpretation).

## Impact

- **MODIFIED**: `server/main.go` — `proxyHandler` Director must delete the `Authorization` header from the outbound request to Immich.
- **MODIFIED**: `src/composables/useImmich.ts` — `apiRequest` 401 handler should redirect via router (not `window.location.href`) and set a flag to prevent the guard from immediately re-attempting auto-login into a loop.
- **MODIFIED**: `src/router/index.ts` — guard must track consecutive auto-login failures and fall back to manual login after one failure (no retry storm).
- **MODIFIED**: `src/stores/auth.ts` — expose a `loginFailed` flag or counter so the guard can consult it.
- No changes to UI components, swipe logic, or stores besides auth/router.