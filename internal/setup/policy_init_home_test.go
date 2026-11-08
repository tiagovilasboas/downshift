// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"path/filepath"
	"strings"
	"testing"
)

// The project policy step is skipped, with a reason, when the project file
// would be the user policy (<state dir>/policy.json): run from the directory
// that holds the state directory. Any other directory keeps the step.
func TestPlanPolicyInitSkipsTheUserPolicyPath(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "src", "app")
	custom := filepath.Join(t.TempDir(), "state")
	cases := []struct {
		name               string
		projectDir, state  string
		wantSkip, wantPlan bool
	}{
		{"home with the default state dir", home, "", true, false},
		{"home with a trailing separator", home + string(filepath.Separator), "", true, false},
		{"a project under home", project, "", false, true},
		{"home with a state dir elsewhere", home, custom, false, true},
		{"the parent of a custom state dir", filepath.Dir(custom), custom, false, true},
		{"no project dir", "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok := PlanPolicyInit(Options{Home: home, StateDir: c.state, ProjectDir: c.projectDir})
			if c.projectDir == "" {
				if ok {
					t.Fatalf("no project dir planned %+v", p)
				}
				return
			}
			if !ok {
				t.Fatal("no plan returned")
			}
			skipped := strings.Contains(p.Skip, "is your user policy file")
			if skipped != c.wantSkip {
				t.Errorf("Skip = %q, want user-policy skip %v", p.Skip, c.wantSkip)
			}
			if (len(p.After) > 0) != c.wantPlan {
				t.Errorf("After = %q, want a file planned: %v", p.After, c.wantPlan)
			}
		})
	}
}
