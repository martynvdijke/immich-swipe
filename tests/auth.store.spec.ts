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

  it('loginWithCredentials fallback error for unknown status codes', async () => {
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
        return new Response(JSON.stringify({}), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        })
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
      expect(result.error).toContain('server URL')
    }
  })
})

describe('auth store init', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    sessionStorage.clear()
    vi.unstubAllGlobals()
  })

  it('loads session from sessionStorage', () => {
    sessionStorage.setItem(
      'immich-swipe-session',
      JSON.stringify({
        token: 'stored-token',
        userName: 'Stored User',
        serverUrl: 'https://immich.example',
      }),
    )
    // Prevent fetchConfig network call
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
    // init() runs in the constructor; fetchConfig fails silently
    expect(auth.sessionToken).toBe('stored-token')
    expect(auth.currentUserName).toBe('Stored User')
    expect(auth.immichServerUrl).toBe('https://immich.example')
    expect(auth.isLoggedIn).toBe(true)
  })

  it('handles corrupt sessionStorage gracefully', () => {
    sessionStorage.setItem('immich-swipe-session', 'not-json-at-all')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
    expect(sessionStorage.getItem('immich-swipe-session')).toBeNull()
  })

  it('handles missing sessionStorage entry', () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
    const auth = useAuthStore()
    expect(auth.sessionToken).toBeNull()
    expect(auth.currentUserName).toBe('')
    expect(auth.immichServerUrl).toBe('')
  })
})

describe('auth store fetchConfig', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('stores config data on success', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          users: ['Alice', 'Bob'],
          defaultServerUrl: 'https://default.immich',
          version: 'v1.2.3',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    const auth = useAuthStore()
    // Wait for init() -> fetchConfig()
    await vi.waitFor(() => {
      expect(auth.envUsers).toEqual(['Alice', 'Bob'])
    })
    expect(auth.defaultServerUrl).toBe('https://default.immich')
    expect(auth.serverVersion).toBe('v1.2.3')
  })

  it('handles fetchConfig network failure gracefully', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockRejectedValue(new TypeError('Network error'))

    const auth = useAuthStore()
    // Wait for init() -> fetchConfig() to settle
    await vi.waitFor(() => {
      // fetchConfig catches silently, envUsers stays empty
      expect(auth.envUsers).toEqual([])
    })
    expect(auth.defaultServerUrl).toBeNull()
    expect(auth.serverVersion).toBe('')
  })
})

describe('auth store computed properties', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  it('authHeader returns Authorization header when logged in', () => {
    const auth = useAuthStore()
    auth.sessionToken = 'my-token'
    expect(auth.authHeader).toEqual({ Authorization: 'Bearer my-token' })
  })

  it('authHeader returns empty object when not logged in', () => {
    const auth = useAuthStore()
    auth.sessionToken = null
    expect(auth.authHeader).toEqual({})
  })

  it('isLoggedIn reflects sessionToken state', () => {
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(false)
    auth.sessionToken = 't'
    expect(auth.isLoggedIn).toBe(true)
    auth.sessionToken = null
    expect(auth.isLoggedIn).toBe(false)
  })
})

describe('auth store logout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  it('clears all state and sessionStorage on logout', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    // Set up logged-in state
    const auth = useAuthStore()
    sessionStorage.setItem(
      'immich-swipe-session',
      JSON.stringify({
        token: 't',
        userName: 'u',
        serverUrl: 'https://s',
      }),
    )
    auth.sessionToken = 't'
    auth.currentUserName = 'u'
    auth.immichServerUrl = 'https://s'

    auth.logout()

    expect(auth.sessionToken).toBeNull()
    expect(auth.currentUserName).toBe('')
    expect(auth.immichServerUrl).toBe('')
    expect(auth.autoLoginBlocked).toBe(false)
    expect(auth.isLoggedIn).toBe(false)
    expect(sessionStorage.getItem('immich-swipe-session')).toBeNull()
  })

  it('sends POST /api/auth/logout with auth header when logged in', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    const auth = useAuthStore()
    auth.sessionToken = 'my-session'

    auth.logout()

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
      method: 'POST',
      headers: { Authorization: 'Bearer my-session' },
    })
  })

  it('succeeds when not logged in (no fetch call)', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    const auth = useAuthStore()
    auth.logout()
    const logoutCalls = fetchMock.mock.calls.filter(([url]) =>
      String(url).includes('/api/auth/logout'),
    )
    expect(logoutCalls.length).toBe(0)
  })
})

describe('auth store loginManual failures', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  it('returns false on 401', async () => {
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
        return new Response(JSON.stringify({ error: 'bad key' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginManual('bad-key', 'https://immich.example')
    expect(result).toBe(false)
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
  })

  it('returns false on network error', async () => {
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
    const result = await auth.loginManual('key', 'https://immich.example')
    expect(result).toBe(false)
    expect(auth.sessionToken).toBeNull()
  })
})

describe('auth store loginWithUser failures', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  it('returns false on backend failure', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: ['Alice'] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(JSON.stringify({ error: 'unknown user' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithUser('Alice')
    expect(result).toBe(false)
    expect(auth.sessionToken).toBeNull()
  })

  it('returns false on network error', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: ['Alice'] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      throw new TypeError('Network error')
    })

    const auth = useAuthStore()
    const result = await auth.loginWithUser('Alice')
    expect(result).toBe(false)
    expect(auth.sessionToken).toBeNull()
  })
})

describe('auth store saveSession', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  it('loginWithCredentials does not store password in sessionStorage', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          token: 'session-x',
          userName: 'User',
          serverUrl: 'https://immich.example',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    const auth = useAuthStore()
    await auth.loginWithCredentials('a@b.com', 'supersecret', 'https://immich.example')

    const stored = JSON.parse(sessionStorage.getItem('immich-swipe-session') || '{}')
    expect(stored.password).toBeUndefined()
  })
})
