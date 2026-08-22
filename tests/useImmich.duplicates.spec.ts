import { createPinia, setActivePinia } from 'pinia'
import { vi } from 'vitest'
import { useImmich } from '@/composables/useImmich'
import { useReviewedStore } from '@/stores/reviewed'
import { usePreferencesStore } from '@/stores/preferences'
import type { ImmichAsset } from '@/types/immich'
import { seedAuthSession } from './helpers/seedAuth'

/**
 * Duplicates scope tests: GET /search/duplicates feeds a client-side queue
 * grouped by duplicateId (groups ordered oldest-first), ungrouped/reviewed
 * assets are dropped, the progress total counts remaining duplicates, and
 * endpoint errors surface through the standard error state.
 */
function makeAsset(id: string, duplicateId: string | undefined, taken: string): ImmichAsset {
  return {
    id,
    ownerId: 'owner',
    type: 'IMAGE',
    originalPath: `/lib/${id}.jpg`,
    originalFileName: `${id}.jpg`,
    fileCreatedAt: taken,
    fileModifiedAt: taken,
    localDateTime: taken,
    updatedAt: taken,
    isFavorite: false,
    isArchived: false,
    isTrashed: false,
    isOffline: false,
    hasMetadata: true,
    duplicateId,
  }
}

const DUPLICATES_PAYLOAD = [
  // Newest group second (groups sort by oldest member)
  makeAsset('new-a', 'dup-new', '2024-06-01T10:00:00Z'),
  makeAsset('new-b', 'dup-new', '2024-06-01T10:01:00Z'),
  // Oldest group first
  makeAsset('old-a', 'dup-old', '2023-01-15T08:00:00Z'),
  makeAsset('old-b', 'dup-old', '2023-01-15T08:00:05Z'),
  // Noise: no duplicateId
  makeAsset('solo', undefined, '2022-03-03T00:00:00Z'),
]

describe('useImmich duplicates scope', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    seedAuthSession('test-token', 'Alice', 'http://immich.example.com')

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, ..._rest: unknown[]) => {
      const url = typeof input === 'string' ? input : input.toString()

      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null }), { status: 200 })
      }
      if (url.includes('/api/search/duplicates')) {
        if (duplicatesFails) {
          return new Response(JSON.stringify({ message: 'not found' }), { status: 404 })
        }
        return new Response(JSON.stringify(DUPLICATES_PAYLOAD), { status: 200 })
      }
      if (url.includes('/api/search/metadata')) {
        return new Response(
          JSON.stringify({ assets: { items: [], total: 0, nextPage: null } }),
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
    duplicatesFails = false
  })

  let duplicatesFails = false

  async function loadDuplicatesFeed() {
    const immich = useImmich()
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'duplicates' })
    await immich.loadInitialAsset()
    return immich
  }

  /** Skip the current card and wait until the feed actually advanced. */
  async function advance(immich: ReturnType<typeof useImmich>): Promise<void> {
    const previousId = immich.currentAsset.value?.id
    immich.skipPhoto()
    await vi.waitFor(() => expect(immich.currentAsset.value?.id).not.toBe(previousId))
  }

  it('serves duplicate groups adjacently, oldest group first', async () => {
    const immich = await loadDuplicatesFeed()
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('old-b'))

    const seen: string[] = []
    seen.push(immich.currentAsset.value!.id) // old-a
    await advance(immich) // -> old-b
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('new-a'))
    seen.push(immich.currentAsset.value!.id)
    await advance(immich) // -> new-a
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('new-b'))
    seen.push(immich.currentAsset.value!.id)
    await advance(immich) // -> new-b
    seen.push(immich.currentAsset.value!.id)

    expect(seen).toEqual(['old-a', 'old-b', 'new-a', 'new-b'])
  })

  it('drops ungrouped assets and already-reviewed duplicates', async () => {
    const reviewed = useReviewedStore()
    reviewed.markReviewed('new-b', 'keep')

    const immich = await loadDuplicatesFeed()
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('old-b'))
    expect(immich.currentAsset.value?.id).toBe('old-a')

    await advance(immich) // old-a -> old-b
    await vi.waitFor(() => expect(immich.nextAsset.value?.id).toBe('new-a'))
    expect(immich.currentAsset.value?.id).toBe('old-b')

    await advance(immich) // old-b -> new-a
    expect(immich.currentAsset.value?.id).toBe('new-a')
    expect(immich.nextAsset.value).toBeNull()
  })

  it('reports the remaining duplicate count as review total', async () => {
    const reviewed = useReviewedStore()
    reviewed.markReviewed('old-b', 'delete')

    const immich = await loadDuplicatesFeed()

    expect(immich.reviewTotal.value).toBe(3)
  })

  it('surfaces endpoint errors through the error state', async () => {
    duplicatesFails = true

    const immich = await loadDuplicatesFeed()

    expect(immich.currentAsset.value).toBeNull()
    expect(immich.error.value).toBeTruthy()
  })
})
