#!/usr/bin/env bash
# run_scenario.sh <label> <order> <tool-json>
# Runs a real `claude -p` with a scrubbed environment (env -i) against the local mock API
# on 127.0.0.1: no credentials, no proxy variables, nothing leaves the machine.
# Env: FX_ROOT (work dir), DOWNSHIFT_BIN (a built downshift), CLAUDE_BIN (default: claude).
set -u
FX_DIR=$(cd "$(dirname "$0")" && pwd); export FX_DIR
: "${FX_ROOT:?set FX_ROOT}" "${DOWNSHIFT_BIN:?set DOWNSHIFT_BIN}"; export FX_ROOT DOWNSHIFT_BIN
CLAUDE_BIN=$(command -v "${CLAUDE_BIN:-claude}"); NODE_DIR=$(dirname "$(command -v node)")
label=$1; order=$2; tool=$3
R=$FX_ROOT/runs/$label
[ -e "$R" ] && { echo "refusing to reuse $R" >&2; exit 2; }
mkdir -p "$R/home/.claude" "$R/work"
python3 -I "$FX_DIR/mksettings.py" "$R/home/.claude/settings.json" "$order"
PORT=$(python3 -I -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])')
: > "$R/mock.jsonl"; : > "$R/fixture.log"; : > "$R/cheap.jsonl"
MOCK_TOOL="$tool" MOCK_LOG="$R/mock.jsonl" python3 -I "$FX_DIR/mockapi.py" "$PORT" & MPID=$!
sleep 0.7
cd "$R/work"
env -i PATH="$FX_DIR/bin:$(dirname "$DOWNSHIFT_BIN"):$NODE_DIR:/usr/local/bin:/usr/bin:/bin" HOME="$R/home" \
  FIXTURE_LOG="$R/fixture.log" CHEAP_SUBAGENTS_LOG="$R/cheap.jsonl" \
  ANTHROPIC_BASE_URL="http://127.0.0.1:$PORT" ANTHROPIC_API_KEY=sk-ant-mock-0000 \
  DISABLE_TELEMETRY=1 DISABLE_AUTOUPDATER=1 DISABLE_ERROR_REPORTING=1 CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 \
  timeout 90 "$CLAUDE_BIN" -p "RUN-SCENARIO $label" --model claude-sonnet-5-5 \
  --output-format json --debug-file "$R/debug.log" > "$R/claude.out" 2> "$R/claude.err"
echo "exit=$?" > "$R/claude.exit"
kill $MPID 2>/dev/null; wait $MPID 2>/dev/null
# what doctor says about this exact settings file
HOME="$R/home" "$DOWNSHIFT_BIN" doctor --json --harness claude-code > "$R/doctor.json" 2>/dev/null
