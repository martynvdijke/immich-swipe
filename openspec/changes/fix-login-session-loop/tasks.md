## 1. Go Proxy Header Hygiene

- [x] 1.1 In `server/main.go` `proxyHandler` Director, delete `Authorization`, `x-immich-user-token`, `x-immich-session-token`, and `x-immich-share-key` from `req.Header` before setting `x-api-key`
- [x] 1.2 Rebuild the Go binary and verify it compiles (`go build ./...` in `server/`)
- [ ] 1.3 Manual smoke test: with a valid session, confirm a proxied `/api/users/me` (or `/search/metadata`) returns 200 instead of 401; confirm the Immich upstream no longer receives the `Authorization` header (check via server logs or a debug request dump)

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
- [ ] 5.3 End-to-end: single env user → app should log in once and load photos without looping; kill the Go server (invalidate session) → next API call 401 → app navigates to `/login` once and stays (no loop)
- [ ] 5.4 End-to-end: multi env user → select a user → loads; simulate 401 → lands on `/select-user` or `/login` without looping
- [ ] 5.5 End-to-end: manual login mode → enter URL + key → loads; simulate 401 → lands on `/login` without looping