package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/core/domain"
	"github.com/thushan/olla/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (a *Application) telemetryConfig() config.TelemetryConfig {
	if a != nil && a.Config != nil {
		return a.Config.Telemetry
	}
	return config.TelemetryConfig{}
}

func annotateRequestSpan(ctx context.Context, telemetryCfg config.TelemetryConfig, pr *proxyRequest) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}

	attrs := []attribute.KeyValue{}
	if pr.model != "" {
		attrs = append(attrs, attribute.String("gen_ai.request.model", pr.model))
	}
	if pr.translatorMode != "" {
		attrs = append(attrs, attribute.String("translator.mode", string(pr.translatorMode)))
	}
	if pr.clientIP != "" {
		attrs = append(attrs, attribute.String("client.address", pr.clientIP))
	}
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}

	if telemetryCfg.PayloadCapture.Enabled && pr.profile != nil && pr.profile.Prompt != "" {
		span.SetAttributes(attribute.String("gen_ai.prompt", truncateForTelemetry(pr.profile.Prompt, telemetryCfg.PayloadCapture.MaxPromptBytes)))
	}
}

func annotateRoutingSpan(ctx context.Context, pr *proxyRequest, endpoints []*domain.Endpoint) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.Int("routing.endpoint_count", len(endpoints)),
	}
	if pr.profile != nil && pr.profile.RoutingDecision != nil {
		attrs = append(attrs,
			attribute.String("routing.strategy", pr.profile.RoutingDecision.Strategy),
			attribute.String("routing.reason", pr.profile.RoutingDecision.Reason),
		)
	}
	if len(endpoints) > 0 {
		names := make([]string, 0, len(endpoints))
		for _, endpoint := range endpoints {
			names = append(names, endpoint.Name)
		}
		attrs = append(attrs, attribute.String("routing.endpoints", strings.Join(names, ",")))
	}
	span.SetAttributes(attrs...)
}

func annotateResponseSpan(ctx context.Context, telemetryCfg config.TelemetryConfig, pr *proxyRequest, responseText string) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	if pr.stats.ProviderMetrics != nil {
		pm := pr.stats.ProviderMetrics
		attrs := []attribute.KeyValue{}
		if pm.Model != "" {
			attrs = append(attrs, attribute.String("gen_ai.response.model", pm.Model))
		}
		if pm.InputTokens > 0 {
			attrs = append(attrs, attribute.Int("gen_ai.usage.input_tokens", int(pm.InputTokens)))
		}
		if pm.OutputTokens > 0 {
			attrs = append(attrs, attribute.Int("gen_ai.usage.output_tokens", int(pm.OutputTokens)))
		}
		if pm.TotalTokens > 0 {
			attrs = append(attrs, attribute.Int("gen_ai.usage.total_tokens", int(pm.TotalTokens)))
		}
		if len(attrs) > 0 {
			span.SetAttributes(attrs...)
		}
	}
	if telemetryCfg.PayloadCapture.Enabled && responseText != "" {
		span.SetAttributes(attribute.String("gen_ai.completion", truncateForTelemetry(responseText, telemetryCfg.PayloadCapture.MaxResponseBytes)))
	}
}

func recordOTelMetrics(ctx context.Context, pr *proxyRequest, status string) {
	telemetry.RecordRequestMetrics(
		ctx,
		status,
		pr.stats.Latency,
		pr.stats.EndpointType,
		pr.stats.EndpointName,
		pr.model,
		pr.translatorMode,
		pr.isStreaming,
		pr.stats.ProviderMetrics,
	)
}

func truncateForTelemetry(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

type telemetryCaptureWriter struct {
	http.ResponseWriter
	buf   bytes.Buffer
	limit int
}

func newTelemetryCaptureWriter(w http.ResponseWriter, limit int) *telemetryCaptureWriter {
	return &telemetryCaptureWriter{
		ResponseWriter: w,
		limit:          limit,
	}
}

func (w *telemetryCaptureWriter) Write(data []byte) (int, error) {
	if w.limit > 0 && w.buf.Len() < w.limit {
		remaining := w.limit - w.buf.Len()
		if remaining > len(data) {
			remaining = len(data)
		}
		w.buf.Write(data[:remaining])
	}
	return w.ResponseWriter.Write(data)
}

func (w *telemetryCaptureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *telemetryCaptureWriter) CapturedString() string {
	return w.buf.String()
}

func extractHumanReadableCompletion(raw string, isStreaming bool) string {
	if raw == "" {
		return ""
	}
	if isStreaming {
		return extractStreamingCompletion(raw)
	}
	return extractJSONCompletion(raw)
}

func extractJSONCompletion(raw string) string {
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return raw
	}

	choices, ok := body["choices"].([]any)
	if !ok || len(choices) == 0 {
		return raw
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return raw
	}
	if message, ok := choice["message"].(map[string]any); ok {
		if content, ok := message["content"].(string); ok {
			return content
		}
	}
	if text, ok := choice["text"].(string); ok {
		return text
	}
	return raw
}

func extractStreamingCompletion(raw string) string {
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			continue
		}
		choices, ok := body["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}
		if delta, ok := choice["delta"].(map[string]any); ok {
			if content, ok := delta["content"].(string); ok {
				out.WriteString(content)
			}
			continue
		}
		if text, ok := choice["text"].(string); ok {
			out.WriteString(text)
		}
	}
	return out.String()
}
