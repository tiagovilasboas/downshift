package cursor_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// AUDIT: with the real catalog, an UPSHIFT of a risky COMPLEX task must not
// write a model whose catalog tier is below the current model's.
func TestUpshiftTier_CursorUpshiftWritesSmallerModel(t *testing.T) {
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", "")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "no-quota.json"))
	cur := "claude-4.5-sonnet-thinking" // catalog tier mid
	session := []string{"composer-2.5-fast", cur, "composer-2.5"}
	ev := cursor.Event{
		ToolName:      "Task",
		SessionModels: &session,
		ToolInput: json.RawMessage(`{
			"task": "rearchitect the payment flow across services and migrate the auth tokens",
			"model": "` + cur + `"
		}`),
	}
	out, note, d := cursor.Handle(ev, cat)
	m := decodeUpdated(t, out)
	t.Logf("cx=%s tier=%s verdict=%s safe=%s risk=%v note=%q", d.Complexity, d.Tier, d.Verdict, d.SafeVerdict, d.Risk, note)
	if m == nil {
		return
	}
	written, _ := m["model"].(string)
	wm, _ := cat.LookupByID("cursor", written)
	if wm.Tier < core.TierMid {
		t.Fatalf("wrote %q (catalog tier %s) over %s (mid) for a %s task with risk %v",
			written, wm.Tier, cur, d.Complexity, d.Risk)
	}
}
