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

var setupTargets = []setupTarget{
	{
		Name:      "claude-code",
		ConfigDir: ".claude",
		SkillsDir: ".claude/skills",
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
			// CLAUDE_CODE_SUBAGENT_MODEL_FORCE is not written (owner decision O8): with
			// it set, Claude Code overrides the model the orchestrator chose per call.
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
			// The opt-in exec wrapper rewrites an allowlisted Bash command (shellrewrite.go).
			{Event: "PreToolUse", Matcher: "Bash", Args: "claude-code"},
			{Event: "PostToolUse", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			// Bash output reaches the context sensor (observe only: bytes and
			// the compressor's potential, never the text).
			{Event: "PostToolUse", Matcher: "Bash", Args: "claude-code-post-tool-use"},
			{Event: "PostToolUseFailure", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			{Event: "SessionStart", Args: "claude-code-session-start"},
			{Event: "SubagentStop", Args: "claude-code-subagent-stop"},
			{Event: "UserPromptSubmit", Args: "dci hook claude-code"},
		},
		StatusLine: &setupStatusLine{
			Key:       "statusLine",
			Args:      "quota claude-statusline",
			ChainArgs: "quota import --harness claude-code --source claude-statusline",
		},
		// User scope lives in ~/.claude.json, which Claude Code rewrites itself.
		Mcp: &setupMcp{Kind: mcpViaCLI, File: ".claude.json"},
	},
	{
		Name:      "cursor",
		ConfigDir: ".cursor",
		File:      ".cursor/hooks.json",
		HooksPath: []string{"hooks"},
		Defaults:  []setupDefault{{Key: "version", Value: 1}},
		Hooks: []setupHook{
			{Event: "sessionStart", Args: "cursor-touch"},
			{Event: "beforeSubmitPrompt", Matcher: "UserPromptSubmit", Args: "cursor-touch"},
			{Event: "preToolUse", Matcher: "Task", Args: "cursor"},
			// Shell command compaction (updated_input.command). Unverified live.
			{Event: "preToolUse", Matcher: "Shell", Args: "cursor"},
			// Shell advise fallback: when the Shell rewrite is ignored, the agent is
			// told to rerun a compressing command through downshift compact. Matcher
			// "*" sends every shell command to the hook; the family check is inside.
			{Event: "beforeShellExecution", Matcher: "*", Args: "cursor"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "subagentStop", Args: "end cursor"},
		},
		Mcp: &setupMcp{Kind: mcpJSONMap, File: ".cursor/mcp.json"},
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
		Steering: &setupSteering{Path: ".config/opencode/AGENTS.md", Content: compactSteering, Managed: true},
	},
	{
		Name:      "codex",
		ConfigDir: ".codex",
		SkillsDir: ".codex/skills",
		File:      ".codex/hooks.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Hooks: []setupHook{
			{Event: "PreToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "codex"},
			// Shell rewrite: allowlisted Bash commands become `downshift compact -- ...` (shellrewrite.go).
			{Event: "PreToolUse", Matcher: "^Bash$", Args: "codex"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "PostToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "end codex"},
		},
		Notes: []string{
			"Codex also needs [features] hooks = true and [features.multi_agent_v2] enabled = true in config.toml.",
			"Codex skips a new hook until you trust it: run /hooks in Codex, then start a new session.",
		},
		// Global instructions. Codex reads AGENTS.override.md first; if that file
		// exists it wins and the managed block is not read.
		Steering: &setupSteering{Path: ".codex/AGENTS.md", Content: compactSteering, Managed: true},
		Mcp:      &setupMcp{Kind: mcpTOMLBlock, File: ".codex/config.toml"},
	},
	{
		Name:      "antigravity",
		ConfigDir: ".gemini",
		File:      ".gemini/config/hooks.json",
		Hooks: []setupHook{
			{Event: "PreToolUse", Matcher: "invoke_subagent", Args: "antigravity"},
		},
		Steering: &setupSteering{Path: ".gemini/AGENTS.md", Content: compactSteering, Managed: true},
		Mcp:      &setupMcp{Kind: mcpJSONMap, File: ".gemini/config/mcp_config.json"},
	},
	{
		Name:      "kirocrew",
		ConfigDir: ".kiro",
		// The hook goes in agent.kiro_hooks, not in the generated agent spec
		// (~/.kiro/agents/kirocrew.json is rewritten on every restart).
		// KiroCrew takes one executable and no arguments, hence the wrapper.
		File:        ".kiro/crew/config.json",
		HooksPath:   []string{"agent", "kiro_hooks"},
		RequireFile: true,
		Hooks: []setupHook{
			{Event: "preToolUse", Matcher: "*", Args: "kirocrew", Wrapper: "kirocrew-pre-tool-use"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "postToolUse", Matcher: "spawn_run", Args: "end kirocrew", Wrapper: "kirocrew-post-tool-use"},
		},
		Notes: []string{
			"Restart KiroCrew so it regenerates its agent spec with the hook, then run `downshift doctor`.",
		},
		// Dedicated workspace steering file (KiroCrew reads steering/*.md with
		// frontmatter), so setup owns the whole file.
		Steering: &setupSteering{
			Path:        ".kiro/crew/workspace/.kiro/steering/downshift-compact.md",
			Frontmatter: "---\ninclusion: always\n---\n\n",
			Content:     compactSteering,
		},
		// Crew's own hook page. Exit codes there do not enforce policy, and a
		// Downshift row duplicates agent.kiro_hooks (often still pointing at a
		// checkout). Setup drops that row; other hooks in the file stay.
		Registry: ".kiro/crew/hooks.json",
		Mcp:      &setupMcp{Kind: mcpJSONMap, File: ".kiro/settings/mcp.json"},
	},
	{
		// Claude Desktop Chat is an MCP host, not a hook harness: it has no hook
		// file, no skills and no steering, so this target only registers the MCP
		// server. File is empty on purpose; setup reports it as detected-not-wired
		// and the server is added through the shared JSON-map writer. Chat reads
		// <UserConfigDir>/Claude/claude_desktop_config.json; on macOS that is
		// ~/Library/Application Support/Claude (what os.UserConfigDir resolves to).
		// ConfigDir gates the write: setup never creates the file when the Claude
		// directory is absent. Chat must be restarted to pick the server up, and
		// only providers/status/playbook/apply work from Chat (no spawn tools).
		Name:      "claude-desktop",
		ConfigDir: "Library/Application Support/Claude",
		Manual:    "Claude Desktop Chat is an MCP host with no hooks, skills or steering, so setup only registers the MCP server there.",
		Notes: []string{
			"Restart Claude Desktop to pick up the MCP server; only providers/status/playbook/apply work from Chat.",
		},
		Mcp: &setupMcp{Kind: mcpJSONMap, File: "Library/Application Support/Claude/claude_desktop_config.json"},
	},
}
