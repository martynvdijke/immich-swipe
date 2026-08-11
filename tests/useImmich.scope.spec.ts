import { createPinia, setActivePinia } from 'pinia'
import { vi } from 'vitest'
import type { Mock } from 'vitest'
import { useImmich } from '@/composables/useImmich'
import { usePreferencesStore } from '@/stores/preferences'
import { seedAuthSession } from './helpers/seedAuth'

/**
 * Scoped feed + review-progress tests: buildSearchFilters() merges the active
 * scope (album / dateRange / favorites) and selected person into the
 * /search/metadata body, random mode falls back to the scoped metadata page
 * when a filter is active, and fetchReviewTotal() reads assets.total.
 */
describe('useImmich scoped feed', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    // Persisted scope/person from a previous test must not leak into the
    // next one (preferences loadFromStorage rehydrates from localStorage).
    localStorage.clear()
    sessionStorage.clear()
    // Mock auth as logged in with session (must seed before the auth store is
    // instantiated inside useImmich(), which reads the registry at creation).
    seedAuthSession('test-token', 'Alice', 'http://immich.example.com')

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, ..._rest: unknown[]) => {
      const url = typeof input === 'string' ? input : input.toString()

      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null }), { status: 200 })
      }
      if (url.includes('/api/search/random')) {
        return new Response('[]', { status: 200 })
      }
      if (url.includes('/api/search/metadata')) {
        return new Response(
          JSON.stringify({ assets: { items: [], total: 42, nextPage: null } }),
          { status: 200 },
        )
      }
      if (url.includes('/thumbnail')) {
        return new Response('', { status: 200 })
      }
      return new Response(JSON.stringify({}), { status: 200 })
    }))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function metadataBodies(): Record<string, unknown>[] {
    const fetchMock = fetch as unknown as Mock
    return fetchMock.mock.calls
      .filter((call) => {
        const url = String(call[0])
        const opts = (call[1] ?? {}) as { method?: string }
        return url.includes('/api/search/metadata') && opts.method === 'POST'
      })
      .map((call) => JSON.parse(((call[1] as { body?: string })?.body ?? '{}')))
  }

  function randomCalls(): unknown[] {
    const fetchMock = fetch as unknown as Mock
    return fetchMock.mock.calls.filter((call) => String(call[0]).includes('/api/search/random'))
  }

  it('merges album scope into the metadata search body', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-1' })

    await immich.loadInitialAsset()

    const bodies = metadataBodies()
    expect(bodies.length).toBeGreaterThan(0)
    expect(bodies[0]).toMatchObject({ albumId: 'album-1' })
  })

  it('merges dateRange scope into the metadata search body', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'dateRange', from: '2024-01-01', to: '2024-12-31' })

    await immich.loadInitialAsset()

    const bodies = metadataBodies()
    expect(bodies[0]).toMatchObject({
      takenAfter: '2024-01-01',
      takenBefore: '2024-12-31',
    })
  })

  it('merges favorites scope into the metadata search body', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'favorites' })

    await immich.loadInitialAsset()

    const bodies = metadataBodies()
    expect(bodies[0]).toMatchObject({ isFavorite: true })
  })

  it('merges the selected person into the metadata search body', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setSelectedPerson('person-7')

    await immich.loadInitialAsset()

    const bodies = metadataBodies()
    expect(bodies[0]).toMatchObject({ personIds: ['person-7'] })
  })

  it('uses the scoped metadata page instead of /search/random when a filter is active', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-1' })

    await immich.loadInitialAsset()

    expect(randomCalls().length).toBe(0)
    expect(metadataBodies().length).toBeGreaterThan(0)
  })

  it('still uses /search/random for an unfiltered library feed', async () => {
    const immich = useImmich()

    await immich.loadInitialAsset()

    expect(randomCalls().length).toBeGreaterThan(0)
  })

  it('fetches review total via a size:1 metadata call and exposes it', async () => {
    const immich = useImmich()

    await immich.loadInitialAsset()

    expect(immich.reviewTotal.value).toBe(42)
    const bodies = metadataBodies()
    expect(bodies.some((b) => b.size === 1)).toBe(true)
  })
})
