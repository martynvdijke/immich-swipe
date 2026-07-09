import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'immich-swipe-session'

export type LoginMethod = 'env-user' | 'manual'

export const useAuthStore = defineStore('auth', () => {
  const sessionToken = ref<string | null>(null)
  const currentUserName = ref<string>('')
  const immichServerUrl = ref<string>('')
  const envUsers = ref<string[]>([])
  const defaultServerUrl = ref<string | null>(null)

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
      }
    } catch {
      // Backend not available yet
    }
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
      sessionToken.value = data.token
      currentUserName.value = data.userName || userName
      immichServerUrl.value = data.serverUrl || ''
      saveSession()
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
      sessionToken.value = data.token
      currentUserName.value = data.userName || 'manual'
      immichServerUrl.value = serverUrl
      saveSession()
      return true
    } catch {
      return false
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
    isLoggedIn,
    authHeader,
    fetchConfig,
    loginWithUser,
    loginManual,
    logout,
  }
})
