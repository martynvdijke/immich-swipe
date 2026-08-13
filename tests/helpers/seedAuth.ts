/**
 * Test helpers for seeding a logged-in auth state.
 *
 * The auth store's sessionToken/currentUserName/immichServerUrl are computed
 * from the ACTIVE session in the multi-session registry, so tests must seed
 * localStorage BEFORE the first useAuthStore() call (init() reads the
 * registry at store creation).
 */

export interface SeedSession {
  token: string
  userName: string
  serverUrl: string
  /** Session mode: 'apiKey' | 'accessToken' (optional). */
  mode?: string
}

const SESSIONS_KEY = 'immich-swipe-sessions'
const ACTIVE_KEY = 'immich-swipe-active-session'

export function sessionKey(serverUrl: string, userName: string): string {
  return `${serverUrl || 'unknown-server'}|${userName || 'default-user'}`
}

/** Seed a single logged-in session and make it active. */
export function seedAuthSession(token: string, userName: string, serverUrl: string, mode?: string) {
  localStorage.setItem(
    SESSIONS_KEY,
    JSON.stringify(mode ? [{ token, userName, serverUrl, mode }] : [{ token, userName, serverUrl }]),
  )
  localStorage.setItem(ACTIVE_KEY, sessionKey(serverUrl, userName))
}

/** Seed multiple sessions; `activeKey` selects the active one. */
export function seedAuthSessions(sessions: SeedSession[], activeKey: string) {
  localStorage.setItem(SESSIONS_KEY, JSON.stringify(sessions))
  localStorage.setItem(ACTIVE_KEY, activeKey)
}
