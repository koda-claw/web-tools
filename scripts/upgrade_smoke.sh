#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

tmp_dir="$(mktemp -d)"
cleanup() {
  for pid in ${server_pids:-}; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

case "$(uname -s)" in
  Darwin) goos="darwin" ;;
  Linux) goos="linux" ;;
  FreeBSD) goos="freebsd" ;;
  MINGW*|MSYS*|CYGWIN*) goos="windows" ;;
  *) echo "unsupported OS for upgrade smoke: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) goarch="arm64" ;;
  x86_64|amd64) goarch="amd64" ;;
  armv7l|armv6l) goarch="arm" ;;
  *) echo "unsupported arch for upgrade smoke: $(uname -m)" >&2; exit 1 ;;
esac

asset="web-tools-$goos-$goarch"
bin_name="web-tools"
if [ "$goos" = "windows" ]; then
  asset="$asset.exe"
  bin_name="web-tools.exe"
fi

old_bin="$tmp_dir/old-$bin_name"
target_bin="$tmp_dir/$bin_name"
new_bin="$tmp_dir/release/v9.9.9/$asset"
skill_source="$tmp_dir/SKILL.md"
skill_dir="$tmp_dir/skills"

mkdir -p "$tmp_dir/release/v9.9.9"

echo "[upgrade-smoke] build old and new binaries"
go build -ldflags "-X main.version=v0.0.1" -o "$old_bin" .
go build -ldflags "-X main.version=v9.9.9" -o "$new_bin" .
cp "$old_bin" "$target_bin"
chmod +x "$target_bin" "$new_bin"

cat > "$skill_source" <<'SKILL'
---
name: web-tools
description: upgrade smoke
---
SKILL

checksum_file="$tmp_dir/release/v9.9.9/checksums.txt"
(
  cd "$tmp_dir/release/v9.9.9"
  rm -f checksums.txt
  for f in web-tools-*; do
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$f"
    else
      shasum -a 256 "$f"
    fi
  done > checksums.txt
)

port_file="$tmp_dir/port"
python3 - "$tmp_dir/release" "$port_file" <<'PY' &
import functools
import http.server
import pathlib
import socketserver
import sys

root = pathlib.Path(sys.argv[1])
port_file = pathlib.Path(sys.argv[2])
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(root))
with socketserver.TCPServer(("127.0.0.1", 0), handler) as httpd:
    port_file.write_text(str(httpd.server_address[1]))
    httpd.serve_forever()
PY
server_pid="$!"
server_pids="$server_pid"

i=0
while [ ! -s "$port_file" ]; do
  i=$((i + 1))
  if [ "$i" -gt 50 ]; then
    echo "server did not start" >&2
    exit 1
  fi
  sleep 0.1
done
base_url="http://127.0.0.1:$(cat "$port_file")"

before_hash="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$target_bin" | awk '{print $1}'; else shasum -a 256 "$target_bin" | awk '{print $1}'; fi)"

echo "[upgrade-smoke] check mode does not modify files"
"$old_bin" upgrade --check --json --version v9.9.9 --base-url "$base_url" --bin "$target_bin" --skill-dir "$skill_dir" --skill-source "$skill_source" >/tmp/web-tools-upgrade-check.json
after_check_hash="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$target_bin" | awk '{print $1}'; else shasum -a 256 "$target_bin" | awk '{print $1}'; fi)"
test "$before_hash" = "$after_check_hash"
test ! -f "$skill_dir/web-tools/SKILL.md"

echo "[upgrade-smoke] upgrade binary and skill"
"$old_bin" upgrade --json --version v9.9.9 --base-url "$base_url" --bin "$target_bin" --skill-dir "$skill_dir" --skill-source "$skill_source" >/tmp/web-tools-upgrade.json
"$target_bin" --version | grep "v9.9.9" >/dev/null
test -f "$skill_dir/web-tools/SKILL.md"

echo "[upgrade-smoke] only-skill does not replace binary"
only_bin="$tmp_dir/only-$bin_name"
cp "$old_bin" "$only_bin"
only_before="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$only_bin" | awk '{print $1}'; else shasum -a 256 "$only_bin" | awk '{print $1}'; fi)"
"$old_bin" upgrade --only-skill --json --version v9.9.9 --bin "$only_bin" --skill-dir "$tmp_dir/only-skills" --skill-source "$skill_source" >/tmp/web-tools-upgrade-only-skill.json
only_after="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$only_bin" | awk '{print $1}'; else shasum -a 256 "$only_bin" | awk '{print $1}'; fi)"
test "$only_before" = "$only_after"

echo "[upgrade-smoke] checksum mismatch preserves old binary"
bad_dir="$tmp_dir/bad-release/v9.9.9"
mkdir -p "$bad_dir"
cp "$new_bin" "$bad_dir/$asset"
printf '%064d  %s\n' 0 "$asset" > "$bad_dir/checksums.txt"
bad_port_file="$tmp_dir/bad-port"
python3 - "$tmp_dir/bad-release" "$bad_port_file" <<'PY' &
import functools
import http.server
import pathlib
import socketserver
import sys

root = pathlib.Path(sys.argv[1])
port_file = pathlib.Path(sys.argv[2])
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(root))
with socketserver.TCPServer(("127.0.0.1", 0), handler) as httpd:
    port_file.write_text(str(httpd.server_address[1]))
    httpd.serve_forever()
PY
bad_server_pid="$!"
server_pids="$server_pids $bad_server_pid"
i=0
while [ ! -s "$bad_port_file" ]; do
  i=$((i + 1))
  if [ "$i" -gt 50 ]; then
    echo "bad server did not start" >&2
    exit 1
  fi
  sleep 0.1
done
bad_url="http://127.0.0.1:$(cat "$bad_port_file")"
bad_target="$tmp_dir/bad-$bin_name"
cp "$old_bin" "$bad_target"
bad_before="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$bad_target" | awk '{print $1}'; else shasum -a 256 "$bad_target" | awk '{print $1}'; fi)"
if "$old_bin" upgrade --json --version v9.9.9 --base-url "$bad_url" --bin "$bad_target" --skip-skill >/tmp/web-tools-upgrade-bad.json 2>/tmp/web-tools-upgrade-bad.err; then
  echo "expected checksum mismatch to fail" >&2
  exit 1
fi
bad_after="$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "$bad_target" | awk '{print $1}'; else shasum -a 256 "$bad_target" | awk '{print $1}'; fi)"
test "$bad_before" = "$bad_after"

echo "[upgrade-smoke] ok"
