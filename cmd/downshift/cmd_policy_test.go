// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// isolatePolicy points HOME and the state directory at fresh temp dirs and
// makes the temp dir the working directory, so no real file is read.
func isolatePolicy(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(home, ".downshift"))
	project := t.TempDir()
	t.Chdir(project)
	return project
}

func writeProjectPolicy(t *testing.T, project, body string) {
	t.Helper()
	path := policy.ProjectPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyCheckAcceptsAbsentFiles(t *testing.T) {
	isolatePolicy(t)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "policy ok: 0 profiles, 0 rules") || !strings.Contains(out.String(), "project file: none") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestPolicyCheckAcceptsFloorAndRoutingInOneFile(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"min_tier":"mid","routing":{"profiles":[{"name":"reviewer","tier":"frontier","harness":"claude-code"}],"rules":[{"id":"review","kinds":["review"],"profile":"reviewer"}]}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "policy ok: 1 profiles, 1 rules") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestPolicyCheckRefusesWithTheField(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"routing":{"critical_threshold":150}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(errw.String(), "routing.critical_threshold") {
		t.Fatalf("stderr = %q, want the field named", errw.String())
	}
}

func TestPolicyCheckReportsFloorRefusal(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"min_tier":"huge","routing":{}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 1 || !strings.Contains(errw.String(), "min_tier") {
		t.Fatalf("rc = %d, stderr %q; want the tier floor refused", rc, errw.String())
	}
}

func TestPolicyUsageIsAUsageError(t *testing.T) {
	for _, args := range [][]string{nil, {"list"}, {"check", "extra"}} {
		var out, errw bytes.Buffer
		if rc := runPolicy(args, &out, &errw); rc != 2 {
			t.Errorf("runPolicy(%v) rc = %d, want 2", args, rc)
		}
	}
}

// skipWithoutSearchPermissionCheck skips where a directory's search bit does
// not stop this process: Windows, and root, which bypasses permission checks.
func skipWithoutSearchPermissionCheck(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory search permission is not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory search permission")
	}
}

// TestPolicyCheckExitCodes covers the states a policy file can be in. An absent
// file passes. A refused file fails with its field named. A file that cannot be
// looked up because a directory above it is not searchable fails too: it must
// not pass as if no file existed.
func TestPolicyCheckExitCodes(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T)
		wantRC  int
		wantErr string
	}{
		{
			name:    "no file is absent and passes",
			arrange: func(t *testing.T) { isolatePolicy(t) },
			wantRC:  0,
		},
		{
			name: "invalid file is refused with its field",
			arrange: func(t *testing.T) {
				writeProjectPolicy(t, isolatePolicy(t), `{"version":1,"routing":{"critical_threshold":150}}`)
			},
			wantRC:  1,
			wantErr: "routing.critical_threshold",
		},
		{
			name: "unsearchable directory is an error, not an absent file",
			arrange: func(t *testing.T) {
				skipWithoutSearchPermissionCheck(t)
				project := isolatePolicy(t)
				sub := filepath.Join(project, "sub")
				if err := os.Mkdir(sub, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Chdir(sub)
				if err := os.Chmod(project, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(project, 0o755) })
			},
			wantRC:  1,
			wantErr: "permission denied",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.arrange(t)
			var out, errw bytes.Buffer
			rc := runPolicy([]string{"check"}, &out, &errw)
			if rc != tc.wantRC {
				t.Fatalf("rc = %d, want %d; stdout %q stderr %q", rc, tc.wantRC, out.String(), errw.String())
			}
			if tc.wantErr != "" && !strings.Contains(errw.String(), tc.wantErr) {
				t.Fatalf("stderr = %q, want it to contain %q", errw.String(), tc.wantErr)
			}
		})
	}
}
