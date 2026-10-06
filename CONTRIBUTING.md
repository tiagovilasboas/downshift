# Contributing to Downshift

Thanks for helping make agent harnesses cheaper and more predictable. This
project is small and focused on purpose — contributions that keep it that way
are the most valuable.

Repository: https://github.com/tiagovilasboas/downshift

## The most useful contribution: a misrouted prompt

Downshift classifies a task's complexity with deterministic rules.
Rules are only as good as the prompts they've seen. If you find a prompt it gets
wrong, that's gold.

Open an issue titled `misroute: <short description>` with:

1. **The prompt** (redact anything private).
2. **What `downshift` returned:**
   ```bash
   downshift try "<the prompt>"
   ```
3. **What you expected** — the complexity class and why.

Accepted misroutes become test cases in `classifier_edge_test.go`, then the
signals are tuned until they pass. Your prompt becomes a permanent regression
guard.

Never tune against maintainer eval-only splits (downshift-labs `eval/`).
Don't copy its prompts into tests or the seed, and don't change it in the same
PR as signals (see `benchmark/README.md`; CI enforces this).

## Project layout

```
cmd/downshift/                 # binary: hook mode + try + models subcommands
internal/core/                 # harness-agnostic brain
  classifier.go                #   task text → complexity (scored, deterministic)
  signals.go                   #   RawSignals table (exported, tunable)
  escalation.go                #   EscalationIntent (Trivial/Normal/Review/Preserved)
  models.go                    #   Tier, Effort types (no model ID strings)
  policy.go                    #   complexity → tier → Decision (Route)
  capabilities.go              #   HarnessCapabilities per harness; Plan()
  resolver.go                  #   Resolver interface (catalog implements this)
internal/catalog/              # model data (IDs, costs, effort maps, routing flags)
  catalog.go                   #   Load(), LookupByID (exact→alias→family), EffortFor
  policy.go                    #   mergeEntries + validateEntries
  catalog.json                 #   embedded default (committed + go:embed'd)
internal/adapters/<harness>/   # one adapter per harness
  claudecode/                  #   PreToolUse + updatedInput for Claude Code
  cursor/                      #   preToolUse + updated_input for Cursor
  codex/                       #   PreToolUse + updatedInput + reasoning_effort for Codex
internal/hookutil/             # shared utilities (StringField)
internal/routingv2/shadow/     # opt-in candidate observation; does not change hooks
internal/routingv2/training/   # local prompt-free loop events, shadow-report, offline learning
internal/models/               # models subcommands (list, check, pull)
```

**Dependency rule:** `core` never imports `catalog` or any adapter.
Direction: `catalog → core`, `adapters → core + hookutil`, `models → catalog + core`.

## Adding a new harness adapter

Use `internal/adapters/claudecode/` as the template. An adapter:

1. Decodes the harness's subagent-spawn event.
2. Extracts the subagent's task text and current model ID.
3. Calls `core.Route(prompt, harnessID, currentModelID, resolver)`.
4. Exposes `Event.TaskText()` so the shared runner can derive local features;
   never persist the raw task text.
5. Translates the returned `core.Decision` into the harness's mechanism.
6. **Fails open** — any error returns an allow decision and exits 0.

The shared hook runner records an opaque review ID for every real routing
decision. Do not add harness-specific training or completion behavior. Manual
reviews use `downshift feedback`; only explicit `--required-tier` labels are
training targets. `success` alone is not a minimum-tier label. Observe a
candidate with `DOWNSHIFT_SHADOW_WEIGHTS` and `downshift shadow-report` before
any manual activation. See `docs/CLASSIFIER-SHADOW.md`.

Then:
- Add catalog entries for the new harness in `internal/catalog/catalog.json`
  (no Go changes needed — it's just JSON data with `id`, `family`, `tier`,
  `effort_map`, and cost fields).
- Wire a new subcommand in `cmd/downshift/main.go`.
- Add table-driven tests that inject `catalog.Load()` as the resolver.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full step-by-step guide.

## Updating model data

Model IDs, costs, and effort mappings live in `internal/catalog/catalog.json`.
To add or update a model:

1. Edit `catalog.json` directly — no Go changes needed.
2. Add a `family` field (the stable prefix for version-agnostic matching).
3. Add `"routing": "explicit_only"` if the model should never be an automatic
   routing target (e.g. reserved top-tier models like `gpt-6-astra`).
4. Use `{ "_section": "your label" }` objects as human-readable section
   headers — the parser silently skips any entry where `id` or `harness` is
   empty, so these are safe to include.
5. Open a PR with a source link (provider docs or pricing page).

Or run `downshift models pull` with the provider's API key to discover new
models automatically (they arrive with `tier: "unknown"` for you to assign).

## Development

```bash
go build ./...                  # compile
go test -race -cover ./...      # run tests
go vet ./...                    # vet
goreleaser check                # validate release config (needs goreleaser installed)
```

Requires Go 1.27+.

### CI tiers

| Workflow | When | What |
|----------|------|------|
| `ci` | Every PR / push to `main` / tags | Fast path: `go test`, benchmark **gate**, smoke; outcome eval only when routing/outcome paths change |
| `ci-full` | Weekdays 09:00 UTC + manual dispatch | Race + cover, govulncheck, all benchmark splits, full outcome verify |

Docs-only diffs skip the Go job. Nightly `ci-full` catches regressions if a PR did not touch scoped paths.

## Ground rules

- **Keep tests green.** Run `go test ./...` before every commit; run `go test -race -cover ./...` before router/core changes (or wait for `ci-full`).
- **Every classifier change ships a test.** Add a table-driven case in
  `classifier_edge_test.go` that proves the improvement.
- **Small, focused PRs.** One concern per PR. No "also fixed X" bundling.
- **Conventional commits in English.** `feat(core): ...`, `fix(claudecode): ...`, `docs: ...`
- **Fail-open, always.** A cost optimizer must never block a subagent spawn.
- **Honest in docs.** Don't oversell what a heuristic can do. State the blast radius.

## Commits and attribution

**Humans only in commit history.**

Do not add AI agents (Claude, Cursor, Codex, etc.) as co-authors via the
`Co-Authored-By` trailer. Permanent commit history should reflect human
contributors; automated attribution belongs in CI logs and PR descriptions.

✅ **Allowed:**
```
Co-Authored-By: Alice Engineer <alice@company.com>
Co-Authored-By: Bob Developer <bob@company.com>
Author: Tiago Vilas Boas <tcarvalhovb@gmail.com>
```

❌ **Not allowed:**
```
Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
Co-Authored-By: Cursor <cursoragent@cursor.com>
Author: Claude <noreply@anthropic.com>
```

Before committing, check your message for an agent `Co-Authored-By` line, and make
sure your git author is your own identity. If your editor or agent tool appends such
a trailer by default, remove it. A `commit-msg` hook that rejects agent trailers is
a one-file addition if you want it locally.

AI-assisted contributions are welcome. You are responsible for what you submit:
review it, run `go test ./...`, and say in the PR description if a tool wrote
significant parts. Prose in the PR is enough; no trailer is needed.

## Code style

- Idiomatic Go. `gofmt` before committing.
- Comments explain *why*, not *what*. The code says what.
- No new runtime dependencies — the single-binary, zero-runtime promise is a feature.

## Contributor licence

By submitting a contribution, you agree that your contribution is licensed
under the [Apache License, Version 2.0](LICENSE), and you have the right to
license it.

The maintainer will credit significant contributors in project documentation.

## Licence

Downshift is published under the [Apache License, Version 2.0](LICENSE).
Prior BUSL text is archived in [LICENSE-BSL-1.1-ARCHIVE.md](LICENSE-BSL-1.1-ARCHIVE.md).
