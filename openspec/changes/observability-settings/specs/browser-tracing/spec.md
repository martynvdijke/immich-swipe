## ADDED Requirements

### Requirement: Browser OpenTelemetry initialization from settings

When OpenTelemetry is enabled, the system SHALL initialize a `WebTracerProvider` with a batch span processor exporting via OTLP/HTTP to the configured endpoint (`<endpoint>/v1/traces`), using a `TraceIdRatioBased` sampler derived from the configured sampling ratio, and a `MeterProvider` with a periodic metric reader (30s interval) exporting to `<endpoint>/v1/metrics`. Both providers SHALL use the service name `immich-swipe-web`. The SDK SHALL be loaded lazily via dynamic import only when enabled.

#### Scenario: Enabled configuration exports traces and metrics
- **WHEN** OpenTelemetry is enabled with endpoint `https://collector.example.com:4318` and the app runs
- **THEN** spans and metrics are exported to `https://collector.example.com:4318/v1/traces` and `.../v1/metrics`

#### Scenario: SDK not loaded when disabled
- **WHEN** OpenTelemetry is disabled
- **THEN** no OTel SDK code is loaded and no requests are sent to any OTLP endpoint

#### Scenario: Sampling ratio respected
- **WHEN** the sampling ratio is set to 25%
- **THEN** approximately a quarter of trace roots are exported while all metric counters continue to be reported

### Requirement: API call tracing

The system SHALL instrument `fetch` and `XMLHttpRequest` while OpenTelemetry is active so that every request to the app's `/api/*` endpoints (including asset thumbnail and preload traffic) produces a span with method, URL path, and response status attributes.

#### Scenario: API request creates a span
- **WHEN** the app loads an asset thumbnail through `/api/assets/<id>/thumbnail` while OpenTelemetry is active
- **THEN** a span for that request with method `GET`, the URL path, and the response status is exported

#### Scenario: No spans when disabled
- **WHEN** OpenTelemetry is disabled
- **THEN** API requests produce no spans and the browser globals are left unpatched

### Requirement: Review action stats

The system SHALL emit OpenTelemetry metrics (counters) and a `swipe.review_action` span for each review action with the following counter names and attributes:
- `swipe.kept` / `swipe.deleted` / `swipe.undo` / `swipe.album_added`, each with attributes `assetType` and `personFiltered` (boolean)
- `swipe.person_filter` with attribute `personName`

#### Scenario: Keep action increments counter
- **WHEN** the user keeps a photo while OpenTelemetry is active
- **THEN** the `swipe.kept` counter is incremented with the asset type attribute and a `swipe.review_action` span is exported

#### Scenario: No stats when disabled
- **WHEN** OpenTelemetry is disabled and the user reviews photos
- **THEN** no counters are incremented and no review spans are exported

### Requirement: Teardown on disable

When OpenTelemetry is disabled or its configuration is removed, the system SHALL shut down the tracer and meter providers, disable registered instrumentations (restoring patched browser globals), and stop all OTLP traffic.

#### Scenario: Toggling off stops OTLP traffic
- **WHEN** the user disables OpenTelemetry in settings
- **THEN** the providers are shut down, instrumentations are disabled, and no further requests reach the configured OTLP endpoint
