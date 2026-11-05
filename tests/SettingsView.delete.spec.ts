import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SettingsView from '@/views/SettingsView.vue'
import { useAuthStore } from '@/stores/auth'
import { seedAuthSession } from './helpers/seedAuth'

vi.mock('@/composables/useUmami', () => ({
  loadUmami: vi.fn(async () => true),
  trackPageView: vi.fn(),
  trackEvent: vi.fn(),
  umamiStatus: () => ({
    ready: { value: false },
    loading: { value: false },
    error: { value: null },
  }),
}))

vi.mock('@/composables/useOtel', () => ({
  initOtel: vi.fn(async () => true),
  shutdownOtel: vi.fn(),
  recordSwipeAction: vi.fn(),
  recordPersonFilter: vi.fn(),
  traceReviewAction: vi.fn(),
}))

const Blank = { template: '<div />' }

let pinia: Pinia
let router: Router
let fetchMock: ReturnType<typeof vi.fn>

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

async function mountView() {
  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/settings', component: Blank },
      { path: '/login', component: Blank },
      { path: '/privacy', component: Blank },
      { path: '/terms', component: Blank },
      { path: '/account/delete', component: Blank },
    ],
  })
  await router.push('/settings')
  await router.isReady()
  const wrapper = mount(SettingsView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return wrapper
}

describe('SettingsView account deletion and export', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    pinia = createPinia()
    setActivePinia(pinia)
    seedAuthSession('test-token', 'Alice', 'http://server-a', 'apiKey')

    fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return json({ defaultServerUrl: null, version: 'test' })
      }
      if (url.includes('/api/admin/email')) {
        return json({ configured: false })
      }
      if (url.includes('/api/auth/account/export')) {
        return json({ userName: 'Alice', keptCount: 2 })
      }
      if (url.includes('/api/auth/account') && init?.method === 'DELETE') {
        return json({ status: 'deleted', immichAccount: 'retained' })
      }
      return json({ status: 'ok' })
    })
    vi.stubGlobal('fetch', fetchMock)
    // Preserve the URL constructor (jsdom's request/cookie code needs it) and
    // stub only the object-URL helpers used by the export download.
    vi.stubGlobal(
      'URL',
      Object.assign(URL, {
        createObjectURL: vi.fn(() => 'blob:mock'),
        revokeObjectURL: vi.fn(),
      })
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('will not delete until the typed user name matches', async () => {
    const wrapper = await mountView()
    await wrapper.find('[data-testid="account-delete-btn"]').trigger('click')
    await wrapper.find('[data-testid="account-delete-confirm"]').setValue('alice')
    await wrapper.find('[data-testid="account-delete-confirm-btn"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="account-delete-error"]').text()).toContain('Type')
    expect(fetchMock.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false)
    expect(useAuthStore().isLoggedIn).toBe(true)
  })

  it('deletes with the typed confirmation, clears the local session and redirects to login', async () => {
    const wrapper = await mountView()
    await wrapper.find('[data-testid="account-delete-btn"]').trigger('click')
    await wrapper.find('[data-testid="account-delete-confirm"]').setValue('Alice')
    await wrapper.find('[data-testid="account-delete-confirm-btn"]').trigger('click')
    await flushPromises()

    const deleteCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'DELETE')
    expect(deleteCall).toBeTruthy()
    expect(String(deleteCall?.[0])).toContain('/api/auth/account')
    expect(JSON.parse(String(deleteCall?.[1]?.body))).toEqual({ confirm: 'Alice' })

    expect(useAuthStore().isLoggedIn).toBe(false)
    expect(localStorage.getItem('immich-swipe-sessions')).toBeNull()
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('offers an account export without credentials', async () => {
    const wrapper = await mountView()
    await wrapper.find('[data-testid="account-export-btn"]').trigger('click')
    await flushPromises()

    const exportCall = fetchMock.mock.calls.find(([input]) =>
      String(input).includes('/api/auth/account/export')
    )
    expect(exportCall).toBeTruthy()
    expect((window.URL.createObjectURL as ReturnType<typeof vi.fn>)).toHaveBeenCalled()
  })

  it('shows privacy and terms links', async () => {
    const wrapper = await mountView()
    const hrefs = wrapper.findAll('a').map((a) => a.attributes('href'))
    expect(hrefs).toEqual(expect.arrayContaining(['/privacy', '/terms', '/account/delete']))
  })
})
