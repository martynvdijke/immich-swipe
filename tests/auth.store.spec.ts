import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'

describe('auth store loginWithCredentials', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('stores session token on success and clears autoLoginBlocked', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null, version: 'test' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(
          JSON.stringify({
            token: 'swipe-session',
            userName: 'Display Name',
            serverUrl: 'https://immich.example',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    auth.autoLoginBlocked = true

    const result = await auth.loginWithCredentials(
      'user@example.com',
      'secret',
      'https://immich.example',
    )

    expect(result).toEqual({ ok: true })
    expect(auth.sessionToken).toBe('swipe-session')
    expect(auth.currentUserName).toBe('Display Name')
    expect(auth.immichServerUrl).toBe('https://immich.example')
    expect(auth.autoLoginBlocked).toBe(false)
    expect(auth.isLoggedIn).toBe(true)

    const stored = JSON.parse(sessionStorage.getItem('immich-swipe-session') || '{}')
    expect(stored.token).toBe('swipe-session')
    expect(stored.userName).toBe('Display Name')
    expect(stored.serverUrl).toBe('https://immich.example')
    expect(stored.password).toBeUndefined()
    expect(stored.accessToken).toBeUndefined()

    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    expect(loginCall).toBeTruthy()
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({
      email: 'user@example.com',
      password: 'secret',
      serverUrl: 'https://immich.example',
    })
  })

  it('returns backend error message on failure without creating a session', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(JSON.stringify({ error: 'invalid email or password' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithCredentials(
      'user@example.com',
      'wrong',
      'https://immich.example',
    )

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toBe('invalid email or password')
    }
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
    expect(sessionStorage.getItem('immich-swipe-session')).toBeNull()
  })

  it('loginManual still works alongside credentials', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(
          JSON.stringify({
            token: 'manual-session',
            userName: 'manual',
            serverUrl: 'https://immich.example',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const success = await auth.loginManual('valid-key', 'https://immich.example')
    expect(success).toBe(true)
    expect(auth.sessionToken).toBe('manual-session')
    expect(auth.isLoggedIn).toBe(true)
  })

  it('loginWithUser still works', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(
          JSON.stringify({ users: ['Alice'], defaultServerUrl: null }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      if (url.includes('/api/auth/login')) {
        return new Response(
          JSON.stringify({
            token: 'env-session',
            userName: 'Alice',
            serverUrl: 'https://immich.example',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const success = await auth.loginWithUser('Alice')
    expect(success).toBe(true)
    expect(auth.sessionToken).toBe('env-session')
    expect(auth.currentUserName).toBe('Alice')
  })

  it('handles network error in loginWithCredentials', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      throw new TypeError('Network error')
    })

    const auth = useAuthStore()
    const result = await auth.loginWithCredentials(
      'user@example.com',
      'secret',
      'https://immich.example',
    )

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toBeTruthy()
    }
    expect(auth.isLoggedIn).toBe(false)
  })

  it('maps password-login-disabled style failures', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(
          JSON.stringify({ error: 'password login is disabled on this Immich server' }),
          { status: 403, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithCredentials(
      'user@example.com',
      'secret',
      'https://immich.example',
    )

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error.toLowerCase()).toContain('password login')
    }
  })
})
