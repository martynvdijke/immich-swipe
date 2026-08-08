## 1. Foundation: dependencies and settings store

- [ ] 1.1 Add pinned OTel dependencies to `package.json`: `@opentelemetry/api`, `@opentelemetry/sdk-trace-web`, `@opentelemetry/exporter-trace-otlp-http`, `@opentelemetry/sdk-metrics`, `@opentelemetry/exporter-metrics-otlp-http`, `@opentelemetry/instrumentation-fetch`, `@opentelemetry/instrumentation-xml-http-request` (all on the same release line) and run `npm install`
- [ ] 1.2 Create `src/types/observability.ts` with `ObservabilitySettings` (umami: enabled/serverUrl/websiteId/hostUrl; otel: enabled/endpoint/samplingPercent), default values, and a validation helper (http(s) URLs, website ID non-empty when enabled, sampling 0–100)
- [ ] 1.3 Create `src/stores/observability.ts` following the `preferences.ts` pattern: storage key `immich-swipe-observability:<server>:<user>`, load-on-init/watch-server-user, persist-on-change, `setUmami`/`setOtel` actions, `initialized` flag
- [ ] 1.4 Add unit tests for the observability store (defaults, persistence round-trip, per-server/user isolation) and validation helper

## 2. Umami integration

- [ ] 2.1 Create `src/composables/useUmami.ts`: `loadUmami(config)` injects the `<script async>` with `src=<server>/script.js`, `data-website-id`, `data-host-url` (optional), `data-auto-track="false"`, dedupes per configuration, resolves on `load` / rejects on `error`
- [ ] 2.2 Add `trackPageView()` using `window.umami?.track()` guarded by readiness state, and `trackEvent(name, payload)` safe-call helper (never throws, silent when unavailable)
- [ ] 2.3 Wire page-view tracking: in `src/router/index.ts` or `main.ts`, call `trackPageView()` from `router.afterEach` only when Umami is active
- [ ] 2.4 Add unit tests for script injection (attributes, no duplicate injection, disabled → no script) and event helpers with a mocked `window.umami`

## 3. Browser OpenTelemetry

- [ ] 3.1 Create `src/composables/useOtel.ts` with lazy `initOtel(config)`: dynamic `import()` of the OTel SDK modules, `WebTracerProvider` + `BatchSpanProcessor` + `OTLPTraceExporter` to `<endpoint>/v1/traces`, `TraceIdRatioBased` sampler from `samplingPercent`, `MeterProvider` + `PeriodicExportingMetricReader` (30s) to `<endpoint>/v1/metrics`, service name `immich-swipe-web`
- [ ] 3.2 Register `FetchInstrumentation` and `XMLHttpRequestInstrumentation` (apply custom `http` attributes) and set global tracer/meter providers
- [ ] 3.3 Implement `shutdownOtel()`: `instrumentation.disable()` (restoring patched globals), `provider.shutdown()`, reset global providers to noop; guard against double-init and init-after-shutdown
- [ ] 3.4 Add stats instrumentation API: counters `swipe.kept`/`swipe.deleted`/`swipe.undo`/`swipe.album_added` (attrs `assetType`, `personFiltered`) and `swipe.person_filter` (attr `personName`), plus a `swipe.review_action` span helper; all no-ops when OTel is inactive
- [ ] 3.5 Add unit tests: sampler from percent, endpoint URL construction, teardown disables instrumentation and stops exports (mocked exporters), no-op behavior when disabled

## 4. Settings UI

- [ ] 4.1 Create `src/views/SettingsView.vue`: two sections (Umami, OpenTelemetry) with enable toggles, URL/ID inputs, sampling input, inline validation errors, Save button that persists via the store and hot-applies changes, and a status hint showing Umami script load state
- [ ] 4.2 Register `/settings` route in `src/router/index.ts` with `requiresAuth` meta and lazy component import
- [ ] 4.3 Add a settings (gear) entry to `src/components/AppHeader.vue` linking to `/settings`, consistent with the existing Person/Album/Scope buttons (desktop + mobile, aria-label, active styling)
- [ ] 4.4 Add component tests for SettingsView: validation errors block saving, save persists + applies, previous config stays active on invalid submit

## 5. Event wiring

- [ ] 5.1 In `src/composables/useImmich.ts` add a `trackReviewAction` helper calling the Umami/OTel composables, and call it from `keepPhoto`, `deletePhoto`, `undoLastAction` (passing the undone action type), `keepPhotoToAlbum`, and `toggleFavorite` (keep direction only) — behavior unchanged when observability is off
- [ ] 5.2 In `src/views/HomeView.vue` emit `swipe.person_filter` from `handlePersonSelect` (with person name) and `handlePersonClear`
- [ ] 5.3 Initialize integrations on app start: a bootstrap step (e.g. in `main.ts` or `App.vue` setup) that reads the store and calls `loadUmami`/`initOtel` when enabled, and reacts to store changes so hot-apply/reconfigure/teardown work
- [ ] 5.4 Add a test asserting review actions emit events/counters only when integrations are active (mocked `window.umami` / OTel composable)

## 6. Docs and verification

- [ ] 6.1 Update `README.md`: document the Settings view, Umami setup (server URL + website ID), OTLP endpoint + sampling, browser-only scope (server OTel stays env-driven), required collector CORS config, and OTel version pinning note
- [ ] 6.2 Add a comment to `env.example` clarifying that server-side OTel remains configured via `OTEL_*` env vars and is separate from browser settings
- [ ] 6.3 Run `npm run type-check` and `npm test`, fix any failures; run `npm run build` and confirm the OTel/Umami code lands in a lazily-loaded async chunk (not the main bundle)
