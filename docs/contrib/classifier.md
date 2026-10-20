# Classifier (v1 production path)

Contributor reference for the deterministic classifier that powers **hook routing**.
User-facing Portuguese overview: [pt/02-classificador.md](../pt/02-classificador.md).

Experimental v2 is documented in [capability-router-v2.md](../design/capability-router-v2.md).

---

## Complexity classes

| Class | Typical work |
|-------|----------------|
| **TRIVIAL** | Rename, format, typo, trivial git |
| **SIMPLE** | Single-file or isolated change |
| **MEDIUM** | Multi-file feature work |
| **COMPLEX** | Architecture, concurrency, infra, security review depth |

If signals are ambiguous, policy may leave the session model unchanged (fail-open
toward **no rewrite** when confidence is low).

---

The full weight table, the structural bonuses, and how each harness turns
the tier into a model id are in [complexity-weights.md](../reference/complexity-weights.md).

## Scoring (`internal/classify/signals.go` + `internal/classify/classifier.go`)

1. **Weighted regex vote** — each pattern adds to a complexity bucket; highest sum wins.
2. **Tie-break** — adjacent ties resolve toward the **higher** capability tier (never
   under-route on a tie).
3. **Confidence margin** — a win needs at least **2 points** over the runner-up, or
   the classifier is treated as low-confidence (conservative routing).
4. **Semantic boost (in-process hash)** — policy in `internal/fuse` (the embedder is `internal/semantic`; `core.ClassifyWithSemantic` is a wrapper): nearest centroid can **raise**
   complexity when regex is hesitant; it **never lowers** a class. It is ignored for no-signal prompts and cannot create COMPLEX without regex evidence. A lift is recorded as `SEMANTIC_BOOST`. `DOWNSHIFT_MINILM=0` disables. Graphify and external MiniLM commands were removed on 2026-10-17.
6. **Language guard** — `internal/classify/lang_guard.go`: unsupported or mixed-helper-brief foreign cues set
   `UnsupportedLanguage` and clear confidence (M3 mixed bucket).
7. **Risk floor and abstention** — `internal/classify/risk.go`: Trivial/Simple + risk → Medium with `RiskFloor`.
   Confident Medium/Simple + risk on bounded delegation briefs or narrow high-stakes patterns
   → `RiskAbstain` (confidence cleared, tier unchanged); counts as `deterministicLift` for R7/R9.

Multilingual E5 (shadow and fuse) was removed on 2026-10-17. Logs written before that date may still contain `E5_BOOST`, `E5_ABSTAIN`, or `e5_fuse`; new decisions do not.

---

## Go-native bilingual domain enrichment (enabled by default)

Additional engineering vocabulary is enabled by default and lives in
**`internal/classify/domain_signals.go`**, separate from the original
baseline rules in `signals.go`. To restore baseline routing, set
`DOWNSHIFT_DOMAIN_SIGNALS=0`.

```sh
downshift try "Investigue o hydration mismatch no React"
go test ./internal/classify ./internal/core
DOWNSHIFT_DOMAIN_SIGNALS=0 downshift try "Investigue o hydration mismatch no React"
```

The Go-native vocabulary now contains **96 rules across 12 domains**:
frontend, backend, fullstack, devops, finops, devsecops, mobile, CI/CD,
testing/QA, data engineering, AI/agents, and observability. The original
32-rule table remains in `domain_signals.go`; the 64 incremental rules
live in `domain_signals_extended.go`. Each rule requires **an action
plus bounded technical context**. A framework
name (React, Kubernetes, SwiftUI, etc.) alone never raises the tier.
The table does not hardcode model IDs or change risk-floor policy.

**Bilingual parity is a contribution requirement**: each added or modified
rule must have an **English, Brazilian Portuguese and mixed-language** example
in `domain_signals_test.go` or `domain_signals_extended_test.go`.
The combined test set has **288 lexical examples** (96 from the original
32 rules, plus 192 from the 64 new rules), checks each rule against
English/PT-BR/mixed prompts, and checks that these scoped examples are
not classified below medium. This does **not**
prove global equal routing outcomes for arbitrary EN/PT prompts: continued calibration
requires reviewed holdout corpus evaluation, including over-routing, risk
false negatives, confidence, and latency.

Reference vocabularies used for manual curation (do not bulk-copy rules):

- [GitHub Linguist](https://github.com/github-linguist/linguist):
  language names and aliases (e.g. TypeScript / TSX, SwiftUI)
- [OWASP ASVS](https://github.com/OWASP/ASVS):
  authorization, access-control and software security terminology
- [FinOps FOCUS glossary](https://focus.finops.org/docs/specification/v1-2/glossary/):
  amortized cost, effective cost, chargeback and allocation terms
- [Stack Exchange tag synonyms](https://api.stackexchange.com/docs/synonyms-by-tags):
  terminology discovery; verify context and PT-BR translations independently

### Bilingual dictionaries and bounded evidence

The Go tables store rule identity, domain, weight, complexity, and an action
family reference; synonyms and technical target regex fragments live in two
embedded dictionaries:

- `internal/classify/lexicon/en.json`
- `internal/classify/lexicon/pt-BR.json`

The JSON schema keeps the original `order` of every regex alternative,
allowing compilation into the original Go/RE2 patterns without losing aliases.
`domain_lexicon.go` validates both dictionaries once at startup; no JSON,
network, or LLM work runs during per-prompt classification. Every new rule
must have EN/PT-BR/mixed-language positive examples and negative controls.

**Bounded domain evidence** (default on) limits correlated regex hits in the
same specialty and complexity class to `peak weight + 1 corroboration point`.
The original general signals and risk floors remain unchanged. This is an
interpretable, deterministic heuristic, not a learned probability estimate.
Set `DOWNSHIFT_DOMAIN_DEDUP=0` to compare the old additive scoring against
the grouped candidate without disabling the bilingual dictionary.

This change is not a substitute for independent real-task benchmarking.
Review cost, task success, tier over-routing, bilingual parity and p95
latency before tuning weights based on telemetry.

Calibration is **reviewed evaluation**, not LLM fine-tuning: propose candidate
regex/weights from labeled misroutes, test against EN/PT/mixed positives and
negative controls, then promote only after independent holdout evidence.
Prefer a small number of reviewed domain changes per iteration, measure
classifier latency and over-routing in the corpus, and avoid adding
generic framework-name-only or documentation-keyword signals. The current
96-rule expansion is a deliberate coverage checkpoint, **not a target to
maximize regex count**. Avoid changing multiple unrelated weights in one PR. Neither externally
sourced text nor model output becomes a rule automatically.

---

## Tuning workflow

1. Reproduce with `downshift try "<prompt>" [harness]`.
2. Open a `misroute:` issue (see [CONTRIBUTING.md](../../CONTRIBUTING.md)).
3. Add a case to `internal/core/classifier_edge_test.go` (or related edge tests).
4. Adjust `RawSignals` in `internal/classify/signals.go`; never tune against maintainer eval-only splits (downshift-labs).

---

## Shadow / v2 (observation only)

`DOWNSHIFT_SHADOW_WEIGHTS` records a v2 candidate next to the production decision.
It does **not** change hook JSON. Labels for supervised training require explicit
`--required-tier` on feedback. See [classifier-shadow.md](../design/classifier-shadow.md).

---

## Tests and benchmarks

| Asset | Role |
|-------|------|
| `internal/core/classifier_edge_test.go` | Regression from misroute reports |
| `benchmark/seed.json` | Train/tune signals (not in same PR as signal edits + fresh) |
| Eval-only splits (labs) | Never use to edit signals in the same change set |
