// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Harness hook wiring for `downshift setup` is data. Each row says where a
// harness keeps its hook file, what shape its entries have and which Downshift
// subcommand runs on which event. The merge code in setup.go reads this table
// and never names a harness. A harness that declares
// HarnessCapabilities.ChildEndEvent has a row for that event (checked by
// TestSetupTargetsWireEveryChildEndEvent); Claude Code keeps its historical
// aliases for the same runner. Sources: docs/guides/install.md,
// docs/reference/session-models.md, docs/guides/quota.md.

// setupHook is one Downshift hook: `<binary> <Args>` on Event.
type setupHook struct {
	Event   string // event key in the harness file
	Matcher string // tool matcher; empty means any (and an existing matcher is kept)
	Args    string // subcommand and arguments after the binary
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
	// Manual explains why setup does not write this harness.
	Manual string
	// Plugin is the id ("name@marketplace") under which this harness can load
	// the same hooks from a plugin instead of File. Empty when the harness has
	// no plugin system. Doctor reads it; setup warns when it is enabled.
	Plugin string
	// ModConflicts are plugin ids that carry a mod the Downshift plugin already
	// bundles (hooks/hooks.json modules). Both enabled means the mod runs
	// twice; doctor warns. Harness data: only a harness with mods lists any.
	ModConflicts []string
	// Mods are sibling plugin ids ("name@marketplace") setup installs next to
	// Plugin. They carry no routing, so a failed install is only a note.
	Mods []string
}

var setupTargets = []setupTarget{
	{
		Name:      "claude-code",
		ConfigDir: ".claude",
		File:      ".claude/settings.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Plugin:    "downshift@downshift",
		// Main-session policy: start in plan mode, plan on Opus and execute on
		// Sonnet (opusplan). Downshift routes the subagents underneath; see
		// docs/reference/session-models.md. Never replaces a value the user already set.
		Defaults: []setupDefault{
			{
				Key:      "model",
				Value:    "opusplan",
				Optional: true,
				Note: `the main session keeps its pinned "model"; set "model": "opusplan" in ~/.claude/settings.json ` +
					"(or run /model opusplan) to plan on Opus and execute on Sonnet",
			},
			{
				Parent:   "permissions",
				Key:      "defaultMode",
				Value:    "plan",
				Optional: true,
				Note: `sessions keep their "permissions.defaultMode"; set it to "plan" in ~/.claude/settings.json ` +
					"to start every session in plan mode",
			},
			// Claude Code's built-in "You should know" mod (off by default). The key is
			// a sub-key of the enabledPlugins map, which the user usually already has
			// (the Downshift plugin itself), so "only when unset" applies to the
			// sub-key. Each check is a billed background request to the session's
			// model, so it is opt-in (--you-should-know): setup only suggests it.
			{
				Parent:    "enabledPlugins",
				Key:       "cc-plugin-you-should-know@builtin",
				Value:     true,
				Optional:  true,
				OptIn:     true,
				OptInFlag: "--you-should-know",
				Suggest: "You should know is off: each check is a billed background request to the session's model. " +
					"Run `downshift setup --you-should-know` to turn it on",
				Label: "you-should-know",
				Info: "on (built-in Claude Code mod; each check is a billed request; " +
					"/plugin disable cc-plugin-you-should-know@builtin to turn off)",
				Note: `"cc-plugin-you-should-know@builtin" is not true in enabledPlugins; set it to true in ~/.claude/settings.json ` +
					"(or run /plugin enable cc-plugin-you-should-know@builtin) to turn on You should know; each check is a billed request",
			},
		},
		Hooks: []setupHook{
			{Event: "PreToolUse", Matcher: "Agent|Task", Args: "claude-code"},
			{Event: "PostToolUse", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			{Event: "PostToolUseFailure", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			{Event: "SubagentStop", Args: "claude-code-subagent-stop"},
			{Event: "UserPromptSubmit", Args: "dci hook claude-code"},
		},
		StatusLine: &setupStatusLine{
			Key:       "statusLine",
			Args:      "quota claude-statusline",
			ChainArgs: "quota import --harness claude-code --source claude-statusline",
		},
		// The upstream marketplace install of the mod vendored under hooks/cache-tax/.
		ModConflicts: []string{"cache-tax@claude-code-mods"},
		// Sibling mods of the Downshift marketplace (mods/, listed in
		// .claude-plugin/marketplace.json): progress bar and /agents-info panel,
		// file tree pane, downshift-dock band that reopens both. mods/downshift-router stays out: it would route twice.
		Mods: []string{"savvy-progress@downshift", "filetree@downshift", "downshift-dock@downshift"},
	},
	{
		Name:      "cursor",
		ConfigDir: ".cursor",
		File:      ".cursor/hooks.json",
		HooksPath: []string{"hooks"},
		Defaults:  []setupDefault{{Key: "version", Value: 1}},
		Hooks: []setupHook{
			{Event: "preToolUse", Matcher: "Task", Args: "cursor"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "subagentStop", Args: "end cursor"},
		},
	},
	{
		Name:      "codex",
		ConfigDir: ".codex",
		File:      ".codex/hooks.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Hooks: []setupHook{
			{Event: "PreToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "codex"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "PostToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "end codex"},
		},
		Notes: []string{
			"Codex also needs [features] hooks = true and [features.multi_agent_v2] enabled = true in config.toml.",
			"Codex skips a new hook until you trust it: run /hooks in Codex, then start a new session.",
		},
	},
	{
		Name:      "antigravity",
		ConfigDir: ".gemini",
		File:      ".gemini/config/hooks.json",
		Hooks: []setupHook{
			{Event: "PreToolUse", Matcher: "invoke_subagent", Args: "antigravity"},
		},
	},
	{
		Name:      "kirocrew",
		ConfigDir: ".kiro",
		Manual: "KiroCrew accepts only an absolute executable path with no arguments, so it needs the wrapper script " +
			"scripts/kirocrew-pre-tool-use.sh registered in ~/.kiro/crew/config.json (agent.kiro_hooks.preToolUse). " +
			"See docs/guides/install.md, section KiroCrew.",
	},
	{
		Name:      "opencode",
		ConfigDir: ".config/opencode",
		Manual: "opencode is wired by its own plugin (phase 2), which forwards subagent_spawn_intent to " +
			"`downshift opencode` and the child_done observation to `downshift end opencode`; there is no hook file " +
			"to write yet.",
		Hooks: []setupHook{
			// The observation envelope the plugin forwards when a child ends
			// (F1 parses the child_done JSON envelope; see internal/adapters/opencode).
			{Event: "child_done", Args: "end opencode"},
		},
	},
}
