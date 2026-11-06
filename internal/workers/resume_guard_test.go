// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "testing"

func TestLaunchResumeRefusesFlagShapedSessionID(t *testing.T) {
	for _, id := range []string{"", "-x", "--dangerously-bypass-approvals-and-sandbox", "a b", "a/../b"} {
		if _, err := (byAgent{}).LaunchResume(Spec{Agent: "codex"}, id); err != errBadSessionID {
			t.Errorf("LaunchResume(%q) err = %v, want errBadSessionID", id, err)
		}
	}
}
