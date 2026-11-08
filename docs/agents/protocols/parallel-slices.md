# Parallel Slices Protocol

How several agents build one feature at the same time without colliding.

Use it when a feature splits into parts that live in separate files, such as a
page made of components (`Header.tsx`, `Hero.tsx`, `SwapWidget.tsx`,
`Footer.tsx`) or an API made of independent handlers. Do not use it for a
change that only works when every part lands together, or for one small file.

## Roles

| Step | Agent | Writes |
|------|-------|--------|
| 1. Slice plan | `architect` | the plan only |
| 2. Slices | one `coder` per slice | only the files it owns |
| 3. Slice check | `tester` | nothing |
| 4. Assembly | the lead `coder` (or the orchestrator) | only the shared files |
| 5. Assembled check | `tester` | nothing |

## 1. Slice plan

Before any code, `architect` writes the plan. A slice is ready only when every
field is filled:

```yaml
slices:
  shared:                       # assembly only; no slice writes these
    files: ["app/page.tsx", "app/layout.tsx"]
  contracts:                    # fixed before the wave starts
    - "Hero props: { title: string, cta: { label: string, href: string } }"
    - "Design tokens: styles/tokens.css (read-only for every slice)"
  waves:
    - - id: header-hero
        owner: coder
        files: ["components/Header.tsx", "components/Hero.tsx"]
        tests: ["components/Hero.test.tsx"]
        done: "renders title and CTA from props; test passes"
      - id: swap-widget
        owner: coder
        files: ["components/SwapWidget.tsx"]
        tests: ["components/SwapWidget.test.tsx"]
        done: "converts with the rate prop; rejects negative input"
```

Rules for the plan:

1. **Disjoint files.** No file appears in two slices, and no slice owns a
   shared file. Two slices that need the same file are one slice.
2. **Contracts first.** Props, schemas, tokens and routes are decided in the
   plan. A slice that needs a new contract stops and reports it.
3. **Read-only inputs are named.** Tokens, types and existing helpers a slice
   reads but must not change are listed under `contracts`.
4. **Waves scale with the work.** A wave holds every ready independent slice,
   up to the cap: `min(ready independent slices, cap)`. The default cap is 6.
   Precedence: user, then harness, then policy
   (`orchestration.max_parallel`), then the default. The effective value is in
   the Downshift MCP `playbook` (`max_parallel`). While `status` reports a
   provider rate limited or short of headroom, use `when_strained`: the cap
   halved, minimum 1. A slice that depends on another slice's output goes in a
   later wave; 1-2 point parts are done inline, not sliced.

## 2. Slices

Each `coder` receives exactly one slice: its `files`, `tests`, the
`contracts` and the `done` line. It does not see or edit the other slices.

- Write only the owned files and their tests. Needing any other file is a
  blocker, reported with the handoff format, not a quick edit.
- Do not touch the shared files, even to wire the slice in.
- Run the slice's own tests before reporting.

## 3. Slice check

When a wave ends, `tester` runs each slice's tests and checks the boundary:

```bash
git diff --name-only <base>   # every changed file must belong to one slice
```

A file changed outside the plan fails the wave. Report it; do not repair it.

## 4. Assembly

Only after every slice in the last wave passes, the lead edits the shared
files: imports, composition, routing. Assembly adds no logic that belongs in
a slice. If a slice does not fit its contract, send it back to its `coder`.

## 5. Assembled check

`tester` runs the full gate on the assembled result: the repository's test
command, then the end-to-end or browser suite when the project has one
(for example `downshift compactor -- playwright test`). For a visual
feature, also render the page and compare it with the plan's `done` lines.

## Handoff

Each slice reports with the [handoff protocol](handoff.md), adding the slice
id:

```yaml
handoff:
  from: coder
  to: tester
  reason: complete
  slice: swap-widget
  artifacts:
    - path: "components/SwapWidget.tsx"
      status: ready_for_review
    - path: "components/SwapWidget.test.tsx"
      status: complete
```

## Failure modes

| Symptom | Cause | Fix |
|---------|-------|-----|
| Two agents edited the same file | Plan had overlapping `files` | Merge the slices; replan |
| Slices do not fit at assembly | Contract left vague | Fix the contract; resend the slice |
| Wave stalls on one slice | Hidden dependency | Move it to the next wave |
| Assembly grows logic | Slice boundary wrong | Move the logic into a slice |

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
