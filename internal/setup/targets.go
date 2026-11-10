// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/mods"
)

// Harness hook wiring for `downshift setup` is data. Each row says where a
// harness keeps its hook file, what shape its entries have and which Downshift
// subcommand runs on which event. The merge code in setup.go reads this table
// and never names a harness. A harness that declares
// HarnessCapabilities.ChildEndEvent has a row for that event (checked by
// TestSetupTargetsWireEveryChildEndEvent); Claude Code keeps its historical
// aliases for the same runner. Sources: docs/guides/install.md,
// docs/reference/session-models.md, docs/guides/quota.md.

var Targets = []Target{
	{
		Name:      "claude-code",
		ConfigDir: ".claude",
		SkillsDir: ".claude/skills",
		File:      ".claude/settings.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Plugin:    "downshift@downshift",
		// The live agents band and pane (mods/downshift-agents). It observes
		// only; --no-mods leaves it out.
		Mods: []string{mods.Agents},
		// Claude Code compacts through its shell-rewrite hook, so its global
		// instructions get only the agent triggers, in their own marked block.
		Steering: &Steering{
			Path:    ".claude/CLAUDE.md",
			Content: AgentTriggers,
			Managed: true,
			Begin:   AgentsBlockBegin,
			End:     AgentsBlockEnd,
		},
		// Main-session policy: start in plan mode, plan on Opus and execute on
		// Sonnet (opusplan). Downshift routes the subagents underneath; see
		// docs/reference/session-models.md. Never replaces a value the user already set.
		Defaults: []Default{
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
		// v0.4.x wrote the code intelligence hook, removed in #720.
		Retired: []Hook{{Event: "UserPromptSubmit", Args: "dci hook claude-code"}},
		Hooks: []Hook{
			{Event: "PreToolUse", Matcher: "Agent|Task", Args: "claude-code", Rewrites: RewritesModel},
			// The opt-in exec wrapper rewrites an allowlisted Bash command (shellrewrite.go).
			{Event: "PreToolUse", Matcher: "Bash", Args: "claude-code", Rewrites: RewritesShell},
			{Event: "PostToolUse", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			// Bash output reaches the context sensor (bytes and the
			// compressor's potential, never the text) and the shared post-run
			// compaction (internal/postrun), which may answer updatedToolOutput.
			{Event: "PostToolUse", Matcher: "Bash", Args: "claude-code-post-tool-use"},
			// Read, Grep and Glob feed the main-session work count (mainwork);
			// a large Grep or Glob result is also compacted after the run.
			{Event: "PostToolUse", Matcher: "Read|Grep|Glob", Args: "claude-code-post-tool-use"},
			{Event: "PostToolUseFailure", Matcher: "Agent|Task", Args: "claude-code-post-tool-use"},
			{Event: "SessionStart", Args: "claude-code-session-start"},
			{Event: "SubagentStop", Args: "claude-code-subagent-stop"},
		},
		StatusLine: &StatusLine{
			Key:       "statusLine",
			Args:      "quota claude-statusline",
			ChainArgs: "quota import --harness claude-code --source claude-statusline",
		},
		// User scope lives in ~/.claude.json, which Claude Code rewrites itself.
		Mcp: &Mcp{Kind: McpViaCLI, File: ".claude.json"},
	},
	{
		Name:      "cursor",
		ConfigDir: ".cursor",
		File:      ".cursor/hooks.json",
		HooksPath: []string{"hooks"},
		Defaults:  []Default{{Key: "version", Value: 1}},
		Hooks: []Hook{
			{Event: "sessionStart", Args: "cursor-touch"},
			{Event: "beforeSubmitPrompt", Matcher: "UserPromptSubmit", Args: "cursor-touch"},
			{Event: "preToolUse", Matcher: "Task", Args: "cursor", Rewrites: RewritesModel},
			// Shell command compaction (updated_input.command). Unverified live.
			{Event: "preToolUse", Matcher: "Shell", Args: "cursor", Rewrites: RewritesShell},
			// Shell advise fallback: when the Shell rewrite is ignored, the agent is
			// told to rerun a compressing command through downshift compact. Matcher
			// "*" sends every shell command to the hook; the family check is inside.
			// The hook answers a command it does not own with the neutral {}, never allow.
			{Event: "beforeShellExecution", Matcher: "*", Args: "cursor"},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "subagentStop", Args: "end cursor"},
		},
		Mcp: &Mcp{Kind: McpJSONMap, File: ".cursor/mcp.json"},
	},
	{
		Name:      "opencode",
		ConfigDir: ".config/opencode",
		// The bridge is a plugin OpenCode loads from its config directory. It
		// forwards the subagent intent to `downshift opencode`, applies the
		// route and retry answers, rewrites an allowlisted shell command, and
		// sends the child_done observation to `downshift end opencode`.
		Files:    OpenCodeBridgeFiles(),
		Mcp:      &Mcp{Kind: McpJSONMap, File: ".config/opencode/opencode.jsonc", Layout: McpLayoutOpenCode},
		Steering: &Steering{Path: ".config/opencode/AGENTS.md", Content: HarnessSteering, Managed: true},
	},
	{
		Name:      "codex",
		ConfigDir: ".codex",
		SkillsDir: ".codex/skills",
		File:      ".codex/hooks.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Hooks: []Hook{
			{Event: "PreToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "codex", Rewrites: RewritesModel},
			// Shell rewrite: allowlisted Bash commands become `downshift compact -- ...` (shellrewrite.go).
			{Event: "PreToolUse", Matcher: "^Bash$", Args: "codex", Rewrites: RewritesShell},
			// Child end (N5). Unverified: needs a real payload (docs/reference/harness-matrix.md).
			{Event: "PostToolUse", Matcher: "(^Agent$|.*spawn_agent$)", Args: "end codex"},
		},
		Notes: []string{
			"Codex also needs [features] hooks = true and [features.multi_agent_v2] enabled = true in config.toml.",
			"Codex skips a new hook until you trust it: run /hooks in Codex, then start a new session.",
		},
		// Global instructions. Codex reads AGENTS.override.md first; if that file
		// exists it wins and the managed block is not read.
		Steering: &Steering{Path: ".codex/AGENTS.md", Content: HarnessSteering, Managed: true},
		Mcp:      &Mcp{Kind: McpTOMLBlock, File: ".codex/config.toml"},
	},
	{
		// Kiro IDE (VS Code extension). Distinct from KiroCrew (the daemon):
		// Kiro stores hooks in ~/.kiro/hooks/ (multiple JSON files, v1 schema),
		// uses ~/.kiro/skills/ for skills, and reads steering from .kiro/steering/.
		// KiroCrew is configured separately through .kiro/crew/config.json.
		Name:      "kiro",
		ConfigDir: ".kiro",
		SkillsDir: ".kiro/skills",
		File:      ".kiro/hooks/downshift.json",
		HooksPath: []string{"hooks"},
		Nested:    true,
		Hooks: []Hook{
			// Model routing for subagent spawns.
			{Event: "PreToolUse", Matcher: "invoke_sub_agent", Args: "kiro", Rewrites: RewritesModel},
			// Shell rewrite: allowlisted Bash commands become `downshift compact -- ...` (shellrewrite.go).
			{Event: "PreToolUse", Matcher: "execute_bash", Args: "kiro", Rewrites: RewritesShell},
			// Child end (N5). Matcher tracks subagent completion.
			{Event: "PostToolUse", Matcher: "invoke_sub_agent", Args: "end kiro"},
			// Bash output reaches the context sensor (observe only: bytes and
			// the compressor's potential, never the text).
			{Event: "PostToolUse", Matcher: "execute_bash", Args: "kiro-post-tool-use"},
			// Read, Grep and Glob feed only the main-session work count (mainwork).
			{Event: "PostToolUse", Matcher: "read_file|grep_search|file_search", Args: "kiro-post-tool-use"},
			{Event: "SessionStart", Args: "kiro-session-start"},
		},
		Notes: []string{
			"Kiro reads hooks from ~/.kiro/hooks/ automatically. Restart the Kiro IDE to pick up new hooks.",
		},
		// Global instructions. Kiro reads steering files from .kiro/steering/ with
		// frontmatter-based inclusion rules.
		Steering: &Steering{
			Path:        ".kiro/steering/downshift-compact.md",
			Frontmatter: "---\ninclusion: always\n---\n\n",
			Content:     HarnessSteering,
		},
		Mcp: &Mcp{Kind: McpJSONMap, File: ".kiro/settings/mcp.json"},
	},
	{
		Name:      "antigravity",
		ConfigDir: ".gemini",
		File:      ".gemini/config/hooks.json",
		Hooks: []Hook{
			{Event: "PreToolUse", Matcher: "invoke_subagent", Args: "antigravity", Rewrites: RewritesModel},
			{Event: "PreToolUse", Matcher: "run_command", Args: "antigravity", Rewrites: RewritesShell},
		},
		Steering: &Steering{Path: ".gemini/AGENTS.md", Content: HarnessSteering, Managed: true},
		Mcp:      &Mcp{Kind: McpJSONMap, File: ".gemini/config/mcp_config.json"},
	},
	{
		Name:      "kirocrew",
		ConfigDir: ".kiro",
		SkillsDir: ".kiro/skills",
		// The hook goes in agent.kiro_hooks, not in the generated agent spec
		// (~/.kiro/agents/kirocrew.json is rewritten on every restart).
		// KiroCrew takes one executable and no arguments, hence the wrapper.
		File:        ".kiro/crew/config.json",
		HooksPath:   []string{"agent", "kiro_hooks"},
		RequireFile: true,
		Hooks: []Hook{
			{Event: "preToolUse", Matcher: "*", Args: "kirocrew", Wrapper: "kirocrew-pre-tool-use"},
			// Child end (N5). Matcher uses fnmatch glob: *spawn_run* matches both
			// the bare name and the MCP-qualified mcp_kirocrew_core_spawn_run.
			{Event: "postToolUse", Matcher: "*spawn_run*", Args: "end kirocrew", Wrapper: "kirocrew-post-tool-use"},
		},
		Notes: []string{
			"Restart KiroCrew so it regenerates its agent spec with the hook, then run `downshift doctor`.",
		},
		// Dedicated workspace steering file (KiroCrew reads steering/*.md with
		// frontmatter), so setup owns the whole file.
		Steering: &Steering{
			Path:        ".kiro/crew/workspace/.kiro/steering/downshift-compact.md",
			Frontmatter: "---\ninclusion: always\n---\n\n",
			Content:     HarnessSteering,
		},
		// Crew's own hook page. Exit codes there do not enforce policy, and a
		// Downshift row duplicates agent.kiro_hooks (often still pointing at a
		// checkout). Setup drops that row; other hooks in the file stay.
		Registry: ".kiro/crew/hooks.json",
		Mcp:      &Mcp{Kind: McpJSONMap, File: ".kiro/settings/mcp.json"},
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
		// The app rewrites this file from memory while open (OwnerApp), so setup
		// warns when it is running and doctor reports a dropped entry.
		Mcp: &Mcp{Kind: McpJSONMap, File: "Library/Application Support/Claude/claude_desktop_config.json", OwnerApp: "Claude",
			Env: map[string]string{policy.WorkersEnv: policy.WorkersOff}},
	},
}
