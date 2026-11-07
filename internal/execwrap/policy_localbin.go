// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Project-local tool directories (`node_modules/.bin`, `.venv/bin`)
// ════════════════════════════════════════════════════════════════════════════

// nodeModulesBinPrefixes are the relative paths to a project's local JS bin
// directory, matching the `./vendor/bin/` convention used for PHP tools.
var nodeModulesBinPrefixes = []string{
	"./node_modules/.bin/",
	"node_modules/.bin/",
}

// venvBinPrefixes are the relative paths to a project's local Python
// virtualenv bin directory.
var venvBinPrefixes = []string{
	"./.venv/bin/",
	".venv/bin/",
	"./venv/bin/",
	"venv/bin/",
}

// nodeModulesBinTool extracts the bare tool name from a `./node_modules/.bin/<tool>`
// or `node_modules/.bin/<tool>` path. Package managers and nested paths are
// refused so a local bin path only reaches leaf JS tools.
func nodeModulesBinTool(prog string) (string, bool) {
	for _, p := range nodeModulesBinPrefixes {
		if rest, ok := strings.CutPrefix(prog, p); ok {
			if !validLocalToolName(rest) {
				return "", false
			}
			switch rest {
			case "npm", "pnpm", "yarn", "bun", "npx":
				return "", false
			}
			return rest, true
		}
	}
	return "", false
}

// stripNodeModulesBin returns a copy of argv with a `./node_modules/.bin/` or
// `node_modules/.bin/` prefix removed from argv[0], or argv unchanged.
func stripNodeModulesBin(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	if tool, ok := nodeModulesBinTool(argv[0]); ok {
		out := make([]string, len(argv))
		out[0] = tool
		copy(out[1:], argv[1:])
		return out
	}
	return argv
}

// venvBinTool extracts the bare tool name from a `./.venv/bin/<tool>`,
// `.venv/bin/<tool>`, `./venv/bin/<tool>` or `venv/bin/<tool>` path. Environment
// runners (`uv`, `poetry`, `pdm`, `pip`, `pip3`) and nested paths are refused.
func venvBinTool(prog string) (string, bool) {
	for _, p := range venvBinPrefixes {
		if rest, ok := strings.CutPrefix(prog, p); ok {
			if !validLocalToolName(rest) {
				return "", false
			}
			switch rest {
			case "uv", "poetry", "pdm", "pip", "pip3":
				return "", false
			}
			return rest, true
		}
	}
	return "", false
}

// stripVenvBin returns a copy of argv with a local virtualenv bin prefix
// removed from argv[0], or argv unchanged.
func stripVenvBin(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	if tool, ok := venvBinTool(argv[0]); ok {
		out := make([]string, len(argv))
		out[0] = tool
		copy(out[1:], argv[1:])
		return out
	}
	return argv
}

func validLocalToolName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/")
}
