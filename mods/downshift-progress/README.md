# downshift-progress

Live progress for Claude Code subagents and flows: a band above the prompt and an
agents pane, with Downshift routing figures. Original Downshift code, Apache-2.0.

**Status: in development, not in the marketplace yet.** Today it only registers the
`/downshift-progress` command, which answers `downshift-progress 0.1.0 loaded`.

## Layout

Folders name the domain (screaming architecture). `hooks/register.tsx` is a router
only; every feature has its own folder under `hooks/features/` and shared code under
`hooks/shared/`.

```
hooks/register.tsx          router
hooks/features/{band,agents,flow,status}/
hooks/shared/{data,format}/
```

Subfolders under `hooks/` are imported by `register.tsx`, loaded by the engine and
their `*.test.ts` files are discovered by `claude plugin test` (verified, see the PR
that added this scaffold).

## Develop

```
claude plugin validate mods/downshift-progress
claude plugin test mods/downshift-progress
```

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
