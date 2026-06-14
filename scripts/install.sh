#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"
VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)}"

mkdir -p "$BIN_DIR"

cd "$ROOT_DIR"
go build -ldflags "-X main.version=$VERSION" -o web-tools .
mv web-tools "$BIN_DIR/web-tools"

if [ -n "$SKILL_DIR" ]; then
  mkdir -p "$SKILL_DIR"
  rm -rf "$SKILL_DIR/web-tools"
  cp -R "$ROOT_DIR/skills/web-tools" "$SKILL_DIR/web-tools"
fi

echo "Installed web-tools $VERSION to $BIN_DIR/web-tools"
if [ -n "$SKILL_DIR" ]; then
  echo "Installed skill to $SKILL_DIR/web-tools"
else
  echo "Set SKILL_DIR to install the agent skill, for example:"
  echo "  SKILL_DIR=\"\$HOME/.codex/skills\" sh scripts/install.sh"
fi
