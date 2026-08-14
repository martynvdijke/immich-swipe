import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

/**
 * Router-guard tests (login-account-setup).
 *
 * The guard never auto-logs-in: a visit without an active session always lands
 * on /login, which shows the configured env users as one-click options plus the
 * manual login / account-creation forms. Sessions are restored on reload, and
 * /login stays reachable while logged in (add-person flow).
 */

async function stubBackend(users: string[], loginOk = true, passwordRequired = false) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/api/auth/config')) {
      return new Response(
        JSON.stringify({ users, defaultServerUrl: null, version: 'test' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }
    if (url.includes('/api/auth/login')) {
      if (!loginOk) {
        return new Response(
          JSON.stringify(
            passwordRequired
              ? { error: 'password required', code: 'password_required' }
              : { error: 'unknown user' },
          ),
          { status: 401, headers: { 'Content-Type': 'application/json' } },
        )
      }
      return new Response(
        JSON.stringify({ token: 'session-x', userName: 'Alice', serverUrl: 'https://immich', mode: 'apiKey' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }
    if (url.includes('/search/random')) {
      return new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    if (url.includes('/search/metadata')) {
      return new Response(
        JSON.stringify({ assets: { items: [], total: 0, nextPage: null } }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }
    return new Response('ok', { status: 200 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('router guard: no auto-login, /login is the landing page', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    localStorage.clear()
    // The router is a module singleton: pushing the same path the previous
    // test ended on is a duplicate navigation that never re-runs the guard.
    // Start every test from /login (via a neutral no-users stub) so each
    // test's push below is a real navigation.
    await stubBackend([], true)
    if (router.currentRoute.value.path !== '/login') {
      await router.push('/login')
    }
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  /** Seed a persisted multi-session registry and force a fresh pinia so the
   *  auth store is instantiated AFTER seeding (init() reads localStorage at
   *  store creation). */
  function seedStoredSessions() {
    localStorage.setItem(
      'immich-swipe-sessions',
      JSON.stringify([
        { token: 't-alice', userName: 'Alice', serverUrl: 'https://immich', mode: 'apiKey' },
        { token: 't-bob', userName: 'Bob', serverUrl: 'https://immich', mode: 'apiKey' },
      ]),
    )
    localStorage.setItem('immich-swipe-active-session', 'https://immich|Alice')
    setActivePinia(createPinia())
  }

  it('does not auto-log-in a single env user: lands on /login', async () => {
    await stubBackend(['Alice'], true)
    const auth = useAuthStore()
    const loginSpy = vi.spyOn(auth, 'loginWithUser')

    await router.push('/')

    expect(loginSpy).not.toHaveBeenCalled()
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('lands on /login with multiple env users (no /select-user redirect)', async () => {
    await stubBackend(['Alice', 'Bob'], true)
    const auth = useAuthStore()

    await router.push('/')

    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
    expect(auth.envUsers).toEqual(['Alice', 'Bob'])
  })

  it('lands on /login with no env users', async () => {
    await stubBackend([], true)
    const auth = useAuthStore()

    await router.push('/')

    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('redirects /select-user to /login when not logged in', async () => {
    await stubBackend(['Alice'], true)
    const auth = useAuthStore()

    await router.push('/select-user')

    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('restores a persisted session on navigation (reload without re-login)', async () => {
    await stubBackend([], true)
    seedStoredSessions()

    await router.push('/')

    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.sessionCount).toBe(2)
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('allows /login while logged in (add-person flow)', async () => {
    await stubBackend([], true)
    seedStoredSessions()

    await router.push('/login')

    const auth = useAuthStore()
    // Existing sessions are NOT wiped by visiting /login
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.sessionCount).toBe(2)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('redirects /select-user to / while a session is active', async () => {
    await stubBackend([], true)
    seedStoredSessions()

    await router.push('/select-user')

    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(router.currentRoute.value.path).toBe('/')
  })
})
