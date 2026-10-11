# Interpreting an audit

| Symptom | Likely cause | Check |
|---|---|---|
| `events` "not created yet" | Hook never ran or points at a missing binary | `which downshift`; hooks block; restart the harness |
| Hook runs, spawns keep the parent model | Rewrite not honored on this plan, or no session models | `docs/harness-matrix.md`; `session-models.json` |
| Stats show routing but savings are 0 | Requested model unknown, so the baseline is unrouted | `docs/stats-export.md` |
| Decisions say downshift, harness used another model | A model setting outranks the hook | Verify on the user's harness version before asserting. `downshift doctor` lists any model-override variable a harness declares (for Claude Code, `CLAUDE_CODE_SUBAGENT_MODEL`, reported by a third-party router); Downshift has not verified that it outranks a hook rewrite. |
| Quota commands report "unknown" | No fresh native evidence | `downshift quota status --harness <h>`; evidence must be recent |
| A project floor seems ignored | Invalid `.downshift/policy.json`, or the harness did not send `cwd` and the hook ran outside the project | Validate the file; run the hook from the project directory |
| High fixed context | Large always-loaded rule files or many skills | Section 3 of SKILL.md |

Evidence levels: observed (you ran it), inferred (reasoned from output), unverified (not run or not confirmed).
