import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SettingsView from '@/views/SettingsView.vue'
import { useObservabilityStore } from '@/stores/observability'
import { seedAuthSession } from './helpers/seedAuth'

const m = vi.hoisted(() => ({
  loadUmami: vi.fn(async () => true),
  initOtel: vi.fn(async () => true),
}))

vi.mock('@/composables/useUmami', () => ({
  loadUmami: m.loadUmami,
  trackPageView: vi.fn(),
  trackEvent: vi.fn(),
  umamiStatus: () => ({
    ready: { value: false },
    loading: { value: false },
    error: { value: null },
  }),
}))

vi.mock('@/composables/useOtel', () => ({
  initOtel: m.initOtel,
  shutdownOtel: vi.fn(),
  recordSwipeAction: vi.fn(),
  recordPersonFilter: vi.fn(),
  traceReviewAction: vi.fn(),
}))

let pinia: Pinia

function mountView() {
  const wrapper = mount(SettingsView, {
    global: { plugins: [pinia] },
  })
  return wrapper
}

describe('SettingsView', () => {
  beforeEach(() => {
    localStorage.clear()
    pinia = createPinia()
    setActivePinia(pinia)
    m.loadUmami.mockClear()
    m.initOtel.mockClear()
    m.loadUmami.mockResolvedValue(true)
    m.initOtel.mockResolvedValue(true)

    // Seed a logged-in session before the view (and its stores) instantiate
    seedAuthSession('test-token', 'Alice', 'http://server-a')
  })

  it('blocks saving while umami is enabled but the website ID is missing', async () => {
    const wrapper = mountView()
    await nextTick()

    await wrapper.find('[data-testid="umami-enabled"]').setValue(true)
    await wrapper.find('[data-testid="umami-server-url"]').setValue('https://umami.example.com')
    // websiteId left empty -> invalid

    const saveBtn = wrapper.find('[data-testid="save-btn"]')
    expect(saveBtn.attributes('disabled')).toBeDefined()
    await saveBtn.trigger('click')

    const store = useObservabilityStore()
    expect(store.settings.umami.enabled).toBe(false)
    expect(m.loadUmami).not.toHaveBeenCalled()
  })

  it('saves a valid umami config, persists it and hot-applies', async () => {
    const wrapper = mountView()
    await nextTick()

    await wrapper.find('[data-testid="umami-enabled"]').setValue(true)
    await wrapper.find('[data-testid="umami-server-url"]').setValue('https://umami.example.com/')
    await wrapper.find('[data-testid="umami-website-id"]').setValue('web-1')
    await wrapper.find('[data-testid="save-btn"]').trigger('click')
    await nextTick()

    const store = useObservabilityStore()
    expect(store.settings.umami.enabled).toBe(true)
    expect(store.settings.umami.serverUrl).toBe('https://umami.example.com')
    expect(m.loadUmami).toHaveBeenCalledWith(store.settings.umami)

    const keys = Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-observability'))
    expect(keys.length).toBe(1)
    const stored = JSON.parse(localStorage.getItem(keys[0]) || '{}')
    expect(stored.umami.enabled).toBe(true)
    expect(stored.umami.websiteId).toBe('web-1')
  })

  it('keeps the previously active config when a new submit is invalid', async () => {
    // Start from a persisted valid config.
    const store = useObservabilityStore()
    store.setUmami({
      enabled: true,
      serverUrl: 'https://umami.example.com',
      websiteId: 'web-1',
    })
    await nextTick()

    const wrapper = mountView()
    await nextTick()

    // Draft starts from the active config.
    await wrapper.find('[data-testid="umami-server-url"]').setValue('')
    await wrapper.find('[data-testid="save-btn"]').trigger('click')

    // Store must still hold the previous (valid) config.
    expect(store.settings.umami.enabled).toBe(true)
    expect(store.settings.umami.serverUrl).toBe('https://umami.example.com')
    expect(m.loadUmami).not.toHaveBeenCalled()
  })

  it('saves otel settings with endpoint and sampling and applies them', async () => {
    const wrapper = mountView()
    await nextTick()

    await wrapper.find('[data-testid="otel-enabled"]').setValue(true)
    await wrapper.find('[data-testid="otel-endpoint"]').setValue('https://collector.example.com:4318')
    await wrapper.find('[data-testid="otel-sampling"]').setValue(25)
    await wrapper.find('[data-testid="save-btn"]').trigger('click')
    await nextTick()

    const store = useObservabilityStore()
    expect(store.settings.otel.enabled).toBe(true)
    expect(store.settings.otel.endpoint).toBe('https://collector.example.com:4318')
    expect(store.settings.otel.samplingPercent).toBe(25)
    expect(m.initOtel).toHaveBeenCalledWith(store.settings.otel)
  })

  it('shows umami status hint text', async () => {
    const wrapper = mountView()
    await nextTick()
    expect(wrapper.text()).toContain('Not loaded.')
  })

  describe('account password section', () => {
    it('is hidden for access-token sessions (Immich password login)', async () => {
      localStorage.clear()
      pinia = createPinia()
      setActivePinia(pinia)
      seedAuthSession('test-token', 'Alice', 'http://server-a', 'accessToken')

      const wrapper = mountView()
      await nextTick()

      expect(wrapper.find('[data-testid="account-save-btn"]').exists()).toBe(false)
    })

    it('is visible for API-key sessions and saves a password', async () => {
      localStorage.clear()
      pinia = createPinia()
      setActivePinia(pinia)
      seedAuthSession('test-token', 'Alice', 'http://server-a', 'apiKey')

      const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
        const url = String(input)
        if (url.includes('/api/auth/config')) {
          return new Response(JSON.stringify({ users: [] }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        if (url.includes('/api/auth/account')) {
          return new Response(JSON.stringify({ status: 'ok' }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        return new Response('not found', { status: 404 })
      })
      vi.stubGlobal('fetch', fetchMock)

      const wrapper = mountView()
      await nextTick()

      const saveBtn = wrapper.find('[data-testid="account-save-btn"]')
      expect(saveBtn.exists()).toBe(true)

      await wrapper.find('[data-testid="account-new-password"]').setValue('newpass123')
      await wrapper.find('[data-testid="account-confirm-password"]').setValue('newpass123')
      await saveBtn.trigger('click')
      await nextTick()

      const accountCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/account'))
      expect(accountCall).toBeTruthy()
      const init = accountCall?.[1] as unknown as RequestInit
      expect(JSON.parse(String(init.body))).toEqual({ password: 'newpass123' })
      expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer test-token')
      expect(wrapper.find('[data-testid="account-error"]').exists()).toBe(false)
    })

    it('rejects mismatched confirmation and short passwords', async () => {
      seedAuthSession('test-token', 'Alice', 'http://server-a', 'apiKey')

      const wrapper = mountView()
      await nextTick()

      const saveBtn = wrapper.find('[data-testid="account-save-btn"]')

      // Mismatch
      await wrapper.find('[data-testid="account-new-password"]').setValue('newpass123')
      await wrapper.find('[data-testid="account-confirm-password"]').setValue('different!')
      await saveBtn.trigger('click')
      await nextTick()
      expect(wrapper.find('[data-testid="account-error"]').text()).toContain('do not match')

      // Too short
      await wrapper.find('[data-testid="account-new-password"]').setValue('short')
      await wrapper.find('[data-testid="account-confirm-password"]').setValue('short')
      await saveBtn.trigger('click')
      await nextTick()
      expect(wrapper.find('[data-testid="account-error"]').text()).toContain('8 characters')
    })
  })
})
