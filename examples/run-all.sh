#!/usr/bin/env bash
# Adapter smoke: pipe example PreToolUse payloads through downshift (no live harness).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Keep fixture decisions out of the operator's real stats.
SMOKE_DIR="$(mktemp -d)"
trap 'rm -rf "$SMOKE_DIR"' EXIT
export DOWNSHIFT_EVENT_LOG="${DOWNSHIFT_EVENT_LOG:-$SMOKE_DIR/events.jsonl}"

if ! command -v downshift >/dev/null 2>&1; then
  echo "downshift not in PATH — install first (see docs/install.md)" >&2
  exit 1
fi

echo "== claude-code (task fixture) =="
downshift claude-code < examples/claude-code-pretooluse-task.json >/dev/null

echo "== cursor (task fixture) =="
downshift cursor < examples/cursor-pretooluse-task.json >/dev/null

echo "== claude-code (session_models rewrite) =="
downshift claude-code < examples/claude-code-pretooluse-session.json >/dev/null

echo "OK: all adapter smoke checks passed"
