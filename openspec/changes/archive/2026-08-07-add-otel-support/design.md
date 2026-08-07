## Context

immich-swipe's Go server (`server/main.go`) is a reverse proxy for an Immich instance. It is built on the standard `net/http` library and uses `httputil.NewSingleHostReverseProxy` to forward requests to an upstream Immich server. The server also serves static files and handles user config with API keys. The `server/go.mod` is minimal — module `immich-swipe/server`, go 1.26.5, with zero dependencies currently.

There is no OpenTelemetry instrumentation whatsoever — no traces, no metrics, no logs. As a proxy, the critical OTel use case is trace context propagation: incoming `traceparent` headers must be forwarded to the upstream Immich server so distributed traces span the full client → proxy → Immich request lifecycle.

## Goals / Non-Goals

**Goals:**
- Full OTel implementation from scratch — traces, metrics, and logs
- OTLP exporters (gRPC and HTTP/protobuf) for all three pillars
- Incoming HTTP request tracing via `otelhttp` middleware — automatic spans for every proxied request
- Outgoing HTTP tracing via `otelhttp` transport — trace context propagation through the reverse proxy to upstream Immich
- OTel-native HTTP metrics (request count, duration, proxy upstream latency) exported via OTLP
- OTel logs with OTLP export and slog bridge for log-to-trace correlation
- Set service name default to `immich-swipe` via `OTEL_SERVICE_NAME`
- Standard OTel env var support: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_SERVICE_NAME`
- Graceful degradation: no OTel config → noop, partial failure → warn + fallback
- Unit tests for telemetry init and integration test for trace context propagation
- All existing tests pass, CI stays green

**Non-Goals:**
- Not adding DB query tracing — the proxy has no database
- Not instrumenting the Immich upstream server — only the proxy itself
- Not adding OTel auto-instrumentation agents or sidecars
- Not changing the Dockerfile — OTel config is env-var driven
- Not instrumenting the frontend (Vue/Vite) — only the Go server

## Decisions

**Decision 1: OTLP gRPC as primary exporter, HTTP/protobuf as secondary**

Both `otlptracegrpc`/`otlpmetricgrpc`/`otlploggrpc` and `otlptracehttp`/`otlpmetrichttp`/`otlploghttp` will be supported. The protocol is selected via `OTEL_EXPORTER_OTLP_PROTOCOL` (default: `grpc`).

Rationale: gRPC is the default OTel protocol and the most efficient for high-throughput. HTTP/protobuf is useful when gRPC is blocked. Supporting both adds minimal binary cost.

Alternative considered: Only gRPC. Rejected: HTTP/protobuf is required by the OTel specification and needed for some environments.

**Decision 2: otelhttp for both incoming and outgoing HTTP tracing**

Use `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` for both:
- `otelhttp.NewMiddleware()` as incoming request middleware
- `otelhttp.NewTransport()` as the reverse proxy's outgoing HTTP transport

Rationale: otelhttp automatically creates spans with HTTP semantic convention attributes for incoming requests, and propagates trace context via `traceparent` headers on outgoing requests. This is the critical feature for a reverse proxy — trace context flows from client → proxy → upstream Immich server.

Trade-off: Adds a dependency on `contrib/instrumentation`. The contrib module is maintained by the OTel project and has compatibility guarantees.

**Decision 3: OTel metrics via OTLP export, no Prometheus bridge needed**

immich-swipe has no existing Prometheus metrics, so OTel metric instruments will be exported directly via OTLP. No Prometheus exporter is needed.

Rationale: Without existing Prometheus scrapers to preserve, OTLP is the cleaner path.

**Decision 4: Config via standard OTel env vars only**

The app relies on the Go OTel SDK's automatic env var detection. Do NOT duplicate OTEL_* vars in app config.

Rationale: The OTel SDK already reads all standard env vars. Duplicating this is unnecessary and risks drift from the spec.

**Decision 5: New telemetry.go file for OTel initialization**

- `server/telemetry.go`: new file with `initTelemetry()` function that initializes tracer, meter, and logger providers
- `server/main.go`: add `otelhttp` middleware, wrap reverse proxy transport with `otelhttp`, update shutdown

Rationale: A dedicated telemetry file keeps OTel initialization separate from the proxy logic. The server currently has no telemetry code, so a new file is the cleanest approach.

**Decision 6: OTel logs with slog bridge**

Add OTel logs SDK (`otel/log v0.20.0`, `sdk/log v0.20.0`) with OTLP log export and the OTel slog bridge to route slog records through the OTel logs SDK with automatic trace context injection.

Rationale: The server uses `log` package currently. Migrating to `log/slog` with the OTel slog bridge provides structured logging with trace correlation.

**Decision 7: Trace context propagation through the reverse proxy**

Wrap the reverse proxy's transport with `otelhttp.NewTransport()` so outgoing requests to the upstream Immich server carry the `traceparent` header from the incoming request's span context.

Rationale: This is the core OTel value proposition for a proxy — without it, traces stop at the proxy and the upstream Immich request is invisible. With it, the full request lifecycle is visible in a single trace.

Alternative considered: Manually copy `traceparent` header. Rejected: `otelhttp.NewTransport()` handles this automatically and correctly, including baggage propagation.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| OTLP exporter connection blocks startup | Move exporter connection to background goroutine with timeout; server starts with noop fallback immediately |
| Adding many new dependencies to a previously dep-free go.mod | All deps are from the OTel project (well-maintained, semver-stable); pin versions explicitly |
| `otelhttp` transport changes proxy forwarding behavior | Test thoroughly — otelhttp.Transport wraps http.RoundTripper transparently; verify Immich API responses unchanged |
| Trace context propagation may break if Immich rejects unknown headers | `traceparent` is a W3C standard header; Immich should ignore unknown headers. Test against real Immich instance. |
| Logs SDK is still v0.20.0 (unstable) | Pin version explicitly; API may change in future |
| No service name currently set | Add `OTEL_SERVICE_NAME` default "immich-swipe" in resource detection |

## Open Questions

- Should we add a health check endpoint for the OTel exporter? — Deferred
- Should the frontend (Vue/Vite) also be instrumented with OTel? — Out of scope; separate concern
- Should we add metrics for proxy-specific dimensions (e.g., upstream host, cache hit/miss)? — Consider during implementation