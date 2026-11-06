// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift/internal/compressor"

// genericFamilies lists the families whose clean output may also be compacted
// by the generic chain (compressor.FormatGeneric) when it matches none of the
// family's own formats: base64 blobs elided, repeated lines folded, a long
// output capped. They are tools whose output is a log (test runners, builds,
// linters, package scripts, make). The git and gh families are left out: their
// output is the answer the agent asked for, so none of it is cut.
var genericFamilies = map[Family]bool{
	FamilyPlaywright: true, FamilyCypress: true, FamilyVitest: true,
	FamilyJest: true, FamilyNpmTest: true, FamilyPnpmTest: true, FamilyYarnTest: true,
	FamilyPytest: true, FamilyCargoTest: true, FamilyGoTest: true,
	FamilyPHPUnit: true, FamilyFlutterTest: true, FamilyDartTest: true,
	FamilyGradle: true, FamilyMaven: true, FamilyMake: true,
	FamilyNextBuild: true, FamilyTSC: true, FamilyESLint: true,
	FamilyRuff: true, FamilyMypy: true, FamilyGolangciLint: true,
	FamilyGoBuild: true, FamilyCargoBuild: true, FamilyDotnetBuild: true,
	FamilyDenoTest: true, FamilyDenoCheck: true, FamilyDenoLint: true,
	FamilyZigBuild: true, FamilyZigTest: true,
	FamilyFlake8: true, FamilyPylint: true, FamilyUnittest: true,
	FamilyTox: true, FamilyNox: true, FamilyCoverage: true,
	FamilyDjangoTest: true, FamilyDjangoCheck: true, FamilyDjangoMigrations: true,
	FamilyDetoxTest: true, FamilyDetoxBuild: true,
	FamilyStylelint: true, FamilyHTMLHint: true, FamilyHTMLValidate: true,
	FamilyTailwind: true, FamilySass: true, FamilyPostCSS: true,
	FamilyNextLint: true, FamilyJSCheckRun: true, FamilyKnip: true,
	FamilyDepcheck: true, FamilyMadge: true,
}

// genericAccepted reports whether f accepts the formats that are not a tool's
// own shape: the generic chain, and carriage-return progress frames collapsed
// (compressor.FormatProgress), which npm install also prints.
func genericAccepted(f Family, got compressor.Format) bool {
	switch got {
	case compressor.FormatGeneric:
		return genericFamilies[f]
	case compressor.FormatProgress:
		return genericFamilies[f] || f == FamilyNpmInstall
	}
	return false
}
