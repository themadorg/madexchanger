import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
name, host, logf = sys.argv[1], sys.argv[2], sys.argv[3]
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        b = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        with open(logf, 'a') as f:
            f.write(f"{name} got PATH={self.path} FROM={self.headers.get('X-Mail-From')} TO={self.headers.get_all('X-Mail-To')} BYTES={len(b)}\n")
        self.send_response(200); self.end_headers()
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(f"{name} relay\n".encode())
    def log_message(self, *a): pass
HTTPServer((host, 80), H).serve_forever()
