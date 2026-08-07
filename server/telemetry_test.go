package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

func TestTelemetry_OTLPProtocol(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		expected string
	}{
		{"defaults to grpc", "", "grpc"},
		{"http protobuf", "http/protobuf", "http/protobuf"},
		{"case insensitive", "GRPC", "grpc"},
		{"unknown falls back to grpc", "bogus", "grpc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", tc.env)
			if got := otlpProtocol(); got != tc.expected {
				t.Fatalf("otlpProtocol() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestTelemetry_SamplerFromEnv(t *testing.T) {
	cases := []struct {
		name      string
		sampler   string
		arg       string
		wantDesc  string
		wantRatio string
		prefix    bool
	}{
		{"always off", "always_off", "", "AlwaysOffSampler", "", false},
		{"always on", "always_on", "", "AlwaysOnSampler", "", false},
		{"traceidratio with arg", "traceidratio", "0.25", "TraceIDRatioBased", "0.25", true},
		{"traceidratio invalid arg defaults to 1", "traceidratio", "not-a-number", "TraceIDRatioBased", "1", true},
		{"parentbased always on", "parentbased_always_on", "", "ParentBased{root:AlwaysOnSampler", "", true},
		{"unknown falls back to parentbased always on", "bogus", "", "ParentBased{root:AlwaysOnSampler", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", tc.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tc.arg)
			desc := samplerFromEnv().Description()
			if tc.prefix {
				if !strings.HasPrefix(desc, tc.wantDesc) {
					t.Fatalf("sampler Description() = %q, want prefix %q", desc, tc.wantDesc)
				}
				return
			}
			if tc.wantRatio != "" {
				if !strings.HasPrefix(desc, tc.wantDesc) || !strings.Contains(desc, tc.wantRatio) {
					t.Fatalf("sampler Description() = %q, want prefix %q containing %q", desc, tc.wantDesc, tc.wantRatio)
				}
				return
			}
			if desc != tc.wantDesc {
				t.Fatalf("sampler Description() = %q, want %q", desc, tc.wantDesc)
			}
		})
	}
}

func resourceServiceName(t *testing.T, res *resource.Resource) string {
	t.Helper()
	if res == nil {
		return ""
	}
	for _, kv := range res.Attributes() {
		if kv.Key == semconv.ServiceNameKey {
			return kv.Value.AsString()
		}
	}
	return ""
}

func TestTelemetry_ServiceNameDefaults(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	if got := resourceServiceName(t, buildResource()); got != defaultServiceName {
		t.Fatalf("service.name = %q, want %q", got, defaultServiceName)
	}
}

func TestTelemetry_ServiceNameFromEnv(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "my-custom-service")
	if got := resourceServiceName(t, buildResource()); got != "my-custom-service" {
		t.Fatalf("service.name = %q, want %q", got, "my-custom-service")
	}
}

func TestTelemetry_NoopWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	tel := initTelemetry()
	if tel == nil || !tel.disabled {
		t.Fatal("expected a disabled (noop) telemetry instance without an OTLP endpoint")
	}
	if tel.proxyTransport == nil {
		t.Fatal("disabled instance must still provide a working default transport")
	}
	// Must not panic.
	tel.shutdown(context.Background())
}

func TestTelemetry_UnreachableEndpointDoesNotBlock(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	start := time.Now()
	tel := initTelemetry()
	if tel == nil {
		t.Fatal("initTelemetry returned nil")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("initTelemetry blocked for %v with an unreachable endpoint", elapsed)
	}
	tel.shutdown(context.Background())
}

func TestTelemetry_EnabledWithEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	tel := initTelemetry()
	if tel == nil || tel.disabled {
		t.Fatal("expected an enabled telemetry instance")
	}
	if tel.middleware == nil || tel.proxyTransport == nil {
		t.Fatal("enabled instance must expose middleware and a proxy transport")
	}
	if tel.httpRequestsTotal == nil || tel.httpRequestDuration == nil || tel.proxyUpstreamDuration == nil {
		t.Fatal("enabled instance must create the standard instruments")
	}
	tel.shutdown(context.Background())
}

// TestProxyTracePropagation verifies that a request through the reverse proxy
// carries the W3C traceparent of the caller's span to the upstream Immich
// server (client → proxy → upstream in a single distributed trace).
func TestProxyTracePropagation(t *testing.T) {
	var traceparent string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent = r.Header.Get("traceparent")
		if r.Header.Get("x-api-key") != "server-side-key" {
			t.Errorf("upstream missing server-side x-api-key, got %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	srv := NewServer(Config{ServerURL: upstream.URL})
	srv.transport = otelhttp.NewTransport(http.DefaultTransport)
	token := srv.session.CreateAPIKey("Alice", "server-side-key", upstream.URL)

	ctx, parent := tp.Tracer("test").Start(context.Background(), "parent")
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	parent.End()

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d body=%s", rr.Code, rr.Body.String())
	}
	if traceparent == "" {
		t.Fatal("upstream did not receive a traceparent header")
	}
	wantPrefix := "00-" + parent.SpanContext().TraceID().String() + "-"
	if !strings.HasPrefix(traceparent, wantPrefix) {
		t.Fatalf("traceparent %q does not share the parent trace id (prefix %q)", traceparent, wantPrefix)
	}
}

// TestMiddlewareExtractsIncomingTraceparent verifies incoming W3C traceparent
// headers create a span in the same trace (task 4.2).
func TestMiddlewareExtractsIncomingTraceparent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tel := &telemetry{middleware: otelhttp.NewMiddleware("immich-swipe")}
	handler := tel.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	ctx, parent := tp.Tracer("test").Start(context.Background(), "incoming-parent")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	parent.End()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	spans := sr.Ended()
	if len(spans) == 0 {
		t.Fatal("middleware did not create any spans")
	}
	for _, s := range spans {
		if s.SpanContext().TraceID() != parent.SpanContext().TraceID() {
			t.Fatalf("span trace id %s does not match incoming traceparent trace id %s",
				s.SpanContext().TraceID(), parent.SpanContext().TraceID())
		}
	}
}

// TestProxyTransportResponsesUnchanged verifies the otelhttp-wrapped transport
// does not alter upstream status codes or bodies (task 5.3).
func TestProxyTransportResponsesUnchanged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"hello":1}`))
	}))
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	srv.transport = otelhttp.NewTransport(http.DefaultTransport)
	token := srv.session.CreateAPIKey("Alice", "k", upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]int
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unexpected response body %q: %v", rr.Body.String(), err)
	}
	if body["hello"] != 1 {
		t.Fatalf("response body changed through wrapped transport: %s", rr.Body.String())
	}
}
