## 1. Add OTel Dependencies

- [x] 1.1 Add `go.opentelemetry.io/otel`, `sdk`, `trace`, `metric`, `sdk/metric` for the core OTel SDK
- [x] 1.2 Add `go.opentelemetry.io/otel/exporters/otlp/otlptrace`, `otlptracegrpc`, `otlptracehttp` for OTLP trace export
- [x] 1.3 Add `go.opentelemetry.io/otel/exporters/otlp/otlpmetric`, `otlpmetricgrpc`, `otlpmetrichttp` for OTLP metric export
- [x] 1.4 Add `go.opentelemetry.io/otel/log`, `sdk/log`, `otlploggrpc`, `otlploghttp` for OTel logs
- [x] 1.5 Add `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` for HTTP instrumentation (incoming + outgoing)
- [x] 1.6 Add OTel slog bridge dependency for log-to-trace correlation
- [x] 1.7 Run `go mod tidy` to resolve all new dependencies

## 2. Create server/telemetry.go — OTel Initialization

- [x] 2.1 Create `server/telemetry.go` with `initTelemetry()` function that initializes tracer, meter, and logger providers
- [x] 2.2 Select exporter protocol based on `OTEL_EXPORTER_OTLP_PROTOCOL` (default: `grpc`), supporting both gRPC and HTTP/protobuf for all three pillars
- [x] 2.3 Configure `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG` via OTel SDK sampler
- [x] 2.4 Configure `OTEL_RESOURCE_ATTRIBUTES` via OTel SDK resource detection, with `OTEL_SERVICE_NAME` defaulting to `immich-swipe`
- [x] 2.5 Add graceful shutdown: `defer tp.Shutdown()` with timeout, flush pending spans/metrics/logs
- [x] 2.6 Add graceful degradation: if OTLP exporter connection fails, log warning and fall back to noop

## 3. Add OTel Metrics

- [x] 3.1 Create OTel meter and instruments for HTTP request count (`otel_http_requests_total`) and duration (`otel_http_request_duration_seconds`) with method/path/status labels
- [x] 3.2 Add proxy upstream latency metric (`otel_proxy_upstream_duration_seconds`)
- [x] 3.3 Initialize OTel meter provider with OTLP exporter

## 4. Integrate Incoming HTTP Request Tracing

- [x] 4.1 Add `otelhttp.NewMiddleware("immich-swipe")` to the HTTP server handler chain in `main.go`
- [x] 4.2 Verify trace context propagation from incoming `traceparent` headers

## 5. Integrate Outgoing HTTP Tracing (Reverse Proxy)

- [x] 5.1 Wrap the reverse proxy's transport with `otelhttp.NewTransport()` so outgoing requests to upstream Immich carry `traceparent` header
- [x] 5.2 Verify trace context flows from client → proxy → upstream Immich in a single distributed trace
- [x] 5.3 Test that Immich API responses are unchanged with the wrapped transport

## 6. Add OTel Logs

- [x] 6.1 Initialize OTel logger provider with OTLP log exporter (gRPC primary, HTTP secondary)
- [x] 6.2 Wire the OTel slog bridge so slog log records flow through the OTel logs SDK with trace context
- [x] 6.3 Verify log-to-trace correlation: logs emitted within a span include trace_id and span_id

## 7. Write Tests

- [x] 7.1 Write unit tests for `initTelemetry()`: OTLP endpoint config, noop fallback, sampling config, resource attributes
- [x] 7.2 Write integration test that verifies trace context propagation through the proxy (incoming `traceparent` → outgoing request carries it)
- [x] 7.3 Write test that verifies graceful degradation (unreachable OTLP endpoint doesn't crash server)
- [x] 7.4 Write test that verifies Immich API responses are unchanged with otelhttp transport

## 8. Docker & Verification

- [x] 8.1 Update `docker-compose.yml`: add `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`
- [x] 8.2 Run `go vet ./...` in `server/` — no new warnings
- [x] 8.3 Run `go test ./...` in `server/` — all tests pass
- [x] 8.4 Run `go build -o /dev/null .` in `server/` — binary compiles cleanly
- [x] 8.5 Commit all changes with a conventional commit message (done by orchestrator: "feat: add OpenTelemetry observability to the Go server")