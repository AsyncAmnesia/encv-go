import http.server
import socketserver

INDEX_HTML = """<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>ENCV Preview Index</title></head>
<body style="font-family:sans-serif;max-width:720px;margin:40px auto;padding:20px;line-height:1.6;">
<h1>ENCV Mobile - Preview Index</h1>
<p>All services are managed by pm2 (ecosystem.config.cjs). Run: <code>pm2 list</code></p>
<ul>
  <li><a href="http://localhost:16666/" target="_blank">http://localhost:16666/</a> - main app (encv-mobile)</li>
  <li><a href="http://localhost:16666/openlist-ui/" target="_blank">http://localhost:16666/openlist-ui/</a> - plugin-openlist web</li>
  <li><a href="http://localhost:16666/api/service-guard" target="_blank">http://localhost:16666/api/service-guard</a> - backend health</li>
  <li><a href="http://localhost:16666/__gateway/health" target="_blank">http://localhost:16666/__gateway/health</a> - gateway health</li>
</ul>
<h2>Recent changes (D15)</h2>
<ul>
  <li>vite.config.ts: removed broken <code>__PURE__</code> placeholder replace that produced illegal nested comments breaking JS parsing</li>
  <li>vite.config.ts: removed <code>isLocalHost</code> filter in detectedHost middleware - localhost is the most common sandbox dev access mode</li>
  <li>ecosystem.config.cjs: split <code>encv-mobile-vite</code> (:8100) out of start-preview.sh to prevent restart-loop</li>
</ul>
</body></html>
"""

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.end_headers()
        self.wfile.write(INDEX_HTML.encode('utf-8'))
    def log_message(self, *args):
        pass

with socketserver.TCPServer(('0.0.0.0', 15000), H) as s:
    s.serve_forever()
