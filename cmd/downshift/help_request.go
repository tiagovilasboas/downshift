// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxSuggestDistance is the largest edit distance at which an unknown command
// name still gets a "Did you mean" suggestion. Further away, the name is
// most likely not a typo, and a guess would only mislead.
const maxSuggestDistance = 3

// legacyNames are dispatch names kept for old hook files and shell rc files.
// They answer --help and run, but they are not suggested for a typo.
var legacyNames = map[string]string{
	"exec":                      "compact",
	"compactor":                 "compact",
	"shell-init":                "enable",
	"claude-code-post-tool-use": "",
	"claude-code-subagent-stop": "",
}

// commandHelpRequest answers `downshift <command> [subcommand] --help|-h` and
// `downshift help <command>` from helpTopics. handled is false when the
// arguments are not such a request, so dispatch carries on unchanged. A help
// flag after a bare `--` belongs to the wrapped program (compact), not to us.
func commandHelpRequest(args []string, out, errw io.Writer) (code int, handled bool) {
	if len(args) == 0 {
		return 0, false
	}
	if args[0] == "help" && len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		topic := helpTopicFor(args[1:])
		if text, ok := helpTopics[topic]; ok {
			fmt.Fprint(out, text)
			return 0, true
		}
		fmt.Fprintln(errw, unknownCommandLine(args[1]))
		return 2, true
	}
	if !hasHelpFlag(args[1:]) {
		return 0, false
	}
	text, ok := helpTopics[helpTopicFor(args)]
	if !ok {
		return 0, false
	}
	fmt.Fprint(out, text)
	return 0, true
}

// hasHelpFlag reports whether rest asks for help before any bare `--`.
func hasHelpFlag(rest []string) bool {
	for _, a := range rest {
		if a == "--" {
			return false
		}
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// helpTopicFor names the help entry for args: "<command> <subcommand>" when
// such an entry exists, else the command itself. A legacy name resolves to the
// command it replaced.
func helpTopicFor(args []string) string {
	name := args[0]
	if canonical, ok := legacyNames[name]; ok && canonical != "" {
		name = canonical
	}
	if len(args) >= 2 {
		if sub := name + " " + args[1]; hasTopic(sub) {
			return sub
		}
	}
	return name
}

func hasTopic(name string) bool {
	_, ok := helpTopics[name]
	return ok
}

// unknownCommandLine is the single line printed for a command that does not
// exist. It is one line on purpose: the full help is one `help` away.
func unknownCommandLine(name string) string {
	if near, ok := closestCommand(name, suggestableCommands()); ok {
		return fmt.Sprintf("unknown command %q. Did you mean %q? Run `downshift help`.", name, near)
	}
	return fmt.Sprintf("unknown command %q. Run `downshift help`.", name)
}

// suggestableCommands lists the top-level commands a typo can be corrected to.
func suggestableCommands() []string {
	var names []string
	for topic := range helpTopics {
		if strings.Contains(topic, " ") {
			continue
		}
		if _, legacy := legacyNames[topic]; legacy {
			continue
		}
		names = append(names, topic)
	}
	sort.Strings(names)
	return names
}

// closestCommand returns the candidate with the smallest edit distance to name,
// if that distance is within maxSuggestDistance. Ties go to the first name in
// candidates, which is sorted, so the answer is stable.
func closestCommand(name string, candidates []string) (string, bool) {
	best, bestDist := "", maxSuggestDistance+1
	for _, c := range candidates {
		if d := editDistance(name, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= maxSuggestDistance
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// helpTopics maps a command path ("stats", "models list") to its help text.
// Every top-level command and every subcommand that has its own flags is an
// entry; a subcommand without one is answered by its command's text.
var helpTopics = mergeTopics(dailyHelp, modelsHelp, hookHelpTopics, maintainerHelp, experimentalHelp, policyHelp, playbookHelp)

// mergeTopics joins the help groups into one table. Keys must not repeat across
// groups; TestHelpTopicsAreUnique enforces that.
func mergeTopics(groups ...map[string]string) map[string]string {
	all := make(map[string]string)
	for _, g := range groups {
		for k, v := range g {
			all[k] = v
		}
	}
	return all
}
