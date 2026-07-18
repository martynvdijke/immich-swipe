import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'immich-swipe-session'

export type LoginMethod = 'env-user' | 'manual' | 'credentials'

export type LoginResult =
  | { ok: true }
  | { ok: false; error: string }

export const useAuthStore = defineStore('auth', () => {
  const sessionToken = ref<string | null>(null)
  const currentUserName = ref<string>('')
  const immichServerUrl = ref<string>('')
  const envUsers = ref<string[]>([])
  const defaultServerUrl = ref<string | null>(null)
  const serverVersion = ref<string>('')
  // Set by the 401 handler / failed auto-login to prevent the router guard
  // from re-attempting auto-login into an infinite loop.
  const autoLoginBlocked = ref(false)

  const isLoggedIn = computed(() => sessionToken.value !== null)

  const authHeader = computed(() => {
    if (!sessionToken.value) return {} as Record<string, string>
    return { 'Authorization': `Bearer ${sessionToken.value}` }
  })

  function init() {
    // Load session from sessionStorage
    try {
      const stored = sessionStorage.getItem(STORAGE_KEY)
      if (stored) {
        const data = JSON.parse(stored)
        sessionToken.value = data.token ?? null
        currentUserName.value = data.userName ?? ''
        immichServerUrl.value = data.serverUrl ?? ''
      }
    } catch {
      sessionStorage.removeItem(STORAGE_KEY)
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
    sessionToken.value = data.token ?? null
    currentUserName.value = data.userName || fallbackUserName
    immichServerUrl.value = data.serverUrl || fallbackServerUrl
    saveSession()
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

  function logout() {
    // Attempt backend logout (best-effort)
    if (sessionToken.value) {
      fetch('/api/auth/logout', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${sessionToken.value}` },
      }).catch(() => {})
    }
    sessionToken.value = null
    currentUserName.value = ''
    immichServerUrl.value = ''
    sessionStorage.removeItem(STORAGE_KEY)
    autoLoginBlocked.value = false
  }

  function saveSession() {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
      token: sessionToken.value,
      userName: currentUserName.value,
      serverUrl: immichServerUrl.value,
    }))
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
    fetchConfig,
    loginWithUser,
    loginManual,
    loginWithCredentials,
    logout,
  }
})
