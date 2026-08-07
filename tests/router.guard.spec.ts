import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

/**
 * Router-guard loop-prevention tests (fix-login-session-loop).
 *
 * The guard lives in src/router/index.ts and runs on every navigation. These
 * tests exercise the four end-to-end scenarios from the change's manual
 * verification tasks, automated:
 *  - 5.3 single env user: auto-login once, no loop when blocked / on failure
 *  - 5.4 multi env user: /select-user stays reachable even when blocked
 *  - 5.5 manual login mode: no env users -> manual /login, no loop
 */

async function stubBackend(users: string[], loginOk = true) {
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
        return new Response(JSON.stringify({ error: 'unknown user' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(
        JSON.stringify({ token: 'session-x', userName: 'Alice', serverUrl: 'https://immich' }),
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

describe('router guard login loop prevention', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    sessionStorage.clear()
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

  it('auto-logs-in a single env user when not blocked (5.3 happy path)', async () => {
    await stubBackend(['Alice'], true)
    const auth = useAuthStore()

    await router.push('/')

    expect(auth.isLoggedIn).toBe(true)
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.autoLoginBlocked).toBe(false)
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('skips auto-login and redirects to /login when autoLoginBlocked is set (5.3 no loop)', async () => {
    await stubBackend(['Alice'], true)
    const auth = useAuthStore()
    auth.autoLoginBlocked = true
    const loginSpy = vi.spyOn(auth, 'loginWithUser')

    await router.push('/')

    expect(loginSpy).not.toHaveBeenCalled()
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('trips the loop guard on failed auto-login and lands on /login (5.3 failure)', async () => {
    await stubBackend(['Alice'], false)
    const auth = useAuthStore()

    await router.push('/')

    expect(auth.autoLoginBlocked).toBe(true)
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')

    // A second navigation must not retry auto-login (loop broken).
    const loginSpy = vi.spyOn(auth, 'loginWithUser')
    await router.push('/login')
    expect(loginSpy).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('keeps /select-user reachable with multiple env users even when blocked (5.4)', async () => {
    await stubBackend(['Alice', 'Bob'], true)
    const auth = useAuthStore()
    auth.autoLoginBlocked = true

    await router.push('/select-user')

    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/select-user')
  })

  it('stays on manual /login with no env users and never auto-logs-in (5.5)', async () => {
    await stubBackend([], true)
    const auth = useAuthStore()

    await router.push('/')

    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
    expect(auth.autoLoginBlocked).toBe(false)
  })
})
