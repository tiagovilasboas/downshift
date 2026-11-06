// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

// The isolation flags each worker CLI starts with, on the launch and on the
// resume: what a worker may load from its directory and its user, and which
// tools a read worker holds. The recipes in launch_agents.go build on these.

// claudeIsolation keeps a worker from loading the user's MCP servers. Without
// it a worker inherits every server the user registered, claude.ai connectors
// (mail, GitHub, documents) and Downshift itself included: under
// bypassPermissions those tools run without approval, and their definitions
// cost context on every call (measured 2026-10-31: 31 servers and 323 tools
// inherited, none with the flag).
//
// --setting-sources user keeps it from loading the project and local settings
// files of the directory it runs in. In print mode Claude skips the workspace
// trust dialog, so without the flag a .claude/settings.json in that directory
// (or in a dependency folder the worker was pointed at) could run hooks, set
// env such as ANTHROPIC_BASE_URL, or widen permissions before the permission
// mode applies. The user's own settings still load.
var claudeIsolation = []string{"--strict-mcp-config", "--setting-sources", "user"}

// claudeReadTools makes read a tool limit, not only a permission mode: the
// worker gets the tools that read files and nothing that runs code or writes,
// and anything that would ask for permission is denied, since nobody is there
// to answer.
var claudeReadTools = []string{"--tools", "Read,Grep,Glob", "--permission-prompts", "none"}

// claudeModeArgs are the print-mode flags shared by a launch and a resume: the
// permission mode, the read tool limit for the plan mode, and the isolation.
// Every value-taking flag here is followed by another flag, so the variadic
// --tools never swallows a positional argument.
func claudeModeArgs(mode string) []string {
	args := []string{"-p", "--output-format", "json", "--permission-mode", mode}
	if mode == "plan" {
		args = append(args, claudeReadTools...)
	}
	return append(args, claudeIsolation...)
}

// codexMCPOff keeps a worker from loading any MCP server, Downshift's own
// included: `-c` values are TOML, so this sets mcp_servers to an empty table.
var codexMCPOff = []string{"-c", "mcp_servers={}"}
