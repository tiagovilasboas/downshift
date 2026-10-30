// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package limits

// registry holds limitations for all known harnesses. Statuses follow the
// harness matrix (docs/reference/harness-matrix.md): only a Completed harness
// claims full support, and only Completed records usage. parity_test.go holds
// this table to the matrix rows.
var registry = map[string]HarnessLimits{
	"kirocrew": {
		HarnessID: "kirocrew",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "none",
				Reason:  "preToolUse hook is allow/block only; no updated_input channel",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "ACP provider does not expose inputTokens/outputTokens (F7); nothing recorded",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "no rewrite channel; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "none",
				Reason:  "closed harness; model fixed at session start",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "uses context_used as proxy; not actual token counts",
			},
		},
	},
	"claude-code": {
		HarnessID: "claude-code",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "full",
				Reason:  "rewrite observed honored on a real child (2026-10-06)",
			},
			{
				Feature: "token_counts",
				Status:  "full",
				Reason:  "usage read from the child transcript at SubagentStop",
			},
			{
				Feature: "compaction",
				Status:  "full",
				Reason:  "automatic via preToolUse command rewrite",
			},
			{
				Feature: "mid_session_routing",
				Status:  "full",
				Reason:  "subagent model can be changed per spawn",
			},
			{
				Feature: "savings_estimation",
				Status:  "full",
				Reason:  "based on actual token usage from the child transcript",
			},
		},
	},
	"cursor": {
		HarnessID: "cursor",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "rewrite emitted; honored unconfirmed, discarded on Free and legacy Pro",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no usage field is read at subagentStop",
			},
			{
				Feature: "compaction",
				Status:  "partial",
				Reason:  "shell rewrite wired but unverified live; otherwise run 'downshift compact --' by hand",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "honored unconfirmed: the child executor model is absent from the transcript",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"codex": {
		HarnessID: "codex",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "rewrite emitted on a real spawn; honored inferred from the next spawn, not observed",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no usage source; child end parser unverified",
			},
			{
				Feature: "compaction",
				Status:  "partial",
				Reason:  "Bash PreToolUse rewrite wired, unverified live",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "same evidence as model_rewrite: honored inferred, not observed",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"antigravity": {
		HarnessID: "antigravity",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "one observed child on desktop 2.21.1 (requested pro, executed flash_lite)",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no child end recorded; postToolUse result fields are undocumented",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "no shell rewrite wired; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "one observed child on one build; quota and alias mapping unresolved",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage source; list-price estimate only",
			},
		},
	},
	"opencode": {
		HarnessID: "opencode",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "partial",
				Reason:  "route wire v1 and conformance green; forwarding plugin in development, no live spawn observed",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "usage seam not proven; no child end recorded",
			},
			{
				Feature: "compaction",
				Status:  "manual",
				Reason:  "shell rewrite not wired; agent must run 'downshift compact --' itself",
			},
			{
				Feature: "mid_session_routing",
				Status:  "partial",
				Reason:  "route answer computed; forwarding plugin in development, no live spawn observed",
			},
			{
				Feature: "savings_estimation",
				Status:  "partial",
				Reason:  "no usage seam; list-price estimate only",
			},
		},
	},
	// Claude Desktop surfaces. The Code tab is the claude-code harness and is
	// not listed here. Chat and Cowork are separate surfaces with their own
	// limits; doctor shows these two only when Claude Desktop is installed.
	"claude-desktop-chat": {
		HarnessID: "claude-desktop-chat",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "none",
				Reason:  "Chat spawns no subagent with a model and exposes no shell tool, so there is nothing to rewrite (F2-chat-mcp, 2026-10-30)",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "no child end or usage source over the MCP surface",
			},
			{
				Feature: "compaction",
				Status:  "none",
				Reason:  "no shell tool in Chat, so 'downshift compact --' cannot be run from it",
			},
			{
				Feature: "mid_session_routing",
				Status:  "none",
				Reason:  "no model-carrying spawn to route (F2-chat-mcp, 2026-10-30)",
			},
			{
				Feature: "savings_estimation",
				Status:  "none",
				Reason:  "no usage source; nothing to estimate against",
			},
			{
				Feature: "mcp",
				Status:  "partial",
				Reason:  "read-only MCP tools (providers, status, playbook, apply) answer over stdio; delegate/collect/cancel fail with ErrNoVerifiedMode unless DOWNSHIFT_WORKERS is set (F2-chat-mcp, 2026-10-30)",
			},
		},
	},
	"claude-desktop-cowork": {
		HarnessID: "claude-desktop-cowork",
		Limitations: []Limitation{
			{
				Feature: "model_rewrite",
				Status:  "none",
				Reason:  "the Linux arm64 agent VM shares only granted folders over virtiofs, so host hooks, the plugin, ~/.claude and the macOS binary never reach the guest (F3-cowork-vm, 2026-10-30)",
			},
			{
				Feature: "token_counts",
				Status:  "none",
				Reason:  "the downshift binary and hooks do not reach the guest, so no child end or usage is recorded (F3-cowork-vm, 2026-10-30)",
			},
			{
				Feature: "compaction",
				Status:  "none",
				Reason:  "the macOS downshift binary never reaches the guest VM, so 'downshift compact --' is not available there (F3-cowork-vm, 2026-10-30)",
			},
			{
				Feature: "mid_session_routing",
				Status:  "none",
				Reason:  "no host hook reaches the guest to route a spawn (F3-cowork-vm, 2026-10-30)",
			},
			{
				Feature: "savings_estimation",
				Status:  "none",
				Reason:  "no usage source inside the guest VM; nothing to estimate against",
			},
			{
				Feature: "mcp",
				Status:  "unverified",
				Reason:  "no log shows an MCP server entering the guest VM, and none was seen failing either (F3-cowork-vm, 2026-10-30)",
			},
		},
	},
}
