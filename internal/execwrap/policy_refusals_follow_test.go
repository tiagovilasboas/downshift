// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// A follow, watch or server form never exits, and the compactor holds output
// until exit, so it is refused; the one-shot form of the same family wraps.
func TestDecide_RefusesFormsThatNeverExit(t *testing.T) {
	const exe = "/opt/ds/downshift"
	cases := []struct {
		command string
		family  Family // the family of a wrapped form; empty when refused
	}{
		{"docker logs api", FamilyDockerLogs},
		{"docker logs -f api", ""},
		{"docker logs --follow api", ""},
		{"docker logs -tf api", ""},
		{"docker logs --follow=true api", ""},
		{"kubectl logs api --tail=50", FamilyKubectlLogs},
		{"kubectl logs -f api", ""},
		{"kubectl logs api --follow", ""},
		{"journalctl -u nginx -n 50", FamilyJournalctl},
		{"journalctl -f", ""},
		{"journalctl -fu nginx", ""},
		{"journalctl --follow -u nginx", ""},
		{"aws logs tail /aws/lambda/fn --since 1h", FamilyAwsLogs},
		{"aws logs tail /aws/lambda/fn --follow", ""},
		{"logcli query q1", FamilyLogCLI},
		{"logcli query --tail q1", ""},
		{"logcli query -t q1", ""},
		{"jest", FamilyJest},
		{"jest -w 4", FamilyJest}, // -w is --maxWorkers
		{"jest --watchAll=false", FamilyJest},
		{"jest --watch", ""},
		{"jest --watchAll", ""},
		{"npx jest --watchAll", ""},
		{"vitest run", FamilyVitest},
		{"npx vitest run src", FamilyVitest},
		{"vitest watch", ""},
		{"vitest dev", ""},
		{"npx vitest watch", ""},
		{"vitest --watch", ""},
		{"vitest -w", ""},
		{"tsc --noEmit", FamilyTSC},
		{"tsc --watch", ""},
		{"tsc -w", ""},
		{"npx tsc -b --watch", ""},
		{"vite build", FamilyViteBuild},
		{"vite preview", ""},
		{"npx vite preview --port 4000", ""},
		{"flutter test", FamilyFlutterTest},
		{"flutter run", ""},
		{"flutter run -d chrome", ""},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			d := Decide(c.command, exe)
			if c.family == "" {
				if d.Rewrite || d.Reason != ReasonNotAllowlisted {
					t.Fatalf("Decide(%q) = %+v, want refused as %q", c.command, d, ReasonNotAllowlisted)
				}
				return
			}
			if !d.Rewrite || d.Family != c.family {
				t.Fatalf("Decide(%q) = %+v, want wrapped as %q", c.command, d, c.family)
			}
		})
	}
}
