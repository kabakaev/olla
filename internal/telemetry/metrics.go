package telemetry

import (
	"context"

	"github.com/thushan/olla/internal/core/constants"
	"github.com/thushan/olla/internal/core/domain"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type instruments struct {
	requestsTotal      metric.Int64Counter
	requestLatencyMs   metric.Int64Histogram
	inputTokens        metric.Int64Counter
	outputTokens       metric.Int64Counter
	totalTokens        metric.Int64Counter
}

var meterInstruments instruments

func InitMetrics(meter metric.Meter) {
	if meter == nil {
		return
	}

	meterInstruments.requestsTotal, _ = meter.Int64Counter("olla.requests.total")
	meterInstruments.requestLatencyMs, _ = meter.Int64Histogram("olla.requests.latency_ms")
	meterInstruments.inputTokens, _ = meter.Int64Counter("gen_ai.client.token.usage.input")
	meterInstruments.outputTokens, _ = meter.Int64Counter("gen_ai.client.token.usage.output")
	meterInstruments.totalTokens, _ = meter.Int64Counter("gen_ai.client.token.usage.total")
}

func RecordRequestMetrics(
	ctx context.Context,
	status string,
	latencyMs int64,
	providerType string,
	endpointName string,
	model string,
	mode constants.TranslatorMode,
	isStreaming bool,
	pm *domain.ProviderMetrics,
) {
	attrs := []attribute.KeyValue{
		attribute.String("status", status),
		attribute.String("backend.type", providerType),
		attribute.String("endpoint.name", endpointName),
		attribute.String("gen_ai.request.model", model),
		attribute.String("translator.mode", string(mode)),
		attribute.Bool("streaming", isStreaming),
	}

	if meterInstruments.requestsTotal != nil {
		meterInstruments.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if meterInstruments.requestLatencyMs != nil && latencyMs >= 0 {
		meterInstruments.requestLatencyMs.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
	}
	if pm == nil {
		return
	}
	if meterInstruments.inputTokens != nil && pm.InputTokens > 0 {
		meterInstruments.inputTokens.Add(ctx, int64(pm.InputTokens), metric.WithAttributes(attrs...))
	}
	if meterInstruments.outputTokens != nil && pm.OutputTokens > 0 {
		meterInstruments.outputTokens.Add(ctx, int64(pm.OutputTokens), metric.WithAttributes(attrs...))
	}
	if meterInstruments.totalTokens != nil && pm.TotalTokens > 0 {
		meterInstruments.totalTokens.Add(ctx, int64(pm.TotalTokens), metric.WithAttributes(attrs...))
	}
}
