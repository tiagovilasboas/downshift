# Surgical roadmap (2026-10-21)

Small, separately mergeable changes, in the order to do them. Each line is one
PR (or one local action), one reason to change, with the check that closes it.
Facts were verified on 2026-10-21 against `origin/main` (1a4e3641) and the
owner's machine; items marked **decision** wait for the owner.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

## A. Today, on this machine (no PR)

| # | Action | Closes when |
|---|--------|-------------|
| A1 | Reinstall the binary from `main` with the version stamp; make `~/.local/bin/downshift` a symlink to `~/go/bin/downshift` so Cursor and Codex hooks run the same file as PATH | `downshift version` shows `main-<commit>`, not `dev`; `downshift doctor` has no ✗ on "hook binary" |
| A2 | Keep the event-log reset documented: the 2026-10-20 history is in `events.jsonl.bak-pre-kirocrew-only-20261020` (380 KB). Decide whether to restore it or start clean | **decision**; `stats --days=0` either shows the history or the README of the state dir says why not |

## B. Install hygiene (one PR each)

| # | Change | Closes when |
|---|--------|-------------|
| B1 | `make install` and `make build` stamp `buildVersion`/`buildCommit`/`buildDate` from `git describe` (today `LDFLAGS=-s -w` only, so every repo build reports `dev`) | a build from the repo never prints `dev` |
| B2 | `doctor`: warn when the hook binary was built from a dirty tree or a commit not on `main` (`go version -m`: `vcs.modified=true`, `vcs.revision`) — extends #379, which already refuses temp and world-writable binaries | `doctor` prints `! built from a dirty tree` for such a binary; green for a main build |
| B3 | `stats`: when `events.jsonl.bak-*` files sit next to the log, print one line naming them and their span, like the existing rotated-backup note | the note appears with the backup above present |

## C. Honest numbers

| # | Change | Closes when |
|---|--------|-------------|
| C1 | Split "held" in `session` and the panel: `held by guardrail` (R5 risk, R1/R8 doubt) vs `upshift not applied`. Today 36 of 58 "held" in one session were upshifts not applied with the child on haiku, which reads as saving | header shows both counts; QA recompute from the log matches |
| C2 | `report`: show the price source for gateway runs (`snapshot` or `unknown`) next to the dollar figure, never a tier price for a gateway id | a gateway run with no snapshot price prints `unknown`, not a number |
| C3 | Decide the rule for a requested `sonnet` that ran on `haiku` with verdict OK (6 cases in the last audit): count as saved or not | **decision**; one table test either way |

## D. OpenRouter and gateways

| # | Change | Closes when |
|---|--------|-------------|
| D1 | Document the `claude-or` profile as the alias workaround it is: `ANTHROPIC_DEFAULT_*_MODEL` maps tiers to gateway ids, Claude Code refuses a raw slug in `updatedInput.model` (schema: `sonnet\|opus\|haiku\|fable`), so recorded names are aliases until the harness changes | `docs/guides/config.md` section + a `doctor` note when `ANTHROPIC_BASE_URL` points at a gateway |
| D2 | Silence the `unrecognized_model` noise in `claude-or` by setting the context-window variable the Claude Code docs name for gateway ids (read the doc first; do not guess the variable) | a `claude-or -p` run prints no `unrecognized_model` line |
| D3 | OpenCode: the plugin must send the requested model to the hook. Without it every OpenCode decision is "requested model unknown, held" and the exact-id path (#361, proven answering) never fires | medium, not small: one PR in the OpenCode plugin, then `downshift try --harness opencode` on a trivial task picks the live small row |
| D4 | OmniRoute: fill `Price` and `ListModels` only when its docs confirm the endpoint and fields (today UNKNOWN, adapter returns `ErrUnsupported`) | blocked on research; no code until confirmed |

## E. Compactor

| # | Change | Closes when |
|---|--------|-------------|
| E1 | Playwright: free text after a `✓` line still compacts (`✓ 1 panic (1.0s)`); tsc: a directory component with a danger word still compacts (`* error/tsconfig.json`). Both low, both still open on main per the last AppSec pass | byte-identical regression tests for both inputs |
| E2 | Family inventory doc: list every family on main (git now has 15, plus the package-manager, PHP, linter, infra and AWS families from #368/#387), which ones compress and which only pass through, in `docs/design/context-optimization.md` | the table matches `FamilyOf` (a test that diffs the doc against the code, or a generated table) |
| E3 | Conformance: confirm `knownViolations` is empty on main after #387's new compressors ran through the suite; if any new family is allowlisted there, fix it instead | `grep knownViolations` shows an empty list or none |

## F. CI

| # | Change | Closes when |
|---|--------|-------------|
| F1 | The scheduled `ci-full` run of 2026-10-20 failed (before #335); #371 opens a tracking issue on failure. Read that issue, fix or close it, and confirm the first nightly after #335 ran `test-os` on windows and macos | one green scheduled run with both `test-os` jobs |
| F2 | Mods have no CI job and the `claude-code/testing` runner is not on this machine, so dock and savvy-progress tests are never executed | **decision**: add a job if a runner exists in CI; otherwise state it in each mod README |

## G. Owner decisions (not small, listed so they stop blocking)

- Economy mode labels: who labels the 974 pending cases (first set: the 19 moved ones).
- DCI hook layering: the shared-runner move changes fail-open and payload limits; accept that or leave the DCI subcommand as is.
- `adapt_eval` still prints USD: keep as an operator tool or retire.
- Site: custom domain and Search Console submission.
- Gateway quality: the bench is synthetic (n=30 per model: DeepSeek 26, MiniMax 25, Haiku 24 at 4000 tokens, inside the noise). Decide what real-task evidence is needed before any default change.
