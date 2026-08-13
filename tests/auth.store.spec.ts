import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'

const SESSIONS_KEY = 'immich-swipe-sessions'
const ACTIVE_KEY = 'immich-swipe-active-session'
const LEGACY_KEY = 'immich-swipe-session'

describe('auth store loginWithCredentials', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
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
    expect(auth.sessionCount).toBe(1)
    expect(auth.activeSessionKey).toBe('https://immich.example|Display Name')

    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].token).toBe('swipe-session')
    expect(stored[0].userName).toBe('Display Name')
    expect(stored[0].serverUrl).toBe('https://immich.example')
    expect(stored[0].password).toBeUndefined()
    expect(stored[0].accessToken).toBeUndefined()
    expect(localStorage.getItem(ACTIVE_KEY)).toBe('https://immich.example|Display Name')

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
    expect(auth.sessionCount).toBe(0)
    expect(localStorage.getItem(SESSIONS_KEY)).toBeNull()
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
    expect(success.ok).toBe(true)
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

describe('auth store multi-session registry', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    sessionStorage.clear()
  })

  function stubLoginOk(fetchMock: ReturnType<typeof vi.fn>) {
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        const body = JSON.parse(String(init?.body || '{}'))
        const name = body.userName ?? body.email
        return new Response(
          JSON.stringify({
            token: `token-${name}`,
            userName: name,
            serverUrl: 'https://immich.example',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })
  }

  it('keeps both persons logged in after a second login', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    const first = await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')
    expect(first).toEqual({ ok: true })
    expect(auth.currentUserName).toBe('alice@example.com')

    const second = await auth.loginWithCredentials('bob@example.com', 'pw', 'https://immich.example')
    expect(second).toEqual({ ok: true })

    expect(auth.sessionCount).toBe(2)
    expect(auth.currentUserName).toBe('bob@example.com')
    expect(auth.activeSessionKey).toBe('https://immich.example|bob@example.com')

    // Registry persisted with both sessions, no secrets leaked
    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(2)
    expect(stored.map((s: { userName: string }) => s.userName)).toEqual([
      'alice@example.com',
      'bob@example.com',
    ])
    expect(stored[0].password).toBeUndefined()
  })

  it('sessions getter lists persons without exposing tokens', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')
    await auth.loginWithCredentials('bob@example.com', 'pw', 'https://immich.example')

    expect(auth.sessions).toHaveLength(2)
    expect(auth.sessions[0].userName).toBe('alice@example.com')
    expect(auth.sessions[0].key).toBe('https://immich.example|alice@example.com')
    expect('token' in auth.sessions[0]).toBe(false)
  })

  it('switchTo activates another stored session without re-login', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')
    await auth.loginWithCredentials('bob@example.com', 'pw', 'https://immich.example')
    const aliceKey = auth.sessions.find((s) => s.userName === 'alice@example.com')!.key

    auth.switchTo(aliceKey)

    expect(auth.currentUserName).toBe('alice@example.com')
    expect(auth.sessionToken).toBe('token-alice@example.com')
    expect(auth.activeSessionKey).toBe(aliceKey)
    expect(localStorage.getItem(ACTIVE_KEY)).toBe(aliceKey)
    // Both sessions remain stored
    expect(auth.sessionCount).toBe(2)
  })

  it('switchTo ignores unknown keys', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()
    await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')

    auth.switchTo('https://other|nobody')

    expect(auth.currentUserName).toBe('alice@example.com')
  })

  it('upserts: logging in the same person replaces the stored record', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')
    await auth.loginWithCredentials('bob@example.com', 'pw', 'https://immich.example')
    // Alice logs in again -> same identity, new token
    await auth.loginWithCredentials('alice@example.com', 'pw', 'https://immich.example')

    expect(auth.sessionCount).toBe(2)
    const alice = auth.sessions.find((s) => s.userName === 'alice@example.com')!
    expect(alice).toBeTruthy()
    // Latest login wins and becomes active
    expect(auth.activeSessionKey).toBe(alice.key)
    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(2)
  })
})

describe('auth store init', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    vi.unstubAllGlobals()
  })

  it('loads the session registry from localStorage', () => {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([
        { token: 'stored-token', userName: 'Stored User', serverUrl: 'https://immich.example' },
      ]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://immich.example|Stored User')
    // Prevent fetchConfig network call
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
    // init() runs in the constructor; fetchConfig fails silently
    expect(auth.sessionToken).toBe('stored-token')
    expect(auth.currentUserName).toBe('Stored User')
    expect(auth.immichServerUrl).toBe('https://immich.example')
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.sessionCount).toBe(1)
  })

  it('migrates the legacy single session from sessionStorage', () => {
    sessionStorage.setItem(
      LEGACY_KEY,
      JSON.stringify({
        token: 'legacy-token',
        userName: 'Legacy User',
        serverUrl: 'https://immich.example',
      }),
    )
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()

    expect(auth.sessionToken).toBe('legacy-token')
    expect(auth.currentUserName).toBe('Legacy User')
    expect(auth.isLoggedIn).toBe(true)
    // Migrated into the registry and the legacy key was removed
    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].token).toBe('legacy-token')
    expect(sessionStorage.getItem(LEGACY_KEY)).toBeNull()
  })

  it('handles corrupt registry gracefully', () => {
    localStorage.setItem(SESSIONS_KEY, 'not-json-at-all')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
    expect(localStorage.getItem(SESSIONS_KEY)).toBeNull()
  })

  it('handles corrupt legacy storage gracefully', () => {
    sessionStorage.setItem(LEGACY_KEY, 'not-json-at-all')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
    expect(sessionStorage.getItem(LEGACY_KEY)).toBeNull()
  })

  it('handles missing registry entry', () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
    const auth = useAuthStore()
    expect(auth.sessionToken).toBeNull()
    expect(auth.currentUserName).toBe('')
    expect(auth.immichServerUrl).toBe('')
    expect(auth.sessionCount).toBe(0)
  })
})

describe('auth store fetchConfig', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
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
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('authHeader returns Authorization header when logged in', () => {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([{ token: 'my-token', userName: 'u', serverUrl: 'https://s' }]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://s|u')
    const auth = useAuthStore()
    expect(auth.authHeader).toEqual({ Authorization: 'Bearer my-token' })
  })

  it('authHeader returns empty object when not logged in', () => {
    const auth = useAuthStore()
    expect(auth.authHeader).toEqual({})
  })

  it('isLoggedIn reflects active session state', () => {
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(false)

    // Re-seed storage and create a fresh store to re-run init()
    setActivePinia(createPinia())
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([{ token: 't', userName: 'u', serverUrl: 'https://s' }]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://s|u')
    const auth2 = useAuthStore()
    expect(auth2.isLoggedIn).toBe(true)
    expect(auth2.sessionToken).toBe('t')
  })
})

describe('auth store logout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    sessionStorage.clear()
  })

  function seedSessions(): string[] {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([
        { token: 't-alice', userName: 'Alice', serverUrl: 'https://immich' },
        { token: 't-bob', userName: 'Bob', serverUrl: 'https://immich' },
      ]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://immich|Alice')
    const auth = useAuthStore()
    return auth.sessions.map((s) => s.key)
  }

  it('removes only the active session and falls back to another person', () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    // Seed BEFORE the store is created so init() picks the registry up
    seedSessions()
    const auth = useAuthStore()
    expect(auth.currentUserName).toBe('Alice')

    const remaining = auth.logout()

    expect(remaining).toBe(true)
    expect(auth.sessionCount).toBe(1)
    expect(auth.currentUserName).toBe('Bob')
    expect(auth.sessionToken).toBe('t-bob')
    expect(auth.isLoggedIn).toBe(true)
    // Registry persisted without Alice
    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].userName).toBe('Bob')
  })

  it('sends POST /api/auth/logout with the active session token', () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    seedSessions()
    const auth = useAuthStore()

    auth.logout()

    expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
      method: 'POST',
      headers: { Authorization: 'Bearer t-alice' },
    })
  })

  it('last session logout clears everything and reports no remaining', () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([{ token: 't', userName: 'u', serverUrl: 'https://s' }]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://s|u')
    const auth = useAuthStore()

    const remaining = auth.logout()

    expect(remaining).toBe(false)
    expect(auth.sessionToken).toBeNull()
    expect(auth.currentUserName).toBe('')
    expect(auth.immichServerUrl).toBe('')
    expect(auth.autoLoginBlocked).toBe(false)
    expect(auth.isLoggedIn).toBe(false)
    expect(auth.sessionCount).toBe(0)
    expect(localStorage.getItem(SESSIONS_KEY)).toBeNull()
    expect(localStorage.getItem(ACTIVE_KEY)).toBeNull()
  })

  it('logoutSession removes a non-active person and keeps the active one', () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValue(new Response(null, { status: 200 }))

    const keys = seedSessions()
    const auth = useAuthStore()
    const bobKey = keys[1]

    const remaining = auth.logoutSession(bobKey)

    expect(remaining).toBe(true)
    expect(auth.sessionCount).toBe(1)
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.sessionToken).toBe('t-alice')
    expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
      method: 'POST',
      headers: { Authorization: 'Bearer t-bob' },
    })
  })

  it('succeeds when not logged in (no fetch call)', () => {
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

describe('auth store removeActiveSession (401 path)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('removes the dead session and switches to another person without a logout call', () => {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([
        { token: 't-alice', userName: 'Alice', serverUrl: 'https://immich' },
        { token: 't-bob', userName: 'Bob', serverUrl: 'https://immich' },
      ]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://immich|Alice')
    const auth = useAuthStore()

    const remaining = auth.removeActiveSession()

    expect(remaining).toBe(true)
    expect(auth.currentUserName).toBe('Bob')
    expect(auth.sessionToken).toBe('t-bob')
    expect(auth.sessionCount).toBe(1)
    // No backend logout call for an already-dead session
    expect(vi.mocked(fetch)).not.toHaveBeenCalledWith(
      expect.stringContaining('/api/auth/logout'),
      expect.anything(),
    )
  })

  it('returns false when the dead session was the only one', () => {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([{ token: 't', userName: 'u', serverUrl: 'https://s' }]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://s|u')
    const auth = useAuthStore()

    const remaining = auth.removeActiveSession()

    expect(remaining).toBe(false)
    expect(auth.isLoggedIn).toBe(false)
    expect(auth.sessionCount).toBe(0)
  })
})

describe('auth store restoreLastActive', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('restores the first stored session when no active pointer is set', () => {
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([
        { token: 't-alice', userName: 'Alice', serverUrl: 'https://immich' },
        { token: 't-bob', userName: 'Bob', serverUrl: 'https://immich' },
      ]),
    )
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(false)

    auth.restoreLastActive()

    expect(auth.isLoggedIn).toBe(true)
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.sessionToken).toBe('t-alice')
  })

  it('does nothing when no sessions are stored', () => {
    const auth = useAuthStore()
    auth.restoreLastActive()
    expect(auth.isLoggedIn).toBe(false)
    expect(auth.sessionCount).toBe(0)
  })
})

describe('auth store loginManual failures', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
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
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
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
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toBe('unknown user')
    }
    expect(auth.sessionToken).toBeNull()
  })

  it('surfaces the password_required code when an account password is set', async () => {
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
        return new Response(JSON.stringify({ error: 'password required', code: 'password_required' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithUser('Alice')
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.code).toBe('password_required')
    }
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
    expect(result.ok).toBe(false)
    expect(auth.sessionToken).toBeNull()
  })
})

describe('auth store local accounts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('loginWithAccount logs in and stores the apiKey session mode', async () => {
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
            token: 'account-session',
            userName: 'Alice',
            serverUrl: 'https://immich.example',
            mode: 'apiKey',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithAccount('Alice', 'secret123', 'https://immich.example')

    expect(result).toEqual({ ok: true })
    expect(auth.sessionToken).toBe('account-session')
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.activeSessionMode).toBe('apiKey')
    expect(auth.isLoggedIn).toBe(true)

    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({
      userName: 'Alice',
      password: 'secret123',
      serverUrl: 'https://immich.example',
    })
  })

  it('loginWithAccount surfaces backend errors and their codes', async () => {
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
          JSON.stringify({ error: 'invalid password', code: 'invalid_password' }),
          { status: 401, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithAccount('Alice', 'wrong', 'https://immich.example')

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.code).toBe('invalid_password')
      expect(result.error).toBe('invalid password')
    }
    expect(auth.isLoggedIn).toBe(false)
  })

  it('setAccountPassword posts to /api/auth/account with the auth header', async () => {
    const fetchMock = vi.mocked(fetch)
    // Seed a stored session so the store is active without a login round-trip.
    localStorage.setItem(
      SESSIONS_KEY,
      JSON.stringify([{ token: 't-alice', userName: 'Alice', serverUrl: 'https://immich.example' }]),
    )
    localStorage.setItem(ACTIVE_KEY, 'https://immich.example|Alice')
    setActivePinia(createPinia())

    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/account')) {
        return new Response(JSON.stringify({ status: 'ok' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.setAccountPassword('oldpass', 'newpass123')

    expect(result).toEqual({ ok: true })

    const accountCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/account'))
    expect(accountCall).toBeTruthy()
    const init = accountCall?.[1] as unknown as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({ currentPassword: 'oldpass', password: 'newpass123' })
    expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer t-alice')
  })

  it('setAccountPassword omits currentPassword when empty and reports errors', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/account')) {
        return new Response(
          JSON.stringify({ error: 'password must be at least 8 characters', code: 'weak_password' }),
          { status: 400, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.setAccountPassword('', 'short')

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.code).toBe('weak_password')
    }
    const accountCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/account'))
    const init = accountCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({ password: 'short' })
  })
})

describe('auth store session persistence', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('loginWithCredentials does not store password in localStorage', async () => {
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
            token: 'session-x',
            userName: 'User',
            serverUrl: 'https://immich.example',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    await auth.loginWithCredentials('a@b.com', 'supersecret', 'https://immich.example')

    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].password).toBeUndefined()
  })
})
