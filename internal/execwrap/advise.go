// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// adviseFamilies are the families whose output is large enough that one extra
// model turn (the exit-2 retry) costs less than the log it saves: test
// runners and build tools. Git read-outs stay out: their output is small, so
// a retry would cost more than the compaction returns.
var adviseFamilies = map[Family]bool{
	FamilyGoTest:      true,
	FamilyPytest:      true,
	FamilyCargoTest:   true,
	FamilyFlutterTest: true,
	FamilyDartTest:    true,
	FamilyGradle:      true,
	FamilyMaven:       true,
	FamilyPHPUnit:     true,
	FamilyVitest:      true,
	FamilyJest:        true,
	FamilyPlaywright:  true,
	FamilyCypress:     true,
	FamilyTSC:         true,
	FamilyNextBuild:   true,
}

// WorthAdvising reports whether a harness that can only advise (exit 2 and
// stderr, no in-place rewrite) should ask for a retry of this family.
func WorthAdvising(f Family) bool { return adviseFamilies[f] }
