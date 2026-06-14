#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"

mkdir -p "$BIN_DIR"

cd "$ROOT_DIR"
go build -o web-tools .
mv web-tools "$BIN_DIR/web-tools"

if [ -n "$SKILL_DIR" ]; then
  mkdir -p "$SKILL_DIR"
  rm -rf "$SKILL_DIR/web-tools"
  cp -R "$ROOT_DIR/skills/web-tools" "$SKILL_DIR/web-tools"
fi

echo "Installed web-tools to $BIN_DIR/web-tools"
if [ -n "$SKILL_DIR" ]; then
  echo "Installed skill to $SKILL_DIR/web-tools"
else
  echo "Set SKILL_DIR to install the agent skill, for example:"
  echo "  SKILL_DIR=\"\$HOME/.codex/skills\" sh scripts/install.sh"
fi
