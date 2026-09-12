import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'

const SESSIONS_KEY = 'immich-swipe-sessions'
const ACTIVE_KEY = 'immich-swipe-active-session'
const LEGACY_KEY = 'immich-swipe-session'

describe('auth store loginWithAccount', () => {
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

    const result = await auth.loginWithAccount(
      'Display Name',
      'secret',
      'https://immich.example',
    )

    expect(result).toEqual({ ok: true, needsApiKey: false })
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
      userName: 'Display Name',
      password: 'secret',
      serverUrl: 'https://immich.example',
    })
  })

  it('returns needsApiKey true when hasApiKey is false', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(JSON.stringify({ token: 't', userName: 'Alice', serverUrl: 'https://immich.example', hasApiKey: false }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      }
      return new Response('not found', { status: 404 })
    })
    const auth = useAuthStore()
    const result = await auth.loginWithAccount('Alice', 'secret123', 'https://immich.example')
    expect(result).toEqual({ ok: true, needsApiKey: true })
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
        return new Response(JSON.stringify({ error: 'invalid password', code: 'invalid_password' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithAccount(
      'Alice',
      'wrong',
      'https://immich.example',
    )

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toBe('invalid password')
      expect(result.code).toBe('invalid_password')
    }
    expect(auth.sessionToken).toBeNull()
    expect(auth.isLoggedIn).toBe(false)
    expect(auth.sessionCount).toBe(0)
    expect(localStorage.getItem(SESSIONS_KEY)).toBeNull()
  })

  it('handles network error in loginWithAccount', async () => {
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
    const result = await auth.loginWithAccount(
      'Alice',
      'secret',
      'https://immich.example',
    )

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toBeTruthy()
    }
    expect(auth.isLoggedIn).toBe(false)
  })

  it('loginWithAccount fallback error for 500', async () => {
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
    const result = await auth.loginWithAccount(
      'Alice',
      'secret',
      'https://immich.example',
    )
    expect(result.ok).toBe(false)
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
        const name = body.userName
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

    const first = await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')
    expect(first.ok).toBe(true)
    expect(auth.currentUserName).toBe('alice')

    const second = await auth.loginWithAccount('bob', 'pw123456', 'https://immich.example')
    expect(second.ok).toBe(true)

    expect(auth.sessionCount).toBe(2)
    expect(auth.currentUserName).toBe('bob')
    expect(auth.activeSessionKey).toBe('https://immich.example|bob')

    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(2)
    expect(stored.map((s: { userName: string }) => s.userName)).toEqual(['alice', 'bob'])
    expect(stored[0].password).toBeUndefined()
  })

  it('sessions getter lists persons without exposing tokens', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')
    await auth.loginWithAccount('bob', 'pw123456', 'https://immich.example')

    expect(auth.sessions).toHaveLength(2)
    expect(auth.sessions[0].userName).toBe('alice')
    expect(auth.sessions[0].key).toBe('https://immich.example|alice')
    expect('token' in auth.sessions[0]).toBe(false)
  })

  it('switchTo activates another stored session without re-login', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')
    await auth.loginWithAccount('bob', 'pw123456', 'https://immich.example')
    const aliceKey = auth.sessions.find((s) => s.userName === 'alice')!.key

    auth.switchTo(aliceKey)

    expect(auth.currentUserName).toBe('alice')
    expect(auth.sessionToken).toBe('token-alice')
    expect(auth.activeSessionKey).toBe(aliceKey)
    expect(localStorage.getItem(ACTIVE_KEY)).toBe(aliceKey)
    expect(auth.sessionCount).toBe(2)
  })

  it('switchTo ignores unknown keys', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()
    await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')

    auth.switchTo('https://other|nobody')

    expect(auth.currentUserName).toBe('alice')
  })

  it('upserts: logging in the same person replaces the stored record', async () => {
    const fetchMock = vi.mocked(fetch)
    stubLoginOk(fetchMock)
    const auth = useAuthStore()

    await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')
    await auth.loginWithAccount('bob', 'pw123456', 'https://immich.example')
    await auth.loginWithAccount('alice', 'pw123456', 'https://immich.example')

    expect(auth.sessionCount).toBe(2)
    const alice = auth.sessions.find((s) => s.userName === 'alice')!
    expect(alice).toBeTruthy()
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
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('no network')))

    const auth = useAuthStore()
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
          defaultServerUrl: 'https://default.immich',
          version: 'v1.2.3',
          oauthEnabled: true,
          oauthButtonText: 'Continue with SSO',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    const auth = useAuthStore()
    await auth.fetchConfig()
    expect(auth.defaultServerUrl).toBe('https://default.immich')
    expect(auth.serverVersion).toBe('v1.2.3')
    expect(auth.oauthEnabled).toBe(true)
    expect(auth.oauthButtonText).toBe('Continue with SSO')
  })

  it('handles fetchConfig network failure gracefully', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockRejectedValue(new TypeError('Network error'))

    const auth = useAuthStore()
    await auth.fetchConfig()
    expect(auth.defaultServerUrl).toBeNull()
    expect(auth.serverVersion).toBe('')
    expect(auth.oauthEnabled).toBe(false)
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
    seedSessions()
    const auth = useAuthStore()
    expect(auth.currentUserName).toBe('Alice')
    const remaining = auth.logout()
    expect(remaining).toBe(true)
    expect(auth.sessionCount).toBe(1)
    expect(auth.currentUserName).toBe('Bob')
    expect(auth.sessionToken).toBe('t-bob')
    expect(auth.isLoggedIn).toBe(true)
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

describe('auth store setApiKey', () => {
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

  it('posts to /api/auth/account/apikey with auth header on success', async () => {
    localStorage.setItem(SESSIONS_KEY, JSON.stringify([{ token: 't-alice', userName: 'Alice', serverUrl: 'https://immich.example' }]))
    localStorage.setItem(ACTIVE_KEY, 'https://immich.example|Alice')
    setActivePinia(createPinia())
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
      if (url.includes('/api/auth/account/apikey')) return new Response(JSON.stringify({ status: 'ok' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      return new Response('not found', { status: 404 })
    })
    const auth = useAuthStore()
    const result = await auth.setApiKey('my-key')
    expect(result).toEqual({ ok: true })
    const call = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/account/apikey'))
    expect(call).toBeTruthy()
    const init = call?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({ apiKey: 'my-key' })
    expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer t-alice')
  })

  it('returns Invalid API key fallback on 401 with empty body', async () => {
    vi.mocked(fetch).mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
      if (url.includes('/api/auth/account/apikey')) return new Response(JSON.stringify({}), { status: 401, headers: { 'Content-Type': 'application/json' } })
      return new Response('not found', { status: 404 })
    })
    const auth = useAuthStore()
    const result = await auth.setApiKey('bad')
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.error).toBe('Invalid API key')
  })

  it('returns fallback on other error', async () => {
    vi.mocked(fetch).mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
      if (url.includes('/api/auth/account/apikey')) return new Response(JSON.stringify({ error: 'oops' }), { status: 500, headers: { 'Content-Type': 'application/json' } })
      return new Response('not found', { status: 404 })
    })
    const auth = useAuthStore()
    const result = await auth.setApiKey('key')
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.error).toBe('oops')
  })

  it('handles network error', async () => {
    vi.mocked(fetch).mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
      throw new TypeError('Network error')
    })
    const auth = useAuthStore()
    const result = await auth.setApiKey('key')
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.error).toBe('Cannot reach server. Please try again.')
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

    expect(result).toEqual({ ok: true, needsApiKey: false })
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

  it('loginWithAccount does not store password in localStorage', async () => {
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
    await auth.loginWithAccount('User', 'supersecret', 'https://immich.example')

    const stored = JSON.parse(localStorage.getItem(SESSIONS_KEY) || '[]')
    expect(stored).toHaveLength(1)
    expect(stored[0].password).toBeUndefined()
  })
})

describe('auth store loginWithAccountCreate', () => {
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

  it('creates an account, logs in and stores the apiKey session mode', async () => {
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
            token: 'created-session',
            userName: 'Bob',
            serverUrl: 'https://immich.example',
            mode: 'apiKey',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithAccountCreate('Bob', 'secret123', 'key-bob', 'https://immich.example')

    expect(result).toEqual({ ok: true, needsApiKey: false })
    expect(auth.sessionToken).toBe('created-session')
    expect(auth.currentUserName).toBe('Bob')
    expect(auth.activeSessionMode).toBe('apiKey')
    expect(auth.isLoggedIn).toBe(true)

    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({
      userName: 'Bob',
      password: 'secret123',
      apiKey: 'key-bob',
      serverUrl: 'https://immich.example',
      create: true,
    })
  })

  it('returns needsApiKey when hasApiKey false', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) return new Response(JSON.stringify({}), { status: 200, headers: { 'Content-Type': 'application/json' } })
      if (url.includes('/api/auth/login')) return new Response(JSON.stringify({ token: 't', userName: 'Bob', serverUrl: 'https://immich.example', hasApiKey: false }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      return new Response('not found', { status: 404 })
    })
    const auth = useAuthStore()
    const result = await auth.loginWithAccountCreate('Bob', 'secret123', '', 'https://immich.example')
    expect(result).toEqual({ ok: true, needsApiKey: true })
  })

  it('surfaces backend error codes (weak_password, invalid_api_key, account_exists)', async () => {
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
          JSON.stringify({ error: 'this user name already has a password', code: 'account_exists' }),
          { status: 400, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithAccountCreate('Alice', 'secret123', 'key-alice', 'https://immich.example')

    expect(result).toEqual({
      ok: false,
      error: 'this user name already has a password',
      code: 'account_exists',
    })
    expect(auth.isLoggedIn).toBe(false)
  })

  it('handles network failure', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockRejectedValue(new Error('network down'))

    const auth = useAuthStore()
    const result = await auth.loginWithAccountCreate('Bob', 'secret123', 'key-bob', 'https://immich.example')

    expect(result).toEqual({ ok: false, error: 'Cannot reach server. Please try again.' })
    expect(auth.isLoggedIn).toBe(false)
  })
})

describe('auth store OAuth login', () => {
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

  function stubConfig(oauthEnabled: boolean) {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(
          JSON.stringify({
            users: [],
            defaultServerUrl: null,
            version: 'test',
            oauthEnabled,
            oauthButtonText: oauthEnabled ? 'Continue with SSO' : '',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })
    return fetchMock
  }

  it('fetchConfig exposes oauthEnabled and oauthButtonText', async () => {
    stubConfig(true)
    const auth = useAuthStore()
    await auth.fetchConfig()
    expect(auth.oauthEnabled).toBe(true)
    expect(auth.oauthButtonText).toBe('Continue with SSO')
  })

  it('startOAuthLogin returns the IdP url', async () => {
    const fetchMock = stubConfig(true)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/oauth/start')) {
        return new Response(JSON.stringify({ url: 'https://idp.example/auth', state: 's' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.startOAuthLogin('https://immich.example')

    expect(result).toEqual({ ok: true, url: 'https://idp.example/auth' })
    const startCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/oauth/start'))
    expect(JSON.parse(String((startCall?.[1] as RequestInit).body))).toEqual({
      serverUrl: 'https://immich.example',
    })
  })

  it('startOAuthLogin surfaces oauth_not_enabled', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ error: 'OAuth disabled', code: 'oauth_not_enabled' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      })
    })

    const auth = useAuthStore()
    const result = await auth.startOAuthLogin('https://immich.example')

    expect(result).toEqual({ ok: false, error: 'OAuth disabled', code: 'oauth_not_enabled' })
  })

  it('loginWithOAuthCode stores an accessToken session', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/oauth/finish')) {
        return new Response(
          JSON.stringify({
            token: 'oauth-session',
            userName: 'SSO User',
            serverUrl: 'https://immich.example',
            mode: 'accessToken',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response('not found', { status: 404 })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithOAuthCode('handoff-123')

    expect(result).toEqual({ ok: true })
    expect(auth.sessionToken).toBe('oauth-session')
    expect(auth.currentUserName).toBe('SSO User')
    expect(auth.activeSessionMode).toBe('accessToken')
    expect(auth.isLoggedIn).toBe(true)
  })

  it('loginWithOAuthCode surfaces invalid_code', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ error: 'invalid or expired code', code: 'invalid_code' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      })
    })

    const auth = useAuthStore()
    const result = await auth.loginWithOAuthCode('stale')

    expect(result).toEqual({ ok: false, error: 'invalid or expired code', code: 'invalid_code' })
    expect(auth.isLoggedIn).toBe(false)
  })
})
