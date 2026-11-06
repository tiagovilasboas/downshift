// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func workersFile(t *testing.T, name, value string) string {
	t.Helper()
	if value == "" {
		return ""
	}
	p := filepath.Join(t.TempDir(), name)
	body := `{"orchestration":{"workers":` + value + `}}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveWorkersPrecedence(t *testing.T) {
	both := []string{"claude", "codex"}
	cases := []struct {
		name          string
		env, user, pj string
		goos          string
		verified      []string
		mode, source  string
		agents        []string
		noteHas       string
	}{
		{"default read for verified", "", "", "", "linux", []string{"claude"}, "read", "default", []string{"claude"}, ""},
		{"default darwin read", "", "", "", "darwin", []string{"claude"}, "read", "default", []string{"claude"}, ""},
		{"default off with none verified", "", "", "", "linux", nil, "off", "default", nil, ""},
		{"env read", "read", "", "", "linux", nil, "read", "env", both, ""},
		{"env bypass is write", "bypass", "", "", "linux", nil, "write", "env", both, ""},
		{"env off beats user write", "off", `"write"`, "", "linux", nil, "off", "env", nil, ""},
		{"env beats project", "read", "", `"off"`, "linux", nil, "read", "env", both, ""},
		{"env invalid falls to user", "yes", `"read"`, "", "linux", nil, "read", "user", both, "ignored"},
		{"env invalid falls to default", "yes", "", "", "linux", nil, "off", "default", nil, "want off, read or bypass"},
		{"user write", "", `"write"`, "", "linux", nil, "write", "user", both, ""},
		{"user beats project", "", `"read"`, `"off"`, "linux", nil, "read", "user", both, ""},
		{"user invalid value fails closed", "", `"bypass"`, "", "linux", []string{"claude"}, "off", "user", nil, "is not off, read or write"},
		{"user invalid json fails closed", "", `"off",`, "", "linux", []string{"claude"}, "off", "user", nil, "invalid JSON"},
		{"project invalid json fails closed", "", "", `"read",`, "linux", []string{"claude"}, "off", "project", nil, "invalid JSON"},
		{"project write ignored", "", "", `"write"`, "linux", []string{"claude"}, "read", "default", []string{"claude"}, "can only lower"},
		{"project read above default ignored", "", "", `"read"`, "linux", nil, "off", "default", nil, "can only lower"},
		{"project read equal to default", "", "", `"read"`, "linux", []string{"claude"}, "read", "project", []string{"claude"}, ""},
		{"project off lowers default", "", "", `"off"`, "linux", []string{"claude"}, "off", "project", nil, ""},
		{"project off", "", "", `"off"`, "linux", nil, "off", "project", nil, ""},
		{"windows default off", "", "", "", "windows", []string{"claude"}, "off", "default", nil, ""},
		{"windows project read ignored", "", "", `"read"`, "windows", []string{"claude"}, "off", "default", nil, "can only lower"},
		{"windows explicit env wins", "read", "", "", "windows", nil, "read", "env", both, ""},
		{"windows explicit user wins", "", `"read"`, "", "windows", nil, "read", "user", both, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]string{WorkersEnv: c.env}
			loc := Locations{User: workersFile(t, "user.json", c.user), Project: workersFile(t, "proj.json", c.pj)}
			got := ResolveWorkers(func(k string) string { return env[k] }, loc, []string{"claude", "codex"}, c.verified, c.goos)
			if got.Mode != c.mode || got.Source != c.source || !reflect.DeepEqual(got.Agents, c.agents) {
				t.Fatalf("got %+v, want mode=%s source=%s agents=%v", got, c.mode, c.source, c.agents)
			}
			if c.noteHas == "" && len(got.Notes) != 0 {
				t.Errorf("unexpected notes %v", got.Notes)
			}
			if c.noteHas != "" && !strings.Contains(strings.Join(got.Notes, "|"), c.noteHas) {
				t.Errorf("notes %v lack %q", got.Notes, c.noteHas)
			}
		})
	}
}

// A user file that exists but cannot be used must not fall back to the default
// read: a symlinked or oversized file holding "off" resolves to off with a note.
func TestResolveWorkersUnusableUserFileFailsClosed(t *testing.T) {
	dir := t.TempDir()
	real := workersFile(t, "real.json", `"off"`)
	link := filepath.Join(dir, "policy.json")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, []byte(`{"orchestration":{"workers":"off"},"pad":"`+strings.Repeat("x", maxBytes)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"symlink": link, "oversized": big} {
		t.Run(name, func(t *testing.T) {
			got := ResolveWorkers(func(string) string { return "" }, Locations{User: path}, []string{"claude"}, []string{"claude"}, "linux")
			if got.Mode != WorkersOff || got.Source != WorkersFromUser || len(got.Agents) != 0 {
				t.Fatalf("got %+v, want off from user", got)
			}
			if !strings.Contains(strings.Join(got.Notes, "|"), "workers off until it is fixed") {
				t.Errorf("notes %v lack the fail-closed reason", got.Notes)
			}
		})
	}
}

func TestResolveWorkersMarkerForcesOff(t *testing.T) {
	env := map[string]string{WorkerMarkerEnv: "1", WorkersEnv: "bypass"}
	got := ResolveWorkers(func(k string) string { return env[k] }, Locations{}, []string{"claude", "codex"}, []string{"claude"}, "linux")
	if got.Mode != "off" || got.Source != "marker" || len(got.Agents) != 0 {
		t.Fatalf("got %+v", got)
	}
}
