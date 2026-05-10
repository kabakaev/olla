package telemetry

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thushan/olla/internal/config"
	otellog "go.opentelemetry.io/otel/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestOTLPHeaders(t *testing.T) {
	// Setup a mock gRPC server
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	serverAddr := lis.Addr().String()
	receivedHeaders := make(chan metadata.MD, 100)

	s := grpc.NewServer(
		grpc.UnknownServiceHandler(func(srv interface{}, stream grpc.ServerStream) error {
			if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
				receivedHeaders <- md
			}
			return nil
		}),
	)

	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	ctx := context.Background()
	cfg := config.TelemetryConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: config.TelemetryOTLPConfig{
			Endpoint: serverAddr,
			Insecure: true,
			Headers: map[string]string{
				"Authorization":   "Bearer test-token-123",
				"X-Custom-Header": "custom-value",
			},
		},
		Traces:  config.TelemetrySignalConfig{Enabled: true},
		Metrics: config.TelemetrySignalConfig{Enabled: true},
		Logs:    config.TelemetryLogsConfig{Enabled: true},
	}

	providers, err := InitProviders(ctx, cfg)
	require.NoError(t, err)

	// Trigger all three signals
	// 1. Trace
	tracer := providers.TracerProvider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	span.End()

	// 2. Metric
	meter := providers.MeterProvider.Meter("test")
	counter, _ := meter.Int64Counter("test-counter")
	counter.Add(ctx, 1)

	// 3. Log
	logger := providers.LoggerProvider.Logger("test")
	var record otellog.Record
	record.SetBody(otellog.StringValue("test log message"))
	logger.Emit(ctx, record)

	// Shutdown will flush everything. We use a small timeout to avoid hanging.
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = providers.Shutdown(shutdownCtx)

	// Wait for headers to be received. We expect signals to send headers.
	// We might get multiple requests (one for each signal or more if retried).
	// We just need to verify that we received the expected headers at least once
	// for each signal type if possible, but at minimum that the headers are correct.

	timeout := time.After(5 * time.Second)
	foundAuth := false
	foundCustom := false

	// We expect at least one set of headers to match.
	for {
		select {
		case md := <-receivedHeaders:
			auth := md.Get("authorization")
			custom := md.Get("x-custom-header")

			if len(auth) > 0 && auth[0] == "Bearer test-token-123" {
				foundAuth = true
			}
			if len(custom) > 0 && custom[0] == "custom-value" {
				foundCustom = true
			}

			if foundAuth && foundCustom {
				return // Success
			}
		case <-timeout:
			t.Fatalf("timed out waiting for OTLP headers. FoundAuth=%v, FoundCustom=%v", foundAuth, foundCustom)
		}
	}
}

func TestOTLPHeadersHTTP(t *testing.T) {
	receivedHeaders := make(chan http.Header, 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	cfg := config.TelemetryConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: config.TelemetryOTLPConfig{
			Endpoint: server.Listener.Addr().String(),
			Protocol: "http",
			Insecure: true,
			Headers: map[string]string{
				"Authorization":   "Bearer test-token-123",
				"X-Custom-Header": "custom-value",
			},
		},
		Traces:  config.TelemetrySignalConfig{Enabled: true},
		Metrics: config.TelemetrySignalConfig{Enabled: true},
		Logs:    config.TelemetryLogsConfig{Enabled: true},
	}

	providers, err := InitProviders(ctx, cfg)
	require.NoError(t, err)

	tracer := providers.TracerProvider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	span.End()

	meter := providers.MeterProvider.Meter("test")
	counter, _ := meter.Int64Counter("test-counter")
	counter.Add(ctx, 1)

	logger := providers.LoggerProvider.Logger("test")
	var record otellog.Record
	record.SetBody(otellog.StringValue("test log message"))
	logger.Emit(ctx, record)

	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = providers.Shutdown(shutdownCtx)

	timeout := time.After(5 * time.Second)
	foundAuth := false
	foundCustom := false

	for {
		select {
		case headers := <-receivedHeaders:
			if headers.Get("Authorization") == "Bearer test-token-123" {
				foundAuth = true
			}
			if headers.Get("X-Custom-Header") == "custom-value" {
				foundCustom = true
			}
			if foundAuth && foundCustom {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for OTLP HTTP headers. FoundAuth=%v, FoundCustom=%v", foundAuth, foundCustom)
		}
	}
}

func TestOTLPHTTPPathPrefix(t *testing.T) {
	receivedPaths := make(chan string, 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPaths <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.TelemetryConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: config.TelemetryOTLPConfig{
			Endpoint: server.Listener.Addr().String(),
			Path:     "/api/default/otel",
			Protocol: "http",
			Insecure: true,
		},
		Traces:  config.TelemetrySignalConfig{Enabled: true},
		Metrics: config.TelemetrySignalConfig{Enabled: true},
		Logs:    config.TelemetryLogsConfig{Enabled: true},
	}

	providers, err := InitProviders(context.Background(), cfg)
	require.NoError(t, err)

	tracer := providers.TracerProvider.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	meter := providers.MeterProvider.Meter("test")
	counter, _ := meter.Int64Counter("test-counter")
	counter.Add(context.Background(), 1)

	logger := providers.LoggerProvider.Logger("test")
	var record otellog.Record
	record.SetBody(otellog.StringValue("test log message"))
	logger.Emit(context.Background(), record)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = providers.Shutdown(shutdownCtx)

	timeout := time.After(5 * time.Second)
	found := map[string]bool{
		"/api/default/otel/v1/traces":  false,
		"/api/default/otel/v1/metrics": false,
		"/api/default/otel/v1/logs":    false,
	}

	for {
		select {
		case requestPath := <-receivedPaths:
			if _, ok := found[requestPath]; ok {
				found[requestPath] = true
			}
			if found["/api/default/otel/v1/traces"] && found["/api/default/otel/v1/metrics"] && found["/api/default/otel/v1/logs"] {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for OTLP HTTP paths, got=%v", found)
		}
	}
}

func TestResolveOTLPHTTPConfig_FullURL(t *testing.T) {
	endpoint, urlPath, insecure := resolveOTLPHTTPConfig(config.TelemetryOTLPConfig{
		Endpoint: "http://otel.example:5080/api/default/otel",
		Insecure: false,
	}, defaultHTTPTracePath)

	require.Equal(t, "otel.example:5080", endpoint)
	require.Equal(t, "/api/default/otel/v1/traces", urlPath)
	require.True(t, insecure)
}

func TestJoinOTLPPath(t *testing.T) {
	require.Equal(t, "/api/default/otel/v1/logs", joinOTLPPath("/api/default/otel/", defaultHTTPLogPath))
	require.Equal(t, "/api/default/otel/v1/metrics", joinOTLPPath("api/default/otel", defaultHTTPMetricPath))
	require.True(t, strings.HasPrefix(joinOTLPPath("", defaultHTTPTracePath), "/"))
}
