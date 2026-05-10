---
title: "OpenTelemetry - Traces, Metrics, and Log Correlation"
description: "Configure OpenTelemetry in Olla for OTLP traces, token metrics, and trace-aware structured logs."
keywords: ["olla opentelemetry", "olla otlp", "olla openobserve", "gen_ai telemetry", "trace correlation"]
---

# OpenTelemetry

Olla supports OpenTelemetry export over OTLP for:

- Distributed traces for HTTP and proxy requests
- Token usage and request metrics
- Log correlation via `trace_id` and `span_id` fields in structured logs

This is designed for backends such as OpenObserve and any OTLP-compatible collector.

## What Phase 1 Includes

- Root HTTP tracing for all requests
- Security and rate-limit rejections included in traces
- GenAI-aligned request and usage attributes
- OTLP metrics for request counts, latency, and token usage
- Optional prompt/response payload capture with truncation limits

## Minimal Configuration

```yaml
telemetry:
  enabled: true
  service_name: "olla"
  service_version: "0.0.0"
  otlp:
    endpoint: "localhost:4317"
    protocol: "grpc"
    insecure: true
    skip_health_traces: true
  traces:
    enabled: true
  metrics:
    enabled: true
  payload_capture:
    enabled: false
    max_prompt_bytes: 32768
    max_response_bytes: 65536
    redact_headers:
      - "authorization"
      - "cookie"
```

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `telemetry.enabled` | bool | `false` | Master switch for OTel export |
| `telemetry.service_name` | string | `"olla"` | Service name sent in OTel resource attributes |
| `telemetry.service_version` | string | `""` | Optional service version override |
| `telemetry.otlp.endpoint` | string | `"localhost:4317"` | OTLP collector endpoint. For HTTP OTLP, a full URL such as `http://host:5080/api/default/otel` is also supported |
| `telemetry.otlp.path` | string | `""` | Optional OTLP HTTP path prefix added before `/v1/traces`, `/v1/metrics`, and `/v1/logs` |
| `telemetry.otlp.protocol` | string | `"grpc"` | OTLP transport protocol: `grpc` or `http` |
| `telemetry.otlp.headers` | map[string]string | `{}` | Custom headers (e.g., `Authorization: "Basic ..."` for OpenObserve) |
| `telemetry.otlp.insecure` | bool | `true` | Use insecure OTLP transport |
| `telemetry.otlp.skip_health_traces` | bool | `false` | Skip HTTP server spans for `/internal/health` |
| `telemetry.traces.enabled` | bool | `true` | Enable trace export |
| `telemetry.metrics.enabled` | bool | `true` | Enable metric export |
| `telemetry.payload_capture.enabled` | bool | `false` | Capture prompt/response text on spans |
| `telemetry.payload_capture.max_prompt_bytes` | int | `32768` | Truncation limit for prompt capture |
| `telemetry.payload_capture.max_response_bytes` | int | `65536` | Truncation limit for response capture |
| `telemetry.payload_capture.redact_headers` | []string | `["authorization","cookie"]` | Reserved for sensitive header redaction policy |

## Environment Variables

All telemetry settings can be overridden with environment variables:

```bash
export OLLA_TELEMETRY_ENABLED=true
export OLLA_TELEMETRY_SERVICE_NAME=olla
export OLLA_TELEMETRY_SERVICE_VERSION=dev
export OLLA_TELEMETRY_OTLP_ENDPOINT=otel-collector:4317
export OLLA_TELEMETRY_OTLP_PATH=/api/default/otel
export OLLA_TELEMETRY_OTLP_PROTOCOL=grpc
export OLLA_TELEMETRY_OTLP_INSECURE=true
export OLLA_TELEMETRY_OTLP_SKIP_HEALTH_TRACES=true
export OLLA_TELEMETRY_TRACES_ENABLED=true
export OLLA_TELEMETRY_METRICS_ENABLED=true
export OLLA_TELEMETRY_PAYLOAD_CAPTURE_ENABLED=false
export OLLA_TELEMETRY_PAYLOAD_CAPTURE_MAX_PROMPT_BYTES=32768
export OLLA_TELEMETRY_PAYLOAD_CAPTURE_MAX_RESPONSE_BYTES=65536
```

## Exported Data

### Traces

Olla creates a root HTTP server span per request. Important request metadata is attached as span attributes, including routing and token usage when available.

If `telemetry.otlp.skip_health_traces` is enabled, requests to `/internal/health` are still served normally but no root HTTP trace span is created for them.

Examples:

- `gen_ai.request.model`
- `gen_ai.response.model`
- `gen_ai.usage.input_tokens`
- `gen_ai.usage.output_tokens`
- `gen_ai.usage.total_tokens`
- `client.address`
- `user_agent.original`
- `request.id`
- `routing.strategy`
- `routing.reason`
- `routing.endpoints`
- `routing.endpoint_count`

When `telemetry.payload_capture.enabled` is `true`, Olla also attaches captured text to that same request span as string attributes:

- `gen_ai.prompt`
- `gen_ai.completion`

This means the prompt and completion text is exported through the OTLP traces pipeline to your collector and trace backend. It is not exported as OTLP metrics, and Olla does not emit it as a separate log record just because payload capture is enabled.

### Metrics

Phase 1 exports OTLP metrics for:

- Total requests
- Request latency
- Input tokens
- Output tokens
- Total tokens

These metrics are labeled with bounded operational dimensions such as backend type, endpoint name, requested model, translator mode, and streaming mode.

### Logs

Olla continues to use `slog`, but request-scoped logs now include:

```json
{
  "trace_id": "c0ffee...",
  "span_id": "deadbeef...",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

This lets you jump between logs and traces in systems such as OpenObserve.

## Payload Capture

Prompt and response capture is **disabled by default**.

Enable it only for debugging:

```yaml
telemetry:
  payload_capture:
    enabled: true
    max_prompt_bytes: 8192
    max_response_bytes: 16384
```

Recommendations:

- Do not enable payload capture in multi-tenant production environments unless you have reviewed privacy requirements.
- Keep byte limits small.
- Prefer temporary enablement during incident debugging.

How capture works:

- Request text is taken from the inspected prompt text, then attached to the active request trace span as `gen_ai.prompt`.
- Response text is captured from the HTTP response body as Olla streams it to the client, then attached to the same span as `gen_ai.completion`.
- `max_prompt_bytes` and `max_response_bytes` are hard byte caps on what Olla stores in those span attributes.
- For non-streaming responses, Olla tries to extract the human-readable completion from the backend JSON payload.
- For streaming responses, Olla reconstructs best-effort readable text from streamed chunks and exports that reconstructed text, not the raw SSE chunk stream.

Important limitation:

- Olla does not upload the full original request JSON or full raw response body unless the prompt/completion extraction itself effectively matches that content. In practice, payload capture is aimed at prompt/completion text, not arbitrary request fields.

## OpenObserve Example

Example collector target:

```yaml
telemetry:
  enabled: true
  service_name: "olla"
  otlp:
    endpoint: "openobserve-collector:4318"
    protocol: "http"
    headers:
      Authorization: "Bearer o2oi_..."
    insecure: true
```

In OpenObserve, you will typically want to:

- Search traces by `service.name=olla`
- Filter spans by `gen_ai.request.model`
- Build dashboards from token counters and request latency
- Pivot from logs using `trace_id`

## Operational Guidance

- Keep `payload_capture.enabled=false` by default
- Enable both traces and metrics for production observability
- Use JSON logs when shipping logs to a central store
- Do not put high-cardinality values such as request IDs into metric labels

## Related Pages

- [Monitoring](monitoring.md)
- [Configuration Reference](../reference.md)
- [Provider Metrics](../../concepts/provider-metrics.md)
