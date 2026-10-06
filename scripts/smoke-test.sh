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
# Hermetic state dir: never touch the runner's real ~/.downshift / legacy dir.
SMOKE_HOME="$(mktemp -d)"
SMOKE_STATE="$SMOKE_HOME/downshift-state"
trap 'rm -rf "$SMOKE_HOME"' EXIT
mkdir -p "$SMOKE_STATE"
cat > "$SMOKE_STATE/session-models.json" <<'JSON'
{
  "claude-code": ["claude-haiku-4-5", "claude-sonnet-4-6", "claude-opus-4-8"],
  "codex": ["gpt-5.6-luna", "gpt-5.6-terra", "gpt-5.6-sol"],
  "cursor": ["composer-2.5", "claude-4.5-sonnet-thinking", "claude-4.5-opus-high-thinking"],
  "antigravity": ["flash_lite", "flash", "pro"]
}
JSON
hook() { HOME="$SMOKE_HOME" DOWNSHIFT_STATE_DIR="$SMOKE_STATE" "$BIN" "$@" 2>/dev/null; }
expect() { # expect <label> <output> <fixed string>
  if ! printf '%s' "$2" | grep -qF -- "$3"; then
    echo "FAIL: $1: expected [$3] in: $2" >&2
    exit 1
  fi
}

# Claude Code: a trivial Task on opus must be rewritten to the small model.
cc_res=$(echo '{"tool_name":"Task","tool_input":{"prompt":"rename the userId variable to userIdentifier","model":"claude-opus-4-8"}}' | hook claude-code)
expect "claude-code rewrite" "$cc_res" '"updatedInput":{'
# Claude Code accepts the catalog native_name (haiku|sonnet|opus|fable), not the full id.
expect "claude-code rewrite" "$cc_res" '"model":"haiku"'
expect "claude-code rewrite" "$cc_res" 'claude-haiku-4-5'
echo "OK: claude-code rewrites updatedInput.model"

# Claude Code without a session allowlist: allow, no rewrite.
cc_none=$(echo '{"tool_name":"Task","tool_input":{"prompt":"rename the userId variable to userIdentifier","model":"claude-opus-4-8"}}' | HOME="$(mktemp -d)" "$BIN" claude-code 2>/dev/null)
expect "claude-code no session" "$cc_none" '"permissionDecision":"allow"'
if printf '%s' "$cc_none" | grep -qF updatedInput; then
  echo "FAIL: claude-code rewrote without a session allowlist: $cc_none" >&2
  exit 1
fi
echo "OK: claude-code without session does not rewrite"

# Antigravity: trivial subagent on flash must move to flash_lite.
ag_res=$(echo '{"toolCall":{"name":"invoke_subagent","args":{"Subagents":[{"Model":"flash","Prompt":"rename the userId variable to userIdentifier"}]}}}' | hook antigravity)
expect "antigravity rewrite" "$ag_res" '"overwrite":{'
expect "antigravity rewrite" "$ag_res" '"Model":"flash_lite"'
echo "OK: antigravity overwrites Subagents[0].Model"

# Cursor: trivial task on the frontier model must be rewritten.
cur_res=$(echo '{"tool_name":"Task","tool_input":{"task":"rename the userId variable to userIdentifier","model":"claude-4.5-opus-high-thinking"}}' | hook cursor)
expect "cursor rewrite" "$cur_res" '"updated_input":{'
expect "cursor rewrite" "$cur_res" '"permission":"allow"'
echo "OK: cursor rewrites updated_input.model"

# Codex: trivial spawn on sol must move to luna with low effort.
codex_res=$(echo '{"tool_name":"spawn_agent","tool_input":{"message":"rename the userId variable to userIdentifier","model":"gpt-5.6-sol"}}' | hook codex)
expect "codex rewrite" "$codex_res" '"updatedInput":{'
expect "codex rewrite" "$codex_res" '"model":"gpt-5.6-luna"'
echo "OK: codex rewrites updatedInput.model"

echo "=== 4. Benchmark CLI and report gate smoke test ==="
"$BIN" benchmark "$REPO_ROOT/benchmark/tasks.json" --report >/dev/null
"$BIN" benchmark "$REPO_ROOT/benchmark/tasks.json" --min-tier-accuracy=0.60 >/dev/null
echo "OK: benchmark report and gate"

echo "All smoke tests passed successfully!"
