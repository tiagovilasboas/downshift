# Scout then Build

- Shape: two waves. Wave one is one scout (tier small, effort low) that maps the code read-only; wave two is one builder (tier mid, effort medium) that implements from the scout's map.
- Scout: list the files to touch, the entry points, the existing tests and the exact test command, each with `path:line`. Write no code and edit nothing. Stop after the map; it is data, not a plan to follow blindly.
- Builder: receive only the scout's map plus the task, never the scout's transcript. Re-read a file before editing it and verify any claim the change depends on.
- Use when: the task is clear but the code is unfamiliar and a build would otherwise spend its context reading.
- Do NOT use when: the change sits in one or two known files, the task is a one-line edit, or the design itself is undecided (that needs a stronger tier, not a scout).
- Saves (tokens only): the builder skips the exploration reads, which the cheaper small tier pays instead. It costs one extra spawn, so it saves nothing on a task the builder could map in a few reads.
