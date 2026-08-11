import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

const SESSIONS_STORAGE_KEY = 'immich-swipe-sessions'
const ACTIVE_SESSION_KEY = 'immich-swipe-active-session'
const LEGACY_STORAGE_KEY = 'immich-swipe-session'

export type LoginMethod = 'env-user' | 'manual' | 'credentials'

export type LoginResult =
  | { ok: true }
  | { ok: false; error: string }

interface SessionRecord {
  token: string
  userName: string
  serverUrl: string
}

export interface SessionInfo {
  userName: string
  serverUrl: string
  /** Stable identity of the session (`serverUrl|userName`), matching the
   *  normalization used by the per-user storage keys. */
  key: string
}

function sessionKey(serverUrl: string, userName: string): string {
  const server = serverUrl || 'unknown-server'
  const user = userName || 'default-user'
  return `${server}|${user}`
}

export const useAuthStore = defineStore('auth', () => {
  // Registry of all logged-in persons (localStorage persisted).
  const sessionRecords = ref<SessionRecord[]>([])
  // Identity of the active session; null when nobody is active.
  const activeSessionKey = ref<string | null>(null)

  const envUsers = ref<string[]>([])
  const defaultServerUrl = ref<string | null>(null)
  const serverVersion = ref<string>('')
  // Set by the 401 handler / failed auto-login to prevent the router guard
  // from re-attempting auto-login into an infinite loop.
  const autoLoginBlocked = ref(false)

  const activeSession = computed<SessionRecord | null>(() => {
    if (!activeSessionKey.value) return null
    return (
      sessionRecords.value.find(
        (record) => sessionKey(record.serverUrl, record.userName) === activeSessionKey.value
      ) ?? null
    )
  })

  // Public surface stays stable: these reflect the ACTIVE session, so all
  // existing consumers (apiRequest, per-user stores, header, views) keep
  // working and re-react on identity change.
  const sessionToken = computed(() => activeSession.value?.token ?? null)
  const currentUserName = computed(() => activeSession.value?.userName ?? '')
  const immichServerUrl = computed(() => activeSession.value?.serverUrl ?? '')

  const isLoggedIn = computed(() => activeSession.value !== null)

  const sessionCount = computed(() => sessionRecords.value.length)

  // Public list of logged-in persons WITHOUT tokens (safe for components).
  const sessions = computed<SessionInfo[]>(() =>
    sessionRecords.value.map((record) => ({
      userName: record.userName,
      serverUrl: record.serverUrl,
      key: sessionKey(record.serverUrl, record.userName),
    }))
  )

  const authHeader = computed(() => {
    if (!sessionToken.value) return {} as Record<string, string>
    return { 'Authorization': `Bearer ${sessionToken.value}` }
  })

  function persistSessions() {
    if (sessionRecords.value.length === 0) {
      localStorage.removeItem(SESSIONS_STORAGE_KEY)
    } else {
      localStorage.setItem(SESSIONS_STORAGE_KEY, JSON.stringify(sessionRecords.value))
    }
    if (activeSessionKey.value) {
      localStorage.setItem(ACTIVE_SESSION_KEY, activeSessionKey.value)
    } else {
      localStorage.removeItem(ACTIVE_SESSION_KEY)
    }
  }

  function removeRecord(key: string): void {
    const index = sessionRecords.value.findIndex(
      (record) => sessionKey(record.serverUrl, record.userName) === key
    )
    if (index === -1) return
    sessionRecords.value.splice(index, 1)
    if (activeSessionKey.value === key) {
      // Fall back to the next remaining session (first), or none.
      activeSessionKey.value =
        sessionRecords.value.length > 0
          ? sessionKey(sessionRecords.value[0].serverUrl, sessionRecords.value[0].userName)
          : null
    }
    persistSessions()
  }

  function init() {
    // 1) Load the multi-session registry from localStorage.
    try {
      const stored = localStorage.getItem(SESSIONS_STORAGE_KEY)
      if (stored) {
        const parsed = JSON.parse(stored)
        if (Array.isArray(parsed)) {
          sessionRecords.value = parsed.filter(
            (item): item is SessionRecord =>
              item &&
              typeof item.token === 'string' &&
              typeof item.userName === 'string' &&
              typeof item.serverUrl === 'string'
          )
        }
      }
    } catch {
      localStorage.removeItem(SESSIONS_STORAGE_KEY)
    }

    // 2) Migrate the legacy single-session key (sessionStorage) if the
    //    registry is empty. The migrated session becomes active.
    if (sessionRecords.value.length === 0) {
      try {
        const legacy = sessionStorage.getItem(LEGACY_STORAGE_KEY)
        if (legacy) {
          const data = JSON.parse(legacy)
          if (data?.token) {
            sessionRecords.value = [
              {
                token: data.token,
                userName: data.userName ?? '',
                serverUrl: data.serverUrl ?? '',
              },
            ]
            activeSessionKey.value = sessionKey(data.serverUrl ?? '', data.userName ?? '')
          }
          sessionStorage.removeItem(LEGACY_STORAGE_KEY)
        }
      } catch {
        sessionStorage.removeItem(LEGACY_STORAGE_KEY)
      }
    }

    // 3) Restore the persisted active pointer (validated against the
    //    registry). A missing pointer leaves the app inactive — the router
    //    guard calls restoreLastActive() to activate the first session.
    try {
      const active = localStorage.getItem(ACTIVE_SESSION_KEY)
      if (
        active &&
        sessionRecords.value.some(
          (record) => sessionKey(record.serverUrl, record.userName) === active
        )
      ) {
        activeSessionKey.value = active
      }
    } catch {
      // ignore corrupt pointer
    }

    // Persist any migration so localStorage matches the in-memory registry.
    if (sessionRecords.value.length > 0) {
      persistSessions()
    }

    // Fetch backend config for env-configured users
    fetchConfig()
  }

  async function fetchConfig() {
    try {
      const res = await fetch('/api/auth/config')
      if (res.ok) {
        const data = await res.json()
        envUsers.value = data.users || []
        defaultServerUrl.value = data.defaultServerUrl || null
        serverVersion.value = data.version || ''
      }
    } catch {
      // Backend not available yet
    }
  }

  function applyLoginSuccess(data: { token?: string; userName?: string; serverUrl?: string }, fallbackUserName: string, fallbackServerUrl = '') {
    const token = data.token ?? null
    const userName = data.userName || fallbackUserName
    const serverUrl = data.serverUrl || fallbackServerUrl
    if (!token) return

    const key = sessionKey(serverUrl, userName)
    const existing = sessionRecords.value.findIndex(
      (record) => sessionKey(record.serverUrl, record.userName) === key
    )
    const record: SessionRecord = { token, userName, serverUrl }
    if (existing >= 0) {
      // Upsert: same person on same server -> replace record in place.
      sessionRecords.value[existing] = record
    } else {
      sessionRecords.value.push(record)
    }
    activeSessionKey.value = key
    persistSessions()
    autoLoginBlocked.value = false
  }

  async function parseLoginError(res: Response, fallback: string): Promise<string> {
    try {
      const data = await res.json()
      if (data?.error && typeof data.error === 'string') {
        return data.error
      }
    } catch {
      // ignore
    }
    return fallback
  }

  async function loginWithUser(userName: string): Promise<boolean> {
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ userName }),
      })
      if (!res.ok) return false
      const data = await res.json()
      applyLoginSuccess(data, userName)
      return true
    } catch {
      return false
    }
  }

  async function loginManual(apiKey: string, serverUrl: string): Promise<boolean> {
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ apiKey, serverUrl }),
      })
      if (!res.ok) return false
      const data = await res.json()
      applyLoginSuccess(data, 'manual', serverUrl)
      return true
    } catch {
      return false
    }
  }

  async function loginWithCredentials(email: string, password: string, serverUrl: string): Promise<LoginResult> {
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, serverUrl }),
      })
      if (!res.ok) {
        const fallback =
          res.status === 401
            ? 'Invalid email or password'
            : res.status === 403
              ? 'Password login is disabled on this Immich server'
              : 'Failed to connect. Please check your server URL and credentials.'
        return { ok: false, error: await parseLoginError(res, fallback) }
      }
      const data = await res.json()
      applyLoginSuccess(data, email, serverUrl)
      return { ok: true }
    } catch {
      return { ok: false, error: 'Cannot reach server. Please try again.' }
    }
  }

  /** Activate a stored session without any credential re-entry. */
  function switchTo(key: string): void {
    if (
      !sessionRecords.value.some(
        (record) => sessionKey(record.serverUrl, record.userName) === key
      )
    ) {
      return
    }
    activeSessionKey.value = key
    persistSessions()
    autoLoginBlocked.value = false
  }

  /** Restore the last active (else first) stored session; used on reload. */
  function restoreLastActive(): void {
    if (sessionRecords.value.length === 0 || activeSession.value) return
    activeSessionKey.value = sessionKey(
      sessionRecords.value[0].serverUrl,
      sessionRecords.value[0].userName
    )
    persistSessions()
  }

  /**
   * Log out the active person only: best-effort backend logout, removal from
   * the registry, automatic fallback to the next remaining session.
   * Returns true when other sessions remain.
   */
  function logout(): boolean {
    const active = activeSession.value
    if (active) {
      fetch('/api/auth/logout', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${active.token}` },
      }).catch(() => {})
      removeRecord(activeSessionKey.value as string)
    }
    autoLoginBlocked.value = false
    return sessionRecords.value.length > 0
  }

  /** Log out a specific (possibly non-active) person. Returns true when
   *  other sessions remain. */
  function logoutSession(key: string): boolean {
    const record = sessionRecords.value.find(
      (item) => sessionKey(item.serverUrl, item.userName) === key
    )
    if (record) {
      fetch('/api/auth/logout', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${record.token}` },
      }).catch(() => {})
      removeRecord(key)
    }
    return sessionRecords.value.length > 0
  }

  /**
   * Invalidate the active session after a 401 (no backend call needed — the
   * session is already dead). Removes it, falls back to the next remaining
   * session. Returns true when other sessions remain.
   */
  function removeActiveSession(): boolean {
    if (activeSessionKey.value) {
      removeRecord(activeSessionKey.value)
    }
    return sessionRecords.value.length > 0
  }

  // Initialize on store creation
  init()

  return {
    sessionToken,
    currentUserName,
    immichServerUrl,
    envUsers,
    defaultServerUrl,
    serverVersion,
    autoLoginBlocked,
    isLoggedIn,
    authHeader,
    sessions,
    sessionCount,
    activeSessionKey,
    fetchConfig,
    loginWithUser,
    loginManual,
    loginWithCredentials,
    switchTo,
    restoreLastActive,
    logout,
    logoutSession,
    removeActiveSession,
  }
})
