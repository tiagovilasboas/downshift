# Complexity weights and model choice

How Downshift scores a task and turns that score into a model. The weights
are harness-agnostic. The harness only chooses which model id represents the
tier, and whether the rewrite is applied.

Source of truth: `internal/core/signals.go`, `internal/core/classifier.go`,
`internal/core/policy.go`, `internal/core/risk.go`, `internal/core/models.go`.
Session lists: [session-models.md](session-models.md). What each harness does
with the rewrite: [harness-matrix.md](harness-matrix.md).

## Scoring

Every matching pattern adds its weight to one class: TRIVIAL, SIMPLE, MEDIUM,
or COMPLEX. The class with the highest sum wins. A tie goes to the higher
class, so the task is not under-powered.

Confidence is a margin, not a fifth class. The winner is confident only when
it leads the runner-up by **2 or more**. Below that, a downshift or a weak
upshift is not applied. The spawn stays on the model it already had.

No pattern at all scores **MEDIUM**, and that result is not confident.

Extra points, applied after the regex table:

| Condition | Points |
|---|---|
| No regex matched, the prompt is `add Name(` with one call and at most 25 words | +2 SIMPLE |
| The prompt contains `implement` and at least three exported calls `Name(` | +2 COMPLEX |
| Some signal already matched and the prompt has at most 6 words | +1 TRIVIAL |
| Some signal already matched, the prompt has at least 80 words and is not a bounded brief | +1 COMPLEX |
| Bounded brief: at least 80 words and two of three limits (scope limit such as `read-only` or `edit only`, a test command such as `go test`, a reply budget such as `reply <= 150 words`) | TRIVIAL votes dropped; a single COMPLEX row (other than review or retry wording) dropped; +2 MEDIUM |
| Long brief (80+ words) that is not bounded | A TRIVIAL keyword does not vote when it sits inside a backtick command quote, is negated (`do not git commit`, `never push`), is a `git branch` label, or is a path segment such as `mech-rename` |

A TRIVIAL or SIMPLE task that touches risk (secret, auth, crypto, payment,
vulnerability, PII, destructive command, prompt injection, or concurrency) is lifted to
MEDIUM. MEDIUM and COMPLEX are not lifted further by risk. The risk table is
`internal/core/risk.go`. It sets a floor. It does not add to the vote.

**Input cap.** The scoring regex table and the structural scoring rules read at
most the first 32 KB of the prompt (cut on a rune boundary), so cost stays flat
for the 1 MiB the hook accepts. A signal past byte 32 768 is ignored by design.
Risk and language checks inspect the full input, so a risky or unsupported-language
tail cannot bypass them. The word-count rules (6 and 80 words, the `add Name(` stub, the doc-edit damper)
still count words over the full prompt.

An optional later step can only raise the class: the in-process word-hash
semantic boost. It never lowers a class, is ignored for no-signal prompts, and
cannot create COMPLEX without regex evidence. Turn it off with `DOWNSHIFT_MINILM=0`.

## Language coverage

English and Portuguese are covered by the signal and risk rules. Portuguese risk
terms include permission, privilege, security, injection and personal-data terms;
the risk floor is shared by every harness.

The language guard (`internal/core/lang_guard.go`) detects unsupported-language
indications before a confident routing change can be applied. It looks for distinct
Spanish, French or German function words, selected foreign Latin letters, or at
least four non-Latin letters. **Mixed English helper briefs** (`fix|rename|format|move|add this helper;` plus foreign prose) set
`UnsupportedLanguage` so a confident trivial downshift cannot slip through (M3 / H14).
Other mixed-language instructions can trigger the same guard even when an English verb
matches a cheap-task signal. Detection drops confidence without changing the scored
class; uncertain changes preserve the requested model. Existing deterministic safety
floors can still raise an undersized model. A prompt with no matching signal also abstains.

**Risk abstention (H14):** when risk categories match and the class stays Medium (or Simple),
`applyRiskFloor` may set `RiskAbstain` and clear confidence without changing complexity:
long **bounded delegation briefs** (scope + verify cues such as “stay within these modules”
and “run the relevant tests”), and narrow **high-stakes** phrases from the M3 parallel bucket
(for example idempotent payment/refund wording or “Revise as permissões…”). `RiskAbstain`
counts as a deterministic lift for guardrails R7/R9. Generic short “implement … auth” tasks
stay confident Medium when the pattern does not match.

This is a conservative heuristic, not a complete language detector. Short foreign
phrases may escape detection, while identifiers or quoted text may trigger it.
Scoring inspects a rune-aligned prefix of at most 32 KB; risk and language
checks inspect the full input. Do not interpret English and Portuguese
coverage as calibrated accuracy or support for all multilingual prompts.

English and Portuguese coverage is the shipped language contract. Multilingual E5
and external MiniLM were removed on 2026-10-17; they do not extend coverage.
The archived initiative note is [multilingual semantic intelligence](../internal/tasks/2026-10-16-multilingual-semantic-intelligence/README.md).

## Weights

Weight is the integer added when the pattern matches. Patterns are regular
expressions, case-insensitive. Several matches in the same class add up.

### COMPLEX

| Weight | Matches |
|---|---|
| 3 | architect, re-architect, migrate, redesign, security audit/review, code review, cross-system, etcd/raft/consensus, race condition, deadlock, multi-system/service/region/tenant, entire codebase/system/platform, design doc, RFC, service mesh, event sourcing, CQRS, chaos engineering, end-to-end encryption, data lineage, cross-cluster, memory leak, cascading failure, streaming pipeline, model serving, key management, recursive-descent, longest common subsequence, operator precedence, postfix operator, backtracking, cron, `fn func(`, previous attempt / tentativa anterior, compilation failed, subagent failed / tests failed, fix the build |
| 2 | rewrite, distributed, trade-off, think through, why … fail/crash/hang/break/regress, directed graph, binary heap, concurrent/goroutine, a fixed-size grid such as `[9][9]` |

### MEDIUM

| Weight | Matches |
|---|---|
| 3 | zero-downtime / online schema change |
| 2 | refactor, implement, integrate, feature, across N files, debug, revisar/revisão |
| 1 | endpoint, module |

### SIMPLE

| Weight | Matches |
|---|---|
| 3 | rotating cube, three.js scene, hello world, landing page, one file, one function, one component |
| 2 | add field/param/flag/property/column/argument/method/function, fix the bug, write a function/test/helper, explain (only at the start), single file/function |
| 1 | what is / what does / what are |

### TRIVIAL

| Weight | Matches |
|---|---|
| 3 | rename, renomear, format, indent, prettier, eslint, git commit/push/pull/status/add/stash/log/diff/checkout/branch, fix a typo, add a comment, move the file, delete the file, bump the version, listar arquivos |
| 2 | lint, boilerplate, remove unused/dead |

## From class to tier

The tier does not depend on the harness.

| Class | Tier | Effort |
|---|---|---|
| TRIVIAL | small | low |
| SIMPLE | small | low |
| MEDIUM | mid | mid |
| COMPLEX | frontier | high |
| anything else | mid | mid |

Effort is written only on harnesses that accept it (Codex `reasoning_effort`,
Grok config). It is the same mapping everywhere.

## How the harness picks the model

The weights stop at the tier. The model id comes from the operator's session
list for that harness, ordered from least to most capable
(`~/.downshift/session-models.json`). Downshift picks the listed model whose
catalog tier matches the decision. It does not invent an id that is not on
the list, so an expensive frontier model stays out until it is appended.

What changes per harness is the wire format and whether the executor honors
the rewrite, not the score:

| Harness | What the hook can do |
|---|---|
| Claude Code | Writes the family name (`haiku`, `sonnet`, `opus`, `fable`) into `updatedInput.model`. A full id is rejected. |
| Codex | Writes the model id and `reasoning_effort`. A held decision writes neither; an unconfident one changes effort only together with a model rewrite. |
| Cursor | Writes `updated_input.model`. Free and legacy Pro plans discard it. Usage-based plans are not yet proven. |
| Antigravity | Overwrites the subagent model. Observed on desktop 2.21.1 only. |
| KiroCrew | No rewrite channel. Exit 0 allows, exit 2 blocks and asks for the cheaper model. |
| Grok CLI | No hook rewrite. Effort is pinned in `config.toml`. |

Local tier memory (`internal/adapt`) runs after this score. It does not add
weight. It can move the tier only when a reviewed outcome already recorded a
hit or a miss for the same task shape. No memory file means the table above
is the whole decision.
