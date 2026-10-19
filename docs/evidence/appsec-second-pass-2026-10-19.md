<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# AppSec second pass, static review (2026-10-19)

What this page answers: which of the first AppSec review's findings (L1 to L5) are still open on current `main`, and what a
review of the code that pass never reached found. The first review (on `ac98599f`) was incomplete: its worktree was deleted
mid-run, so its M1 to M5 and L1 to L5 list is taken from the hand-over, not from a report on disk.

- **Base.** `origin/main` at `f9929c2a` (after #220).
- **Method.** Static reading of the code and workflows. No exploit, no network scan, no network call. Four behaviours were
  confirmed by running a locally built binary against an empty temporary `HOME` (and a temporary `DOWNSHIFT_STATE_DIR`
  where noted), never the real home: setup on a symlinked `settings.json`, setup followed by undo, `context enable` on a clean
  home, and file modes under `~/.downshift`.
- **Standard.** OWASP ASVS 5.0. Only chapter-level IDs are cited (V1 Encoding and Sanitization, V5 File Handling,
  V12 Secure Communication, V13 Configuration, V14 Data Protection, V15 Secure Coding and Architecture, V16 Logging). Requirement
  numbers were not cited because they could not be checked offline. No CVE, CWE or CVSS value is used.
- **Confidence.** `confirmed` means the behaviour was read in the code and, where stated, reproduced locally.
  `plausible` means the code allows it but it needs a condition outside this repository (a compromised upstream, a
  local attacker, a harness behaviour not verified).

## Verdict

**No CRITICAL and no HIGH finding.** One MEDIUM (the installer's integrity chain) and a set of LOW and INFO items.

- Privacy contract holds: no prompt, task text, file path or API key reaches `events.jsonl`, `agents.jsonl`,
  `loop-events.jsonl`, `dci-events.jsonl`, `probe.jsonl`, stderr or error strings (evidence in "Telemetry and privacy").
- No path from network or CLI output into a shell. Every subprocess in product code is started with an argument vector, and
  the one `sh -c` (`eval-outcome --record`) takes a maintainer-typed string.
- Provider API keys are read from the environment and sent only in a request header to the fixed provider URL. Nothing in the
  product reads a URL from the environment (grep of every `Getenv`).
- Of the first review: M1, M3, M4 (partly), M5 (by design) are closed; M2 is closed for the no-rewrite path of Claude Code
  and Codex only. L1, L2, L3, L4 are still open, L5 is mostly closed.

## Previous findings, status on `main`

| ID | Subject | Status | Evidence |
|---|---|---|---|
| M1 | Stored XSS in the dashboard | Closed (#191, #217) | `web/app.js` escapes every interpolated string (`escapeHtml`, lines 36 to 255); numbers go through `Number()`; CSP is sent as a header with `frame-ancestors 'none'` (`internal/server/server.go:504`). |
| M2 | `allow` overriding `ask` rules | Partly closed (#192) | Claude Code and Codex answer `{}` when nothing is rewritten (`claudecode.go:289`, `codex.go:228`). Still open: see F-07. |
| M3 | DCI symlink following | Closed (#192) | `WriteSecureFile` uses a temp file, 0600 and rename (`internal/codeintel/security.go:359`); telemetry opens with 0600 and tightens existing files. Not re-tested here. |
| M4 | Unsigned releases | Mostly closed (#204) | goreleaser pinned to `v2.18.3`, build provenance attested (`ci.yml:197`). The installer does not verify the attestation: see F-01. |
| M5 | Regex-only risk floor | Mitigated, residual by design (#208, #216) | Vocabulary widened. A lexical floor stays bypassable by wording the vocabulary lacks; not a defect of this pass. |
| **L1** | `settings.json` write | **Open, confirmed** | See F-02. |
| **L2** | `~/.downshift` 0755 and config 0644 | **Open, confirmed** | See F-03. |
| **L3** | Server hardening | **Open, confirmed** | See F-04. |
| **L4** | LaunchAgent | **Open, confirmed** | See F-05. |
| L5 | CI | Mostly closed (#212, #213, #214) | Every action is pinned by commit SHA, default `contents: read`, write scopes only on `sync-prices` and `release`. Residual items in F-06. |

## Findings

### MEDIUM

#### F-01 Installer integrity chain proves only that the download is intact, not who made it

- **Where.** `install.sh:135` to `:166` (checksum), `:111` to `:113` and `:56` to `:77` (version), `:39` to `:49` (latest tag).
- **Scenario.** `checksums.txt` is fetched from the same release as the archive (`install.sh:141`). An attacker who can replace
  release assets (a stolen maintainer token, a compromised release job) replaces both and the check passes. The new provenance
  attestation (#204) is never consulted by the installer, so the only users protected are those who run
  `gh attestation verify` by hand. The installed binary runs as a hook on every subagent spawn and, through the
  `UserPromptSubmit` hook, sees every prompt. The version argument is not validated: it goes straight into the URL and into
  a `grep` pattern (a value with `/` or regex characters changes which asset is fetched and checked). `curl` is called
  without `--proto '=https'`.
- **Confidence.** Confirmed that no authenticity check exists; exploitation is plausible (needs control of the release).
- **Fix.** (1) When `gh` is present, run `gh attestation verify "$TMP/$ARCHIVE" --repo tiagovilasboas/downshift` and fail on
  error; otherwise print the exact verify command. (2) Validate the version with
  `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$` before building any URL. (3) `curl --proto '=https' --tlsv1.2`.
  (4) Install through `mv` to a temp name in the destination directory, then rename, so a partial copy is never executed.
  Regression sensor: extend `install_test.go` with a bad-version and a tampered-archive case. ASVS: V15 (supply chain), V12.

### LOW

#### F-02 (previous L1) Setup writes through a fixed temp name, replaces symlinked dotfiles, no fsync

- **Where.** `cmd/downshift/setup_write.go:21` (`MkdirAll` 0755), `:34` (marker 0644), `:39` to `:46` (fixed
  `settings.json.downshift-tmp`, `os.WriteFile`, `os.Rename`), `:128` (undo `os.WriteFile` 0644 then chmod).
- **Scenario.** Reproduced with an empty `HOME`: `~/.claude/settings.json` was a symlink to `~/dot/settings.json`. After
  `downshift setup --yes`, `~/.claude/settings.json` is a regular file and `~/dot/settings.json` is untouched, so the user's
  dotfiles repository silently diverges and the next dotfiles sync removes Downshift's hooks (or restores a stale file).
  `--undo` leaves a regular file too. The temp name is predictable, and `os.WriteFile` follows a symlink planted at that name
  (needs a local writer in the user's own config directory, so low). No `fsync` of the file or directory: a crash can leave an
  empty `settings.json`. File mode is preserved from the original (0600 stayed 0600), which is correct. Backups
  (`*.downshift-bak-<stamp>`) are never pruned and sit next to the file with the original mode.
- **Confidence.** Confirmed (reproduced).
- **Fix.** Resolve the path with `filepath.EvalSymlinks` and write to the real target's directory; `os.CreateTemp` in that
  directory (0600, unique), write, `Sync`, `Chmod` to the original mode, `Rename`, `Sync` the directory. Create the directory
  with 0700 only when setup itself creates it. Make undo use the same helper. Keep at most N backups. Test: symlinked file stays
  a symlink; interrupted write leaves the original.

#### F-03 (previous L2) State directory 0755 and config 0644

- **Where.** `internal/contextopt/registry.go:107` (`MkdirAll` 0755), `:116` to `:119` (fixed `.tmp`, 0644);
  `install.sh:185` to `:187` (`mkdir -p`, default umask).
- **Scenario.** Reproduced: on a clean home, `downshift context enable` creates `~/.downshift` as `drwxr-xr-x` and
  `context-optimization.json` as `-rw-r--r--`. Every later writer calls `MkdirAll(..., 0700)`, which never tightens an existing
  directory, so the directory stays world-traversable. Files holding telemetry are 0600, so content stays protected; the
  exposure is the directory listing (which products and harnesses are used) and a config another user could read.
- **Confidence.** Confirmed (reproduced).
- **Fix.** One helper in `internal/paths` (`EnsureStateDir`: `MkdirAll` 0700 then `Chmod` 0700 when owned by the caller),
  used by registry, install.sh (`umask 077` around the `mkdir`) and setup. Write the config with the same atomic helper as
  `discovery.Merge`. Test: mode of the directory after `context enable` on an empty home.

#### F-04 (previous L3) Dashboard server

- **Where.** `internal/server/server.go:482` to `:485` (`http.ListenAndServe`, no timeouts), `:496` (`http.FileServer`),
  `cmd/downshift/main.go:87` to `:96` (web directory falls back to `./web` in the working directory).
- **Scenario.** (1) No `ReadHeaderTimeout`, `ReadTimeout` or `IdleTimeout`: a local client holding connections open exhausts the
  server (loopback only). (2) `FileServer` lists any directory under the web root (for example `/brand/`). (3) When `web/` is not
  next to the binary, it serves `./web` of whatever directory the command was run in. (4) `/api/status` and `/events` need no
  token: on a shared host any local user can read routing telemetry. The Host and Origin checks (`localOnly`) do stop DNS
  rebinding and cross-site reads.
- **Confidence.** Confirmed (1), (2), (3); plausible (4) (multi-user machines only).
- **Fix.** `http.Server{ReadHeaderTimeout: 5s, ReadTimeout: 15s, IdleTimeout: 60s}`; wrap the file system in one that refuses
  directories without `index.html`; embed `web/` with `go:embed` and drop the working-directory fallback; optional per-run
  random token in the URL fragment for the SSE and status calls.

#### F-05 (previous L4) LaunchAgent script

- **Where.** `cmd/dsmon/dsmon-launchd-setup.sh:41` to `:54`, `:29` (unquoted heredoc).
- **Scenario.** Logs go to the fixed, world-writable-directory path `/tmp/dsmon-server.log`: another local user can pre-create
  it as a symlink and have launchd append the server output into a file of the victim (append only, low value). The plist passes
  `--events`, `--agents` and `--web`, but `serve` ignores arguments (`main.go:87` to `:96`), so the script reads as if the server
  honoured a path it does not, and the `--events` path points at the legacy `~/.harness-downshift`. The heredoc expands `$BINARY`
  and `$WEB_DIR` unescaped into XML (a path with `&` or `<` produces an invalid plist; self-inflicted).
- **Confidence.** Confirmed.
- **Fix.** Log to `~/Library/Logs/downshift/dsmon-server.log` (created 0700); drop the unused arguments; generate the plist with
  `plutil` or escape XML; `chmod 600` the plist.

#### F-06 (previous L5) CI residuals

- **Where.** `.github/workflows/ci.yml:192` (`cache: true` in the job that holds `contents: write`, `id-token: write`,
  `attestations: write`), `ci.yml:137` and `ci-full.yml` (`astral-sh/setup-uv` without a pinned uv version), `.github/dependabot.yml`
  (only `github-actions`; `orchestration/uv.lock` has no update source), every `actions/checkout` (no `persist-credentials: false`).
- **Scenario.** A cache entry written by a less trusted run can be restored into the release job and influence the build that is
  then attested. The token that checkout persists in `.git/config` is available to every later step, including `go test` on
  repository code. These are hygiene gaps, not an exploit path today (no `pull_request_target`, no secrets in PR runs, fork PRs get a
  read-only token).
- **Confidence.** Plausible.
- **Fix.** `cache: false` in the release job; `persist-credentials: false` everywhere except `sync-prices`; pin the uv version;
  add `pip`/`uv` to Dependabot. Branch and tag protection could not be inspected from the repository and were not reviewed.
- **Checked and fine.** `sync-prices.yml`: fetched JSON never reaches a shell. The values that reach `changes.txt` and the PR
  body are catalog ids and numbers formatted by the script after the range and 5x move checks (#213), and the body is written to a
  file and passed with `--body-file`. The `eval "$(bash scripts/ci-classify-changes.sh)"` output is three fixed assignments.

#### F-07 (previous M2, residual) Explicit `allow` still emitted in several places

- **Where.** `claudecode.go:275` and `codex.go:214` (`permissionDecision: "allow"` on a rewrite);
  `internal/adapters/cursor/cursor.go:211` (`permission: "allow"` on every answer); `internal/adapters/antigravity/antigravity.go:194`
  (`decision: "allow"` on every answer).
- **Scenario.** A user who set an `ask` or `force_ask` rule for the spawn tool may have it skipped when Downshift answers with an
  explicit allow. Matchers limit this to the spawn tool (`Agent|Task`, `Task`, `invoke_subagent`), and the subagent's own tool
  calls still go through the harness permission flow, so the impact is bounded to "a spawn the user wanted to confirm is not
  confirmed". Claude Code requires `allow` to accept `updatedInput`, so the rewrite path cannot simply drop it. Cursor does not
  enforce `ask` on `preToolUse` (`vendor-hook-docs-2026-10-11.md`). Antigravity's precedence between hook `allow` and `force_ask`
  is not documented.
- **Confidence.** Plausible (harness precedence not verified live).
- **Fix.** Antigravity and Cursor: answer without a decision when nothing is rewritten, if the protocol allows it; otherwise
  document the precedence after a live probe. Add a line to `docs/reference/harness-matrix.md` per harness: "rewrite path
  bypasses ask rules: yes/no".

#### F-08 `models check` and `models pull`: key forwarded on redirect, unbounded body, unsanitised ids

- **Where.** `internal/models/check.go:36`, `:87` to `:134` (default `http.Client{}` with only a timeout; `json.NewDecoder(resp.Body)`
  with no limit), `check.go:66` and `pull.go:56` to `:65` (ids printed and stored as received).
- **Scenario.** `internal/discovery` refuses redirects ("Go would forward x-api-key") but this code path does not. Go strips
  `Authorization` on a cross-host redirect, so the OpenAI and xAI keys are safe, but the Anthropic key travels in `x-api-key`
  and is forwarded. It needs `api.anthropic.com` to answer with a redirect, so it is an upstream or TLS compromise. A hostile
  body is read without a cap (memory), and model ids with terminal escape sequences are printed to the user's terminal and
  stored in the user catalog as `tier=unknown`.
- **Confidence.** Plausible (needs a hostile upstream); the inconsistency is confirmed.
- **Fix.** Reuse the discovery client (no redirects, 10 s timeout), wrap the body in `io.LimitReader(resp.Body, 4<<20)`, and
  accept only ids matching `^[A-Za-z0-9][A-Za-z0-9._:/+-]{0,127}$` (the pattern the Cursor source already uses). Test:
  `httptest` server that redirects and asserts the second host receives no key.

#### F-09 Discovery: environment inherited by listing CLIs, PATH-resolved binaries, unvalidated ids

- **Where.** `internal/discovery/sources.go:263` to `:275` (`runCLI` sets no `cmd.Env`), `internal/discovery/cursor.go:29` to `:45`,
  `sources.go:92` to `:101`, `:133` to `:142`; ids unvalidated in `sources.go:66`, `:115`, `:162`, `:231`.
- **Scenario.** `kiro-cli`, `grok`, `cursor-agent` and `agent` run with the full environment, including `ANTHROPIC_API_KEY`,
  `OPENAI_API_KEY` and `XAI_API_KEY`, although `grok models` "lists without authentication". When `cursor-agent` is absent,
  `agent models --help` is executed on whichever `agent` binary is first on `PATH` to prove its identity: the identity probe
  runs the unknown program. PATH hijack needs a writable directory early on `PATH` (already code execution for that user). Output
  is capped at 4 MiB and stderr is discarded, so no argument injection or output flood exists, and arguments are constants.
  Discovered ids are only used when the catalog has a tier for them (`order.go`: unranked ids are never targets), but a
  family match lets an id with an arbitrary suffix reach `ordered`.
- **Confidence.** Confirmed (environment inheritance); plausible (the rest).
- **Fix.** `cmd.Env = []string{"PATH="+..., "HOME="+...}` plus only the variables the specific CLI needs (none for `grok`);
  refuse to run `agent` unless it resolves to an absolute path outside the working directory and a user-writable temp dir;
  apply the Cursor id pattern to all sources.

#### F-10 Score index accepts any finite score

- **Where.** `internal/scoreindex/sources.go:239`, `:301` to `:309`; read side `internal/scoreindex/hook.go:61` to `:65`.
- **Scenario.** Transport is sound: fixed HTTPS URLs (no environment override exists), body caps of 4 MiB and 8 MiB (applied
  after transparent gzip decoding), 10 s timeout per source, same-host same-scheme redirects, at most 3 redirects, empty or
  unmapped answers never replace the cache, rows only for ids in the embedded map, `scores.json` written 0600 atomically.
  A compromised upstream could still serve any finite number for a mapped model (for example a very high score for a small
  model), which the router would compare. Impact is limited: capability routing is shadow-only unless
  `DOWNSHIFT_SCORE_ROUTING=1`, and `scoreSelect` never goes below the catalog tier (`score_select.go:108`).
- **Confidence.** Plausible.
- **Fix.** Reject rows outside a per-source plausible range (Epoch ECI 50 to 250, AA index 0 to 100) and a source whose
  average moved more than 3x since the cached copy, keep the old rows and record `error: implausible` in `Status`.

#### F-11 Setup: binary path written as found, plugin tracks the default branch

- **Where.** `cmd/downshift/setup.go:92` to `:97` (`os.Executable()` stored as the hook command), `setup_merge.go:15` to `:21`
  (`quoteCommand` quotes only for space or tab), `setup_plugin.go:18` and `:82` to `:87` (`claude plugin marketplace add
  tiagovilasboas/downshift` with no ref), `hooks/hooks.json` (bare `downshift` resolved through `PATH`).
- **Scenario.** Running `setup` from a build directory, a download folder or a temp directory writes that path into
  `~/.claude/settings.json`; if the directory is writable by others or later cleaned, the hook either breaks or executes a
  replaced binary on every prompt. A path with `"`, `$` or a backtick is not escaped inside the hook string. The plugin path is
  safer on one point (setup checks that the login shell resolves `downshift` to this binary) but the check is made once; the
  marketplace follows the repository's default branch, so every user receives whatever `main` holds at the next plugin update.
  `claude plugin install` arguments are constants, so no argument injection exists.
- **Confidence.** Confirmed (no validation); exploitation plausible.
- **Fix.** Refuse (or require `--yes-unsafe-path`) when the binary is under the system temp directory, a Go build cache or a
  group/world-writable directory; escape `"`, `$`, backtick and `\` in `quoteCommand`; pin the marketplace to a release tag
  if `claude plugin marketplace add` accepts a ref, and document that update channel.

#### F-12 `downshift-dock` resolves its targets by command name only

- **Where.** `mods/downshift-dock/hooks/register.tsx:32` to `:34`, `:53`.
- **Scenario.** The two buttons run `filetree` and `agents-info open` through `$.command.run`. Both strings are constants, so
  no untrusted text reaches a command. `detect()` accepts any command with that name whose source is `plugin`, so a different
  plugin that registers `/filetree` receives the press (name squatting; needs the user to have installed that plugin).
- **Confidence.** Plausible.
- **Fix.** If the mod API exposes the owning plugin id on `CommandInfo`, match `filetree@downshift` and
  `savvy-progress@downshift`; otherwise note the limit in the mod README.

#### F-13 `eval-outcome` executes model-written code with the full environment

- **Where.** `internal/outcome/outcome.go:92` to `:95` (`go test` on a recorded solution with `os.Environ()`),
  `:144` (`sh -c` of the `--solver` string); `cmd/downshift/outcome.go:68` to `:70`.
- **Scenario.** `--record` and `--verify` compile and run Go code that a model produced (or that sits in a committed
  `runs/*.go.txt`), unsandboxed, with API keys in the environment. It is a maintainer tool, refused when `CI` is set for
  `--record`, and `--verify` runs only recorded files, but a poisoned recorded run, or a prompt-injected model answer, runs as the maintainer.
- **Confidence.** Confirmed (by design); exploitation plausible.
- **Fix.** Run the check with `cmd.Env` limited to `PATH`, `HOME` set to the temp directory, `GOFLAGS=-mod=mod`, `GOPROXY=off`
  and no network where the OS allows; document that `--verify` should run in a throwaway container.

### INFO

- **I-01 Router mod trace.** `mods/downshift-router/hooks/register.ts:214` to `:233`: `DOWNSHIFT_MOD_ROUTER_LOG` names the file,
  written by read-modify-write (races, quadratic), default permissions. Content is metadata (ids, models, elapsed time, one stderr
  line), no prompt. The binary is started with an argument vector (`:271`); the prompt goes to the child's stdin only. Off by default.
- **I-02 Cursor quota bridge.** `internal/cursorbridge/extension/extension.cjs:50` to `:57`: the output path can be overridden
  with `DOWNSHIFT_CURSOR_NATIVE_FILE` (environment of the editor), written 0600 with an exclusive random temp name and rename; the
  payload is only what `decoder.cjs` extracts (percentages, cycle end, model ids matching a strict pattern), responses over 1 MiB
  are refused, wire types are whitelisted. The toast shows the full path. `package.py` packages an explicit file list.
  `scripts/collect-antigravity-evidence.py` opens SQLite read-only, bounds blobs (2 MiB) and rows (256), and prints digests, no
  paths or prompts. A hostile database can only make it fail.
- **I-03 Untrusted repository as input.** `.downshift/policy.json` is read from the project (`internal/policyfile`, regular
  files only, 64 KiB, strict schema). It can only raise the tier, so a cloned repository can raise a user's spend, not lower safety.
  `internal/codeintel/discover.go:91` runs `git ls-files` in the repository with the inherited environment and the repository's own
  `.git/config`; Claude Code already runs git there. `SafeExecGit` (`security.go:427`) and `hookctx.RunCommand` (`hookctx.go:76`)
  have no callers: remove them or wire the former into `discover.go` with `GIT_CONFIG_NOSYSTEM=1`, `-c core.fsmonitor=false`.
- **I-04 Unbounded stdin.** `cmd/downshift/context.go:414` reads `os.Stdin` without a limit (user-invoked filter, memory only).
- **I-05 `probe`.** `cmd/downshift/probe.go:41`: a maintainer hook that keeps short raw values of `model`, `subagent_type`,
  `tool_name` and `source` (up to 80 characters). No prompts or paths by design; remove it from shipped hook tables (it is).
- **I-06 Kiro wrapper.** `scripts/kirocrew-pre-tool-use.sh` execs `$(dirname "$0")/../downshift`; when invoked through a symlink,
  `$0` is the link, so it resolves the wrong directory and fails closed (no spawn blocked: the runner fails open).

## Telemetry and privacy re-check

Method: read every writer under `internal/telemetry`, `internal/routingv2/training`, `internal/codeintel/telemetry.go`,
`cmd/dsmon-hook`, `cmd/downshift/probe.go`, `internal/sensor`, `internal/adapt`, `internal/quota` and every `fmt.Fprint*(os.Stderr`
in `cmd/downshift/hookrun.go`.

- **Task text and prompts.** `telemetry.Event` has no prompt field (its fields are ids, enums, counts). `loop-events.jsonl` stores
  `FeatureVector` floats only (`internal/routingv2/domain/types.go:14`); the shadow classifier stores a hash of the candidate
  weights, not a path (`shadow.go:160`). `agents.jsonl` writes `task: ""` (`cmd/dsmon-hook/main.go:120`).
- **Free-text fields are bounded.** `requested_model_raw` passes `^[A-Za-z0-9._:/-]{1,64}$` or becomes `invalid`
  (`telemetry.go:300`); session ids are hashed after a vocabulary check; effort is an enum; agent type is reduced to a built-in
  name, `custom` or `unspecified`. The pattern admits `/`, so a path-like string up to 64 characters could be kept if a harness sent
  one as a model (INFO; no such harness known).
- **Keys.** Provider keys are read in `internal/models` and `internal/discovery`, placed in a request header, and never formatted
  into an error (`HTTP %d from %s` prints the URL only). No telemetry or log call receives them.
- **stderr.** Hook messages carry the correlation id, model ids, enums and fixed text; JSON errors print the Go decoder's one-line
  message and not the payload.
- **File modes created by the code (verified by reading; the first four also by running with an empty home).**

| File or directory | Mode | Where |
|---|---|---|
| `events.jsonl`, `agents.jsonl`, `loop-events.jsonl`, `dci-events.jsonl`, `probe.jsonl`, `compaction*` | 0600 (existing files tightened) | `telemetry.go:597`, `dsmon-hook/main.go:144`, `training/events.go:135`, `codeintel/telemetry.go:98`, `probe.go:76`, `sensor/compaction.go:113` |
| `scores.json`, `discovered.json`, `catalog.json` (pull), `weights.json`, `quota` snapshots, `adapt-memory.json` | 0600 via temp and rename | `scoreindex/cache.go:127`, `discovery/store.go:78`, `models/pull.go:141`, `weights.go:127`, `quota.go:127`, `adapt/persist.go:75` |
| State directory when created by telemetry, discovery, scores, quota, probe | 0700 | same files |
| **State directory when created by `context enable/disable`, `install.sh`** | **0755** | F-03 |
| **`context-optimization.json`** | **0644** | F-03 |
| Setup backups and the edited harness file | original file's mode (0644 for new files and markers) | `setup_write.go:24`, `:34` |
| `outcome` runs (`run.json`, solutions) | 0644 in a repository directory | maintainer tool, not user data |

## What was not reviewed

- The classifier, risk vocabulary and routing logic in `internal/core`, `internal/classify`, `internal/routingv2` (covered by M5
  and the benchmark evidence, not re-read here), and the compressor behind `claude-code-post-tool-use` (it can rewrite tool
  output seen by the model; its integrity and prompt-injection properties were not examined).
- `internal/codeintel` (DCI) beyond the symlink and git paths named above; the DCI prompt-injection defanging
  (`security.go:490`) was read but not tested.
- The `opencode` adapter and plugin (`internal/adapters/opencode`, `mods/downshift-opencode`), the Codex lifecycle observer, and
  `orchestration/` (Python/LangGraph planner and `uv.lock` contents).
- Vendored third-party mods (`mods/filetree`, `mods/savvy-progress`, `hooks/cache-tax`): grepped for process and network use only.
  `filetree` runs `sh -c` with user-configured "open with" commands and `find`/`du`/`rm`; its input handling was not audited.
- GitHub repository settings: branch and tag protection, required reviews, Actions permissions, secret scanning, environment
  rules. Release artifacts actually published (signatures, attestations) were not downloaded or verified.
- Runtime behaviour of the harnesses (whether Antigravity or Cursor honour `allow` over a user `ask` rule), Windows paths,
  and the `docs/` site content. No dynamic test, fuzzing or dependency scan was run (the module has no third-party Go
  dependencies; the Python lock file was not audited).
- The first review's own notes: only the hand-over summary was available.

## Suggested order

1. F-01 (installer verification and version validation): one script, one test file.
2. F-02 and F-03 together: one atomic-write and state-directory helper in `internal/paths`, reused by setup, registry and install.sh.
3. F-08 and F-09: share one HTTP client and one id validator across `models` and `discovery`.
4. F-04, F-05, F-06, F-07, F-10, F-11 as small independent changes; F-12, F-13 and the INFO items as backlog.
