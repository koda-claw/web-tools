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

echo "[smoke] setup help"
go run . setup --help >/dev/null

echo "[smoke] gui help"
go run . gui --help >/dev/null

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

echo "[smoke] setup command"
go run . setup \
  --provider bigmodel \
  --auth-env ZHIPU_APIKEY \
  --set-env ZHIPU_APIKEY=fake-smoke-token \
  --env-file "$tmp_dir/setup.env" \
  --enable-search-auto \
  --config "$tmp_dir/setup-config.json" \
  --skill-dir "$tmp_dir/setup-skills" \
  --skill-source ./skills/web-tools/SKILL.md \
  --skip-doctor >"$tmp_dir/setup.out"
grep -q 'Configured provider "bigmodel"' "$tmp_dir/setup.out"
grep -q 'Stored ZHIPU_APIKEY' "$tmp_dir/setup.out"
if grep -q 'fake-smoke-token' "$tmp_dir/setup.out"; then
  echo "setup output leaked env value" >&2
  exit 1
fi
test -f "$tmp_dir/setup-skills/web-tools/SKILL.md"
grep -q '"bigmodel"' "$tmp_dir/setup-config.json"
grep -q 'ZHIPU_APIKEY=fake-smoke-token' "$tmp_dir/setup.env"

echo "[smoke] setup check json"
go build -o "$tmp_dir/web-tools" .
setup_check_home="$tmp_dir/setup-check-home"
mkdir -p "$setup_check_home"
HOME="$setup_check_home" "$tmp_dir/web-tools" setup \
  --check \
  --json \
  --skill-dir "$tmp_dir/setup-check-skills" >"$tmp_dir/setup-check.json"
grep -q '"ok": true' "$tmp_dir/setup-check.json"
grep -q '"skill"' "$tmp_dir/setup-check.json"
grep -q '"install_skill"' "$tmp_dir/setup-check.json"
grep -q '"configure_provider"' "$tmp_dir/setup-check.json"

echo "[smoke] gui server"
gui_home="$tmp_dir/gui-home"
mkdir -p "$gui_home"
HOME="$gui_home" "$tmp_dir/web-tools" gui --no-open --port 0 >"$tmp_dir/gui.out" 2>"$tmp_dir/gui.err" &
gui_pid=$!
for _ in {1..50}; do
  if grep -q 'web-tools GUI:' "$tmp_dir/gui.out"; then
    break
  fi
  sleep 0.1
done
gui_url="$(awk '/web-tools GUI:/ {print $3}' "$tmp_dir/gui.out" | tail -n1)"
if [[ -z "$gui_url" ]]; then
  echo "GUI did not print URL" >&2
  cat "$tmp_dir/gui.err" >&2 || true
  kill "$gui_pid" 2>/dev/null || true
  exit 1
fi
curl -fsS "$gui_url/healthz" >"$tmp_dir/gui-health.json"
grep -q '"ok":true' "$tmp_dir/gui-health.json"
curl -fsS "$gui_url/api/status" >"$tmp_dir/gui-status.json"
grep -q '"setup"' "$tmp_dir/gui-status.json"
curl -fsS "$gui_url/api/diagnostics" >"$tmp_dir/gui-diagnostics.json"
grep -q '"agent_guide"' "$tmp_dir/gui-diagnostics.json"
kill "$gui_pid" 2>/dev/null || true
wait "$gui_pid" 2>/dev/null || true

echo "[smoke] ok"
