// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// DOMAIN: Frontend quality checks — next lint, lint/typecheck package scripts
// and read-only dependency and graph auditors (knip, depcheck, madge).

// JS quality family constants.
const (
	FamilyNextLint   Family = "next-lint"
	FamilyJSCheckRun Family = "js-check-script"
	FamilyKnip       Family = "knip"
	FamilyDepcheck   Family = "depcheck"
	FamilyMadge      Family = "madge"
)

// jsCheckScripts is the explicit allowlist of package script names. Any other
// `npm run <name>` stays refused: a script can run anything.
var jsCheckScripts = map[string]bool{
	"lint": true, "typecheck": true, "type-check": true, "check": true,
}

// jsQualityFamily routes the check forms of this file. Output of all of them
// is a log, so a clean run goes through the generic chain only.
func jsQualityFamily(argv []string) Family {
	if f := jsCheckScriptFamily(argv); f != FamilyNone {
		return f
	}
	tool := argv
	if len(tool) > 0 && tool[0] == "npx" {
		tool = tool[1:]
	}
	if len(tool) == 0 {
		return FamilyNone
	}
	args := tool[1:]
	switch tool[0] {
	case "next":
		if len(args) >= 1 && args[0] == "lint" && jsArgsReadOnly(args[1:], "--fix", "--fix-type", "--watch") {
			return FamilyNextLint
		}
	case "knip":
		if jsArgsReadOnly(args, "--fix", "--fix-type", "--watch", "--config", "-c") {
			return FamilyKnip
		}
	case "depcheck":
		if jsArgsReadOnly(args, "--parsers", "--detectors", "--specials") {
			return FamilyDepcheck
		}
	case "madge":
		if madgeReadOnly(args) {
			return FamilyMadge
		}
	}
	return FamilyNone
}

// jsCheckScriptFamily accepts `npm|bun run <script>`, `pnpm|yarn <script>` and
// `pnpm|yarn run <script>` for an allowlisted script name. Arguments that
// rewrite files, watch, or name a remote module are refused.
func jsCheckScriptFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	rest := argv[1:]
	hasRun := rest[0] == "run"
	if hasRun {
		rest = rest[1:]
	}
	switch argv[0] {
	case "npm", "bun":
		if !hasRun {
			return FamilyNone
		}
	case "pnpm", "yarn":
	default:
		return FamilyNone
	}
	if len(rest) == 0 || !jsCheckScripts[rest[0]] {
		return FamilyNone
	}
	if !jsArgsReadOnly(rest[1:], "--fix", "--fix-type", "--write", "--watch") {
		return FamilyNone
	}
	if argv[0] == "npm" && npmHasOptionBeforeSeparator(rest[1:]) {
		return FamilyNone
	}
	return FamilyJSCheckRun
}

// npmHasOptionBeforeSeparator reports a `-` word before `--`: npm reads it as
// its own option (--node-options, --script-shell, --prefix) and not as an
// argument of the script.
func npmHasOptionBeforeSeparator(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}

// neverExitsJS reports a watch or inspector flag: the compactor buffers until
// the process exits, so these forms would hang.
func neverExitsJS(a string) bool {
	return a == "-w" || strings.HasPrefix(a, "--watch") || strings.HasPrefix(a, "--inspect")
}

// prettierFamily accepts prettier in check mode only: `-w`/`--write` rewrites
// files and `--watch*` never exits, even next to `--check`.
func prettierFamily(argv []string) Family {
	check := false
	for _, a := range argv {
		if a == "--write" || neverExitsJS(a) {
			return FamilyNone
		}
		if a == "--check" || a == "-c" || a == "--list-different" || a == "-l" {
			check = true
		}
	}
	if check {
		return FamilyPrettier
	}
	return FamilyNone
}

// jsArgsReadOnly reports false when any argument is a remote URL, a watch or
// inspect flag, or is (or starts with `flag=`) one of the refused flags.
func jsArgsReadOnly(args []string, refused ...string) bool {
	for _, a := range args {
		if strings.Contains(a, "://") || neverExitsJS(a) {
			return false
		}
		for _, r := range refused {
			if a == r || strings.HasPrefix(a, r+"=") {
				return false
			}
		}
	}
	return true
}

// madgeReadOnly accepts `madge --circular <path>`: the circular check prints a
// report. `--image`, and config options that load code, are refused.
func madgeReadOnly(args []string) bool {
	circular := false
	for _, a := range args {
		if a == "--circular" || a == "-c" {
			circular = true
		}
	}
	return circular && len(args) >= 2 &&
		jsArgsReadOnly(args, "--image", "-i", "--webpack-config", "--ts-config", "--config", "-C")
}
