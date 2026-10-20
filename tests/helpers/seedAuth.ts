/**
 * Test helpers for seeding a logged-in auth state.
 *
 * The session itself lives in an HttpOnly cookie, so tests only seed the
 * non-secret identity registry. Tokens are accepted for call-site
 * compatibility and deliberately not persisted.
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
export function seedAuthSession(_token: string, userName: string, serverUrl: string, mode?: string) {
  localStorage.setItem(
    SESSIONS_KEY,
    JSON.stringify(mode ? [{ userName, serverUrl, mode }] : [{ userName, serverUrl }]),
  )
  localStorage.setItem(ACTIVE_KEY, sessionKey(serverUrl, userName))
}

/** Seed multiple sessions; `activeKey` selects the active one. */
export function seedAuthSessions(sessions: SeedSession[], activeKey: string) {
  localStorage.setItem(
    SESSIONS_KEY,
    JSON.stringify(sessions.map(({ userName, serverUrl, mode }) => ({ userName, serverUrl, mode }))),
  )
  localStorage.setItem(ACTIVE_KEY, activeKey)
}
