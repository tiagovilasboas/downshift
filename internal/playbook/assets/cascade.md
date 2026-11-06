# Cascade

- Shape: try the task on tier small (effort low) first, run a verification command, and escalate to tier mid (effort medium) only when verification fails. Escalate once; a second failure goes back to the orchestrator, not to a bigger tier.
- Verification must be a command with an exit code (a test, a type check, a build), chosen before the first attempt. A worker saying it is done is not verification.
- Escalate with the failed attempt's verification output and nothing else; do not forward the small worker's reasoning.
- Use when: the task is well specified, has a cheap deterministic check, and most such tasks succeed on the small tier.
- Do NOT use when: there is no automatic check, a failed attempt can damage state that is hard to undo (migrations, deletes, external calls), or the task is known to be hard.
- Saves (tokens only): every task the small tier passes never touches the mid tier. A task that escalates costs both attempts, so measure the pass rate on the small tier before relying on it.
