// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: argv shapes that never exit. The compactor holds a command's output
// until it exits (gate.go), so a follow, watch or server form would show the
// model nothing while it runs and lose everything if it is killed. Such a form
// is refused and left as written; the one-shot form of the same family wraps.
// ════════════════════════════════════════════════════════════════════════════

// followFlag names the flags of one family that keep the command running: the
// long names (matched as --name or --name=value) and the short letter, also
// found inside a cluster such as -tf. A zero short means none.
type followFlag struct {
	long  []string
	short byte
}

// followFlags lists the allowlisted families with a follow or watch flag.
// jest has no short form: its -w is --maxWorkers.
var followFlags = map[Family]followFlag{
	FamilyDockerLogs:  {long: []string{"follow"}, short: 'f'},
	FamilyKubectlLogs: {long: []string{"follow"}, short: 'f'},
	FamilyJournalctl:  {long: []string{"follow"}, short: 'f'},
	FamilyAwsLogs:     {long: []string{"follow"}},
	FamilyLogCLI:      {long: []string{"tail", "follow"}, short: 't'},
	FamilyJest:        {long: []string{"watch", "watchAll"}},
	FamilyVitest:      {long: []string{"watch"}, short: 'w'},
	FamilyTSC:         {long: []string{"watch"}, short: 'w'},
}

// alwaysRunning are families whose every form starts a server or an app and
// waits: `vite preview` serves the build, `flutter run` runs the app.
var alwaysRunning = map[Family]bool{FamilyVitePreview: true, FamilyFlutterRun: true}

// keepsRunning reports whether the argv of family f follows, watches or
// serves, so it would not exit on its own.
func keepsRunning(f Family, argv []string) bool {
	if alwaysRunning[f] {
		return true
	}
	if f == FamilyVitest && vitestWatchSubcommand(argv) {
		return true
	}
	rule, ok := followFlags[f]
	return ok && namesFollowFlag(argv[1:], rule)
}

// namesFollowFlag reports a word that turns the follow or watch mode on. An
// explicit --name=false keeps it off (as in `jest --watchAll=false`).
func namesFollowFlag(words []string, rule followFlag) bool {
	for _, w := range words {
		if long, ok := strings.CutPrefix(w, "--"); ok {
			for _, n := range rule.long {
				if long == n || (strings.HasPrefix(long, n+"=") && long != n+"=false") {
					return true
				}
			}
			continue
		}
		if rule.short != 0 && shortCluster(w) && strings.IndexByte(w[1:], rule.short) >= 0 {
			return true
		}
	}
	return false
}

// shortCluster reports a single-dash word of letters only, such as -f or -tf.
func shortCluster(w string) bool {
	if len(w) < 2 || w[0] != '-' || w[1] == '-' {
		return false
	}
	for i := 1; i < len(w); i++ {
		if c := w[i] | 0x20; c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

// vitestWatchSubcommand reports `vitest watch` or `vitest dev` (its alias),
// also behind npx, pnpm exec or yarn.
func vitestWatchSubcommand(argv []string) bool {
	for i, w := range argv {
		if w == "vitest" && i+1 < len(argv) {
			return argv[i+1] == "watch" || argv[i+1] == "dev"
		}
	}
	return false
}
