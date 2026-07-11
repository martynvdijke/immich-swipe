import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { usePreferencesStore } from '@/stores/preferences'
import { useAuthStore } from '@/stores/auth'

describe('preferences store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('persists review order and hotkeys', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const prefs = usePreferencesStore()
    prefs.setReviewOrder('chronological')
    prefs.setHotkey('1', 'album-a')
    prefs.setLastUsedAlbumId('album-a')

    // Wait for Vue watchers to flush and persist to localStorage
    await nextTick()

    const keys = Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-preferences'))
    expect(keys.length).toBe(1)

    const stored = JSON.parse(localStorage.getItem(keys[0]) || '{}')
    expect(stored.reviewOrder).toBe('chronological')
    expect(stored.albumHotkeys['1']).toBe('album-a')
    expect(stored.lastUsedAlbumId).toBe('album-a')

    // Recreate store to ensure rehydrate works
    const prefsReloaded = usePreferencesStore()
    expect(prefsReloaded.reviewOrder).toBe('chronological')
    expect(prefsReloaded.albumHotkeys['1']).toBe('album-a')
  })

  it('switches namespace when user changes', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'
    const prefs = usePreferencesStore()
    prefs.setHotkey('2', 'album-a2')
    // Wait for persist after hotkey set
    await nextTick()

    auth.immichServerUrl = 'http://server-b'
    auth.currentUserName = 'Bob'
    // Immediate loadFromStorage fires via watch immediate + storageKey change
    await nextTick()

    // Preferences should reset for the new namespace
    expect(prefs.reviewOrder).toBe('random')
    expect(Object.keys(prefs.albumHotkeys).length).toBe(0)

    const keys = Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-preferences'))
    expect(keys.length).toBe(2)
  })
})
