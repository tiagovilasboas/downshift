# Downshift setup — OS-agnostic analysis and roadmap

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Snapshot: **2026-10-20**. Scope: is `downshift setup` (and the KiroCrew/Kiro IDE
steering install) correct on Windows, Linux and macOS? What is already covered,
what is a real gap.

## Already OS-agnostic (verified in code)

| Concern | Evidence |
|---|---|
| HOME resolution | `setup.go` uses `os.UserHomeDir()` (cross-OS), not `$HOME` |
| Path building | every target path goes through `filepath.FromSlash` + `filepath.Join` (`setup_plan.go`, `setup_plugin.go`, `setup_steering.go`, `setup_write.go`); stored paths use `/` by convention and are converted per-OS |
| OS-specific behavior | build-tagged files exist: `scorerefresh_windows.go`, `internal/execwrap/procgroup_windows.go`, `internal/paths/owned_windows.go` |
| Cross-compile | CI builds `goos: [linux, darwin, windows]` (`.github/workflows/ci.yml`) |
| The new steering install | `setup_steering.go` + the kirocrew `Steering.Path` follow the exact same `FromSlash`/`Join` pattern as every hook `File`; introduces no new coupling |

## Real gaps (pre-existing, not introduced by the steering work)

### GAP-OS-1 — CI runs tests only on Linux

CI cross-COMPILES for windows/darwin but every `runs-on` is `ubuntu-latest`.
A binary that compiles for Windows is not a binary whose setup/hook behavior was
observed on Windows. Path-join, symlink-eval and the exec wrapper can pass on
Linux and still break on Windows.

- **Fix:** add a `windows-latest` (and ideally `macos-latest`) job that runs at
  least `go test ./cmd/downshift/ ./internal/paths/ ./internal/execwrap/`.
- **Acceptance:** green test job on windows-latest in CI.

### GAP-OS-2 — KiroCrew wrapper is a POSIX `.sh` only

`scripts/kirocrew-pre-tool-use.sh` is `#!/bin/sh`. KiroCrew's hook wants an
absolute executable with no arguments. On Windows there is no `/bin/sh`, so the
wrapper cannot run natively; the model-routing hook is POSIX-only today.

- **Note:** the STEERING path (compact + routing guidance) is OS-agnostic — it
  is plain markdown the agent reads, and `downshift compact -- <cmd>` is the
  same binary on every OS. Only the model-routing HOOK wrapper is POSIX-bound.
- **Fix options:** (a) ship a `.cmd`/`.ps1` wrapper for Windows KiroCrew and
  pick it per-OS in setup; (b) confirm whether KiroCrew on Windows can register
  the `downshift` binary directly (no wrapper) — the wrapper exists only because
  KiroCrew rejects arguments, so a Windows build named to encode the subcommand,
  or a tiny `downshift-kirocrew.exe` shim, would remove the `.sh` dependency.
- **Acceptance:** a Windows KiroCrew install runs the preToolUse hook without a
  POSIX shell, observed once.

### GAP-OS-3 — symlinked steering target on Windows

On this machine `~/.kiro/steering` is a symlink to harness-core, and
`atomicWriteFile` resolves symlinks (`EvalSymlinks`). Setup deliberately does
NOT write the Kiro IDE steering for exactly this reason (it would write into a
versioned repo). On Windows, symlinks and `EvalSymlinks` behave differently
(often absent or privileged). This does not affect the KiroCrew steering
(a real local dir), but any future Kiro IDE auto-install must treat the symlink
case per-OS.

- **Acceptance:** documented decision; no Kiro IDE auto-write until the symlink
  case is handled per-OS.

## What is NOT a gap

- The compact steering content and `downshift compact` are OS-agnostic.
- The `setup_steering.go` install path is as cross-OS as the rest of setup.
- No hardcoded `/Users`, `/home`, `/tmp` in setup production code.

## Priority

GAP-OS-1 (CI on Windows) is the highest-leverage: without it, every other OS
claim is unverified. GAP-OS-2 is KiroCrew-model-routing-specific and only bites
Windows KiroCrew users. GAP-OS-3 is a guardrail, already respected.
