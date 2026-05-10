#!/bin/bash

# This script starts a local llama.cpp in a rootless podman container on a 4GB VRAM GPU.
# Install podman and run this script as a non-root user.

set -xeuo pipefail

# 64k context window.
CONTEXT_WINDOW="${1:-65534}" # 65534, 131072

MODEL_ID="unsloth/gemma-4-E4B-it-GGUF"
# MODEL_URL="https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q3_K_M.gguf"
MODEL_URL="https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-IQ4_NL.gguf"
MMPROJ_URL="https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/mmproj-F16.gguf"
MODEL_FILE="models/${MODEL_ID}/$(basename $MODEL_URL)"

SCRIPT_DIR="$(realpath "$(dirname "$0")")"
MODEL_PATH="$SCRIPT_DIR/$MODEL_FILE"

if [ ! -f "$MODEL_PATH" ]; then
    # Create the output directory if it doesn't exist
    mkdir -p "$(dirname "$MODEL_PATH")"
    # Download the file
    echo "Downloading $MODEL_URL to $MODEL_PATH..."
    wget -c "$MODEL_URL" -O "$MODEL_PATH"
fi

if [ -n "${MMPROJ_URL:-}" ]; then
    MMPROJ_FILE="models/${MODEL_ID}/$(basename $MMPROJ_URL)"
    MMPROJ_PATH="$SCRIPT_DIR/$MMPROJ_FILE"
    # Create the output directory if it doesn't exist
    mkdir -p "$(dirname "$MMPROJ_PATH")"
    # Download the file
    echo "Downloading $MMPROJ_URL to $MMPROJ_PATH..."
    wget -c "$MMPROJ_URL" -O "$MMPROJ_PATH"
fi

echo "Starting llama.cpp container..."

CONTAINER=ghcr.io/ggml-org/llama.cpp:server-cuda
podman pull "$CONTAINER"
PODMAN="podman run -p 8012:8012 -p 8080:8080 -p 11434:11434 --name ai --init -d --device nvidia.com/gpu=all -v $SCRIPT_DIR/models:/models -v $SCRIPT_DIR/models:/root/.cache/llama.cpp $CONTAINER"

podman rm -f ai || true

$PODMAN --host 0.0.0.0 --port 11434 \
    -b 64 \
    -ub 64 \
    --fit-target 64 \
    --model "/$MODEL_FILE" \
    --mmproj "/$MMPROJ_FILE" \
    --no-mmproj-offload \
    --cache-type-k q4_0 \
    --cache-type-v q4_0 \
    --jinja \
    -c "$CONTEXT_WINDOW" \
    --parallel 1 \
    --seed 3407 \
    -ngl 99 \
    --temp 1.0 \
    --top-p 0.95 \
    --top-k 64 \
    --repeat-penalty 1.1
