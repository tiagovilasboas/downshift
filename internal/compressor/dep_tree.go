package compressor

import (
	"bytes"
	"regexp"
	"unicode/utf8"
)

// Dependency tree patterns
var (
	// npm ls / pnpm ls / yarn list
	reNpmLsHeader   = regexp.MustCompile(`(?m)^[a-z@][\w./-]*@[\d.]+`)
	reNpmLsTree     = regexp.MustCompile(`(?m)^[├└│ ]+[a-z@][\w./-]*@`)
	reNpmLsDeduped  = regexp.MustCompile(`deduped`)
	reNpmLsExtraneous = regexp.MustCompile(`extraneous`)
	reNpmLsMissing  = regexp.MustCompile(`(?i)(MISSING|UNMET|INVALID|peer dep missing)`)

	// cargo tree
	reCargoTree     = regexp.MustCompile(`(?m)^[a-z_][\w-]* v[\d.]+`)
	reCargoTreeDep  = regexp.MustCompile(`(?m)^[│├└ ]+[a-z_][\w-]* v[\d.]+`)
	reCargoTreeDup  = regexp.MustCompile(`\(\*\)`)

	// go mod graph
	reGoModGraph    = regexp.MustCompile(`(?m)^[\w./]+@v[\d.]+ [\w./]+@v[\d.]+`)
	reGoModWhy      = regexp.MustCompile(`(?m)^# [\w./]+`)
)

func isNpmLs(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reNpmLsHeader.Match(input) && reNpmLsTree.Match(input)
}

func isCargoTree(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reCargoTree.Match(input) && reCargoTreeDep.Match(input)
}

func isGoModGraph(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	// go mod graph: each line is "mod@version dep@version"
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 3 {
		return false
	}
	matches := 0
	for _, line := range lines[:min(10, len(lines))] {
		if reGoModGraph.Match(line) {
			matches++
		}
	}
	return matches >= 3
}

func compressNpmLs(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "npm_ls_too_large"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	var rootPkg []byte
	var directDeps int
	var totalDeps int
	var dedupedCount int
	var issues []string

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Root package (first line usually)
		if rootPkg == nil && reNpmLsHeader.Match(line) && !bytes.HasPrefix(bytes.TrimSpace(line), []byte("├")) {
			rootPkg = bytes.TrimSpace(line)
			continue
		}

		// Count deps
		if reNpmLsTree.Match(line) {
			totalDeps++
			// Direct deps have only one level of tree chars
			trimmed := bytes.TrimLeft(line, "├└│─ ")
			indent := len(line) - len(trimmed)
			if indent <= 4 {
				directDeps++
			}
		}

		if reNpmLsDeduped.Match(line) {
			dedupedCount++
		}

		// Capture issues
		if reNpmLsMissing.Match(line) {
			issues = append(issues, string(bytes.TrimSpace(line)))
			if len(issues) > 5 {
				issues = issues[:5]
			}
		}

		if reNpmLsExtraneous.Match(line) {
			issues = append(issues, string(bytes.TrimSpace(line)))
		}
	}

	out.WriteString("npm ls:\n")
	if rootPkg != nil {
		out.Write(rootPkg)
		out.WriteString("\n")
	}
	out.WriteString("  direct: ")
	out.WriteString(itoa(directDeps))
	out.WriteString(", total: ")
	out.WriteString(itoa(totalDeps))
	if dedupedCount > 0 {
		out.WriteString(", deduped: ")
		out.WriteString(itoa(dedupedCount))
	}
	out.WriteString("\n")

	if len(issues) > 0 {
		out.WriteString("\n⚠️  Issues:\n")
		for _, issue := range issues {
			out.WriteString("  - ")
			out.WriteString(truncateLine(issue, 80))
			out.WriteString("\n")
		}
	}

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "npm_ls_no_gain"
	}

	return result, true, "npm_ls_compacted"
}

func compressCargoTree(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "cargo_tree_too_large"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	var rootCrate []byte
	var directDeps int
	var totalDeps int
	var duplicates int

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Root crate (first line, no tree chars)
		if rootCrate == nil && reCargoTree.Match(line) && !bytes.ContainsAny(line, "│├└") {
			rootCrate = bytes.TrimSpace(line)
			continue
		}

		// Count deps
		if reCargoTreeDep.Match(line) {
			totalDeps++
			trimmed := bytes.TrimLeft(line, "│├└─ ")
			indent := len(line) - len(trimmed)
			if indent <= 4 {
				directDeps++
			}
		}

		if reCargoTreeDup.Match(line) {
			duplicates++
		}
	}

	out.WriteString("cargo tree:\n")
	if rootCrate != nil {
		out.Write(rootCrate)
		out.WriteString("\n")
	}
	out.WriteString("  direct: ")
	out.WriteString(itoa(directDeps))
	out.WriteString(", total: ")
	out.WriteString(itoa(totalDeps))
	if duplicates > 0 {
		out.WriteString(", duplicates: ")
		out.WriteString(itoa(duplicates))
	}
	out.WriteString("\n")

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "cargo_tree_no_gain"
	}

	return result, true, "cargo_tree_compacted"
}

func compressGoModGraph(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "go_mod_graph_too_large"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	// Track unique modules and edges
	modules := make(map[string]bool)
	var edges int

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if reGoModGraph.Match(line) {
			edges++
			parts := bytes.Fields(line)
			if len(parts) >= 2 {
				// Extract module name without version
				mod1 := bytes.Split(parts[0], []byte("@"))[0]
				mod2 := bytes.Split(parts[1], []byte("@"))[0]
				modules[string(mod1)] = true
				modules[string(mod2)] = true
			}
		}
	}

	out.WriteString("go mod graph:\n")
	out.WriteString("  modules: ")
	out.WriteString(itoa(len(modules)))
	out.WriteString(", edges: ")
	out.WriteString(itoa(edges))
	out.WriteString("\n")

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "go_mod_graph_no_gain"
	}

	return result, true, "go_mod_graph_compacted"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
