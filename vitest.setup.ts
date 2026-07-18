// Polyfill localStorage for jsdom environment
import { vi } from 'vitest'

// Proxy-based localStorage mock that supports Object.keys() enumeration
const store: Record<string, string> = {}
const localStorageMock = new Proxy({}, {
  get(_target, prop) {
    if (prop === 'getItem') return (key: string) => store[key] ?? null
    if (prop === 'setItem') return (key: string, value: string) => { store[key] = String(value) }
    if (prop === 'removeItem') return (key: string) => { delete store[key] }
    if (prop === 'clear') return () => { for (const k of Object.keys(store)) delete store[k] }
    if (prop === 'length') return Object.keys(store).length
    if (prop === 'key') return (index: number) => Object.keys(store)[index] ?? null
    return undefined
  },
  has(_target, prop) {
    return prop in store || ['getItem', 'setItem', 'removeItem', 'clear', 'length', 'key'].includes(String(prop))
  },
  ownKeys() {
    return Object.keys(store)
  },
  getOwnPropertyDescriptor(_target: unknown, prop: string) {
    if (prop in store) return { enumerable: true, configurable: true, value: store[prop] }
    return undefined
  },
})

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock })

// sessionStorage mock (auth session)
const sessionStore: Record<string, string> = {}
const sessionStorageMock = {
  getItem: (key: string) => sessionStore[key] ?? null,
  setItem: (key: string, value: string) => { sessionStore[key] = String(value) },
  removeItem: (key: string) => { delete sessionStore[key] },
  clear: () => { for (const k of Object.keys(sessionStore)) delete sessionStore[k] },
  get length() { return Object.keys(sessionStore).length },
  key: (index: number) => Object.keys(sessionStore)[index] ?? null,
}
Object.defineProperty(globalThis, 'sessionStorage', { value: sessionStorageMock })

// Mock matchMedia for ui store
Object.defineProperty(globalThis, 'matchMedia', {
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})
