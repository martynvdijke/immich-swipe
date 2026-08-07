## Why

immich-swipe's Go server (`server/main.go`) is a reverse proxy for an Immich instance built on the standard `net/http` library. It currently has zero OpenTelemetry instrumentation — no traces, no metrics, no logs. As a proxy sitting between clients and the Immich API, it is a critical observability blind spot: there is no visibility into request latency, error rates, proxy forwarding behavior, or trace context propagation between client and upstream Immich server.

Adding full OTel support — traces, metrics, and logs — with OTLP export unlocks distributed tracing through the proxy chain, HTTP metrics for both incoming and proxied outgoing requests, and structured log correlation. The key OTel use case for a reverse proxy is trace context propagation: incoming `traceparent` headers must be forwarded to the upstream Immich server so traces span the full request lifecycle. All three pillars aligned on a single OTLP endpoint simplifies the collector story.

## What Changes

- **Add OTel SDK dependencies** from scratch — `otel`, `sdk`, `trace`, `metric`, `sdk/metric`, `log`, `sdk/log` (currently zero OTel packages in `server/go.mod`)
- **Add OTLP exporters** (gRPC and HTTP/protobuf) for traces, metrics, and logs
- **Add `otelhttp` middleware** for incoming HTTP request tracing — automatic spans for every proxied request
- **Add `otelhttp` transport** for the reverse proxy's outgoing HTTP client — trace context propagation through the proxy to the upstream Immich server
- **Add OTel metrics** — OTel SDK metric instruments for HTTP request count (`otel_http_requests_total`), duration (`otel_http_request_duration_seconds`), and proxy upstream latency
- **Add OTel logs** — OTel logs SDK with OTLP log export and slog bridge for log-to-trace correlation
- **Set service name** — default `OTEL_SERVICE_NAME` to `immich-swipe`
- **Add configurable sampling and resource attributes** — support `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_RESOURCE_ATTRIBUTES` env vars
- **Graceful degradation** — if OTel is not configured (no OTLP endpoint), fall back to no-op propagation without crashing
- **Tests** — unit tests for telemetry initialization and middleware, integration test verifying trace context propagation through the proxy

## Capabilities

### New Capabilities
- `otel-telemetry`: OpenTelemetry-based distributed tracing, metrics, and logs with configurable OTLP export, HTTP request instrumentation, and trace context propagation through the reverse proxy

### Modified Capabilities
<!-- No existing capabilities are having their requirements changed -->

## Impact

- `server/go.mod`: add `go.opentelemetry.io/otel`, `sdk`, `trace`, `metric`, `sdk/metric`, `log`, `sdk/log`, `exporters/otlp/otlptrace` (`otlptracegrpc`, `otlptracehttp`), `exporters/otlp/otlpmetric` (`otlpmetricgrpc`, `otlpmetrichttp`), `exporters/otlp/otlplog` (`otlploggrpc`, `otlploghttp`), `contrib/instrumentation/net/http/otelhttp`, OTel slog bridge
- `server/main.go`: add telemetry initialization, `otelhttp` middleware for incoming requests, `otelhttp` transport for the reverse proxy, update shutdown
- New file for telemetry setup (e.g., `server/telemetry.go`)
- New env vars: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_TRACES_SAMPLER`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`
- `docker-compose.yml`: add `OTEL_*` env vars, document collector endpoint
- CI: no pipeline changes needed — OTel is a pure code addition