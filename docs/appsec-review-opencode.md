# AppSec review — OpenCode bridge (feat/opencode-support)

Scope: hooks/opencode/ (index.ts, models.ts, shell_sessions.ts, shell_trust.ts, wire.ts), internal/adapters/opencode/*.go, cmd/downshift/setup_harness_files.go, setup_mcp_json*.go, doctor_harness_files.go.

## Findings

### LOW — DOWNSHIFT_BIN is trusted without validation
- **File:** hooks/opencode/index.ts:146
- **Scenario:** If an attacker can set `DOWNSHIFT_BIN` in the OpenCode process environment, the bridge runs that binary with crafted JSON input and applies its answer (model switch, shell rewrite). This is the same class as PATH hijacking.
- **Fix:** Resolve the binary to an absolute path and verify it is executable before the first call; or document that `DOWNSHIFT_BIN` must point at the genuine Downshift binary. Not blocking for a CLI tool where the user controls the environment.
- **Status (2026-11-05): fixed.** A set `DOWNSHIFT_BIN` must be an absolute path to an existing file, or the bridge stays off. With it unset, the bridge resolves `downshift` from PATH once at startup, pins that path and runs it (relative PATH entries and files without an execute bit are skipped); not on PATH means the bridge is off, with one log line. A shell rewrite is applied only when the original command has no quote, `$`, backtick, backslash or control character, only runs of ASCII spaces differ, and each wrapper is in command position (at the start, or after `;`, `&&`, `||` or `|`, optionally after `NAME=value` words) and names the pinned binary. A wrapper in argument or redirect position, which would let a command edit or delete the binary, is refused. Tests: `hooks/opencode_plugin_test.go`.

### LOW — setup writes through symlinks without EvalSymlinks
- **File:** cmd/downshift/setup_harness_files.go:38-52
- **Scenario:** If an attacker pre-creates a symlink at `~/.config/opencode/plugins/downshift/index.ts` pointing at a file that already carries the ownership marker, setup overwrites the symlink target. The marker check (`setupOwns`) is the only defense; a file without the marker is refused.
- **Fix:** Call `filepath.EvalSymlinks` on `dest` before reading/writing, mirroring `setup_mcp_json.go`. Consistent with the existing MCP path handling.
- **Status (2026-11-05): fixed differently.** Resolving the link made setup write through it into the link target (for example a dotfiles repository). Setup now refuses a harness file that is itself a link, or whose parent directory below home is a link (Windows junctions included), and leaves it alone; `--undo` refuses the same way and does not remove a linked folder. Test: `TestSetupDoesNotWriteThroughALinkedHarnessFile`.

### INFO — Fail-open is comprehensive
Every bridge call wraps in try/catch and returns null on any failure (missing binary, timeout, non-zero exit, bad JSON). The Go side fails open on invalid requests, oversized payloads, panics and unknown fields. No spawn or shell command is blocked by a Downshift failure.

### INFO — Strict decode and field caps
The adapter strictly decodes wire v1 and drops unknown fields. String fields are capped (model id, agent, version, task text, correlation ids). Usage counts are bounded by `maxUsageValue`. The shell command is capped by `maxShellText`.

### INFO — Task text and command text do not leak
Task text is a classification input only; it is never echoed in responses, logged or persisted. Shell commands are classified by family; only the family and skip reason reach telemetry, never the command text. Directory paths are not persisted.

### INFO — Shell permission scan is fail-closed
`permissionRulesPresent` returns true (block) on any unreadable config, unparseable JSONC, or any permission rule that is not allow-only. The JSONC stripper is string-aware. Trailing commas are handled. The scan walks the whole config tree of each source: the inline environment values, `OPENCODE_CONFIG`, the global, managed and `OPENCODE_CONFIG_DIR` dirs, and, for the session directory and the shell's directory, `opencode.json(c)` and `.opencode` from that directory up to the filesystem root, with the `agent`, `agents`, `mode` and `modes` markdown frontmatter. Each read is capped at 1 MiB. A linked `agent`, `agents`, `mode` or `modes` folder or an entry in one, an oversized or non-regular file, an unknown or relative directory, a frontmatter the scan cannot read (an opening line other than `---`) and a macOS managed profile (`ai.opencode.managed.plist`, also per user) count as "has rules" without being parsed. A linked `opencode.json(c)` is read through to its target, under the same 1 MiB cap. A source outside this list is not seen. Good.

### INFO — File writing is defensive
`planHarnessFiles` refuses to overwrite a file without the ownership marker. Undo checks the marker before restoring. The marker phrase is `Written by \`downshift setup\``. Backups are kept. Foreign files produce a clear error and stop the run.

### INFO — Observation batching is bounded
Usage counts are summed once per assistant message (deduped by message ID). The children map and shellSessions map are capped at 500 entries. The event stream uses an AbortController. Good.

### INFO — No injection vectors
Payloads are constructed with `JSON.stringify` and parsed with strict decode. No shell interpolation (argv array, no shell). No SQL, no template injection.

## Verdict

**PASS.** No BLOCK or HIGH findings. The two LOW findings are defense-in-depth improvements, not merge blockers. The bridge is fail-open everywhere, the adapter strictly validates, and the setup file writing is marker-gated. The telemetry is honest after the candidate-restriction fix (only listed models are routed). The retry fallback is not live-verified but is covered by unit tests and fails open. Recommend fixing the two LOW items before or shortly after merge. Both are fixed as of 2026-11-05 (see each status line).
