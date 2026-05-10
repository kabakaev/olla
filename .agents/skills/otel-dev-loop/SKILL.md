---
name: otel-dev-loop
description: >
  Reusable local OpenTelemetry development loop for Olla. Uses a mock LLM backend
  and a secure OTLP collector to verify traces, metrics, and logs over both gRPC
  and HTTP OTLP transports.
---

# OTel Dev Loop

Use this skill when working on Olla telemetry, traces, metrics, OTLP log export,
or payload-capture behavior.

## Goal

Run a tight agentic loop:

1. Edit code.
2. Run focused tests, then `go test ./...` when changes stabilize.
3. Run the automated E2E test script for both OTLP transports: `./test/scripts/integration/otel-e2e.sh`.
4. Inspect collector output and Olla logs for regressions.
5. Fix and repeat.

## Automated E2E Test

Run the entire chain (mock backend, collector, Olla, and log verification) with one command.
The script must verify both OTLP transports in sequence:

- `grpc` via collector `127.0.0.1:4317`
- `http` via collector `127.0.0.1:4318`

```bash
./test/scripts/integration/otel-e2e.sh
```

## Manual Dev Loop

For detailed debugging or iterative development:

### 1. Start mock backend
A lightweight Python server that mimics an OpenAI-compatible API.
```bash
./test/scripts/integration/start_mock_backend.sh
```

### 2. Restart OTLP collector
Restarts a Podman-based `otelcol-contrib` container with `bearertokenauth` enforced
for both OTLP gRPC and OTLP HTTP receivers.
```bash
./test/scripts/integration/restart_otelcol.sh
```

### 3. Start Olla from repo
Use environment overrides to select the transport under test while reusing the same
authenticated base config.

For gRPC:
```bash
OLLA_TELEMETRY_OTLP_PROTOCOL=grpc \
OLLA_TELEMETRY_OTLP_ENDPOINT=127.0.0.1:4317 \
go run . -c ./test/scripts/integration/olla_otel_config.yaml
```

For HTTP:
```bash
OLLA_TELEMETRY_OTLP_PROTOCOL=http \
OLLA_TELEMETRY_OTLP_ENDPOINT=127.0.0.1:4318 \
OLLA_TELEMETRY_OTLP_PATH=/api/default/otel \
go run . -c ./test/scripts/integration/olla_otel_config.yaml
```

### 4. Send test requests
Non-streaming:
```bash
curl -sS http://127.0.0.1:40124/olla/openai-compatible/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"gemma-4-e4b-it-iq4_nl.gguf",
    "messages":[{"role":"user","content":"test"}],
    "stream":false
  }'
```

Streaming:
```bash
curl -sN http://127.0.0.1:40124/olla/openai-compatible/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"gemma-4-e4b-it-iq4_nl.gguf",
    "messages":[{"role":"user","content":"test"}],
    "stream":true
  }'
```

## Integration Test Files

Location: `test/scripts/integration/`

- `otel-e2e.sh`: Automated E2E test script for both gRPC and HTTP transports.
- `mock_backend.py`: Python mock server.
- `start_mock_backend.sh`: Wrapper to start the mock server.
- `restart_otelcol.sh`: Restarts the Podman collector.
- `otelcol_config.yaml`: Collector config with authenticated OTLP gRPC `:4317` and HTTP `:4318` receivers.
- `olla_otel_config.yaml`: Olla base config with matching OTLP headers; transport, endpoint, and OTLP HTTP path prefix can be overridden by env vars.

## Verification

Run the automated E2E test or perform manual requests and inspect the collector logs (`podman logs otelcol`).
Verification should pass for both `grpc` and `http`.

### What to verify

#### 1. Traces
Check for `Resource Spans` with the following attributes:
- `gen_ai.request.model`: matches the requested model.
- `gen_ai.prompt`: contains the input prompt (if payload capture is enabled).
- `gen_ai.completion`: contains the model response.
- `routing.endpoints`: shows the target endpoint (e.g., `local-openai-compatible`).
- `routing.strategy`: shows the load balancing strategy used.
- `http.response.status_code`: should be `200`.

#### 2. Metrics
Check for `Resource Metrics`:
- `gen_ai.client.token.usage`: histogram of prompt/completion tokens.
- `gen_ai.client.operation.duration`: histogram of request latency.
- `gen_ai.client.active_requests`: gauge of concurrent requests.

#### 3. Logs
Check for `Resource Logs`:
- Verify that Olla's internal logs are exported via OTLP if configured.
- Ensure `service.name` is set to `olla-local-test` (from `olla_otel_config.yaml`).

#### 4. Authentication
- If the collector rejects the data with `401` or `403`, check the `Authorization` header in `olla_otel_config.yaml` and the `bearertokenauth` token in `otelcol_config.yaml`.
- For HTTP transport, also confirm Olla points at `127.0.0.1:4318` and `telemetry.otlp.protocol=http`.
- If using a prefixed HTTP OTLP route, also confirm `telemetry.otlp.path` or `OLLA_TELEMETRY_OTLP_PATH` matches the upstream prefix, for example `/api/default/otel`.
- Successful delivery for both transports confirms that authentication is working correctly.
