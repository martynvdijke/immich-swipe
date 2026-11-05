import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LegalPrivacyView from '@/views/LegalPrivacyView.vue'
import LegalTermsView from '@/views/LegalTermsView.vue'
import LegalDeleteView from '@/views/LegalDeleteView.vue'
import router from '@/router'

const LEGAL = {
  appName: 'Immich Swipe',
  controller: 'Example Photo Cooperative',
  contactEmail: 'privacy@example.com',
  contactAddress: '1 Example Street',
  hostingRegion: 'the Netherlands',
  defaultServerUrl: 'https://immich-swipe.example',
  smtpConfigured: true,
  telemetryConfigured: false,
  privacyUrl: '/privacy',
  termsUrl: '/terms',
  deletionUrl: '/account/delete',
}

let pinia: Pinia

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/legal')) {
        return new Response(JSON.stringify(LEGAL), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ defaultServerUrl: null, version: 'test' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })
  )
}

function mountView(component: Parameters<typeof mount>[0]) {
  return mount(component, { global: { plugins: [pinia] } })
}

describe('public legal pages', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    pinia = createPinia()
    setActivePinia(pinia)
    stubFetch()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('privacy policy names the configured controller and separates the Immich server', async () => {
    const wrapper = mountView(LegalPrivacyView)
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Privacy policy')
    expect(text).toContain('Example Photo Cooperative')
    expect(text).toContain('the Netherlands')
    expect(text).toContain('privacy@example.com')
    // The critical distinction for a self-hosted client.
    expect(text).toContain('does not store your photo library')
    expect(text).toContain('delete your Immich account')
    expect(text).toContain('supervisory authority')
    expect(text).toContain('under 16')
    expect(text).toContain('Backups')
  })

  it('terms of use is reachable and links to privacy and deletion', async () => {
    const wrapper = mountView(LegalTermsView)
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Terms of use')
    expect(text).toContain('Your Immich server')
    expect(wrapper.findAll('a').map((a) => a.attributes('href'))).toEqual(
      expect.arrayContaining(['/privacy', '/account/delete'])
    )
  })

  it('deletion page explains the app/Immich split without requiring a session', async () => {
    const wrapper = mountView(LegalDeleteView)
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Delete your account')
    expect(text).toContain('What is removed')
    expect(text).toContain('What is not removed')
    expect(text).toContain('Immich account and your photos')
    expect(text).toContain('privacy@example.com')
    expect(text).toContain('Backups taken before deletion')
  })

  it('router serves the legal routes while signed out', async () => {
    await router.push('/privacy')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/privacy')

    await router.push('/terms')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/terms')

    await router.push('/account/delete')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/account/delete')
  })
})
