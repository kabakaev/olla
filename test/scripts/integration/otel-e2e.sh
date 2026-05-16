#!/bin/bash

# Olla OTLP End-to-End Test Script
# This script automates the entire verification loop:
# 1. Starts a mock LLM backend
# 2. Restarts the OTLP collector with authentication
# 3. Starts Olla with OTel configuration
# 4. Performs an inference request
# 5. Verifies that the OTLP collector received the telemetry data

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GREY='\033[0;90m'
RESET='\033[0m'

# Configuration
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MOCK_BACKEND="$SCRIPT_DIR/mock_backend.py"
OLLA_CONFIG="$SCRIPT_DIR/olla_otel_config.yaml"
OLLA_PORT=40124
BACKEND_PORT=11434

echo -e "${CYAN}======================================================${RESET}"
echo -e "${CYAN}         Olla OTLP End-to-End Verification            ${RESET}"
echo -e "${CYAN}======================================================${RESET}"

# Cleanup function to ensure no processes are left running
cleanup() {
    echo -e "\n${YELLOW}Cleaning up processes...${RESET}"
    pkill -f "mock_backend.py" || true
    pkill -f "go run . -c $OLLA_CONFIG" || true
    # Force kill if needed
    lsof -ti:$OLLA_PORT | xargs kill -9 2>/dev/null || true
    lsof -ti:$BACKEND_PORT | xargs kill -9 2>/dev/null || true
}
trap cleanup EXIT

# Initial cleanup
cleanup

# Helper to wait for a port to become active
wait_for_port() {
    local port=$1
    local name=$2
    local timeout=60
    echo -n "Waiting for $name on port $port..."
    for i in $(seq 1 $timeout); do
        if (echo > /dev/tcp/127.0.0.1/$port) >/dev/null 2>&1; then
            echo -e " ${GREEN}UP${RESET}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    echo -e " ${RED}FAILED (timeout)${RESET}"
    return 1
}

# 1. Start Mock Backend
echo -e "\n${YELLOW}[1/5] Starting Mock LLM Backend...${RESET}"
python3 "$MOCK_BACKEND" &
wait_for_port $BACKEND_PORT "Mock Backend"

# 2. Restart OTLP Collector
echo -e "\n${YELLOW}[2/5] Restarting OTLP Collector (with Auth enforced)...${RESET}"
"$SCRIPT_DIR/restart_otelcol.sh" > /dev/null
echo -e "${GREEN}✓ Collector restarted${RESET}"

# 3. Start Olla
echo -e "\n${YELLOW}[3/5] Starting Olla with OTel configuration...${RESET}"
# We use a unique request ID to track this specific test run
OTEL_METRIC_EXPORT_INTERVAL=1000 go run . -c "$OLLA_CONFIG" &
wait_for_port $OLLA_PORT "Olla"

# 4. Make Inference Call
echo -e "\n${YELLOW}[4/5] Sending test inference request to Olla...${RESET}"
TEST_ID="e2e-test-$(date +%s)"
RESPONSE=$(curl -sS http://127.0.0.1:$OLLA_PORT/olla/openai-compatible/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d "{
    \"model\":\"gemma-4-e4b-it-iq4_nl.gguf\",
    \"messages\":[{\"role\":\"user\",\"content\":\"Verify OTLP export with ID $TEST_ID\"}],
    \"stream\":false
  }")

if [[ "$RESPONSE" == *"telemetry ok"* ]]; then
    echo -e "${GREEN}✓ Inference call successful${RESET}"
    echo -e "${GREY}Response: $RESPONSE${RESET}"
else
    echo -e "${RED}✗ Inference call failed or returned unexpected response${RESET}"
    echo "Response: $RESPONSE"
    exit 1
fi

# 5. Wait and Check Collector Logs
echo -e "\n${YELLOW}[5/5] Verifying OTLP telemetry delivery...${RESET}"
echo "Waiting 10s for OTel exporters to flush..."
sleep 10

echo "Checking OTLP collector logs for required fields..."
REQUIRED_PATTERNS=(
    "gen_ai.request.model: Str(gemma-4-e4b-it-iq4_nl.gguf)"
    "gen_ai.prompt: Str(Verify OTLP export with ID $TEST_ID"
    "gen_ai.completion: Str(telemetry ok)"
    "routing.endpoints: Str(local-openai-compatible)"
    "http.response.status_code: Int(200)"
    "gen_ai.client.token.usage"
)

MISSING_FIELDS=0
LOGS=""

# Metrics can take longer to export than traces (often a 60s default interval, 
# though we try to flush). We'll retry a few times.
MAX_RETRIES=3
for i in $(seq 1 $MAX_RETRIES); do
    MISSING_FIELDS=0
    LOGS=$(podman logs otelcol 2>&1)
    
    echo "Checking OTLP collector logs for required fields (Attempt $i/$MAX_RETRIES)..."
    for pattern in "${REQUIRED_PATTERNS[@]}"; do
        if echo "$LOGS" | grep -qF "$pattern"; then
            echo -e "  ${GREEN}✓${RESET} Found: $pattern"
        else
            echo -e "  ${RED}✗${RESET} Missing: $pattern"
            MISSING_FIELDS=$((MISSING_FIELDS + 1))
        fi
    done

    if [ $MISSING_FIELDS -eq 0 ]; then
        break
    fi

    if [ $i -lt $MAX_RETRIES ]; then
        echo -e "${YELLOW}Some fields missing. Waiting 15s for OTel flush...${RESET}"
        sleep 15
    fi
done

if [ $MISSING_FIELDS -eq 0 ]; then
    echo -e "\n${GREEN}✓ SUCCESS: All required OTLP fields found in collector logs!${RESET}"
    echo -e "${GREEN}✓ Authentication verified: Collector accepted the authenticated request.${RESET}"
else
    echo -e "\n${RED}✗ FAILURE: $MISSING_FIELDS required fields were still missing after $MAX_RETRIES attempts${RESET}"
    echo -e "${YELLOW}Last 50 lines of collector logs:${RESET}"
    echo "$LOGS" | tail -n 50
    exit 1
fi

echo -e "\n${GREEN}======================================================${RESET}"
echo -e "${GREEN}         OTLP End-to-End Test: PASSED                 ${RESET}"
echo -e "${GREEN}======================================================${RESET}"
