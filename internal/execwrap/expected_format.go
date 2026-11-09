// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift-ai/internal/compressor"

// expectedFormats is the set of compressor formats each family may be
// compacted as. A command of the family is compacted only when the compressor
// recognises its output as one of these formats; any other detected shape (a
// repetitive-log match, a search match) keeps the raw output.
//
// Most families map to a single format. A family whose one command produces
// more than one distinct shape lists them all: gh-run covers both `gh run
// list` (gh_run_list) and `gh run view --log` (gh_run_log); gh-pr covers both
// `gh pr view` (gh_pr) and `gh pr checks` (gh_checks). The compressor detects
// which shape an invocation produced and compacts that one.
//
// A family with no entry here is a pass-through: it runs and its output is
// never compacted (the git families other than git-status, git-log, git-diff
// and git-show, whose output is small ad-hoc text, and every gh family that
// returns small ad-hoc text).
var expectedFormats = map[Family][]compressor.Format{
	FamilyGoTest:     {compressor.FormatGoTest},
	FamilyGitStatus:  {compressor.FormatGitStatus},
	FamilyGitLog:     {compressor.FormatGitLog},
	FamilyGitDiff:    {compressor.FormatGitDiff, compressor.FormatLockfileDiff},
	FamilyGitShow:    {compressor.FormatGitShow},
	FamilyPytest:     {compressor.FormatPytest},
	FamilyVitest:     {compressor.FormatVitest},
	FamilyPlaywright: {compressor.FormatPlaywright},
	FamilyCargoTest:  {compressor.FormatCargoTest},
	FamilyTSC:        {compressor.FormatTSC},
	FamilyJest:       {compressor.FormatJest},
	// A package script runs vitest or jest; the compressor tells them apart
	// by their output (compressor.JSTestRunner on a failed run).
	FamilyNpmTest:      {compressor.FormatVitest, compressor.FormatJest},
	FamilyPnpmTest:     {compressor.FormatVitest, compressor.FormatJest},
	FamilyYarnTest:     {compressor.FormatVitest, compressor.FormatJest},
	FamilyCypress:      {compressor.FormatCypress},
	FamilyNextBuild:    {compressor.FormatNextBuild},
	FamilyPHPUnit:      {compressor.FormatPHPUnit},
	FamilyFlutterTest:  {compressor.FormatFlutterTest},
	FamilyDartTest:     {compressor.FormatFlutterTest},
	FamilyGradle:       {compressor.FormatGradle},
	FamilyMaven:        {compressor.FormatMaven},
	FamilyESLint:       {compressor.FormatEslint},
	FamilyRuff:         {compressor.FormatRuff},
	FamilyMypy:         {compressor.FormatMypy},
	FamilyGolangciLint: {compressor.FormatGolangciLint},
	FamilyGoBuild:      {compressor.FormatGoBuild},
	FamilyCargoBuild:   {compressor.FormatCargoBuild},
	FamilyDotnetBuild:  {compressor.FormatDotnetBuild},
	FamilyNpmInstall:   {compressor.FormatProgress, compressor.FormatNpmAudit, compressor.FormatNpmInstall},
	FamilyPipInstall:   {compressor.FormatPipInstall},
	FamilyBazel:        {compressor.FormatBazel},
	FamilyTurboRun:     {compressor.FormatTurbo},
	FamilyNx:           {compressor.FormatNx},
	FamilyGhRun:        {compressor.FormatGhRunList, compressor.FormatGhRunLog},
	FamilyGhPr:         {compressor.FormatGhPr, compressor.FormatGhChecks},
	FamilyGhIssue:      {compressor.FormatMarkdown},
	FamilyGhRepo:       {compressor.FormatMarkdown},
	// Test runners, analysers and read-only listings whose clean run has a
	// fixed summary shape. A run that fails (non-zero exit) or prints any other
	// shape stays raw; see policy_refusals.go for the argv shapes that wrap.
	FamilyRspec:        {compressor.FormatRspec},
	FamilyRubocop:      {compressor.FormatRubocop},
	FamilyDotnetTest:   {compressor.FormatDotnetTest},
	FamilyPHPStan:      {compressor.FormatPHPStan},
	FamilyDockerBuild:  {compressor.FormatDockerBuild},
	FamilyDockerPS:     {compressor.FormatDockerPs},
	FamilyDockerImages: {compressor.FormatDockerImages},
	FamilyKubectlGet:   {compressor.FormatKubectlPods},
	FamilyGitStash:     {compressor.FormatGitStash},
	// Another program's transcript with a go test run inside: `make check`.
	// Only the clean package lines fold (compressor.FoldGoTestRuns); every
	// other line is kept. git commit, push, pull and fetch print the hooks git
	// ran around their own lines, so they go through the remote compressor,
	// which also cuts the hook runner's banner boxes and rules.
	FamilyMake:      {compressor.FormatGoTestRuns},
	FamilyGitCommit: {compressor.FormatGitRemote},
	FamilyGitPush:   {compressor.FormatGitRemote},
	FamilyGitPull:   {compressor.FormatGitRemote},
	FamilyGitFetch:  {compressor.FormatGitRemote},
	FamilyLs:        {compressor.FormatLs},
	FamilyFind:      {compressor.FormatFind},
	// grep and rg: numbered rows, path:text rows, path:count rows or, with -l,
	// a list of paths; the flags choose (commandPlan).
	FamilyGrep:    {compressor.FormatGrep, compressor.FormatGrepRows, compressor.FormatGrepCounts, compressor.FormatFind},
	FamilyRg:      {compressor.FormatGrep, compressor.FormatGrepRows, compressor.FormatGrepCounts, compressor.FormatFind},
	FamilyRgFiles: {compressor.FormatFind},
	// A file read compacts only a long source file (compressor.FileReadMinLines).
	FamilyFileRead: {compressor.FormatFileRead},
	FamilyGitGrep:  {compressor.FormatGrep},
	// curl and wget -O -: headers and body capped (compressor.HTTPFetchMinBytes).
	// glab read: secrets redacted, nothing cut (compressor.FormatGlab).
	FamilyGlab: {compressor.FormatGlab},
	// Every field kept, one line per element (compressor/agent_desktop.go).
	FamilyAgentDesktop: {compressor.FormatAgentDesktop},
	FamilyCurl:         {compressor.FormatHTTPFetch},
	FamilyWget:         {compressor.FormatHTTPFetch},
	// Logs of no shape of their own: only the generic chain applies.
	FamilyDenoTest:  {compressor.FormatGeneric},
	FamilyDenoCheck: {compressor.FormatGeneric},
	FamilyDenoLint:  {compressor.FormatGeneric},
	FamilyZigBuild:  {compressor.FormatGeneric},
	FamilyZigTest:   {compressor.FormatGeneric},
	// Python quality tools and Django checks: logs, generic chain only.
	FamilyFlake8:           {compressor.FormatGeneric},
	FamilyPylint:           {compressor.FormatGeneric},
	FamilyUnittest:         {compressor.FormatGeneric},
	FamilyTox:              {compressor.FormatGeneric},
	FamilyNox:              {compressor.FormatGeneric},
	FamilyCoverage:         {compressor.FormatGeneric},
	FamilyDjangoTest:       {compressor.FormatGeneric},
	FamilyDjangoCheck:      {compressor.FormatGeneric},
	FamilyDjangoMigrations: {compressor.FormatGeneric},
	FamilyDetoxTest:        {compressor.FormatGeneric},
	FamilyDetoxBuild:       {compressor.FormatGeneric},
	// CSS, HTML and Tailwind tooling: logs, generic chain only.
	FamilyStylelint:    {compressor.FormatGeneric},
	FamilyHTMLHint:     {compressor.FormatGeneric},
	FamilyHTMLValidate: {compressor.FormatGeneric},
	FamilyTailwind:     {compressor.FormatGeneric},
	FamilySass:         {compressor.FormatGeneric},
	FamilyPostCSS:      {compressor.FormatGeneric},
	FamilyNextLint:     {compressor.FormatGeneric},
	FamilyJSCheckRun:   {compressor.FormatGeneric},
	FamilyKnip:         {compressor.FormatGeneric},
	FamilyDepcheck:     {compressor.FormatGeneric},
	FamilyMadge:        {compressor.FormatGeneric},
}

// commandFormats are the families compacted by compressor.CompressCommandOutput.
// The command names the format: a plain list of names has no shape of its own,
// so the compressor does not detect it from the text.
var commandFormats = map[Family]compressor.Format{
	FamilyLs:      compressor.FormatLs,
	FamilyFind:    compressor.FormatFind,
	FamilyGrep:    compressor.FormatGrep,
	FamilyRg:      compressor.FormatGrep,
	FamilyRgFiles: compressor.FormatFind,
	FamilyGitGrep: compressor.FormatGrep,
	// Named by the command line, see commandPlan.
	FamilyFileRead:     compressor.FormatFileRead,
	FamilyGlab:         compressor.FormatGlab,
	FamilyCurl:         compressor.FormatHTTPFetch,
	FamilyWget:         compressor.FormatHTTPFetch,
	FamilyAgentDesktop: compressor.FormatAgentDesktop,
}

// stderrIsReport lists the families whose stderr is part of what the
// compressor reads. Jest's reporters write the whole report to stderr, so a
// green run has an empty stdout. Vitest writes its report to stdout, but the
// Node runtime and jsdom print notices on stderr while a clean run goes on
// (an ExperimentalWarning per worker, "Not implemented: navigation"). For
// these the gate merges stderr into the held buffer, in write order, and lets
// the compressor decide: failure markers, errors and any line its grammar does
// not know still keep the output raw, each stream replayed as it was written.
// git runs its hooks with their output on stderr and prints its own lines
// there too, so for git commit and git push a go test run folds wherever it
// lands and every other line is kept. go test prints its report on stdout,
// but the go command writes module downloads ("go: downloading ...") and
// linker notes ("# pkg.test" then "ld: warning: ...") on stderr while a clean
// run goes on; the go test fold keeps every line it cannot account for, so
// those notes stay and the package lines still fold. make is left out on
// purpose: a target can run for as long as a server does, and holding its
// stderr would hide what it logs. Every other family keeps the rule that any
// stderr byte releases the output untouched.
var stderrIsReport = map[Family]bool{
	FamilyJest:      true,
	FamilyVitest:    true,
	FamilyNpmTest:   true,
	FamilyPnpmTest:  true,
	FamilyYarnTest:  true,
	FamilyGitCommit: true,
	FamilyGitPush:   true,
	FamilyGitPull:   true,
	FamilyGitFetch:  true,
	FamilyGoTest:    true,
	FamilyCargoTest: true,
	FamilyBazel:     true,
	FamilyTurboRun:  true,
}

// compactibleFamily reports whether a family may be compacted at all. A
// pass-through family has no entry in expectedFormats.
func compactibleFamily(f Family) bool {
	return len(expectedFormats[f]) > 0
}

// formatAccepted reports whether the compressor's detected format is one the
// family is allowed to be compacted as. A family with no entry accepts
// nothing, so its output always passes through.
func formatAccepted(f Family, got compressor.Format) bool {
	if genericAccepted(f, got) {
		return true
	}
	for _, want := range expectedFormats[f] {
		if want == got {
			return true
		}
	}
	return false
}

// failedStderrIsReport lists the families whose failed run is still compacted
// when the program wrote to stderr. Vitest prints its failure report on stderr,
// so a failing run always has stderr bytes, and jest prints its whole report
// there; the gate holds them merged with
// stdout in write order (stderrIsReport) and the failure compressor reads the
// merged stream, keeping every line it does not know raw. A failed go test run
// that wrote to stderr stays raw, and so does a failed playwright run: its
// stderr releases the held output.
//
// git writes its whole report to stderr, so a failed push, pull or fetch
// always has stderr bytes; its hooks' output is on stderr too. The failure
// compressor cuts only noise lines and keeps every other line.
var failedStderrIsReport = map[Family]bool{
	FamilyVitest:    true,
	FamilyJest:      true,
	FamilyCargoTest: true,
	FamilyNpmTest:   true,
	FamilyPnpmTest:  true,
	FamilyYarnTest:  true,
	FamilyGitPush:   true,
	FamilyGitPull:   true,
	FamilyGitFetch:  true,
}

// failedFormat is the format passed to compressor.CompressFailed for a
// non-zero exit. CompressFailed summarises a failed go test, vitest,
// playwright, jest, pytest or cargo test run and returns every other format
// unchanged, so the families
// that can compact a failure pass their format and every other family passes
// its first listed format, which CompressFailed declines. The output then
// stays raw and red, which is what a failing gh check or build must do.
//
// A package script family (npm test, pnpm test) may run vitest or jest, so the
// output names the runner (compressor.JSTestRunner). A run whose runner cannot
// be told gets a format CompressFailed declines and stays raw.
func failedFormat(f Family, data []byte) compressor.Format {
	formats := expectedFormats[f]
	for _, want := range formats {
		if want == compressor.FormatGoTest {
			return want
		}
	}
	if formatAccepted(f, compressor.FormatVitest) && formatAccepted(f, compressor.FormatJest) {
		return compressor.JSTestRunner(data)
	}
	if len(formats) > 0 {
		return formats[0]
	}
	return compressor.FormatUnknown
}
