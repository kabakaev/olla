#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="docker.io/otel/opentelemetry-collector-contrib:latest"

podman rm -f otelcol >/dev/null 2>&1 || true
podman run -d \
  --name otelcol \
  --network host \
  -v "$SCRIPT_DIR/otelcol_config.yaml:/etc/otelcol-contrib/config.yaml:ro" \
  "$IMAGE"

echo "otelcol restarted"
echo "inspect with: podman logs -f otelcol"
