---
name: otel-dev-loop
description: >
  Reusable local OpenTelemetry development loop for Olla. Starts a local OTLP
  collector in Podman, runs Olla with an example config against a local
  OpenAI-compatible llama.cpp backend on localhost:11434, and inspects traces,
  metrics, and OTLP logs from collector stdout.
---

# OTel Dev Loop

Use this skill when working on Olla telemetry, traces, metrics, OTLP log export,
or payload-capture behavior.

## Goal

Run a tight agentic loop:

1. Edit code.
2. Run focused tests, then `go test ./...` when changes stabilize.
3. Restart local OTLP collector.
4. Start local llama.cpp backend.
5. Start Olla from repo with example config.
6. Send one non-streaming and one streaming request.
7. Inspect collector output and Olla logs.
8. Fix regressions and repeat.

## Files

- `restart_otelcol.sh`: restarts local Podman collector container.
- `otelcol_config.yaml`: collector config exporting traces, metrics, and logs to stdout.
- `olla_otel_config.yaml`: example Olla config targeting local llama.cpp at `localhost:11434`.
- `start_gemma4_4b_4G.sh`: helper to start local llama.cpp backend.

## Commands

### 1. Start llama.cpp backend

```bash
./.agents/skills/otel-dev-loop/start_gemma4_4b_4G.sh
```

Verify:

```bash
curl localhost:11434/v1/models
```

### 2. Restart OTLP collector

```bash
./.agents/skills/otel-dev-loop/restart_otelcol.sh
```

See collector output:

```bash
podman logs -f otelcol
```

### 3. Start Olla from repo

```bash
go run . -c ./.agents/skills/otel-dev-loop/olla_otel_config.yaml
```

### 4. Send test requests

Non-streaming:

```bash
curl -sS http://127.0.0.1:40124/olla/openai-compatible/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"gemma-4-E4B-it-IQ4_NL.gguf",
    "messages":[{"role":"user","content":"Reply with exactly telemetry ok"}],
    "max_tokens":4,
    "temperature":0,
    "stream":false
  }'
```

Streaming:

```bash
curl -sN http://127.0.0.1:40124/olla/openai-compatible/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"gemma-4-E4B-it-IQ4_NL.gguf",
    "messages":[{"role":"user","content":"Reply with exactly telemetry ok"}],
    "max_tokens":4,
    "temperature":0,
    "stream":true
  }'
```

### 5. What to verify

- Traces:
  - `request.id`
  - `gen_ai.request.model`
  - `gen_ai.prompt`
  - `gen_ai.completion`
  - `gen_ai.usage.*`
  - `routing.*`
- Metrics:
  - `olla.requests.total`
  - `olla.requests.latency_ms`
  - `gen_ai.client.token.usage.*`
  - separate `streaming=true/false` series when expected
- OTLP logs:
  - present only when enabled in `telemetry.logs`
  - respect separate OTLP log level
  - include `trace_id`, `span_id`, `request_id`
- Local logs:
  - still human-readable
  - still include trace/span correlation

## Notes

- Collector uses host networking for easy local OTLP on `localhost:4317`.
- Olla example config enables traces, metrics, OTLP logs, and payload capture for test purposes.
- If testing default behavior, disable `telemetry.logs.enabled` and/or `telemetry.payload_capture.enabled`.
