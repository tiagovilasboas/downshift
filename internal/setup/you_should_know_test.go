// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// When the plugin already carries the hooks, setup says so before writing.
func TestPlanTarget_NotesEnabledPlugin(t *testing.T) {
	home := t.TempDir()
	var target Target
	for _, st := range Targets {
		if st.Plugin != "" {
			target = st
			break
		}
	}
	if target.Plugin == "" {
		t.Skip("no target declares a plugin")
	}
	path := filepath.Join(home, filepath.FromSlash(target.File))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"enabledPlugins":{"`+target.Plugin+`":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := planTarget(Options{Home: home, Binary: "/usr/local/bin/downshift"}, target)
	if p.Err != nil {
		t.Fatal(p.Err)
	}
	if len(p.Notes) == 0 || !strings.Contains(p.Notes[0], "none were written here") {
		t.Fatalf("notes = %v", p.Notes)
	}
	if strings.Contains(string(p.After), `"hooks"`) || strings.Contains(string(p.After), "downshift claude-code") {
		t.Fatalf("hooks must not be written while the plugin is enabled:\n%s", p.After)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p = planTarget(Options{Home: home, Binary: "/usr/local/bin/downshift"}, target)
	for _, n := range p.Notes {
		if strings.Contains(n, "none were written here") {
			t.Fatal("no plugin, no note")
		}
	}
	if !strings.Contains(string(p.After), `"hooks"`) {
		t.Fatalf("without the plugin the hooks are written:\n%s", p.After)
	}
}

const youShouldKnowKey = "cc-plugin-you-should-know@builtin"

func enabledPlugin(t *testing.T, e *setupEnv) (any, bool) {
	t.Helper()
	ep, _ := parseDoc(t, e.read(".claude/settings.json"))["enabledPlugins"].(map[string]any)
	v, ok := ep[youShouldKnowKey]
	return v, ok
}

// "You should know" is a sub-key of the enabledPlugins map, which the user
// usually already fills: only the sub-key decides, never the map.
func TestSetupYouShouldKnowDefault(t *testing.T) {
	on := []string{"--you-should-know"}
	for _, tc := range []struct {
		name        string
		settings    string // empty: no settings file
		args        []string
		want        any // value of the sub-key afterwards
		present     bool
		wantNote    bool // "is not true in enabledPlugins"
		wantSuggest bool // the opt-in suggestion line
		keepOther   bool // downshift@downshift must survive
	}{
		{"default: not written, suggested", `{"hooks":{}}`, nil, nil, false, false, true, false},
		{"default, no file: not written, suggested", "", nil, nil, false, false, true, false},
		{"default beside other plugins: not written", `{"enabledPlugins":{"downshift@downshift":true}}`, nil, nil, false, false, true, true},
		{"default, already true: untouched, no suggestion", `{"enabledPlugins":{"` + youShouldKnowKey + `":true,"downshift@downshift":true}}`, nil, true, true, false, false, true},
		{"default, false set by you: untouched, no suggestion", `{"enabledPlugins":{"` + youShouldKnowKey + `":false,"downshift@downshift":true}}`, nil, false, true, false, false, true},
		{"flag, map absent: created", `{"hooks":{}}`, on, true, true, false, false, false},
		{"flag, no file: created", "", on, true, true, false, false, false},
		{"flag beside other plugins: created", `{"enabledPlugins":{"downshift@downshift":true}}`, on, true, true, false, false, true},
		{"flag, false set by you: untouched and reported", `{"enabledPlugins":{"` + youShouldKnowKey + `":false,"downshift@downshift":true}}`, on, false, true, true, false, true},
		{"flag with --no-session-policy: skipped", `{"enabledPlugins":{"downshift@downshift":true}}`, []string{"--you-should-know", "--no-session-policy"}, nil, false, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newSetupEnv(t, ".claude")
			if tc.settings != "" {
				e.write(".claude/settings.json", tc.settings)
			}
			if code, out, errOut := e.run(append([]string{"claude-code", "--yes"}, tc.args...)...); code != 0 {
				t.Fatalf("exit %d\n%s%s", code, out, errOut)
			} else {
				if got := strings.Contains(out, "is not true in enabledPlugins"); got != tc.wantNote {
					t.Errorf("note printed = %v, want %v\n%s", got, tc.wantNote, out)
				}
				if got := strings.Contains(out, "downshift setup --you-should-know"); got != tc.wantSuggest {
					t.Errorf("suggestion printed = %v, want %v\n%s", got, tc.wantSuggest, out)
				}
			}
			v, ok := enabledPlugin(t, e)
			if ok != tc.present || (ok && v != tc.want) {
				t.Errorf("enabledPlugins[%s] = %v, present %v; want %v, present %v", youShouldKnowKey, v, ok, tc.want, tc.present)
			}
			if tc.keepOther {
				ep := parseDoc(t, e.read(".claude/settings.json"))["enabledPlugins"].(map[string]any)
				if ep["downshift@downshift"] != true {
					t.Errorf("existing plugin entry lost: %v", ep)
				}
			}
		})
	}
}

// Dry run shows the sub-key in the diff and writes nothing; undo restores the
// backup taken before setup added it.
func TestSetupYouShouldKnowDryRunAndUndo(t *testing.T) {
	original := `{"enabledPlugins":{"downshift@downshift":true},"hooks":{}}`
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", original)

	code, out, _ := e.run("claude-code", "--dry-run", "--you-should-know")
	if code != 0 || !strings.Contains(out, youShouldKnowKey) || !strings.Contains(out, "Dry run") {
		t.Fatalf("dry run exit %d, diff must show the key:\n%s", code, out)
	}
	if e.read(".claude/settings.json") != original {
		t.Fatal("dry run wrote the file")
	}

	if code, out, _ := e.run("claude-code", "--yes", "--you-should-know"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if v, ok := enabledPlugin(t, e); !ok || v != true {
		t.Fatalf("not written: %v %v", v, ok)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") != original {
		t.Errorf("undo did not restore the original:\n%s", e.read(".claude/settings.json"))
	}
}
