import { createPinia, setActivePinia } from 'pinia'
import { vi } from 'vitest'
import type { Mock } from 'vitest'
import { useImmich } from '@/composables/useImmich'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import type { ImmichAsset } from '@/types/immich'

describe('useImmich album flow', () => {
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
    setActivePinia(createPinia())
    // Mock auth as logged in with session
    const auth = useAuthStore()
    auth.sessionToken = 'test-token'
    auth.currentUserName = 'Alice'
    auth.immichServerUrl = 'http://immich.example.com'

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, ..._rest: unknown[]) => {
      const url = typeof input === 'string' ? input : input.toString()

      // Mock auth config endpoint
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: [], defaultServerUrl: null }), { status: 200 })
      }

      if (url.includes('/api/albums/')) {
        return new Response('{}', { status: 200 })
      }

      if (url.includes('/api/search/random')) {
        return new Response(JSON.stringify([dummyAsset]), { status: 200 })
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

  it('adds current asset to album and counts as kept', async () => {
    const uiStore = useUiStore()

    const immich = useImmich()
    immich.currentAsset.value = dummyAsset

    await immich.keepPhotoToAlbum({ id: 'album-1', albumName: 'Family' })

    expect(uiStore.keptCount).toBe(1)
    const fetchMock = fetch as unknown as Mock
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/api/albums/album-1/assets'),
      expect.objectContaining({ method: 'PUT' })
    )
  })
})
