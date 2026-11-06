// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"strings"
	"testing"
)

// The trigger phrase and the rule that makes a model dispatch at once, as the
// always-loaded instructions must carry them.
const (
	triggerPhrase  = "`downshift full power`"
	dispatchNowCue = "Do not explore alone."
)

func TestEveryInstructionsTargetCarriesTheAgentTriggers(t *testing.T) {
	for _, tg := range Targets {
		if tg.Steering == nil {
			continue
		}
		if !strings.Contains(tg.Steering.Content, AgentTriggers) {
			t.Errorf("%s: always-loaded instructions lack the agent triggers", tg.Name)
		}
	}
}

func TestAgentTriggersReachEveryHarnessFile(t *testing.T) {
	for _, h := range steeringHarnesses {
		t.Run(h.name, func(t *testing.T) {
			e := newSetupEnv(t, h.configDir)
			if code, out, _ := e.run("--yes", h.name); code != 0 {
				t.Fatalf("setup exit %d\n%s", code, out)
			}
			body := e.read(h.steeringPath)
			for _, want := range []string{triggerPhrase, dispatchNowCue, "downshift compactor --"} {
				if !strings.Contains(body, want) {
					t.Errorf("%s lacks %q", h.steeringPath, want)
				}
			}
		})
	}
}

func TestClaudeCodeGetsTheAgentTriggersBlockOnly(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/CLAUDE.md", "# My rules\n\nAnswer in Portuguese.\n")
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	body := e.read(".claude/CLAUDE.md")
	if !strings.HasPrefix(body, "# My rules\n\nAnswer in Portuguese.\n\n"+AgentsBlockBegin) {
		t.Errorf("user text not kept before the agents block:\n%s", body)
	}
	if !strings.HasSuffix(body, AgentsBlockEnd+"\n") || !strings.Contains(body, dispatchNowCue) {
		t.Errorf("agents block missing or incomplete:\n%s", body)
	}
	if strings.Contains(body, "downshift compactor --") || strings.Contains(body, SteeringBlockBegin) {
		t.Errorf("Claude Code compacts through its hook; the compact steering does not belong here:\n%s", body)
	}
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("second setup exit %d\n%s", code, out)
	}
	if again := e.read(".claude/CLAUDE.md"); again != body {
		t.Errorf("second run changed CLAUDE.md:\n%s", again)
	}
}

func TestClaudeCodeUndoRestoresCLAUDEmd(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	const original = "# My rules\n"
	e.write(".claude/CLAUDE.md", original)
	if code, out, _ := e.run("--yes", "claude-code"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if got := e.read(".claude/CLAUDE.md"); got != original {
		t.Errorf("undo left %q, want the original file", got)
	}
}
