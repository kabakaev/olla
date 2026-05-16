#!/bin/bash

# This script starts a mock OpenAI-compatible backend for OTel dev loop.
# It's a lightweight replacement for a real llama.cpp instance.

set -euo pipefail

SCRIPT_DIR="$(realpath "$(dirname "$0")")"
PYTHON_SCRIPT="$SCRIPT_DIR/mock_backend.py"

echo "Starting mock backend..."
# Ensure port is free
lsof -ti:11434 | xargs kill -9 2>/dev/null || true
python3 "$PYTHON_SCRIPT"
