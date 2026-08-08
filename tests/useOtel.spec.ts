import { beforeEach, describe, expect, it, vi } from 'vitest'

// Mock the entire OTel SDK surface useOtel.ts loads via dynamic import().
const m = vi.hoisted(() => {
  const providerShutdown = vi.fn(async () => {})
  const meterShutdown = vi.fn(async () => {})
  const unload = vi.fn()
  const counterAdd = vi.fn()
  const createCounter = vi.fn(() => ({ add: counterAdd }))
  const meter = { createCounter }
  const tracer = { startSpan: vi.fn(() => ({ end: vi.fn() })) }
  return {
    providerShutdown,
    meterShutdown,
    unload,
    counterAdd,
    createCounter,
    meter,
    tracer,
    TraceIdRatioBasedSampler: vi.fn(),
    WebTracerProvider: vi.fn(function (this: Record<string, unknown>) {
      this.register = vi.fn()
      this.shutdown = providerShutdown
    }),
    BatchSpanProcessor: vi.fn(),
    OTLPTraceExporter: vi.fn(),
    MeterProvider: vi.fn(function (this: Record<string, unknown>) {
      this.shutdown = meterShutdown
    }),
    PeriodicExportingMetricReader: vi.fn(),
    OTLPMetricExporter: vi.fn(),
    registerInstrumentations: vi.fn(() => unload),
    FetchInstrumentation: vi.fn(),
    XMLHttpRequestInstrumentation: vi.fn(),
    resourceFromAttributes: vi.fn(() => ({})),
    ATTR_SERVICE_NAME: 'service.name',
    setGlobalTracerProvider: vi.fn(),
    setGlobalMeterProvider: vi.fn(),
    getTracer: vi.fn(() => tracer),
    getMeter: vi.fn(() => meter),
    traceDisable: vi.fn(),
    metricsDisable: vi.fn(),
  }
})

vi.mock('@opentelemetry/api', () => ({
  trace: {
    setGlobalTracerProvider: m.setGlobalTracerProvider,
    getTracer: m.getTracer,
    disable: m.traceDisable,
  },
  metrics: {
    setGlobalMeterProvider: m.setGlobalMeterProvider,
    getMeter: m.getMeter,
    disable: m.metricsDisable,
  },
}))
vi.mock('@opentelemetry/sdk-trace-web', () => ({
  WebTracerProvider: m.WebTracerProvider,
  TraceIdRatioBasedSampler: m.TraceIdRatioBasedSampler,
  BatchSpanProcessor: m.BatchSpanProcessor,
}))
vi.mock('@opentelemetry/exporter-trace-otlp-http', () => ({ OTLPTraceExporter: m.OTLPTraceExporter }))
vi.mock('@opentelemetry/sdk-metrics', () => ({
  MeterProvider: m.MeterProvider,
  PeriodicExportingMetricReader: m.PeriodicExportingMetricReader,
}))
vi.mock('@opentelemetry/exporter-metrics-otlp-http', () => ({ OTLPMetricExporter: m.OTLPMetricExporter }))
vi.mock('@opentelemetry/instrumentation', () => ({ registerInstrumentations: m.registerInstrumentations }))
vi.mock('@opentelemetry/instrumentation-fetch', () => ({ FetchInstrumentation: m.FetchInstrumentation }))
vi.mock('@opentelemetry/instrumentation-xml-http-request', () => ({ XMLHttpRequestInstrumentation: m.XMLHttpRequestInstrumentation }))
vi.mock('@opentelemetry/resources', () => ({ resourceFromAttributes: m.resourceFromAttributes }))
vi.mock('@opentelemetry/semantic-conventions', () => ({ ATTR_SERVICE_NAME: m.ATTR_SERVICE_NAME }))

// useOtel keeps module-singleton state; load a fresh module per test.
async function loadMod() {
  vi.resetModules()
  vi.clearAllMocks()
  return import('@/composables/useOtel')
}

const enabledConfig = {
  enabled: true,
  endpoint: 'https://collector.example.com:4318',
  samplingPercent: 25,
}

describe('useOtel initOtel', () => {
  beforeEach(async () => {
    await loadMod()
  })

  it('returns false and does not touch the SDK when disabled', async () => {
    const otel = await loadMod()
    const result = await otel.initOtel({ enabled: false, endpoint: '', samplingPercent: 100 })
    expect(result).toBe(false)
    expect(m.WebTracerProvider).not.toHaveBeenCalled()
    expect(m.OTLPTraceExporter).not.toHaveBeenCalled()
    expect(m.registerInstrumentations).not.toHaveBeenCalled()
  })

  it('returns false for a non-http(s) endpoint', async () => {
    const otel = await loadMod()
    const result = await otel.initOtel({ enabled: true, endpoint: 'not-a-url', samplingPercent: 100 })
    expect(result).toBe(false)
    expect(m.WebTracerProvider).not.toHaveBeenCalled()
  })

  it('builds the sampler from the sampling percent', async () => {
    const otel = await loadMod()
    const result = await otel.initOtel(enabledConfig)
    expect(result).toBe(true)
    expect(m.TraceIdRatioBasedSampler).toHaveBeenCalledWith(0.25)
    expect(otel.isOtelActive().value).toBe(true)
  })

  it('clamps the sampling percent to 0-100', async () => {
    const otel = await loadMod()
    await otel.initOtel({ ...enabledConfig, samplingPercent: 150 })
    expect(m.TraceIdRatioBasedSampler).toHaveBeenCalledWith(1)

    await otel.shutdownOtel()
    await otel.initOtel({ ...enabledConfig, samplingPercent: -5 })
    expect(m.TraceIdRatioBasedSampler).toHaveBeenCalledWith(0)
  })

  it('points exporters at <endpoint>/v1/traces and /v1/metrics', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)
    expect(m.OTLPTraceExporter).toHaveBeenCalledWith({
      url: 'https://collector.example.com:4318/v1/traces',
    })
    expect(m.OTLPMetricExporter).toHaveBeenCalledWith({
      url: 'https://collector.example.com:4318/v1/metrics',
    })
  })

  it('registers trace + metric providers and a 30s metric reader', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)
    expect(m.setGlobalTracerProvider).toHaveBeenCalledWith(expect.any(m.WebTracerProvider))
    expect(m.setGlobalMeterProvider).toHaveBeenCalledWith(expect.any(m.MeterProvider))
    expect(m.PeriodicExportingMetricReader).toHaveBeenCalledWith(
      expect.objectContaining({ exportIntervalMillis: 30000 }),
    )
  })

  it('registers fetch + XHR instrumentations', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)
    expect(m.FetchInstrumentation).toHaveBeenCalledTimes(1)
    expect(m.XMLHttpRequestInstrumentation).toHaveBeenCalledTimes(1)
    expect(m.registerInstrumentations).toHaveBeenCalledWith({
      instrumentations: [expect.any(m.FetchInstrumentation), expect.any(m.XMLHttpRequestInstrumentation)],
    })
  })
})

describe('useOtel shutdown + no-op behavior', () => {
  it('record helpers are safe no-ops while inactive', async () => {
    const otel = await loadMod()
    expect(() => otel.recordSwipeAction('kept', { assetType: 'IMAGE' })).not.toThrow()
    expect(() => otel.recordPersonFilter('Alice')).not.toThrow()
    expect(() => otel.traceReviewAction('keep')).not.toThrow()
    expect(m.createCounter).not.toHaveBeenCalled()
  })

  it('shutdownOtel disables instrumentations, shuts down providers and resets globals', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)
    expect(otel.isOtelActive().value).toBe(true)

    await otel.shutdownOtel()
    expect(m.unload).toHaveBeenCalled()
    expect(m.providerShutdown).toHaveBeenCalled()
    expect(m.meterShutdown).toHaveBeenCalled()
    expect(m.traceDisable).toHaveBeenCalled()
    expect(m.metricsDisable).toHaveBeenCalled()
    expect(otel.isOtelActive().value).toBe(false)
  })

  it('is safe to call shutdownOtel when never initialized', async () => {
    const otel = await loadMod()
    await expect(otel.shutdownOtel()).resolves.toBeUndefined()
  })

  it('re-initialization after shutdown works (no double-init crash)', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)
    await otel.shutdownOtel()
    const again = await otel.initOtel(enabledConfig)
    expect(again).toBe(true)
    expect(otel.isOtelActive().value).toBe(true)
  })
})

describe('useOtel stats API', () => {
  it('records swipe counters with attributes when active', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)

    otel.recordSwipeAction('kept', { assetType: 'IMAGE', personFiltered: true })
    expect(m.createCounter).toHaveBeenCalledWith('swipe.kept')
    expect(m.counterAdd).toHaveBeenCalledWith(1, { assetType: 'IMAGE', personFiltered: true })

    otel.recordPersonFilter('Alice')
    expect(m.createCounter).toHaveBeenCalledWith('swipe.person_filter')
    expect(m.counterAdd).toHaveBeenCalledWith(1, { personName: 'Alice' })
  })

  it('emits a swipe.review_action span with attributes when active', async () => {
    const otel = await loadMod()
    await otel.initOtel(enabledConfig)

    otel.traceReviewAction('keep', { assetType: 'IMAGE', personFiltered: false })
    expect(m.tracer.startSpan).toHaveBeenCalledWith('swipe.review_action', {
      attributes: { actionType: 'keep', assetType: 'IMAGE', personFiltered: false },
    })
  })
})
