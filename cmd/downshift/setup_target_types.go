// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Types of the setup target table (setup_targets.go): what one harness row
// can declare for hooks, defaults, status line, steering, skills and MCP.

// setupHook is one Downshift hook: `<binary> <Args>` on Event.
type setupHook struct {
	Event   string // event key in the harness file
	Matcher string // tool matcher; empty means any (and an existing matcher is kept)
	Args    string // subcommand and arguments after the binary
	// Wrapper, when set, is the stem of a script setup generates (setup_wrapper.go)
	// and registers instead of `<binary> <Args>`. For a harness that accepts one
	// executable path and no arguments. A hook already registered under that
	// stem from another directory is left alone.
	Wrapper string
}

// setupStatusLine is the optional status line command (Claude Code).
type setupStatusLine struct {
	Key  string // top-level key holding {"type":"command","command":...}
	Args string
	// ChainArgs is the command a wrapper script can run to feed an existing
	// status line's input to Downshift.
	ChainArgs string
}

// setupDefault is a key setup writes when the file does not have it. Parent,
// when set, names the top-level object that holds Key. An existing value is
// never replaced; Note, when set, is printed if that value differs from Value.
// Optional defaults are the main-session policy and are skipped by
// --no-session-policy. Parent may name a map the user already fills (such as
// enabledPlugins): only Key is checked, never the map. Label and Info are for
// reports (doctor, the setup summary): Label replaces the key path, Info
// describes the value when it is the one setup writes. An OptIn default costs
// money to run, so setup never writes it unprompted: it prints Suggest when the
// key is unset and writes the value only when the user passes OptInFlag.
type setupDefault struct {
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

type setupTarget struct {
	Name string
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
	Defaults   []setupDefault
	Hooks      []setupHook
	StatusLine *setupStatusLine
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
	Steering *setupSteering
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
	// Mcp says how setup --mcp registers `downshift mcp serve` in this harness.
	// Nil means setup does not register it (stage 5 adds the other harnesses).
	Mcp *setupMcp
}

// Ways setup --mcp registers the server (setup_mcp.go).
const (
	// mcpViaCLI delegates the write to the harness CLI (`claude mcp add`); File
	// is only read, to detect an entry without starting any server.
	mcpViaCLI = "cli"
	// mcpTOMLBlock edits a marked block of a TOML file as text; File is that file.
	mcpTOMLBlock = "toml-block"
	// mcpJSONMap edits an mcpServers object in a JSON file; File is that file.
	mcpJSONMap = "json-map"
)

// setupMcp is the per-harness MCP registration fact: a way and a file
// (relative to HOME).
type setupMcp struct {
	Kind string
	File string
}
