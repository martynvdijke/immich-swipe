## 1. Backend session model

- [x] 1.1 Extend `Session` in `server/main.go` with credential mode (`apiKey` | `accessToken`), `AccessToken` field, and optional `UserEmail` / `UserID`
- [x] 1.2 Update `SessionStore.Create` (or add overload) to accept mode-specific credentials without overloading the API key field
- [x] 1.3 Ensure session get/delete/cleanup behavior is unchanged for expiry and sliding TTL

## 2. Backend credential login

- [x] 2.1 Extend `loginHandler` request parsing to accept `{ email, password, serverUrl? }` and reject ambiguous combinations (`email`+`apiKey`, `email`+`userName`)
- [x] 2.2 Implement Immich password login client: `POST {server}/api/auth/login` with `{ email, password }`, parse `accessToken`, `name`, `userEmail`, `userId`
- [x] 2.3 On Immich login success, validate token via `GET {server}/api/users/me` with `Authorization: Bearer <accessToken>`
- [x] 2.4 Create access-token-mode session; return `{ token, userName, serverUrl }` without secrets
- [x] 2.5 Map Immich errors: invalid credentials → 401; password login disabled → clear 4xx; network → 500 with safe message; never log password
- [x] 2.6 Keep existing `userName` and `apiKey` login branches creating API-key-mode sessions

## 3. Backend proxy and logout

- [x] 3.1 Update proxy `Director` to strip client auth headers, then attach `x-api-key` OR Immich Bearer based on session mode (never forward Swipe session token)
- [x] 3.2 Add regression coverage or a focused unit/integration check that API-key mode still strips browser `Authorization`
- [x] 3.3 Implement `POST /api/auth/logout` if missing: delete Swipe session; for access-token mode best-effort call Immich logout; always succeed locally
- [x] 3.4 Wire logout route in `ServeHTTP` if not already registered

## 4. Frontend auth store

- [x] 4.1 Add `loginWithCredentials(email, password, serverUrl)` in `src/stores/auth.ts` posting to `/api/auth/login`
- [x] 4.2 Reuse existing session save path (`sessionToken`, `currentUserName`, `immichServerUrl`, `sessionStorage`)
- [x] 4.3 Reset `autoLoginBlocked` on successful credential login (same as other login methods)
- [x] 4.4 Extend `LoginMethod` type if used for UI/analytics (`'credentials'` alongside env/manual)

## 5. Frontend login UI

- [x] 5.1 Update `LoginView.vue` with mode toggle/tabs: Immich account (email/password) and API key
- [x] 5.2 Prefill server URL from `defaultServerUrl` / existing session server URL for both modes
- [x] 5.3 Wire submit handlers, loading state, and distinct error messages for credential vs API-key failures
- [x] 5.4 From `UserSelectView.vue`, add optional link/button to Immich account login on `/login` so non-env users can still sign in
- [x] 5.5 Verify header logout still clears session and routes correctly for credential sessions

## 6. Docs and config notes

- [x] 6.1 Document Immich account login in `README.md` (password login must be enabled on Immich; API-key path remains)
- [x] 6.2 Note in `env.example` that env API keys are optional when using credential login
- [x] 6.3 Update `AGENTS.md` auth section to mention credential login + dual proxy auth modes

## 7. Tests and verification

- [x] 7.1 Add/adjust frontend tests for `loginWithCredentials` success and failure handling
- [x] 7.2 Add Go tests (or manual test checklist) for login body variants, ambiguous body 400, and proxy header attachment per mode
- [x] 7.3 Run `npm run type-check` and relevant unit tests
- [x] 7.4 Manual E2E: credential login → swipe actions work; logout → second user credential login → isolated reviewed/prefs keys; API-key login still works; env multi-user still works
- [x] 7.5 Manual regression: single env user auto-login does not loop; 401 still lands on login once
