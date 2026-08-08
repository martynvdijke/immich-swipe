import { ref, type Ref } from 'vue'
import { normalizeBaseUrl, type UmamiSettings } from '@/types/observability'

/**
 * Umami analytics integration.
 *
 * Lazy, singleton, and safe: the tracking script is only injected when
 * explicitly loaded with an enabled config, and every `window.umami` call is
 * guarded so it never throws when the script is absent or failing.
 */

interface UmamiState {
  ready: Ref<boolean>
  loading: Ref<boolean>
  error: Ref<string | null>
  scriptEl: HTMLScriptElement | null
  /** Key of the currently injected config (dedupe guard) */
  injectedKey: string | null
  /** Promise for the in-flight injection (dedupe guard) */
  pending: Promise<boolean> | null
}

const state: UmamiState = {
  ready: ref(false),
  loading: ref(false),
  error: ref(null),
  scriptEl: null,
  injectedKey: null,
  pending: null,
}

function configKey(config: UmamiSettings): string {
  return JSON.stringify({
    serverUrl: normalizeBaseUrl(config.serverUrl),
    websiteId: config.websiteId,
    hostUrl: normalizeBaseUrl(config.hostUrl),
  })
}

function removeScript() {
  if (state.scriptEl) {
    state.scriptEl.remove()
    state.scriptEl = null
  }
  state.injectedKey = null
  state.pending = null
  state.ready.value = false
  state.loading.value = false
}

/**
 * Inject the Umami tracking script (async) for the given config.
 * - No-op (resolves `false`) when disabled or misconfigured.
 * - Dedupes: injecting the same config twice resolves the existing promise
 *   instead of adding a second script tag.
 * - Re-injects when the config changes (previous script removed).
 * - Resolves `true` on script load, `false` on script error.
 */
export function loadUmami(config: UmamiSettings): Promise<boolean> {
  if (!config.enabled) {
    removeScript()
    return Promise.resolve(false)
  }

  const serverUrl = normalizeBaseUrl(config.serverUrl).trim()
  const websiteId = config.websiteId.trim()
  if (!serverUrl || !websiteId) {
    removeScript()
    state.error.value = 'Umami requires a server URL and website ID.'
    return Promise.resolve(false)
  }

  const key = configKey(config)
  if (state.scriptEl && state.injectedKey === key) {
    return Promise.resolve(state.ready.value)
  }
  if (state.pending) {
    // Different config requested while another load is in flight: wait for it,
    // then re-inject with the new config (avoids racing script tags).
    return state.pending.then((ok) => {
      if (state.injectedKey === key) return ok
      return loadUmami(config)
    })
  }

  removeScript()
  state.loading.value = true
  state.error.value = null

  const script = document.createElement('script')
  script.async = true
  script.src = `${serverUrl}/script.js`
  script.dataset.websiteId = websiteId
  script.dataset.autoTrack = 'false'
  if (config.hostUrl.trim()) {
    script.dataset.hostUrl = normalizeBaseUrl(config.hostUrl).trim()
  }

  state.pending = new Promise<boolean>((resolve) => {
    script.onload = () => {
      state.ready.value = true
      state.loading.value = false
      state.injectedKey = key
      state.pending = null
      resolve(true)
    }
    script.onerror = () => {
      state.ready.value = false
      state.loading.value = false
      state.error.value = 'Umami script failed to load.'
      state.scriptEl?.remove()
      state.scriptEl = null
      state.pending = null
      resolve(false)
    }
    state.scriptEl = script
    document.head.appendChild(script)
  })

  return state.pending
}

/** Track a page view. No-op until the script has loaded. */
export function trackPageView(url?: string): void {
  if (!state.ready.value || typeof window === 'undefined') return
  try {
    window.umami?.track(url)
  } catch {
    // never throws
  }
}

/** Track a custom event with an optional payload. Safe no-op when unavailable. */
export function trackEvent(name: string, payload?: Record<string, unknown>): void {
  if (!state.ready.value || typeof window === 'undefined') return
  try {
    window.umami?.track(name, payload)
  } catch {
    // never throws
  }
}

/** Reactive status for the Settings view (script load state hint). */
export function umamiStatus() {
  return {
    ready: state.ready,
    loading: state.loading,
    error: state.error,
  }
}

declare global {
  interface Window {
    umami?: {
      track: (nameOrUrl?: string | Record<string, unknown>, data?: Record<string, unknown>) => void
    }
  }
}
