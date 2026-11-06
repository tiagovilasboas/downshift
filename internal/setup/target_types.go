// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

// Types of the setup target table (targets.go): what one harness row
// can declare for hooks, defaults, status line, steering, skills and MCP.

// Hook is one Downshift hook: `<binary> <Args>` on Event.
type Hook struct {
	Event   string // event key in the harness file
	Matcher string // tool matcher; empty means any (and an existing matcher is kept)
	Args    string // subcommand and arguments after the binary
	// Wrapper, when set, is the stem of a script setup generates (wrapper.go)
	// and registers instead of `<binary> <Args>`. For a harness that accepts one
	// executable path and no arguments. A hook already registered under that
	// stem from another directory is left alone.
	Wrapper string
}

// StatusLine is the optional status line command (Claude Code).
type StatusLine struct {
	Key  string // top-level key holding {"type":"command","command":...}
	Args string
	// ChainArgs is the command a wrapper script can run to feed an existing
	// status line's input to Downshift.
	ChainArgs string
}

// Default is a key setup writes when the file does not have it. Parent,
// when set, names the top-level object that holds Key. An existing value is
// never replaced; Note, when set, is printed if that value differs from Value.
// Optional defaults are the main-session policy and are skipped by
// --no-session-policy. Parent may name a map the user already fills (such as
// enabledPlugins): only Key is checked, never the map. Label and Info are for
// reports (doctor, the setup summary): Label replaces the key path, Info
// describes the value when it is the one setup writes. An OptIn default costs
// money to run, so setup never writes it unprompted: it prints Suggest when the
// key is unset and writes the value only when the user passes OptInFlag.
type Default struct {
	Parent   string
	Key      string
	Value    any
	Note     string
	Optional bool
	Label    string
	Info     string
	OptIn    bool
	// OptInFlag is the setup flag that turns an OptIn default on.
	OptInFlag string
	// Suggest is the line printed when an OptIn default is left off.
	Suggest string
}

type Target struct {
	Name string
	// Retired are hooks an older setup wrote for a command that was removed.
	// Setup deletes them from the hook file on every run, plugin or not, so
	// the stub that keeps them harmless can be dropped one day.
	Retired []Hook
	// ConfigDir is the directory (relative to HOME) whose existence means the
	// harness is installed.
	ConfigDir string
	// File is the hook file (relative to HOME). Empty means setup cannot write
	// it (see Manual).
	File string
	// HooksPath is the JSON path to the object that maps event names to hook
	// lists. Empty means the top level of the file.
	HooksPath []string
	// Nested selects the entry shape. True: {"matcher","hooks":[{"type":"command","command"}]}
	// (Claude Code, Codex). False: {"matcher","command"} (Cursor, Antigravity).
	Nested     bool
	Defaults   []Default
	Hooks      []Hook
	StatusLine *StatusLine
	// Notes are printed after the harness is wired (things setup cannot do).
	Notes []string
	// RequireFile makes setup edit File only when it already exists. For a
	// config the harness itself creates and owns: a missing file means the
	// harness is not set up, and creating it here would be a guess.
	RequireFile bool
	// Manual explains why setup does not write this harness.
	Manual string
	// Steering is the optional compact steering setup installs for this harness,
	// independent of hook wiring: a dedicated file (KiroCrew) or a managed block
	// in the harness's global instructions file. A Manual target may still carry one.
	Steering *Steering
	// SkillsDir is where the harness reads user skills, relative to HOME. Setup
	// installs the Downshift skills there; empty for a harness without one.
	SkillsDir string
	// Registry is an informational hook list the harness owns, relative to HOME.
	// Setup removes a Downshift entry from it. The policy hook stays in File.
	// Empty when the harness has no such list. Undo restores a backup only
	// when setup actually rewrote the file.
	Registry string
	// Plugin is the id ("name@marketplace") under which this harness can load
	// the same hooks from a plugin instead of File. Empty when the harness has
	// no plugin system. Doctor reads it; setup warns when it is enabled.
	Plugin string
	// Mods are embedded mod folders (package mods) written under SkillsDir as
	// skills-dir plugins (mods.go). Empty when the harness has none.
	Mods []string
	// Files are whole files setup writes verbatim (harness_files.go): a
	// bridge the harness loads from a folder instead of a JSON hook list.
	// Empty when the harness has no such glue.
	Files []HarnessFile
	// Mcp says how setup --mcp registers `downshift mcp serve` in this harness.
	// Nil means setup does not register it (stage 5 adds the other harnesses).
	Mcp *Mcp
}

// Ways setup --mcp registers the server (mcp.go).
const (
	// McpViaCLI delegates the write to the harness CLI (`claude mcp add`); File
	// is only read, to detect an entry without starting any server.
	McpViaCLI = "cli"
	// McpTOMLBlock edits a marked block of a TOML file as text; File is that file.
	McpTOMLBlock = "toml-block"
	// McpJSONMap edits an mcpServers object in a JSON file; File is that file.
	McpJSONMap = "json-map"
)

// Mcp is the per-harness MCP registration fact: a way and a file
// (relative to HOME).
type Mcp struct {
	Kind string
	File string
	// OwnerApp is the executable name of a desktop app that owns File and
	// rewrites it from memory while running, dropping an entry written from
	// outside (Claude Desktop). Empty when no app does. See owner_app.go.
	OwnerApp string
	// Layout is the entry shape of a McpJSONMap file (mcp_json_layout.go);
	// empty is the mcpServers map with command, args and env.
	Layout string
	// Env is extra environment the entry carries, beside the setup marker.
	// Claude Desktop sets DOWNSHIFT_WORKERS=off: Chat starts the server from
	// "/", where read workers would see the whole disk.
	Env map[string]string
}

// Entry shapes of the JSON harness files (Mcp.Layout). The default keeps
// the servers under "mcpServers" as a command with args and env (Cursor,
// Antigravity, Kiro, Claude Desktop). OpenCode keeps them under "mcp" then
// "servers" as a local entry with one command list and an environment object.
const (
	McpLayoutDefault  = ""
	McpLayoutOpenCode = "opencode"
)
