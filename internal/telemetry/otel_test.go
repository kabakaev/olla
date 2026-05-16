package telemetry

import (
	"context"
	"net"
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
