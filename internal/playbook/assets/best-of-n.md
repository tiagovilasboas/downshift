# Best of N

- Shape: n independent attempts at the same task (tier small, effort low, n of 2 or 3), each in its own isolated worktree, then pick the winner by a verification command. Never pick by reading the attempts and judging.
- Fix the verification command first and run it on every attempt. Among the attempts that pass, take the smallest diff. If none passes, report the failures; do not merge parts of several.
- Attempts must not see each other. Give each the same task text and nothing else.
- Use when: the task is cheap per attempt, has a deterministic pass or fail check, and a single attempt often fails (flaky fix, tricky refactor, test that must go green).
- Do NOT use when: there is no automatic check, the attempts would write to the same files outside a worktree, or the task is cheap enough that one mid-tier attempt is safer.
- Saves (tokens only): it saves a retry loop on the mid tier, only while n attempts on the small tier cost fewer tokens than one mid-tier run plus its retries. It spends more tokens than a single attempt, so use it for reliability first.
