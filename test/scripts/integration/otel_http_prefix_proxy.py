#!/usr/bin/env python3

import http.server
import http.client
from http.server import ThreadingHTTPServer
import sys


PREFIX = "/api/default/otel"
UPSTREAM_HOST = "127.0.0.1"
UPSTREAM_PORT = 4318


class ProxyHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_POST(self):
        if not self.path.startswith(PREFIX + "/"):
            self.send_error(404, "unknown path")
            return

        upstream_path = self.path.removeprefix(PREFIX)
        body_len = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(body_len) if body_len else b""

        headers = {k: v for k, v in self.headers.items() if k.lower() != "host"}
        conn = http.client.HTTPConnection(UPSTREAM_HOST, UPSTREAM_PORT, timeout=30)
        try:
            conn.request("POST", upstream_path, body=body, headers=headers)
            resp = conn.getresponse()
            data = resp.read()

            self.send_response(resp.status, resp.reason)
            for key, value in resp.getheaders():
                if key.lower() in {"connection", "transfer-encoding"}:
                    continue
                self.send_header(key, value)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        finally:
            conn.close()

    def log_message(self, fmt, *args):
        return


class ReusableThreadingHTTPServer(ThreadingHTTPServer):
    allow_reuse_address = True


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 14318
    with ReusableThreadingHTTPServer(("127.0.0.1", port), ProxyHandler) as server:
        server.serve_forever()
