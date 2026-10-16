package claudecode_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// AUDIT: same as the cursor case, on Claude Code with a hook session list
// that is not in least-to-most-capable order.
func TestUpshiftTier_ClaudeCodeUpshiftWritesSmallerModel(t *testing.T) {
	session := []string{"claude-sonnet-5-5", "claude-opus-5-5", "claude-haiku-5-5"}
	ev := claudecode.Event{
		SessionModels: &session,
		ToolInput: json.RawMessage(`{
			"prompt": "rearchitect the payment flow across services and migrate the auth tokens",
			"model": "sonnet"
		}`),
	}
	out, note, d := claudecode.Handle(ev, cat)
	t.Logf("cx=%s tier=%s verdict=%s safe=%s risk=%v note=%q", d.Complexity, d.Tier, d.Verdict, d.SafeVerdict, d.Risk, note)
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.UpdatedInput == nil {
		return
	}
	var m map[string]any
	_ = json.Unmarshal(out.HookSpecificOutput.UpdatedInput, &m)
	written, _ := m["model"].(string)
	wm, _ := cat.LookupByID("claude-code", written)
	if wm.Tier < core.TierMid {
		t.Fatalf("wrote %q (catalog tier %s) over sonnet (mid) for a %s task with risk %v", written, wm.Tier, d.Complexity, d.Risk)
	}
}
