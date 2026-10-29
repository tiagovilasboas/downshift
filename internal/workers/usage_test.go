// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// claudeResult is the shape `claude -p --output-format json` writes: one result
// object with a usage block (field names checked in the installed CLI 2.1.310).
const claudeResult = `{"type":"result","subtype":"success","is_error":false,"result":"done","num_turns":3,` +
	`"usage":{"input_tokens":120,"cache_creation_input_tokens":2000,"cache_read_input_tokens":15000,"output_tokens":340}}`

// codexEvents is the shape `codex exec --json` writes: JSON lines, with usage on
// each turn.completed event (event and field names checked in codex-cli 0.163.0).
const codexEvents = `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"i0","type":"agent_message","text":"hi"}}
{"type":"turn.completed","usage":{"input_tokens":9000,"cached_input_tokens":6000,"output_tokens":400,"reasoning_output_tokens":100}}
not json at all
{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":50}}
`

func TestTokenParsers(t *testing.T) {
	cases := []struct {
		name  string
		parse func([]byte) *Tokens
		in    string
		want  *Tokens
	}{
		{"claude result", claudeTokens, claudeResult, &Tokens{Input: 120, Output: 340, CacheRead: 15000, CacheWrite: 2000, Total: 17460}},
		{"claude without usage", claudeTokens, `{"type":"result","subtype":"success","result":"x"}`, nil},
		{"claude not json", claudeTokens, "boom", nil},
		{"codex turns summed", codexTokens, codexEvents, &Tokens{Input: 4000, Output: 450, CacheRead: 6000, Total: 10450}},
		{"codex without a turn", codexTokens, `{"type":"thread.started"}` + "\n", nil},
		{"codex empty", codexTokens, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.parse([]byte(c.in))
			if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestClaudeWorkerReportsItsTokens(t *testing.T) {
	shim(t, "claude", `echo '`+claudeResult+`'`)
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if tk := proc.Tokens(); tk == nil || tk.Total != 17460 {
		t.Fatalf("tokens = %+v", tk)
	}
}

func TestCodexWorkerAsksForJSONEventsAndReportsItsTokens(t *testing.T) {
	body := `out=""; while [ $# -gt 0 ]; do if [ "$1" = "-o" ]; then out="$2"; fi; shift; done; printf 'final' > "$out"; cat <<'EOF'
` + codexEvents + `EOF`
	dir := shim(t, "codex", body)
	proc, err := ReadOnlyLaunchers()["codex"].Launch(Spec{Agent: "codex", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if args := readFile(t, filepath.Join(dir, "codex.args")); !strings.Contains(args, "--json\n") {
		t.Errorf("codex was not asked for JSON events: %s", args)
	}
	if proc.Report() != "final" {
		t.Errorf("report = %q, want the -o message, not the event stream", proc.Report())
	}
	if tk := proc.Tokens(); tk == nil || tk.Total != 10450 {
		t.Fatalf("tokens = %+v", tk)
	}
}
