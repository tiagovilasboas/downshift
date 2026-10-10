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
| Some signal already matched and the prompt has at least 80 words | +1 COMPLEX |

A TRIVIAL or SIMPLE task that touches risk (secret, auth, crypto, payment,
vulnerability, PII, destructive command, or prompt injection) is lifted to
MEDIUM. MEDIUM and COMPLEX are not lifted further by risk. The risk table is
`internal/core/risk.go`. It sets a floor. It does not add to the vote.

Optional later steps can only raise the class: graph context when the
classifier is not confident, and the local MiniLM boost. Neither lowers a
class. Both can be turned off (`DOWNSHIFT_GRAPHIFY=0`, `DOWNSHIFT_MINILM=0`).

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
| Codex | Writes the model id and `reasoning_effort`. |
| Cursor | Writes `updated_input.model`. Free and legacy Pro plans discard it. Usage-based plans are not yet proven. |
| Antigravity | Overwrites the subagent model. Observed on desktop 2.21.1 only. |
| KiroCrew | No rewrite channel. Exit 0 allows, exit 2 blocks and asks for the cheaper model. |
| Grok CLI | No hook rewrite. Effort is pinned in `config.toml`. |

Local tier memory (`internal/adapt`) runs after this score. It does not add
weight. It can move the tier only when a reviewed outcome already recorded a
hit or a miss for the same task shape. No memory file means the table above
is the whole decision.
