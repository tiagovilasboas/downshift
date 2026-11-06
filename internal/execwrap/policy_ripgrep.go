// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: ripgrep — rg match rows and rg --files (read-only)
// ════════════════════════════════════════════════════════════════════════════

// ripgrep families. rg -n prints path:line:text like grep -rn; rg --files
// prints one path per line like find.
const (
	FamilyRg      Family = "rg"
	FamilyRgFiles Family = "rg-files"
)

// rgFlagSet is the allowlist of one rg form. Short flags may be clustered; a
// short flag with a value must end its cluster and takes the next argument.
// Long flags with a value take it as "--x v" or "--x=v". Everything not named
// is refused: -r (replace in rg), --pre, -z (spawn programs), context, JSON,
// --files-without-match and every other output-shaping flag. -l and -c are
// accepted (a list of files, a count per file), but not together.
type rgFlagSet struct {
	shortPlain, shortValue string
	longPlain              []string
	longValue              []string
}

var rgMatchFlags = rgFlagSet{
	shortPlain: "niSswxFHuLlc",
	shortValue: "egtTm",
	longPlain: []string{
		"line-number", "ignore-case", "smart-case", "case-sensitive", "word-regexp",
		"line-regexp", "fixed-strings", "with-filename", "hidden", "no-ignore",
		"follow", "no-heading", "files-with-matches", "count",
	},
	longValue: []string{"regexp", "glob", "type", "type-not", "max-count"},
}

var rgFilesFlags = rgFlagSet{
	shortPlain: "uL",
	shortValue: "gtT",
	longPlain:  []string{"files", "hidden", "no-ignore", "follow"},
	longValue:  []string{"glob", "type", "type-not"},
}

func inList(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// rgParse splits args into the flags seen ("-n", "--files", ...) and the
// operands, or reports false when a flag is outside set, a value is missing,
// or an operand is "-" or "--".
func rgParse(args []string, set rgFlagSet) (seen map[string]bool, operands []string, ok bool) {
	seen = map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-" || a == "--":
			return nil, nil, false
		case strings.HasPrefix(a, "--"):
			name, _, hasVal := strings.Cut(a[2:], "=")
			switch {
			case inList(set.longPlain, name) && !hasVal:
			case inList(set.longValue, name):
				if !hasVal {
					if i+1 == len(args) {
						return nil, nil, false
					}
					i++
				}
			default:
				return nil, nil, false
			}
			seen["--"+name] = true
		case strings.HasPrefix(a, "-"):
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				c := cluster[j]
				switch {
				case strings.IndexByte(set.shortPlain, c) >= 0:
				case strings.IndexByte(set.shortValue, c) >= 0:
					if j != len(cluster)-1 || i+1 == len(args) {
						return nil, nil, false
					}
					i++
				default:
					return nil, nil, false
				}
				seen["-"+string(c)] = true
			}
		default:
			operands = append(operands, a)
		}
	}
	return seen, operands, true
}

// ripgrepFamily names the rg family of argv, or FamilyNone.
func ripgrepFamily(argv []string) Family {
	if argv[0] != "rg" {
		return FamilyNone
	}
	if rgFilesForm(argv[1:]) {
		return FamilyRgFiles
	}
	if rgMatchForm(argv[1:]) {
		return FamilyRg
	}
	return FamilyNone
}

// rgMatchForm needs at least one path: with no path rg reads standard input
// when it is not a terminal. With -e/--regexp every operand is a path. The
// rows are path:line:text with -n, path:text without, a path per line with -l
// and path:count with -c; -l and -c together are refused. A run whose rows
// carry no path (one file operand) stays raw when it runs: the compressor
// reads a row only under a path operand (compressor.CommandHint.Roots).
func rgMatchForm(args []string) bool {
	seen, operands, ok := rgParse(args, rgMatchFlags)
	if !ok || rgListing(seen) && rgCounting(seen) {
		return false
	}
	if !(seen["-e"] || seen["--regexp"]) {
		if len(operands) == 0 {
			return false
		}
		operands = operands[1:]
	}
	return len(operands) > 0
}

// rgListing and rgCounting report -l / --files-with-matches and -c / --count.
func rgListing(seen map[string]bool) bool  { return seen["-l"] || seen["--files-with-matches"] }
func rgCounting(seen map[string]bool) bool { return seen["-c"] || seen["--count"] }

// rgFilesForm needs --files; paths are optional since --files reads no stdin.
func rgFilesForm(args []string) bool {
	seen, _, ok := rgParse(args, rgFilesFlags)
	return ok && seen["--files"]
}
