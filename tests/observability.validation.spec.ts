import {
  defaultObservabilitySettings,
  isHttpUrl,
  normalizeBaseUrl,
  validateObservabilitySettings,
} from '@/types/observability'

describe('observability validation', () => {
  it('accepts default (all disabled) settings', () => {
    const result = validateObservabilitySettings(defaultObservabilitySettings())
    expect(result.valid).toBe(true)
    expect(result.errors).toEqual({})
  })

  it('requires a server URL and website ID when umami is enabled', () => {
    const settings = defaultObservabilitySettings()
    settings.umami.enabled = true
    const result = validateObservabilitySettings(settings)
    expect(result.valid).toBe(false)
    expect(result.errors['umami.serverUrl']).toBeDefined()
    expect(result.errors['umami.websiteId']).toBeDefined()
  })

  it('rejects non-http(s) umami server and host URLs', () => {
    const settings = defaultObservabilitySettings()
    settings.umami.enabled = true
    settings.umami.serverUrl = 'ftp://umami.example.com'
    settings.umami.websiteId = 'web-1'
    settings.umami.hostUrl = 'not-a-url'
    const result = validateObservabilitySettings(settings)
    expect(result.valid).toBe(false)
    expect(result.errors['umami.serverUrl']).toBe('Must be a valid http(s) URL.')
    expect(result.errors['umami.hostUrl']).toBe('Must be a valid http(s) URL.')
  })

  it('accepts a valid umami config and ignores empty hostUrl', () => {
    const settings = defaultObservabilitySettings()
    settings.umami.enabled = true
    settings.umami.serverUrl = 'https://umami.example.com'
    settings.umami.websiteId = 'web-1'
    const result = validateObservabilitySettings(settings)
    expect(result.valid).toBe(true)
  })

  it('requires a valid http(s) endpoint when otel is enabled', () => {
    const settings = defaultObservabilitySettings()
    settings.otel.enabled = true
    let result = validateObservabilitySettings(settings)
    expect(result.valid).toBe(false)
    expect(result.errors['otel.endpoint']).toBe('Endpoint is required.')

    settings.otel.endpoint = 'http://collector.example.com:4318'
    result = validateObservabilitySettings(settings)
    expect(result.valid).toBe(true)

    settings.otel.endpoint = 'ws://collector.example.com'
    result = validateObservabilitySettings(settings)
    expect(result.errors['otel.endpoint']).toBe('Must be a valid http(s) URL.')
  })

  it('rejects sampling outside 0-100 when otel is enabled', () => {
    const settings = defaultObservabilitySettings()
    settings.otel.enabled = true
    settings.otel.endpoint = 'https://collector.example.com:4318'

    settings.otel.samplingPercent = 101
    expect(validateObservabilitySettings(settings).errors['otel.samplingPercent']).toBeDefined()

    settings.otel.samplingPercent = -1
    expect(validateObservabilitySettings(settings).errors['otel.samplingPercent']).toBeDefined()

    settings.otel.samplingPercent = 50
    expect(validateObservabilitySettings(settings).valid).toBe(true)

    settings.otel.samplingPercent = 0
    expect(validateObservabilitySettings(settings).valid).toBe(true)

    settings.otel.samplingPercent = 100
    expect(validateObservabilitySettings(settings).valid).toBe(true)
  })

  it('collects umami and otel errors simultaneously', () => {
    const settings = defaultObservabilitySettings()
    settings.umami.enabled = true
    settings.otel.enabled = true
    const result = validateObservabilitySettings(settings)
    expect(result.errors['umami.serverUrl']).toBeDefined()
    expect(result.errors['umami.websiteId']).toBeDefined()
    expect(result.errors['otel.endpoint']).toBeDefined()
    expect(result.valid).toBe(false)
  })
})

describe('observability url helpers', () => {
  it('normalizeBaseUrl strips trailing slashes', () => {
    expect(normalizeBaseUrl('https://umami.example.com/')).toBe('https://umami.example.com')
    expect(normalizeBaseUrl('https://umami.example.com///')).toBe('https://umami.example.com')
    expect(normalizeBaseUrl('')).toBe('')
  })

  it('isHttpUrl only accepts http(s) absolute URLs', () => {
    expect(isHttpUrl('https://collector.example.com:4318')).toBe(true)
    expect(isHttpUrl('http://umami.example.com')).toBe(true)
    expect(isHttpUrl('ftp://x')).toBe(false)
    expect(isHttpUrl('collector.example.com')).toBe(false)
    expect(isHttpUrl('')).toBe(false)
  })
})
