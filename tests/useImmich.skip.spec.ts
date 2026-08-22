import { createPinia, setActivePinia } from 'pinia'
import { vi } from 'vitest'
import type { Mock } from 'vitest'
import { useImmich } from '@/composables/useImmich'
import { useReviewedStore } from '@/stores/reviewed'
import { useUiStore } from '@/stores/ui'
import { usePreferencesStore } from '@/stores/preferences'
import type { ImmichAsset } from '@/types/immich'
import { seedAuthSession } from './helpers/seedAuth'

/**
 * Skip-action tests: skipPhoto() marks the asset reviewed as 'skip' and
 * advances without any Immich write request; undo rewinds locally; stats
 * counters stay untouched.
 */
function makeAsset(id: string): ImmichAsset {
  return {
    id,
    ownerId: 'owner',
    type: 'IMAGE',
    originalPath: `/lib/${id}.jpg`,
    originalFileName: `${id}.jpg`,
    fileCreatedAt: '2024-01-01T00:00:00Z',
    fileModifiedAt: '2024-01-01T00:00:00Z',
    localDateTime: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    isFavorite: false,
    isArchived: false,
    isTrashed: false,
    isOffline: false,
    hasMetadata: true,
  }
}

describe('useImmich skip action', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    seedAuthSession('test-token', 'Alice', 'http://immich.example.com')

    const queue = [makeAsset('asset-a'), makeAsset('asset-b')]

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, ...rest: unknown[]) => {
      const url = typeof input === 'string' ? input : input.toString()
      const opts = (rest[0] ?? {}) as { method?: string; body?: string }

      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null }), { status: 200 })
      }
      if (url.includes('/api/search/metadata')) {
        const body = JSON.parse(opts.body ?? '{}')
        // size:1 calls are the review-total probe; page 1 holds a+b, page 2 holds c
        if (body.size !== 1 && body.page === 1) {
          return new Response(
            JSON.stringify({ assets: { items: queue, total: 3, nextPage: '2' } }),
            { status: 200 },
          )
        }
        if (body.size !== 1 && body.page === 2) {
          return new Response(
            JSON.stringify({ assets: { items: [makeAsset('asset-c')], total: 3, nextPage: null } }),
            { status: 200 },
          )
        }
        return new Response(
          JSON.stringify({ assets: { items: [], total: 3, nextPage: null } }),
          { status: 200 },
        )
      }
      if (url.includes('/api/search/random')) {
        return new Response('[]', { status: 200 })
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

  function writeCalls(): { url: string; method?: string }[] {
    const fetchMock = fetch as unknown as Mock
    return fetchMock.mock.calls
      .map((call) => ({
        url: String(call[0]),
        method: ((call[1] ?? {}) as { method?: string }).method,
      }))
      // Only asset/trash/album mutations count as Immich writes; feed reads
      // (POST /api/search/*) and auth calls are not writes.
      .filter((call) =>
        call.method != null &&
        call.method !== 'GET' &&
        ['/api/assets', '/api/trash', '/api/albums', '/api/people'].some((p) => call.url.includes(p))
      )
  }

  it('skip marks reviewed, advances, and performs no Immich write', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-1' })
    await immich.loadInitialAsset()
    expect(immich.currentAsset.value?.id).toBe('asset-a')
    // Scoped-random feeds pick one candidate per page; the preloaded next is
    // page 2's candidate. Wait for the background preload to settle.
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('asset-c'))

    immich.skipPhoto()

    expect(immich.currentAsset.value?.id).toBe('asset-c')
    const reviewed = useReviewedStore()
    expect(reviewed.getDecision('asset-a')).toBe('skip')

    // No non-feed write requests happened (no DELETE /assets etc.)
    expect(writeCalls().length).toBe(0)
  })

  it('undo after skip restores the card locally and clears the mark', async () => {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-1' })
    await immich.loadInitialAsset()
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('asset-c'))

    immich.skipPhoto()
    expect(immich.currentAsset.value?.id).toBe('asset-c')

    await immich.undoLastAction()

    expect(immich.currentAsset.value?.id).toBe('asset-a')
    const reviewed = useReviewedStore()
    expect(reviewed.getDecision('asset-a')).toBe(null)
  })

  it('skip is stats-neutral', async () => {
    const immich = useImmich()
    const ui = useUiStore()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-1' })
    await immich.loadInitialAsset()
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('asset-c'))

    const keptBefore = ui.keptCount
    const deletedBefore = ui.deletedCount

    immich.skipPhoto()
    await immich.undoLastAction()

    expect(ui.keptCount).toBe(keptBefore)
    expect(ui.deletedCount).toBe(deletedBefore)
  })
})
