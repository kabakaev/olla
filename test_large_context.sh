#!/bin/bash

# Create a large text file (approx 35000 bytes)
# using tr to generate random alphanumeric chars
tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 35000 > large_prompt.txt

# Create JSON payload with the large prompt
echo '{"model": "Ministral-3-3B-Instruct-2512-UD-Q5_K_XL.gguf", "messages": [{"role": "user", "content": "'$(cat large_prompt.txt)'"}]}' > large_payload.json

# Send request
curl -v -X POST http://localhost:40114/olla/llamacpp/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d @large_payload.json
