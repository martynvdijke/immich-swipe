import { ref, type Ref } from 'vue'
import { isHttpUrl, normalizeBaseUrl, type OtelSettings } from '@/types/observability'

/**
 * Browser-side OpenTelemetry integration (traces + stats).
 *
 * Everything is lazy: the SDK modules are only loaded via dynamic `import()`
 * when `initOtel` is called with an enabled, valid config. All stats helpers
 * are safe no-ops while OTel is inactive, so the rest of the app never needs
 * to know whether tracking is on.
 *
 * Singleton by design: only one runtime can be active at a time.
 */

export interface SwipeActionAttrs {
  assetType?: string
  personFiltered?: boolean
  albumName?: string
  personName?: string
}

interface OtelRuntime {
  tracer: import('@opentelemetry/api').Tracer
  meter: import('@opentelemetry/api').Meter
  counters: Record<string, import('@opentelemetry/api').Counter>
  unloadInstrumentations: () => void
  shutdown: () => Promise<void>
}

let runtime: OtelRuntime | null = null
let initPromise: Promise<boolean> | null = null

const active = ref(false)
export function isOtelActive(): Readonly<Ref<boolean>> {
  return active
}

const SERVICE_NAME = 'immich-swipe-web'
const METRIC_EXPORT_INTERVAL_MS = 30_000

function counterNameFor(action: string): string {
  return `swipe.${action}`
}

/**
 * Initialize the browser OTel runtime from settings.
 * - No-op (returns `false`) when disabled or misconfigured.
 * - Re-initializes when called again with a different config (tears down first).
 * - Resolves `true` once tracer + meter providers are registered and
 *   fetch/XHR instrumentations are active.
 */
export async function initOtel(config: OtelSettings): Promise<boolean> {
  if (!config.enabled) {
    await shutdownOtel()
    return false
  }

  const endpoint = normalizeBaseUrl(config.endpoint.trim())
  if (!isHttpUrl(endpoint)) {
    await shutdownOtel()
    return false
  }

  // Serialize concurrent init calls.
  if (initPromise) return initPromise

  if (runtime) {
    await shutdownOtel()
  }

  const sampling = Math.min(100, Math.max(0, Number(config.samplingPercent) || 0)) / 100

  initPromise = (async () => {
    try {
      const [api, traceWeb, traceExporter, metrics, metricsExporter, instrumentation, fetchInstMod, xhrInstMod, resources, semconv] =
        await Promise.all([
          import('@opentelemetry/api'),
          import('@opentelemetry/sdk-trace-web'),
          import('@opentelemetry/exporter-trace-otlp-http'),
          import('@opentelemetry/sdk-metrics'),
          import('@opentelemetry/exporter-metrics-otlp-http'),
          import('@opentelemetry/instrumentation'),
          import('@opentelemetry/instrumentation-fetch'),
          import('@opentelemetry/instrumentation-xml-http-request'),
          import('@opentelemetry/resources'),
          import('@opentelemetry/semantic-conventions'),
        ])

      const resource = resources.resourceFromAttributes({
        [semconv.ATTR_SERVICE_NAME]: SERVICE_NAME,
      })

      const traceExporterInst = new traceExporter.OTLPTraceExporter({
        url: `${endpoint}/v1/traces`,
      })

      const tracerProvider = new traceWeb.WebTracerProvider({
        resource,
        sampler: new traceWeb.TraceIdRatioBasedSampler(sampling),
        spanProcessors: [new traceWeb.BatchSpanProcessor(traceExporterInst)],
      })
      tracerProvider.register()
      api.trace.setGlobalTracerProvider(tracerProvider)

      const metricExporter = new metricsExporter.OTLPMetricExporter({
        url: `${endpoint}/v1/metrics`,
      })
      const meterProvider = new metrics.MeterProvider({
        resource,
        readers: [
          new metrics.PeriodicExportingMetricReader({
            exporter: metricExporter,
            exportIntervalMillis: METRIC_EXPORT_INTERVAL_MS,
          }),
        ],
      })
      api.metrics.setGlobalMeterProvider(meterProvider)

      const tracer = api.trace.getTracer(SERVICE_NAME)
      const meter = api.metrics.getMeter(SERVICE_NAME)

      const ignoreUrls = [new RegExp(endpoint.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))]

      const fetchInstrumentation = new fetchInstMod.FetchInstrumentation({
        ignoreUrls,
        applyCustomAttributesOnSpan: (span) => {
          span.setAttribute('app.name', SERVICE_NAME)
        },
      })
      const xhrInstrumentation = new xhrInstMod.XMLHttpRequestInstrumentation({
        ignoreUrls,
        applyCustomAttributesOnSpan: (span) => {
          span.setAttribute('app.name', SERVICE_NAME)
        },
      })

      const unloadInstrumentations = instrumentation.registerInstrumentations({
        instrumentations: [fetchInstrumentation, xhrInstrumentation],
      })

      runtime = {
        tracer,
        meter,
        counters: {},
        unloadInstrumentations,
        shutdown: async () => {
          await Promise.allSettled([tracerProvider.shutdown(), meterProvider.shutdown()])
        },
      }
      active.value = true
      return true
    } catch (e) {
      console.error('Failed to initialize OpenTelemetry', e)
      await shutdownOtel()
      return false
    } finally {
      initPromise = null
    }
  })()

  return initPromise
}

/**
 * Tear down the active runtime: disable instrumentations (restores patched
 * globals), shut down providers (stops all OTLP traffic), and reset the global
 * providers to noop. Safe to call when inactive.
 */
export async function shutdownOtel(): Promise<void> {
  if (initPromise) {
    // A config load is in flight; make sure it does not leave a runtime behind.
    await initPromise.catch(() => {})
  }
  if (!runtime) return

  const { unloadInstrumentations, shutdown } = runtime
  runtime = null
  active.value = false

  try {
    unloadInstrumentations()
  } catch (e) {
    console.error('Failed to disable OTel instrumentations', e)
  }

  await shutdown()

  // Reset globals to noop so app code keeps working after teardown.
  try {
    const api = await import('@opentelemetry/api')
    api.trace.disable()
    api.metrics.disable()
  } catch {
    // SDK not loaded — nothing to reset.
  }
}

function getCounter(action: string): import('@opentelemetry/api').Counter | null {
  if (!runtime) return null
  const name = counterNameFor(action)
  if (!runtime.counters[name]) {
    runtime.counters[name] = runtime.meter.createCounter(name)
  }
  return runtime.counters[name]
}

/**
 * Record a swipe action counter (`swipe.kept`, `swipe.deleted`, ...).
 * No-op while OTel is inactive.
 */
export function recordSwipeAction(action: 'kept' | 'deleted' | 'skipped' | 'undo' | 'album_added', attrs?: SwipeActionAttrs): void {
  const counter = getCounter(action)
  if (!counter) return
  const attributes: import('@opentelemetry/api').Attributes = {}
  if (attrs?.assetType) attributes['assetType'] = attrs.assetType
  if (attrs?.personFiltered !== undefined) attributes['personFiltered'] = attrs.personFiltered
  if (attrs?.albumName) attributes['albumName'] = attrs.albumName
  counter.add(1, attributes)
}

/** Record a person-filter change counter (`swipe.person_filter`). No-op while inactive. */
export function recordPersonFilter(personName: string): void {
  const counter = getCounter('person_filter')
  if (!counter) return
  counter.add(1, { personName })
}

/**
 * Record a `swipe.review_action` span for a review action (keep/delete/undo/...).
 * The span is started and ended immediately, carrying action attributes.
 * No-op while OTel is inactive.
 */
export function traceReviewAction(actionType: string, attrs?: SwipeActionAttrs): void {
  if (!runtime) return
  const attributes: import('@opentelemetry/api').Attributes = { actionType }
  if (attrs?.assetType) attributes['assetType'] = attrs.assetType
  if (attrs?.personFiltered !== undefined) attributes['personFiltered'] = attrs.personFiltered
  if (attrs?.albumName) attributes['albumName'] = attrs.albumName
  if (attrs?.personName) attributes['personName'] = attrs.personName
  const span = runtime.tracer.startSpan('swipe.review_action', { attributes })
  span.end()
}
