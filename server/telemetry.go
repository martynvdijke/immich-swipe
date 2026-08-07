package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const (
	defaultServiceName      = "immich-swipe"
	telemetryShutdownTimeout = 5 * time.Second
	metricReportInterval     = 30 * time.Second
)

// telemetry bundles the OpenTelemetry tracer, meter and logger providers plus
// the HTTP instruments and middleware used by the server. When no OTLP
// endpoint is configured the server degrades gracefully to a noop instance
// (disabled) that leaves the app fully functional without observability.
type telemetry struct {
	disabled bool

	resource        *resource.Resource
	tracerProvider  *sdktrace.TracerProvider
	meterProvider   *sdkmetric.MeterProvider
	loggerProvider  *sdklog.LoggerProvider
	logger          *slog.Logger
	middleware      func(http.Handler) http.Handler
	proxyTransport  http.RoundTripper

	httpRequestsTotal     metric.Int64Counter
	httpRequestDuration   metric.Float64Histogram
	proxyUpstreamDuration metric.Float64Histogram
}

// statusRecorder captures the response status code written by a downstream
// handler so metrics can be labelled with it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// measuringTransport records upstream round-trip latency for the proxy.
type measuringTransport struct {
	base     http.RoundTripper
	duration metric.Float64Histogram
}

func (t *measuringTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", req.Method),
		attribute.Int("http.response.status_code", status),
	}
	t.duration.Record(req.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	return resp, err
}

// otlpProtocol returns the OTLP export protocol from the environment,
// defaulting to gRPC as specified by the change design.
func otlpProtocol() string {
	p := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	if p == "http/protobuf" {
		return "http/protobuf"
	}
	return "grpc"
}

// samplerFromEnv builds an SDK sampler from the standard OTEL_TRACES_SAMPLER
// and OTEL_TRACES_SAMPLER_ARG environment variables. An invalid ratio or an
// unknown sampler name falls back to the specification defaults.
func samplerFromEnv() sdktrace.Sampler {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	arg := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	ratio := 1.0
	if v, err := strconv.ParseFloat(arg, 64); err == nil && v >= 0 && v <= 1 {
		ratio = v
	}
	switch name {
	case "always_off", "never_sample":
		return sdktrace.NeverSample()
	case "always_on", "always_sample":
		return sdktrace.AlwaysSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_on", "parentbased_always_sample":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off", "parentbased_never_sample":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

// buildResource assembles the service resource: OTEL_RESOURCE_ATTRIBUTES and
// process/telemetry attributes come from resource.Default(), and the service
// name defaults to immich-swipe unless OTEL_SERVICE_NAME overrides it.
func buildResource() *resource.Resource {
	name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if name == "" {
		name = defaultServiceName
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(name)),
	)
	if err != nil {
		return resource.Default()
	}
	return res
}

// initTelemetry wires up the full OTel stack from OTEL_* environment
// variables. Without OTEL_EXPORTER_OTLP_ENDPOINT it returns a noop instance so
// the server runs unchanged. Exporter construction never blocks startup.
func initTelemetry() *telemetry {
	res := buildResource()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) == "" {
		return &telemetry{
			disabled:       true,
			resource:       res,
			logger:         logger,
			proxyTransport: http.DefaultTransport,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	protocol := otlpProtocol()

	traceExporter, err := newTraceExporter(ctx, protocol)
	if err != nil {
		log.Printf("telemetry: trace exporter init failed, disabling traces: %v", err)
		traceExporter = nil
	}
	metricExporter, err := newMetricExporter(ctx, protocol)
	if err != nil {
		log.Printf("telemetry: metric exporter init failed, disabling metrics: %v", err)
		metricExporter = nil
	}
	logExporter, err := newLogExporter(ctx, protocol)
	if err != nil {
		log.Printf("telemetry: log exporter init failed, disabling logs: %v", err)
		logExporter = nil
	}

	t := &telemetry{resource: res}

	if traceExporter != nil {
		t.tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithSampler(samplerFromEnv()),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(t.tracerProvider)
	}
	if metricExporter != nil {
		t.meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(metricReportInterval))),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(t.meterProvider)
	}
	if logExporter != nil {
		t.loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
			sdklog.WithResource(res),
		)
		logglobal.SetLoggerProvider(t.loggerProvider)
	}

	// Ensure W3C trace context (and baggage) is propagated in and out.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Wire slog through the OTel log SDK so log records carry trace context.
	if t.loggerProvider != nil {
		handler := otelslog.NewHandler(defaultServiceName, otelslog.WithLoggerProvider(t.loggerProvider))
		t.logger = slog.New(handler)
	} else {
		t.logger = logger
	}
	log.SetOutput(slog.NewLogLogger(t.logger.Handler(), slog.LevelInfo).Writer())

	// Create the shared meter instruments (used by the middleware + transport).
	meter := otel.Meter(defaultServiceName)
	t.httpRequestsTotal, _ = meter.Int64Counter("otel_http_requests_total",
		metric.WithDescription("Total number of HTTP requests handled by the server."))
	t.httpRequestDuration, _ = meter.Float64Histogram("otel_http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests handled by the server."))
	t.proxyUpstreamDuration, _ = meter.Float64Histogram("otel_proxy_upstream_duration_seconds",
		metric.WithDescription("Duration of requests forwarded to the upstream Immich server."))

	t.middleware = otelhttp.NewMiddleware(defaultServiceName)
	t.proxyTransport = otelhttp.NewTransport(&measuringTransport{
		base:     http.DefaultTransport,
		duration: t.proxyUpstreamDuration,
	})

	return t
}

func newTraceExporter(ctx context.Context, protocol string) (sdktrace.SpanExporter, error) {
	if protocol == "http/protobuf" {
		return otlptracehttp.New(ctx)
	}
	return otlptracegrpc.New(ctx)
}

func newMetricExporter(ctx context.Context, protocol string) (sdkmetric.Exporter, error) {
	if protocol == "http/protobuf" {
		return otlpmetrichttp.New(ctx)
	}
	return otlpmetricgrpc.New(ctx)
}

func newLogExporter(ctx context.Context, protocol string) (sdklog.Exporter, error) {
	if protocol == "http/protobuf" {
		return otlploghttp.New(ctx)
	}
	return otlploggrpc.New(ctx)
}

// metricsMiddleware records per-request counters and durations with
// method/path/status labels. Safe to call only on enabled instances.
func (t *telemetry) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", r.Method),
			attribute.String("url.path", r.URL.Path),
			attribute.Int("http.response.status_code", rec.status),
		}
		t.httpRequestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		t.httpRequestDuration.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	})
}

// shutdown flushes and stops all providers with a bounded timeout.
func (t *telemetry) shutdown(ctx context.Context) {
	if t == nil || t.disabled {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, telemetryShutdownTimeout)
	defer cancel()
	if t.tracerProvider != nil {
		_ = t.tracerProvider.Shutdown(ctx)
	}
	if t.meterProvider != nil {
		_ = t.meterProvider.Shutdown(ctx)
	}
	if t.loggerProvider != nil {
		_ = t.loggerProvider.Shutdown(ctx)
	}
}
