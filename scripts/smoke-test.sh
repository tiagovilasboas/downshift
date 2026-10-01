#!/usr/bin/env bash
# Smoke test script for downshift adapters and CLI commands (Task P1.4)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$REPO_ROOT/downshift"

echo "=== 1. Checking downshift binary ==="
if [ ! -x "$BIN" ]; then
  echo "Building downshift binary..."
  (cd "$REPO_ROOT" && go build -ldflags "-s -w" -o downshift ./cmd/downshift)
fi
"$BIN" --help >/dev/null
echo "OK: downshift binary is executable"

echo "=== 2. CLI Try smoke tests across harnesses ==="
# Test default try
out=$("$BIN" try "rename variable x to y")
echo "$out" | grep -q "TRIVIAL"
echo "$out" | grep -q "small"
echo "OK: downshift try (default)"

# Test Antigravity
out=$("$BIN" try "rename variable x to y" antigravity flash)
echo "$out" | grep -q "flash_lite"
echo "$out" | grep -q "DOWNSHIFT"
echo "OK: downshift try (antigravity)"

# Test Cursor
out=$("$BIN" try "rearchitect auth service across systems" cursor)
echo "$out" | grep -q "COMPLEX"
echo "OK: downshift try (cursor)"

# Test Codex
out=$("$BIN" try "git commit changes" codex)
echo "$out" | grep -q "TRIVIAL"
echo "OK: downshift try (codex)"

echo "=== 3. Hook adapters payload smoke tests ==="

# Claude Code adapter
cc_res=$(echo '{"tool_name":"Task","tool_input":{"prompt":"rename variable x to y"}}' | "$BIN" claude-code)
echo "$cc_res" | grep -q '"permissionDecision":"allow"'
echo "OK: claude-code hook response format"

# Antigravity adapter
ag_res=$(echo '{"toolCall":{"name":"invoke_subagent","args":{"Subagents":[{"Model":"flash","Prompt":"rename variable x to y"}]}}}' | "$BIN" antigravity)
echo "$ag_res" | grep -q '"decision":"allow"'
echo "OK: antigravity hook response format"

# Cursor adapter
cur_res=$(echo '{"tool_name":"task","tool_input":{"prompt":"fix typo"}}' | "$BIN" cursor)
echo "$cur_res" | grep -q '"permission":"allow"'
echo "OK: cursor hook response format"

# Codex adapter
codex_res=$(echo '{"tool_name":"spawn_subagent","tool_input":{"task":"lint code"}}' | "$BIN" codex)
echo "$codex_res" | grep -q '"permissionDecision":"allow"'
echo "OK: codex hook response format"

echo "=== 4. Benchmark CLI and report gate smoke test ==="
"$BIN" benchmark "$REPO_ROOT/benchmark/tasks.json" --report >/dev/null
"$BIN" benchmark "$REPO_ROOT/benchmark/tasks.json" --min-tier-accuracy=0.60 >/dev/null
echo "OK: benchmark report and gate"

echo "All smoke tests passed successfully!"
