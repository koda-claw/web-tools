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

echo "[smoke] config help"
go run . config --help >/dev/null

echo "[smoke] skill help"
go run . skill --help >/dev/null

echo "[smoke] version"
go run . --version >/dev/null

echo "[smoke] doctor json"
go run . doctor --json >"$tmp_dir/doctor.json"
grep -q '"checks"' "$tmp_dir/doctor.json"
grep -q '"config"' "$tmp_dir/doctor.json"

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

echo "[smoke] config provider add"
go run . config provider add bigmodel \
  --preset bigmodel \
  --auth-env ZHIPU_APIKEY \
  --enable-search-auto \
  --config "$tmp_dir/config.json" \
  --json >"$tmp_dir/config-provider-add.json"
grep -q '"provider": "bigmodel"' "$tmp_dir/config-provider-add.json"
grep -q '"bigmodel"' "$tmp_dir/config.json"
grep -q '"auth_env": "ZHIPU_APIKEY"' "$tmp_dir/config.json"

echo "[smoke] skill install from source"
go run . skill install \
  --dir "$tmp_dir/skills" \
  --source ./skills/web-tools/SKILL.md \
  --json >"$tmp_dir/skill-install.json"
grep -q '"ok": true' "$tmp_dir/skill-install.json"
test -f "$tmp_dir/skills/web-tools/SKILL.md"

echo "[smoke] ok"
