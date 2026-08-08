## Why

Immich Swipe offers no insight into how it is actually used. Server-side OpenTelemetry exists but is configured exclusively through `OTEL_*` environment variables — changing it requires container reconfiguration, and it says nothing about browser-side usage. Self-hosted owners want to plug in their own Umami instance and OTLP collector directly from the app, with the configuration persisting across sessions instead of being baked into deployment env vars.

## What Changes

- **New Settings view** (`/settings`, reachable from the app header) hosting all observability configuration.
- **Umami analytics integration**: lazy-load the Umami tracking script from a configurable endpoint, send page views on route changes, and emit custom events for review actions (keep, delete, undo, keep-to-album, person filter).
- **Browser-side OpenTelemetry**: initialize the OTel web SDK from the settings (OTLP endpoint, protocol, sampling), producing browser traces and stats (metrics-style event counters) for review actions and API latency.
- **Persistent settings**: observability configuration stored in `localStorage` per server/user, mirroring the existing preferences-store pattern; applied on page load.
- **No server changes**: the Go backend keeps its existing env-var-based OTel stack untouched. Configuring browser observability never requires a rebuild or restart.
- **Graceful degradation**: when Umami/OTel are disabled or misconfigured, the app behaves exactly as today (no failed requests, no console spam, no tracking).

## Capabilities

### New Capabilities
- `observability-settings`: settings UI and persistence for analytics/tracing configuration, plus runtime (un)loading of the integrations
- `umami-analytics`: Umami script injection, page-view tracking, and custom review-event tracking
- `browser-tracing`: browser OpenTelemetry SDK initialization from settings, trace + stats emission for review actions

### Modified Capabilities
<!-- None — `openspec/specs/` currently contains no active specs. -->

## Impact

- **New dependencies**: `@opentelemetry/api`, `@opentelemetry/sdk-trace-web`, `@opentelemetry/exporter-trace-otlp-http`, and the OTel metric SDK (`@opentelemetry/sdk-metrics`) for stats counters.
- **Frontend files**: new `src/views/SettingsView.vue`, new observability store or extended `src/stores/preferences.ts`, new composables (e.g. `src/composables/useObservability.ts`, `useUmami.ts`), `src/router/index.ts` (new route), `src/components/AppHeader.vue` (settings entry), `src/views/HomeView.vue` + `useImmich.ts` (event hooks for keep/delete/undo/album/person).
- **Docs**: `README.md` documents the new settings; `env.example` notes that server-side OTel remains env-var-driven.
- **Not affected**: Go backend (`server/`), auth flow, Docker build, CI/CD.
