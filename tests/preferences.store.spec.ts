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

  it('defaults scope to library and selectedPersonId to null', () => {
    const prefs = usePreferencesStore()
    expect(prefs.scope).toEqual({ kind: 'library' })
    expect(prefs.selectedPersonId).toBeNull()
  })

  it('persists review scope per namespace', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'album', albumId: 'album-x' })
    await nextTick()

    const key = Object.keys(localStorage).find((k) => k.startsWith('immich-swipe-preferences'))
    const stored = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored.scope).toEqual({ kind: 'album', albumId: 'album-x' })

    prefs.setScope({ kind: 'dateRange', from: '2024-01-01', to: '2024-12-31' })
    prefs.setScope({ kind: 'favorites' })
    await nextTick()
    const stored2 = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored2.scope).toEqual({ kind: 'favorites' })
  })

  it('rehydrates scope and selectedPersonId from storage on a fresh store', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'
    const prefs = usePreferencesStore()
    prefs.setScope({ kind: 'favorites' })
    prefs.setSelectedPerson('person-9')
    await nextTick()

    // Fresh pinia re-reads the persisted payload
    setActivePinia(createPinia())
    const auth2 = useAuthStore()
    auth2.immichServerUrl = 'http://server-a'
    auth2.currentUserName = 'Alice'
    const prefs2 = usePreferencesStore()
    expect(prefs2.scope).toEqual({ kind: 'favorites' })
    expect(prefs2.selectedPersonId).toBe('person-9')
  })

  it('clears selectedPersonId via setSelectedPerson(null)', () => {
    const prefs = usePreferencesStore()
    prefs.setSelectedPerson('person-1')
    expect(prefs.selectedPersonId).toBe('person-1')
    prefs.setSelectedPerson(null)
    expect(prefs.selectedPersonId).toBeNull()
  })
})
