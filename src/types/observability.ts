/**
 * Observability settings (Umami analytics + browser OTel traces/stats).
 * Persisted per Immich server + user in localStorage (see stores/observability.ts).
 */

export interface UmamiSettings {
  enabled: boolean
  /** Umami instance base URL, e.g. https://umami.example.com (without trailing slash) */
  serverUrl: string
  /** Umami website ID (from the Umami dashboard) */
  websiteId: string
  /** Optional public-facing Umami URL (data-host-url), e.g. behind a reverse proxy */
  hostUrl: string
}

export interface OtelSettings {
  enabled: boolean
  /** OTLP/HTTP collector endpoint, e.g. https://collector.example.com:4318 (without /v1/* suffix) */
  endpoint: string
  /** Trace sampling percent 0-100 (metrics are always reported) */
  samplingPercent: number
}

export interface ObservabilitySettings {
  umami: UmamiSettings
  otel: OtelSettings
}

export type UmamiField = 'umami.serverUrl' | 'umami.websiteId' | 'umami.hostUrl'
export type OtelField = 'otel.endpoint' | 'otel.samplingPercent'

export interface ObservabilityValidation {
  valid: boolean
  errors: Partial<Record<UmamiField | OtelField, string>>
}

export function defaultObservabilitySettings(): ObservabilitySettings {
  return {
    umami: { enabled: false, serverUrl: '', websiteId: '', hostUrl: '' },
    otel: { enabled: false, endpoint: '', samplingPercent: 100 },
  }
}

/** Strips trailing slashes so URL joins like `${base}/script.js` work reliably. */
export function normalizeBaseUrl(url: string): string {
  return url.replace(/\/+$/, '')
}

/** Accepts only http(s) absolute URLs. */
export function isHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function validateObservabilitySettings(settings: ObservabilitySettings): ObservabilityValidation {
  const errors: ObservabilityValidation['errors'] = {}

  if (settings.umami.enabled) {
    if (!settings.umami.serverUrl.trim()) {
      errors['umami.serverUrl'] = 'Server URL is required.'
    } else if (!isHttpUrl(settings.umami.serverUrl)) {
      errors['umami.serverUrl'] = 'Must be a valid http(s) URL.'
    }
    if (!settings.umami.websiteId.trim()) {
      errors['umami.websiteId'] = 'Website ID is required.'
    }
    if (settings.umami.hostUrl.trim() && !isHttpUrl(settings.umami.hostUrl)) {
      errors['umami.hostUrl'] = 'Must be a valid http(s) URL.'
    }
  }

  if (settings.otel.enabled) {
    if (!settings.otel.endpoint.trim()) {
      errors['otel.endpoint'] = 'Endpoint is required.'
    } else if (!isHttpUrl(settings.otel.endpoint)) {
      errors['otel.endpoint'] = 'Must be a valid http(s) URL.'
    }
    const sampling = Number(settings.otel.samplingPercent)
    if (Number.isNaN(sampling) || sampling < 0 || sampling > 100) {
      errors['otel.samplingPercent'] = 'Must be between 0 and 100.'
    }
  }

  return { valid: Object.keys(errors).length === 0, errors }
}
