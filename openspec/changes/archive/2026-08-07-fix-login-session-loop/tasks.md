## 1. Go Proxy Header Hygiene

- [x] 1.1 In `server/main.go` `proxyHandler` Director, delete `Authorization`, `x-immich-user-token`, `x-immich-session-token`, and `x-immich-share-key` from `req.Header` before setting `x-api-key`
- [x] 1.2 Rebuild the Go binary and verify it compiles (`go build ./...` in `server/`)
- [x] 1.3 Proxy header hygiene smoke test (automated): `TestProxyHandler_StripsClientAuthHeaders` in `server/main_test.go` confirms a proxied request returns 200 and the Immich upstream receives only the server-side `x-api-key` — no client `Authorization` or `x-immich-*` headers

## 2. Frontend 401 Handling

- [x] 2.1 In `src/composables/useImmich.ts` `apiRequest`, replace `window.location.href = '/login'` with a router-based navigation (import the router instance) and set `authStore.autoLoginBlocked = true` before navigating
- [x] 2.2 Ensure `authStore.logout()` is still called to clear the session token and `sessionStorage` before the redirect

## 3. Auth Store Loop Guard

- [x] 3.1 In `src/stores/auth.ts`, add an `autoLoginBlocked` ref (default `false`) and expose it
- [x] 3.2 Reset `autoLoginBlocked = false` inside `loginWithUser` and `loginManual` on success (before returning `true`)
- [x] 3.3 Reset `autoLoginBlocked = false` inside `logout()`

## 4. Router Guard Loop Prevention

- [x] 4.1 In `src/router/index.ts`, before each auto-login attempt (`loginWithUser` for single env user), check `authStore.autoLoginBlocked`; if true, skip auto-login and navigate to `/login`
- [x] 4.2 On a failed auto-login, set `authStore.autoLoginBlocked = true` and navigate to `/login` instead of retrying or staying
- [x] 4.3 On the `/select-user` path, if `autoLoginBlocked` is true, still allow the user-selection page to render (manual selection is the recovery path); do not auto-login from the guard while blocked

## 5. Verification

- [x] 5.1 Run `npm run type-check` and fix any TypeScript errors
- [x] 5.2 Run `npm run build` and verify the frontend builds
- [x] 5.3 No-loop E2E (automated): `tests/router.guard.spec.ts` — single env user auto-logs-in once; with `autoLoginBlocked` set it skips auto-login and stays on `/login`; a failed auto-login trips the guard and a second navigation does not retry
- [x] 5.4 No-loop E2E (automated): `tests/router.guard.spec.ts` — multi env user keeps `/select-user` reachable even while `autoLoginBlocked` is set
- [x] 5.5 No-loop E2E (automated): `tests/router.guard.spec.ts` — manual login mode (no env users) stays on `/login` and never auto-logs-in