import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

async function stubBackend() {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/api/auth/config')) {
      return new Response(
        JSON.stringify({ defaultServerUrl: null, version: 'test' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )
    }
    if (url.includes('/api/auth/login')) {
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
    await stubBackend()
    if (router.currentRoute.value.path !== '/login') {
      await router.push('/login')
    }
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

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

  it('does not auto-log-in: lands on /login when not logged in', async () => {
    await stubBackend()
    const auth = useAuthStore()
    // No session, no auto-login method exists
    expect((auth as unknown as Record<string, unknown>).loginWithUser).toBeUndefined()
    await router.push('/')
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('lands on /login with no env users', async () => {
    await stubBackend()
    const auth = useAuthStore()
    await router.push('/')
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('redirects /select-user to /login when not logged in', async () => {
    await stubBackend()
    const auth = useAuthStore()
    await router.push('/select-user')
    expect(auth.isLoggedIn).toBe(false)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('restores a persisted session on navigation (reload without re-login)', async () => {
    await stubBackend()
    seedStoredSessions()
    await router.push('/')
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.currentUserName).toBe('Alice')
    expect(auth.sessionCount).toBe(2)
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('allows /login while logged in (add-person flow)', async () => {
    await stubBackend()
    seedStoredSessions()
    await router.push('/login')
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(auth.sessionCount).toBe(2)
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('redirects /select-user to / while a session is active', async () => {
    await stubBackend()
    seedStoredSessions()
    await router.push('/select-user')
    const auth = useAuthStore()
    expect(auth.isLoggedIn).toBe(true)
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('fetchConfig is called but does not auto-login', async () => {
    await stubBackend()
    const auth = useAuthStore()
    const spy = vi.spyOn(auth, 'fetchConfig')
    await router.push('/')
    expect(spy).toHaveBeenCalled()
    expect(auth.isLoggedIn).toBe(false)
  })
})
