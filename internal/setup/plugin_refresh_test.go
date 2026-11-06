// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"strings"
	"testing"
)

const enabledPluginSettings = `{"enabledPlugins": {"downshift@downshift": true}}`

func hasCall(calls []string, want string) bool {
	for _, c := range calls {
		if strings.HasSuffix(c, want) {
			return true
		}
	}
	return false
}

// An installed plugin stays on the commit Claude Code fetched; setup updates
// it so a new release's agents and skills reach the session.
func TestSetupUpdatesAnAlreadyEnabledPlugin(t *testing.T) {
	_, f := newClaudeEnv(t, enabledPluginSettings)
	code, out, errOut := f.run(nil)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, want := range []string{"plugin marketplace update downshift", "plugin update downshift@downshift"} {
		if !hasCall(f.calls, want) {
			t.Errorf("setup did not run %q: %v", want, f.calls)
		}
	}
	if !strings.Contains(out, "updated the downshift@downshift plugin") {
		t.Errorf("setup must say it updated the plugin:\n%s", out)
	}
}

func TestSetupDryRunDoesNotUpdateThePlugin(t *testing.T) {
	_, f := newClaudeEnv(t, enabledPluginSettings)
	code, out, errOut := f.run(func(o *Options) { o.DryRun = true })
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if hasCall(f.calls, "plugin update downshift@downshift") || hasCall(f.calls, "plugin marketplace update downshift") {
		t.Fatalf("a dry run ran an update: %v", f.calls)
	}
	if !strings.Contains(out, "would run `claude plugin marketplace update downshift`") {
		t.Errorf("dry run must name the update:\n%s", out)
	}
}

func TestSetupPluginUpdateFailureIsANote(t *testing.T) {
	_, f := newClaudeEnv(t, enabledPluginSettings)
	f.failUpdate = true
	code, out, errOut := f.run(nil)
	if code != 0 {
		t.Fatalf("a failed update must not fail setup: exit %d\n%s\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "network unreachable") || !strings.Contains(out, "claude plugin update downshift@downshift") {
		t.Errorf("setup must name the failure and the manual fix:\n%s", out)
	}
}

func TestPluginMarketplaceName(t *testing.T) {
	for in, want := range map[string]string{"downshift@downshift": "downshift", "a@b": "b", "bare": "bare"} {
		if got := pluginMarketplaceName(in); got != want {
			t.Errorf("pluginMarketplaceName(%q) = %q, want %q", in, got, want)
		}
	}
}
