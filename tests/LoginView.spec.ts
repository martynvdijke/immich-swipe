import { flushPromises } from '@vue/test-utils'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '@/views/LoginView.vue'

const m = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  routeQuery: {} as Record<string, unknown>,
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({ query: m.routeQuery }),
    useRouter: () => ({ push: m.push, replace: m.replace }),
  }
})

let pinia: Pinia

function stubLogin(loginResponse: { status: number; body: Record<string, unknown> }, configExtra: Record<string, unknown> = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ defaultServerUrl: null, version: 'test', ...configExtra }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      if (url.includes('/api/auth/login')) {
        return new Response(JSON.stringify(loginResponse.body), {
          status: loginResponse.status,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response('not found', { status: 404 })
    })
  )
}

function mountView() {
  return mount(LoginView, {
    global: { plugins: [pinia] },
  })
}

describe('LoginView', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    pinia = createPinia()
    setActivePinia(pinia)
    m.push.mockClear()
    m.replace.mockClear()
    m.routeQuery = {}
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('renders two tabs Sign in and Create account, Sign in is default', async () => {
    stubLogin({ status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((t) => t.text())).toEqual(['Sign in', 'Create account'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
  })

  it('Sign in tab submits via loginWithAccount and navigates home on success', async () => {
    stubLogin({ status: 200, body: { token: 't', userName: 'Alice', serverUrl: 'https://immich.example', mode: 'apiKey', hasApiKey: true } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#userName').setValue('Alice')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const fetchMock = vi.mocked(fetch)
    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    expect(loginCall).toBeTruthy()
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({ userName: 'Alice', password: 'secret123', serverUrl: 'https://immich.example' })
    expect(m.push).toHaveBeenCalledWith('/')
  })

  it('routes to /settings with needsApiKey toast when hasApiKey is false', async () => {
    stubLogin({ status: 200, body: { token: 't', userName: 'Alice', serverUrl: 'https://immich.example', mode: 'apiKey', hasApiKey: false } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#userName').setValue('Alice')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(m.push).toHaveBeenCalledWith('/settings')
  })

  it('shows error when login fails', async () => {
    stubLogin({ status: 401, body: { error: 'invalid password', code: 'invalid_password' } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#userName').setValue('Alice')
    await wrapper.find('input#password').setValue('wrong')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('invalid password')
    expect(m.push).not.toHaveBeenCalled()
  })

  it('Create account tab submits via loginWithAccountCreate with apiKey', async () => {
    stubLogin({ status: 200, body: { token: 't', userName: 'Bob', serverUrl: 'https://immich.example', mode: 'apiKey', hasApiKey: true } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[role="tab"]').find((b) => b.text() === 'Create account')!.trigger('click')
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#createUserName').setValue('Bob')
    await wrapper.find('input#createPassword').setValue('secret123')
    await wrapper.find('input#createApiKey').setValue('key-bob')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const fetchMock = vi.mocked(fetch)
    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({ userName: 'Bob', password: 'secret123', apiKey: 'key-bob', serverUrl: 'https://immich.example', create: true })
    expect(m.push).toHaveBeenCalledWith('/')
  })

  it('Create account routes to /settings when needsApiKey', async () => {
    stubLogin({ status: 200, body: { token: 't', userName: 'Bob', serverUrl: 'https://immich.example', mode: 'apiKey', hasApiKey: false } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[role="tab"]').find((b) => b.text() === 'Create account')!.trigger('click')
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#createUserName').setValue('Bob')
    await wrapper.find('input#createPassword').setValue('secret123')
    await wrapper.find('input#createApiKey').setValue('')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(m.push).toHaveBeenCalledWith('/settings')
  })

  it('Create account validates short password client-side', async () => {
    stubLogin({ status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[role="tab"]').find((b) => b.text() === 'Create account')!.trigger('click')
    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#createUserName').setValue('Bob')
    await wrapper.find('input#createPassword').setValue('short')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('8 characters')
    expect(vi.mocked(fetch).mock.calls.filter(([url]) => String(url).includes('/api/auth/login')).length).toBe(0)
  })

  it('hides the SSO button when OAuth is not enabled', async () => {
    stubLogin({ status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).not.toContain('Login with SSO')
    expect(wrapper.text()).not.toContain('Continue with SSO')
  })

  it('shows the SSO button with the server button text and starts the flow', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/api/auth/config')) {
          return new Response(
            JSON.stringify({
              defaultServerUrl: 'https://immich.example',
              version: 'test',
              oauthEnabled: true,
              oauthButtonText: 'Continue with SSO',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        if (url.includes('/api/auth/oauth/start')) {
          return new Response(JSON.stringify({ url: 'https://idp.example/auth', state: 's' }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        return new Response('not found', { status: 404 })
      }),
    )
    const wrapper = mountView()
    await flushPromises()
    const sso = wrapper.findAll('button').find((b) => b.text() === 'Continue with SSO')!
    expect(sso.exists()).toBe(true)
    const realLocation = window.location
    const fakeLocation = { href: '' }
    Object.defineProperty(window, 'location', { value: fakeLocation, writable: true, configurable: true })
    try {
      await wrapper.find('input#serverUrl').setValue('https://immich.example')
      await sso.trigger('click')
      await flushPromises()
    } finally {
      Object.defineProperty(window, 'location', { value: realLocation, configurable: true })
    }
    const fetchMock = vi.mocked(fetch)
    const startCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/oauth/start'))
    expect(startCall).toBeTruthy()
    expect(JSON.parse(String((startCall?.[1] as RequestInit).body))).toEqual({ serverUrl: 'https://immich.example' })
    expect(fakeLocation.href).toBe('https://idp.example/auth')
  })

  it('completes SSO login from an oauthCode query', async () => {
    m.routeQuery = { oauthCode: 'handoff-123' }
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/api/auth/config')) {
          return new Response(JSON.stringify({ defaultServerUrl: null, version: 'test' }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        if (url.includes('/api/auth/oauth/finish')) {
          return new Response(
            JSON.stringify({ token: 't', userName: 'SSO User', serverUrl: 'https://immich.example', mode: 'accessToken' }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          )
        }
        return new Response('not found', { status: 404 })
      }),
    )
    mountView()
    await flushPromises()
    expect(m.replace).toHaveBeenCalledWith({ path: '/login', query: {} })
    expect(m.push).toHaveBeenCalledWith('/')
  })

  it('shows an error for an oauthError query', async () => {
    m.routeQuery = { oauthError: 'invalid_state' }
    stubLogin({ status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('SSO login expired')
    expect(m.push).not.toHaveBeenCalled()
  })
})
