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

function stubLogin(configUsers: string[], loginResponse: { status: number; body: Record<string, unknown> }) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/config')) {
        return new Response(JSON.stringify({ users: configUsers, defaultServerUrl: null, version: 'test' }), {
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

  it('renders configured users as one-click options', async () => {
    stubLogin(['Martyn', 'Eve'], { status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()

    const buttons = wrapper.findAll('button').filter((b) => b.text() === 'Martyn' || b.text() === 'Eve')
    expect(buttons.map((b) => b.text())).toEqual(['Martyn', 'Eve'])
  })

  it('one-click login navigates home on success', async () => {
    stubLogin(['Martyn'], {
      status: 200,
      body: { token: 't', userName: 'Martyn', serverUrl: 'https://immich', mode: 'apiKey' },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text() === 'Martyn')!.trigger('click')
    await flushPromises()

    expect(m.push).toHaveBeenCalledWith('/')
  })

  it('switches to the swipe tab and pre-fills the user when a password is required', async () => {
    stubLogin(['Martyn'], { status: 401, body: { error: 'password required', code: 'password_required' } })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text() === 'Martyn')!.trigger('click')
    await flushPromises()

    const swipeTab = wrapper
      .findAll('[role="tab"]')
      .find((b) => b.text() === 'Swipe account')!
    expect(swipeTab.attributes('aria-selected')).toBe('true')
    const userNameInput = wrapper.find('input#userName').element as HTMLInputElement
    expect(userNameInput.value).toBe('Martyn')
  })

  it('shows an error toast-style message when a picker login fails', async () => {
    stubLogin(['Martyn'], { status: 401, body: { error: 'unknown user' } })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text() === 'Martyn')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('unknown user')
    expect(m.push).not.toHaveBeenCalled()
  })

  it('create-account tab submits userName, password and apiKey', async () => {
    stubLogin(['Martyn'], {
      status: 200,
      body: { token: 't', userName: 'Bob', serverUrl: 'https://immich', mode: 'apiKey' },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper
      .findAll('[role="tab"]')
      .find((b) => b.text() === 'Create account')!
      .trigger('click')

    await wrapper.find('input#serverUrl').setValue('https://immich.example')
    await wrapper.find('input#userName').setValue('Bob')
    await wrapper.find('input#password').setValue('secret123')
    await wrapper.find('input#apiKey').setValue('key-bob')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const fetchMock = vi.mocked(fetch)
    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/login'))
    const init = loginCall?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toEqual({
      userName: 'Bob',
      password: 'secret123',
      apiKey: 'key-bob',
      serverUrl: 'https://immich.example',
    })
    expect(m.push).toHaveBeenCalledWith('/')
  })

  it('hides the SSO button when OAuth is not enabled', async () => {
    stubLogin([], { status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Login with SSO')
  })

  it('shows the SSO button with the server button text and starts the flow', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/api/auth/config')) {
          return new Response(
            JSON.stringify({
              users: [],
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

    // jsdom doesn't navigate: capture the redirect target instead.
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
    expect(JSON.parse(String((startCall?.[1] as RequestInit).body))).toEqual({
      serverUrl: 'https://immich.example',
    })
    expect(fakeLocation.href).toBe('https://idp.example/auth')
  })

  it('completes SSO login from an oauthCode query', async () => {
    m.routeQuery = { oauthCode: 'handoff-123' }
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url.includes('/api/auth/config')) {
          return new Response(JSON.stringify({ users: [], defaultServerUrl: null, version: 'test' }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        if (url.includes('/api/auth/oauth/finish')) {
          return new Response(
            JSON.stringify({
              token: 't',
              userName: 'SSO User',
              serverUrl: 'https://immich.example',
              mode: 'accessToken',
            }),
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
    stubLogin([], { status: 200, body: {} })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('SSO login expired')
    expect(m.push).not.toHaveBeenCalled()
  })
})
