#!/usr/bin/env bash
# Adapter smoke: pipe example PreToolUse payloads through downshift (no live harness).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

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
