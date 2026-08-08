## Context

The app is a Vue 3 + TypeScript + Tailwind SPA with a Go backend that serves the static build, handles session auth, and reverse-proxies to Immich. The Go server already ships OpenTelemetry support (`server/telemetry.go`) configured exclusively through `OTEL_*` env vars — traces, metrics and logs for server-side HTTP handling and proxy calls. There is no frontend analytics or tracing of any kind, no Umami integration, and no settings UI anywhere (preferences like review order/scope live in `localStorage` per server+user via `src/stores/preferences.ts`).

This change adds browser-side observability — Umami analytics and OpenTelemetry traces/stats — configured by the user in a new persistent Settings view. The Go backend and its env-var OTel stack are deliberately untouched.

## Goals / Non-Goals

**Goals:**
- A `/settings` view (auth-guarded, reachable from the app header) to configure observability.
- Umami integration: lazy script injection, page views on route change, custom events for review actions (keep, delete, undo, keep-to-album, person filter).
- Browser OTel: web SDK initialized from settings, tracing API calls and review actions, plus metric counters ("stats") for review actions and API latency.
- Settings persist in `localStorage` per server+user (same pattern as existing preferences) and apply immediately on change and on page load.
- Graceful degradation: with integrations disabled or misconfigured, the app behaves exactly as today.

**Non-Goals:**
- No server-side changes: the Go backend's env-var OTel stack stays as-is; no new backend APIs.
- No in-app stats dashboard — "stats" here means OTel metrics export, not UI visualization.
- No gRPC OTLP from the browser (browser JS supports OTLP/HTTP only); no auto-instrumentation of DOM interactions or user clicks beyond review events.
- No Umami feature flags / A/B testing / custom event dashboard configuration.

## Decisions

### D1: Separate `observability` store with its own storage key
A new Pinia store `src/stores/observability.ts` holds the config (`umami`, `otel` sections) and persists to `localStorage` key `immich-swipe-observability:<server>:<user>` — mirroring the `immich-swipe-preferences` pattern (per-server/per-user scoping, `initialized` flag, persist-on-watch).
- **Why**: keeps concerns separate from review preferences; same consistency pattern the app already uses.
- **Alternatives considered**: extending `preferences.ts` (rejected: mixes unrelated concerns, grows the existing payload); server-side persistence via a new Go API (rejected: adds backend surface, storage, and deployment complexity for a browser-only feature; env-var server OTel already covers server observability).

### D2: Lazy dynamic loading of all tracking code
Umami script injection and all OTel packages are loaded via dynamic `import()` only when their respective setting is enabled and configured. When disabled, no tracking code is in the critical bundle path and no network calls happen.
- **Why**: keeps the main bundle lean for the common case; Vite code-splits dynamic imports automatically; matches "opt-in observability".
- **Alternatives considered**: static imports of OTel SDK (rejected: adds ~100KB+ to every page load even when unused).

### D3: New dependencies (pinned to one OTel core version)
- `@opentelemetry/api` (peer for all SDK packages)
- `@opentelemetry/sdk-trace-web` (WebTracerProvider)
- `@opentelemetry/exporter-trace-otlp-http` (OTLPTraceExporter)
- `@opentelemetry/sdk-metrics` (MeterProvider)
- `@opentelemetry/exporter-metrics-otlp-http` (OTLPMetricExporter)
- `@opentelemetry/instrumentation-fetch`, `@opentelemetry/instrumentation-xml-http-request` (API call tracing)

All `@opentelemetry/*` runtime deps pinned to the same minor version line (the JS SDK is released in lockstep; mixing majors breaks).
- **Why**: explicit instrumentations (`fetch` + `xhr`) cover exactly the `/api/*` traffic that matters (the app uses `fetch`; preload/image loads use `fetch` with auth headers — XHR instrumentation covers nothing today, but guards future use and is trivial).
- **Alternatives considered**: `@opentelemetry/auto-instrumentations-web` bundle (rejected: drags in document-load/user-interaction instrumentation that duplicates Umami page views and adds noise for a swipe app); manual spans only (rejected: loses automatic API-latency stats without hand-rolled wrappers).

### D4: Umami integration approach
- Script tag injected lazily with `async`, `src = <server>/script.js`, `data-website-id`, optional `data-host-url`, and `data-auto-track="false"` — page views are fired manually from `router.afterEach` to guarantee SPA route tracking without duplicates.
- Custom events via `window.umami?.track(event, { ... })` — safe-call pattern, never throws, never logs errors.
- Event payloads stay small: `{ assetId }` for keep/delete/undo, `{ albumName }` for keep-to-album, `{ personName }` for person-filter.
- **Why**: manual tracking gives deterministic SPA behavior regardless of Umami script version auto-track behavior.
- **Alternatives considered**: relying on Umami's built-in auto-track for SPAs (rejected: version-dependent behavior, risk of double counts with manual events); loading the script at build time via `index.html` (rejected: no configuration, always loads).

### D5: OTel browser setup
- `WebTracerProvider` + `BatchSpanProcessor` with `OTLPTraceExporter({ url: <endpoint>/v1/traces })`; sampler `TraceIdRatioBased(ratio)` from settings (default 1.0).
- `MeterProvider` + `PeriodicExportingMetricReader` (30s interval, matching the server's `metricReportInterval`) with `OTLPMetricExporter({ url: <endpoint>/v1/metrics })`.
- Instrumentations registered: fetch + xml-http-request → spans for every `/api/*` call (includes asset thumbnail/preload traffic, giving real API-latency stats).
- Manual tracer spans/counters from the review composable:
  - counters: `swipe.kept`, `swipe.deleted`, `swipe.undo`, `swipe.album_added` (attributes: `asset_type`, `person_filtered`)
  - span: `swipe.review_action` per action with the same attributes
- Disable/teardown: `provider.shutdown()`, `instrumentation.disable()`, global providers reset to noop — so toggling off in settings stops all traffic immediately.
- Only `http/protobuf` (JSON) is offered in the UI — gRPC is impossible in browsers.
- **Why**: matches the existing server-side metric names (`otel_http_requests_total`, `otel_http_request_duration_seconds`) in spirit and keeps the whole stack on standard OTLP.
- **Alternatives considered**: `@opentelemetry/sdk-trace-base` + manual spans only (rejected: loses automatic API spans); including document-load instrumentation (rejected: duplicates Umami page views).

### D6: Settings UI
- `SettingsView.vue` at route `/settings` (requiresAuth), linked from a gear icon in `AppHeader.vue` (alongside the existing Person/Album/Scope buttons).
- Two sections:
  - **Umami**: enable toggle, server URL, website ID, optional host URL (used when Umami sits behind a proxy).
  - **OpenTelemetry**: enable toggle, OTLP endpoint (e.g. `https://collector.example.com:4318`), sampling ratio slider/input (0–100%).
- Validation: URLs must parse as http(s); website ID non-empty when enabled; endpoint must include scheme and host. Invalid config is refused with inline error text and the previous persisted value stays active.
- "Save" persists immediately and hot-applies: toggling on loads the integration, toggling off tears it down. A small status hint shows whether the Umami script loaded (`window.umami` present after load event / `script.onerror`).

### D7: Event wiring points
Review-action events are emitted inside `useImmich.ts` at the existing action sites (`keepPhoto`, `deletePhoto`, `undoLastAction`, `keepPhotoToAlbum`, `toggleFavorite`) via a tiny `trackReviewAction` helper that fans out to both Umami and OTel — one call site per action, no UI component changes. Person-filter events are emitted from `HomeView.vue`'s `handlePersonSelect`/`handlePersonClear`. Page-view tracking lives in `main.ts`/router `afterEach`.

## Risks / Trade-offs

- **CORS on the OTLP endpoint** — the OTLP HTTP exporter POSTs `application/json` with custom headers from the browser origin; a stock collector/receiver will reject preflight → Mitigation: document required collector CORS config (`http.cors.allowed_origins`) in README; users without CORS control can reverse-proxy the collector same-origin.
- **Umami script blocked/failing** — tracking silently degrades → Mitigation: `script.onerror` + safe-call `window.umami?.` pattern; a status hint in Settings; no console spam, no app impact.
- **OTel SDK bundle size** — shipped only when enabled via dynamic import → Mitigation: lazy loading (D2); verify `vite build` output shows a separate async chunk.
- **OTel package version drift** — SDK packages must be the same release line → Mitigation: pin exact versions in `package.json`, note in README; CI `type-check`/tests catch API mismatches.
- **Trace payload contains asset URLs/IDs** — privacy-relevant for shared collectors → Mitigation: document in README; self-hosted collectors are the target audience.
- **Double tracking of page views** (Umami auto-track vs manual) → Mitigation: `data-auto-track="false"` (D4) plus manual `router.afterEach` tracking.
- **Teardown incompleteness** — instrumentations patch globals; `disable()` restores them → Mitigation: D5 teardown path; integration test toggling settings off asserts no further OTLP requests.
- **Sampling ratio misconfiguration (0% → no traces)** — user confusion → Mitigation: default 100%, explicit "0 disables sampling" hint, status text in Settings.

## Migration Plan

Pure frontend addition; no backend, schema, or data migration.

1. Add pinned OTel deps; implement store, composables, Settings view, router route, header entry.
2. Wire events into `useImmich.ts`/`HomeView.vue` (no behavior change when observability is off).
3. Document in README (settings, OTLP CORS, Umami setup); `env.example` gets a comment that server-side OTel remains env-driven.
4. Rollback = revert the PR; users can also simply disable the integrations in Settings.

## Open Questions

- None blocking. Minor: whether to also expose an OTel "service name" override in Settings (default `immich-swipe-web`) — leaning yes, trivial to add and useful when several apps share one collector.
