#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

echo "[smoke] go test ./..."
go test ./...

echo "[smoke] root help"
go run . --help >/dev/null

echo "[smoke] web-search help"
go run . web-search --help >/dev/null

echo "[smoke] web-reader help"
go run . web-reader --help >/dev/null

echo "[smoke] version"
go run . --version >/dev/null

echo "[smoke] web-search argument validation"
if go run . web-search >"$tmp_dir/web-search-empty.out" 2>"$tmp_dir/web-search-empty.err"; then
  echo "expected web-search without query to fail" >&2
  exit 1
fi
grep -q "accepts 1 arg" "$tmp_dir/web-search-empty.err"

echo "[smoke] web-reader local text"
printf 'first line\nsecond line\n' >"$tmp_dir/note.txt"
go run . web-reader "$tmp_dir/note.txt" --format text >"$tmp_dir/note.out"
grep -q "first line" "$tmp_dir/note.out"
grep -q "second line" "$tmp_dir/note.out"
if grep -q "<!-- source:" "$tmp_dir/note.out"; then
  echo "expected text format to omit metadata comments" >&2
  exit 1
fi

echo "[smoke] ok"
