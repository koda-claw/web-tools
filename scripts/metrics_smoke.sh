#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

tmp_dir="$(mktemp -d)"
cleanup() {
  if [[ -n "${server_pid:-}" ]]; then
    kill "$server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

bin="$tmp_dir/web-tools"
metrics_file="$tmp_dir/metrics.json"
config_file="$tmp_dir/config.json"

echo "[metrics-smoke] build"
go build -o "$bin" .

cat > "$config_file" <<JSON
{
  "reader": {
    "cache_dir": "$tmp_dir/cache",
    "browser_fallback": false,
    "min_content_length": 20
  },
  "search": {
    "searxng_url": "http://127.0.0.1:__PORT__",
    "default_engine": "searxng",
    "default_limit": 3
  }
}
JSON

port_file="$tmp_dir/port"
python3 - "$port_file" <<'PY' &
import json
import pathlib
import socketserver
import sys
from http.server import BaseHTTPRequestHandler

port_file = pathlib.Path(sys.argv[1])

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_HEAD(self):
        self.send_response(200)
        self.end_headers()

    def do_GET(self):
        if self.path.startswith("/search"):
            body = json.dumps({
                "number_of_results": 1,
                "results": [{
                    "title": "Metrics Smoke",
                    "url": f"http://127.0.0.1:{self.server.server_address[1]}/article",
                    "content": "safe snippet",
                    "parsed_url": ["http", f"127.0.0.1:{self.server.server_address[1]}", "/article"]
                }]
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path == "/article":
            body = ("<html><head><title>Metrics Smoke</title></head><body><article><p>"
                    + ("safe reader content " * 80)
                    + "</p></article></body></html>").encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_response(404)
        self.end_headers()

with socketserver.TCPServer(("127.0.0.1", 0), Handler) as httpd:
    port_file.write_text(str(httpd.server_address[1]))
    httpd.serve_forever()
PY
server_pid="$!"

for _ in {1..50}; do
  [[ -s "$port_file" ]] && break
  sleep 0.1
done
port="$(cat "$port_file")"
sed -i.bak "s/__PORT__/$port/g" "$config_file"

echo "[metrics-smoke] empty metrics"
WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" metrics --json >"$tmp_dir/empty.json"
grep -q '"schema_version": 1' "$tmp_dir/empty.json"

echo "[metrics-smoke] collect search and reader"
WEB_TOOLS_CONFIG="$config_file" WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" web-search "private search query marker" --json >"$tmp_dir/search.json"
WEB_TOOLS_CONFIG="$config_file" WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" web-reader "http://127.0.0.1:$port/article" --json --no-cache >"$tmp_dir/reader.json"
WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" metrics --range 24h --json >"$tmp_dir/metrics-24h.json"
grep -q '"web-search"' "$tmp_dir/metrics-24h.json"
grep -q '"web-reader"' "$tmp_dir/metrics-24h.json"

echo "[metrics-smoke] privacy"
if grep -E 'private search query marker|127\.0\.0\.1:' "$metrics_file" >/dev/null; then
  echo "metrics file leaked query or URL" >&2
  exit 1
fi
if grep -E 'safe reader content' "$metrics_file" >/dev/null; then
  echo "metrics file leaked content" >&2
  exit 1
fi

echo "[metrics-smoke] disable"
disabled_file="$tmp_dir/disabled.json"
printf 'disabled metrics note\n' > "$tmp_dir/note.txt"
WEB_TOOLS_NO_METRICS=1 WEB_TOOLS_METRICS_FILE="$disabled_file" "$bin" web-reader "$tmp_dir/note.txt" --format text >/dev/null
test ! -f "$disabled_file"

echo "[metrics-smoke] reset"
WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" metrics reset --json >"$tmp_dir/reset.json"
grep -q '"ok":true' "$tmp_dir/reset.json"
WEB_TOOLS_METRICS_FILE="$metrics_file" "$bin" metrics --range all --json >"$tmp_dir/metrics-all.json"
if grep -q '"web-search"' "$tmp_dir/metrics-all.json"; then
  echo "metrics reset did not clear commands" >&2
  exit 1
fi

echo "[metrics-smoke] ok"
