# Misroute triage

A misroute is a decision the user would not have made: a child that failed because its
model was too small, or a trivial task that kept the frontier model. Fixing them is how
the deterministic classifier improves; nothing is learned from raw usage.

## Collect, without the prompt leaving the machine

1. `downshift session --why` (or `/downshift --why`): the tier, the reason codes and the
   guardrail that held or moved the spawn.
2. `downshift doctor`: the last hook decision and whether the hook fires at all.
3. `downshift report`: rework and cost per completed task for the period, to check whether
   the misroute repeats. Nothing here creates a tier label.

## Report

Open an issue titled `misroute: <short description>` with the redacted prompt, the
`downshift session --why` output and the expected model, as in `CONTRIBUTING.md`. Accepted misroutes
become test cases in `classifier_edge_test.go`.

If the user runs a session-observer skill (for example `task-observer`), its recorded
corrections are a good source for this list; copy the decision evidence, not the task text.
