import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import type { UmamiSettings } from '@/types/observability'

// useUmami keeps module-singleton state; reload a fresh module per test.
async function loadMod() {
  vi.resetModules()
  return import('@/composables/useUmami')
}

function scripts(): HTMLScriptElement[] {
  return Array.from(document.head.querySelectorAll('script[data-website-id]'))
}

function fireLoad(script: HTMLScriptElement) {
  script.dispatchEvent(new Event('load'))
}

const validConfig: UmamiSettings = {
  enabled: true,
  serverUrl: 'https://umami.example.com/',
  websiteId: 'web-1',
  hostUrl: '',
}

describe('useUmami script injection', () => {
  beforeEach(() => {
    scripts().forEach((s) => s.remove())
  })

  it('does not inject a script when disabled', async () => {
    const mod = await loadMod()
    const result = await mod.loadUmami({ ...validConfig, enabled: false })
    expect(result).toBe(false)
    expect(scripts().length).toBe(0)
  })

  it('does not inject a script when serverUrl or websiteId is missing', async () => {
    const mod = await loadMod()
    const noUrl = await mod.loadUmami({ ...validConfig, serverUrl: '' })
    expect(noUrl).toBe(false)
    expect(mod.umamiStatus().error.value).toContain('server URL')

    const noId = await mod.loadUmami({ ...validConfig, websiteId: '' })
    expect(noId).toBe(false)
    expect(mod.umamiStatus().error.value).toContain('server URL')
    expect(scripts().length).toBe(0)
  })

  it('injects an async script with src, website-id, auto-track=false and resolves true on load', async () => {
    const mod = await loadMod()
    const promise = mod.loadUmami(validConfig)
    const els = scripts()
    expect(els.length).toBe(1)

    const script = els[0]
    expect(script.async).toBe(true)
    expect(script.src).toBe('https://umami.example.com/script.js')
    expect(script.dataset.websiteId).toBe('web-1')
    expect(script.dataset.autoTrack).toBe('false')
    expect(script.dataset.hostUrl).toBeUndefined()

    fireLoad(script)
    await expect(promise).resolves.toBe(true)
    expect(mod.umamiStatus().ready.value).toBe(true)
  })

  it('sets data-host-url when hostUrl is configured', async () => {
    const mod = await loadMod()
    const promise = mod.loadUmami({
      ...validConfig,
      hostUrl: 'https://public.example.com/',
    })
    const script = scripts()[0]
    expect(script.dataset.hostUrl).toBe('https://public.example.com')
    fireLoad(script)
    await expect(promise).resolves.toBe(true)
  })

  it('resolves false and reports an error when the script fails to load', async () => {
    const mod = await loadMod()
    const promise = mod.loadUmami(validConfig)
    const script = scripts()[0]
    script.dispatchEvent(new Event('error'))
    await expect(promise).resolves.toBe(false)
    expect(mod.umamiStatus().error.value).toBe('Umami script failed to load.')
    expect(scripts().length).toBe(0)
  })

  it('does not inject a second script for the same config (dedupe)', async () => {
    const mod = await loadMod()
    const p1 = mod.loadUmami(validConfig)
    const p2 = mod.loadUmami(validConfig)
    expect(scripts().length).toBe(1)

    fireLoad(scripts()[0])
    await expect(p1).resolves.toBe(true)
    await expect(p2).resolves.toBe(true)
    expect(scripts().length).toBe(1)
  })

  it('re-injects with a new config (removes the old script)', async () => {
    const mod = await loadMod()
    const p1 = mod.loadUmami(validConfig)
    fireLoad(scripts()[0])
    await p1

    const other: UmamiSettings = { ...validConfig, serverUrl: 'https://umami-other.example.com' }
    const p2 = mod.loadUmami(other)
    const els = scripts()
    expect(els.length).toBe(1)
    expect(els[0].src).toBe('https://umami-other.example.com/script.js')
    fireLoad(els[0])
    await expect(p2).resolves.toBe(true)
  })
})

describe('useUmami event helpers', () => {
  const trackMock = vi.fn()

  beforeEach(() => {
    scripts().forEach((s) => s.remove())
    trackMock.mockClear()
    ;(window as unknown as { umami?: unknown }).umami = { track: trackMock }
  })

  afterEach(() => {
    delete (window as unknown as { umami?: unknown }).umami
  })

  it('does not track before the script has loaded (ready=false)', async () => {
    const mod = await loadMod()
    mod.trackPageView('/settings')
    mod.trackEvent('swipe.keep', { assetId: 'a' })
    expect(trackMock).not.toHaveBeenCalled()
  })

  it('tracks page views and custom events once ready', async () => {
    const mod = await loadMod()
    const promise = mod.loadUmami(validConfig)
    fireLoad(scripts()[0])
    await promise

    mod.trackPageView('/settings')
    expect(trackMock).toHaveBeenCalledWith('/settings')

    mod.trackEvent('swipe.keep', { assetId: 'asset-1', assetType: 'IMAGE' })
    expect(trackMock).toHaveBeenCalledWith('swipe.keep', { assetId: 'asset-1', assetType: 'IMAGE' })
  })

  it('never throws when window.umami throws', async () => {
    const mod = await loadMod()
    const promise = mod.loadUmami(validConfig)
    fireLoad(scripts()[0])
    await promise

    ;(window as unknown as { umami?: { track: () => void } }).umami = {
      track: () => {
        throw new Error('boom')
      },
    }
    expect(() => mod.trackPageView('/settings')).not.toThrow()
    expect(() => mod.trackEvent('swipe.keep', {})).not.toThrow()
  })
})
