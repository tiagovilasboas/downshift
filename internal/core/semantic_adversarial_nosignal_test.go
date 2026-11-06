// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

func semAdvNoSignalCases() []semAdvCase {
	filler := strings.TrimSpace(strings.Repeat("the quick brown fox jumps over the lazy dog ", 556)) // ~5k words
	return []semAdvCase{
		// Single stray words.
		{"word hook", "hook", true},
		{"word payload", "payload", true},
		{"word reply", "reply", true},
		{"word summary", "summary", true},
		{"word ok", "ok", true},
		{"word yes", "yes", true},
		{"word thanks", "thanks", true},
		{"word continue", "continue", true},
		{"word done", "done", true},
		{"word lgtm", "lgtm", true},
		{"word wip", "wip", true},
		{"word Hook capitalised", "Hook", true},
		{"word HOOK upper", "HOOK", true},
		{"word hook trailing punct", "hook?", true},
		{"word subagent", "subagent", true},
		{"word pipeline", "pipeline", true},
		{"word system", "system", true},
		{"word complex", "complex", true},
		// Degenerate input.
		{"empty", "", true},
		{"spaces", "     ", true},
		{"tabs and newlines", " \t\n\r\n\t ", true},
		{"emoji", "🚀🔥👍", true},
		{"emoji spaced", "🚀 🔥 👍 🎉 🤖", true},
		{"punctuation", "?!...;;--", true},
		{"punctuation brackets", "{}[]()<>//\\\\", true},
		{"digits", "42 1337 3.14159", true},
		{"zero width and nbsp", "\u200b \u200b", true},
		// Volume and repetition.
		{"filler 5k words", filler, true},
		{"repeated hook x1000", strings.TrimSpace(strings.Repeat("hook ", 1000)), true},
		{"repeated payload x200", strings.TrimSpace(strings.Repeat("payload reply ", 200)), true},
		{"lorem ipsum", semAdvLorem, true},
		{"lorem ipsum x50", strings.TrimSpace(strings.Repeat(semAdvLorem+" ", 50)), true},
		// Mixed languages.
		{"pt/en stray words", "oi tudo bem hook payload valeu obrigado", true},
		{"pt sentence", "bom dia, segue o resumo da conversa de ontem", true},
		{"pt/en greeting", "hello pessoal, só passando pra dizer valeu", true},
		// Pasted artifacts with no task.
		{"json blob", `{"id": 42, "name": "widget", "tags": ["a", "b"], "nested": {"ok": true, "count": 3}, "items": [{"k": "v"}, {"k": "w"}]}`, true},
		{"python traceback", "Traceback (most recent call last):\n  File \"app.py\", line 12, in main\n    run()\n  File \"app.py\", line 8, in run\n    int(x)\nValueError: invalid literal for int() with base 10: 'abc'", true},
		{"java stack trace", "Exception in thread \"main\" java.lang.NullPointerException\n\tat com.example.App.run(App.java:42)\n\tat com.example.App.main(App.java:10)", true},
		{"access log", "2026-10-11T12:00:00Z INFO request served status=200 latency=12ms path=/api/v1/items\n2026-10-11T12:00:01Z WARN slow request status=200 latency=950ms path=/api/v1/items\n2026-10-11T12:00:02Z ERROR connection refused upstream=10.0.0.4:5432", true},
		{"csv rows", "id,name,qty\n1,apple,3\n2,pear,7\n3,plum,1", true},
		{"yaml snippet", "server:\n  port: 8080\n  host: 0.0.0.0\nlogging:\n  level: info", true},
		{"hex dump", "00000000  7f 45 4c 46 02 01 01 00  00 00 00 00 00 00 00 00  |.ELF............|", true},
		{"url only", "https://example.com/some/path?q=1&r=2", true},
		{"base64 blob", "SGVsbG8sIHdvcmxkISBUaGlzIGlzIGEgYmFzZTY0IGJsb2IgdGhhdCBtZWFucyBub3RoaW5n", true},
	}
}

// TestSemAdvNoSignalNeverLifted is the core regression: a no-signal prompt
// stays at the Medium default after fusion, identical to Classify.
func TestSemAdvNoSignalNeverLifted(t *testing.T) {
	semAdvEnv(t)
	for _, tc := range semAdvNoSignalCases() {
		t.Run(tc.name, func(t *testing.T) {
			base := core.Classify(tc.prompt)
			if base.NoSignal != tc.noSignal {
				t.Fatalf("precondition: Classify(%q).NoSignal=%v want %v (scores %v)", semAdvShort(tc.prompt), base.NoSignal, tc.noSignal, base.Scores)
			}
			sem := core.ClassifyWithSemantic(tc.prompt)
			if !base.NoSignal {
				return
			}
			if sem.Complexity > core.Medium {
				t.Fatalf("no-signal prompt lifted to %s by semantic fusion", sem.Complexity)
			}
			if sem.Complexity != base.Complexity {
				t.Fatalf("semantic changed no-signal complexity: base=%s sem=%s", base.Complexity, sem.Complexity)
			}
			if sem.Confident != base.Confident || sem.NoSignal != base.NoSignal || sem.RiskFloor != base.RiskFloor {
				t.Fatalf("semantic changed no-signal flags: base=%+v sem=%+v", base, sem)
			}
		})
	}
}

// TestSemAdvNoSignalGuardIsLoadBearing proves the corpus actually exercises
// the guard: the raw semantic layer, asked directly, would lift at least one
// no-signal prompt. If it lifts none, the no-signal test above is vacuous.
func TestSemAdvNoSignalGuardIsLoadBearing(t *testing.T) {
	semAdvEnv(t)
	lifted := 0
	for _, tc := range semAdvNoSignalCases() {
		if !core.Classify(tc.prompt).NoSignal {
			continue
		}
		if label, ok := semantic.MaybeAugment(tc.prompt, core.Medium.String(), false); ok {
			lifted++
			t.Logf("raw semantic would lift %q to %s; fusion holds it at MEDIUM", semAdvShort(tc.prompt), label)
		}
	}
	if lifted == 0 {
		t.Fatalf("raw semantic layer lifts no no-signal prompt in the corpus; the guard is not exercised")
	}
}

// TestSemAdvRouteNoSignalNeverFrontier: end to end through Route with the
// Claude Code catalog, no no-signal input reaches the frontier tier.
func TestSemAdvRouteNoSignalNeverFrontier(t *testing.T) {
	semAdvEnv(t)
	for _, tc := range semAdvNoSignalCases() {
		if !core.Classify(tc.prompt).NoSignal {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			for _, cur := range []string{"", "claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"} {
				d := classifiedRoute(tc.prompt, "claude-code", cur, cat)
				if d.Tier == core.TierFrontier {
					t.Fatalf("current=%q: no-signal prompt routed to frontier (complexity %s, model %s)", cur, d.Complexity, d.Model.ID)
				}
				if d.Complexity > core.Medium {
					t.Fatalf("current=%q: no-signal prompt has complexity %s", cur, d.Complexity)
				}
			}
		})
	}
}
