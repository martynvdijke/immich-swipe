import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useObservabilityStore } from '@/stores/observability'
import { useAuthStore } from '@/stores/auth'

function observabilityKeys(): string[] {
  return Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-observability'))
}

describe('observability store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('defaults to everything disabled', () => {
    const store = useObservabilityStore()
    expect(store.settings.umami).toEqual({
      enabled: false,
      serverUrl: '',
      websiteId: '',
      hostUrl: '',
    })
    expect(store.settings.otel).toEqual({
      enabled: false,
      endpoint: '',
      samplingPercent: 100,
    })
  })

  it('persists umami + otel settings and rehydrates on a fresh store', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const store = useObservabilityStore()
    store.setUmami({
      enabled: true,
      serverUrl: 'https://umami.example.com/',
      websiteId: 'web-1',
    })
    store.setOtel({ enabled: true, endpoint: 'https://collector.example.com:4318/', samplingPercent: 25 })

    await nextTick()

    const keys = observabilityKeys()
    expect(keys.length).toBe(1)
    const stored = JSON.parse(localStorage.getItem(keys[0]) || '{}')
    expect(stored.umami).toMatchObject({ enabled: true, websiteId: 'web-1' })
    expect(stored.umami.serverUrl).toBe('https://umami.example.com')
    expect(stored.otel.samplingPercent).toBe(25)
    expect(stored.otel.endpoint).toBe('https://collector.example.com:4318')

    // Fresh pinia re-reads the persisted payload (trailing slashes normalized).
    setActivePinia(createPinia())
    const auth2 = useAuthStore()
    auth2.immichServerUrl = 'http://server-a'
    auth2.currentUserName = 'Alice'
    const store2 = useObservabilityStore()
    expect(store2.settings.umami.enabled).toBe(true)
    expect(store2.settings.umami.serverUrl).toBe('https://umami.example.com')
    expect(store2.settings.otel.samplingPercent).toBe(25)
  })

  it('isolates settings per server/user namespace', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const store = useObservabilityStore()
    store.setUmami({ enabled: true, serverUrl: 'https://umami-a.example.com', websiteId: 'web-a' })
    await nextTick()

    auth.immichServerUrl = 'http://server-b'
    auth.currentUserName = 'Bob'
    await nextTick()

    // New namespace resets to defaults.
    expect(store.settings.umami.enabled).toBe(false)
    expect(store.settings.umami.serverUrl).toBe('')

    const keys = observabilityKeys()
    expect(keys.length).toBe(2)
  })

  it('handles corrupted stored JSON by falling back to defaults', async () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    localStorage.setItem(
      'immich-swipe-observability:http://server-a:Alice',
      '{not valid json',
    )
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const store = useObservabilityStore()
    expect(store.settings.umami.enabled).toBe(false)
    expect(store.settings.otel.samplingPercent).toBe(100)
    consoleError.mockRestore()
  })

  it('setUmami/setOtel merge partial patches and normalize trailing slashes', () => {
    const store = useObservabilityStore()
    store.setUmami({ enabled: true })
    expect(store.settings.umami.enabled).toBe(true)
    expect(store.settings.umami.websiteId).toBe('')

    store.setUmami({ serverUrl: 'https://umami.example.com///' })
    expect(store.settings.umami.serverUrl).toBe('https://umami.example.com')

    store.setOtel({ endpoint: 'https://collector.example.com:4318/' })
    expect(store.settings.otel.endpoint).toBe('https://collector.example.com:4318')
  })
})
