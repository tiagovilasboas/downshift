# Audit at Scale

- Shape: many readers (tier small, effort low), each over a disjoint set of files, then one reviewer (tier mid, effort medium) that merges their findings. Stay within the fan-out cap shown by the `playbook` tool.
- Split by file set before spawning: no file appears in two sets. Give each reader the same checklist and the same finding format (`path:line`, rule, one-line evidence).
- Readers report findings only and never edit. They say "none" for a clean set rather than padding.
- Reviewer: drop duplicates, reject any finding without `path:line` evidence, re-open the cited line for every finding it keeps, and rank by severity.
- Use when: the same check applies across many files or modules (security sweep, API misuse, dead code, naming).
- Do NOT use when: the audit needs whole-system reasoning that spans file sets, the code base is small enough for one reader, or quota status reports strain.
- Saves (tokens only): the bulk of the reading runs on the small tier instead of one large context on a bigger one; the mid tier reads only the findings and the lines they cite.
