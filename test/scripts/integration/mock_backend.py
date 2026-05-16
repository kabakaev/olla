import http.server
import json
import sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/v1/models':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            response = {
                "object": "list",
                "data": [
                    {
                        "id": "gemma-4-e4b-it-iq4_nl.gguf",
                        "object": "model",
                        "created": 1677652288,
                        "owned_by": "ollama"
                    }
                ]
            }
            self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"OK")

    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        post_data = self.rfile.read(content_length)
        
        try:
            req = json.loads(post_data)
        except:
            req = {}

        is_stream = req.get('stream', False)
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json' if not is_stream else 'text/event-stream')
        self.end_headers()

        if is_stream:
            # Send a few chunks
            chunks = ["telemetry ", "ok", ""]
            for i, chunk in enumerate(chunks):
                if chunk == "":
                    self.wfile.write(b"data: [DONE]\n\n")
                else:
                    data = {
                        "id": "chatcmpl-123",
                        "object": "chat.completion.chunk",
                        "created": 1677652288,
                        "model": req.get("model", "mock-model"),
                        "choices": [{"index": 0, "delta": {"content": chunk}, "finish_reason": None if i < len(chunks)-2 else "stop"}]
                    }
                    self.wfile.write(f"data: {json.dumps(data)}\n\n".encode())
                self.wfile.flush()
        else:
            response = {
                "id": "chatcmpl-123",
                "object": "chat.completion",
                "created": 1677652288,
                "model": req.get("model", "mock-model"),
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": "telemetry ok"},
                        "finish_reason": "stop"
                    }
                ],
                "usage": {
                    "prompt_tokens": 10,
                    "completion_tokens": 2,
                    "total_tokens": 12
                }
            }
            self.wfile.write(json.dumps(response).encode())

def run(port=11434):
    server_address = ('', port)
    httpd = http.server.HTTPServer(server_address, Handler)
    print(f"Mock backend listening on port {port}...")
    httpd.serve_forever()

if __name__ == '__main__':
    port = 11434
    if len(sys.argv) > 1:
        port = int(sys.argv[1])
    run(port)
