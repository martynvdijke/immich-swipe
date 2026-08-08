import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Mock } from 'vitest'
import { useImmich } from '@/composables/useImmich'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import type { ImmichAsset } from '@/types/immich'

// Simulate the observability integrations: while `active.value` is false the
// spies swallow everything (no-op), exactly like the real composables do when
// Umami/OTel are disabled. This lets the spec assert events/counters are only
// emitted when integrations are active.
const m = vi.hoisted(() => {
  const active = { value: false }
  const events: Array<[string, Record<string, unknown> | undefined]> = []
  const counters: Array<[string, Record<string, unknown> | undefined]> = []
  const spans: Array<[string, Record<string, unknown> | undefined]> = []
  return {
    active,
    events,
    counters,
    spans,
    trackEvent: vi.fn((name: string, payload?: Record<string, unknown>) => {
      if (active.value) events.push([name, payload])
    }),
    recordSwipeAction: vi.fn((action: string, attrs?: Record<string, unknown>) => {
      if (active.value) counters.push([action, attrs])
    }),
    traceReviewAction: vi.fn((actionType: string, attrs?: Record<string, unknown>) => {
      if (active.value) spans.push([actionType, attrs])
    }),
  }
})

vi.mock('@/composables/useUmami', () => ({
  trackEvent: m.trackEvent,
  trackPageView: vi.fn(),
  loadUmami: vi.fn(async () => false),
}))

vi.mock('@/composables/useOtel', () => ({
  recordSwipeAction: m.recordSwipeAction,
  recordPersonFilter: vi.fn(),
  traceReviewAction: m.traceReviewAction,
  initOtel: vi.fn(async () => false),
  shutdownOtel: vi.fn(async () => {}),
}))

describe('useImmich observability events', () => {
  const dummyAsset: ImmichAsset = {
    id: 'asset-1',
    ownerId: 'owner-1',
    type: 'IMAGE',
    originalPath: '/tmp/file.jpg',
    originalFileName: 'file.jpg',
    fileCreatedAt: new Date().toISOString(),
    fileModifiedAt: new Date().toISOString(),
    localDateTime: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    isFavorite: false,
    isArchived: false,
    isTrashed: false,
    isOffline: false,
    hasMetadata: false,
  }

  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    m.active.value = false
    m.events.length = 0
    m.counters.length = 0
    m.spans.length = 0

    const auth = useAuthStore()
    auth.sessionToken = 'test-token'
    auth.currentUserName = 'Alice'
    auth.immichServerUrl = 'http://immich.example.com'

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null }), { status: 200 })
      }
      if (url.includes('/api/albums/')) {
        return new Response('{}', { status: 200 })
      }
      if (url.includes('/api/search/random')) {
        return new Response(JSON.stringify([dummyAsset]), { status: 200 })
      }
      if (url.includes('/api/assets')) {
        return new Response('{}', { status: 200 })
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

  it('emits nothing while integrations are inactive, but the review still works', async () => {
    const uiStore = useUiStore()
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.keepPhoto()

    expect(uiStore.keptCount).toBe(1)
    expect(m.events).toHaveLength(0)
    expect(m.counters).toHaveLength(0)
    expect(m.spans).toHaveLength(0)
  })

  it('emits swipe.keep + kept counter + review span on keepPhoto when active', async () => {
    m.active.value = true
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.keepPhoto()

    expect(m.events).toEqual([
      ['swipe.keep', { assetId: 'asset-1', assetType: 'IMAGE' }],
    ])
    expect(m.counters).toEqual([
      ['kept', { assetType: 'IMAGE', personFiltered: false, albumName: undefined }],
    ])
    expect(m.spans).toEqual([
      ['keep', { assetType: 'IMAGE', personFiltered: false, albumName: undefined }],
    ])
  })

  it('emits swipe.delete + deleted counter on deletePhoto when active', async () => {
    m.active.value = true
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.deletePhoto()

    expect(m.events).toEqual([
      ['swipe.delete', { assetId: 'asset-1', assetType: 'IMAGE' }],
    ])
    expect(m.counters).toEqual([
      ['deleted', { assetType: 'IMAGE', personFiltered: false, albumName: undefined }],
    ])
  })

  it('emits swipe.album_add + album_added counter with album name on keepPhotoToAlbum', async () => {
    m.active.value = true
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.keepPhotoToAlbum({ id: 'album-1', albumName: 'Family' })

    expect(m.events).toEqual([
      ['swipe.album_add', { assetId: 'asset-1', assetType: 'IMAGE', albumName: 'Family' }],
    ])
    expect(m.counters).toEqual([
      ['album_added', { assetType: 'IMAGE', personFiltered: false, albumName: 'Family' }],
    ])
  })

  it('emits swipe.undo with the undone action type on undoLastAction', async () => {
    m.active.value = true
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.keepPhoto()
    m.events.length = 0
    m.counters.length = 0
    m.spans.length = 0

    await immich.undoLastAction()

    expect(m.events).toEqual([
      ['swipe.undo', { assetId: 'asset-1', assetType: 'IMAGE', actionType: 'keep' }],
    ])
    expect(m.counters).toEqual([
      ['undo', { assetType: 'IMAGE', personFiltered: false, albumName: undefined }],
    ])
    expect(m.spans).toEqual([
      ['keep', { assetType: 'IMAGE', personFiltered: false, albumName: undefined }],
    ])
  })

  it('emits keep event when favoriting (keep direction) on toggleFavorite', async () => {
    m.active.value = true
    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.toggleFavorite()

    expect(m.events).toEqual([
      ['swipe.keep', { assetId: 'asset-1', assetType: 'IMAGE' }],
    ])
    const fetchMock = fetch as unknown as Mock
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/api/assets/asset-1'),
      expect.objectContaining({ method: 'PUT' }),
    )
  })
})
