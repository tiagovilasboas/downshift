# Companions: tools that work next to Downshift

Downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Downshift optimizes delegated execution: before a subagent spawns it picks that
subagent's model and effort, and around that it holds under doubt, applies
floors, records what each child cost and measures routed against control. The
levers it does not own, above all compressing what the agent reads, live in
other tools. This page says which
ones Downshift knows about, what it does with each (detect, record, document,
or nothing), and what it will never do on their behalf.

**Default and optional.** Headroom is the default companion: when its binary
is on PATH, `downshift context enable` records it and `downshift doctor`
reports it. Everything else on this page is optional and documentation only.

**Claude Code first.** Downshift's core, adapters and docs serve every
harness it ships an adapter for (Claude Code, Cursor, Codex, Antigravity,
KiroCrew; Grok by config). Claude Code is where it is most complete and most
verified: rewrite honored with evidence, session model inferred from the
current model, usage and child-end hooks, the `/downshift` command and the
statusline. The companion table below says per harness what each tool
supports; the Downshift side of that table is in
[harness-matrix.md](../reference/harness-matrix.md).

## What Downshift never does for a companion

- It does not install, launch, wrap, unwrap or configure a third-party tool.
  `downshift context enable headroom` writes one field in Downshift's own
  `context-optimization.json`; it runs no `headroom` command and touches no
  harness settings. (An earlier RTK integration ran the companion's own
  `init`; it was removed in #71 and this rule comes from that.)
- It does not bundle third-party skills, rules or agent files in `skills/`.
  Most are single-harness, they add always-loaded context to every session,
  and some carry licences (CC BY) or default telemetry that Downshift does
  not want to ship under its own name. One exception, by owner decision
  2026-10-17: the plugin bundles the [cache-tax mod](../../hooks/cache-tax/README.md)
  (MIT, vendored and pinned, no always-loaded context, no telemetry, no
  network call but its own opt-in warming ping). The NOTICE file carries its
  licence. Mods are the exception category to the never-bundle rule (owner decision
  2026-10-17): cache-tax inside the plugin, [savvy-progress](../../mods/savvy-progress/README.md)
  and [filetree](../../mods/filetree/README.md) as sibling plugins of the marketplace
  (`savvy-progress@downshift`, `filetree@downshift`), each vendored and pinned with its
  licence in NOTICE.
- It does not count a companion's savings as its own. `downshift stats` is
  routing; `headroom savings` is compression. When a companion is recorded,
  `doctor` says so, so a before/after comparison is not attributed to routing
  alone.

## Headroom (default companion)

[Headroom](https://github.com/chopratejas/headroom), Apache 2.0, is a local
proxy that compresses what the agent reads (tool results, logs, files, search
hits, history) before it reaches the model API, keeps the originals for
retrieval, and wraps supported CLIs with one command. It addresses the lever
Downshift's own native compressor only observes: on Claude Code a PostToolUse
hook cannot replace a tool result, a proxy can.

**How the two fit.** Headroom sits between the harness and the API, so the
Downshift hook still sees the original tool output and the original task
text; routing decisions do not change. Downshift's native provider keeps
recording observe-mode compaction counts. Nothing in either tool needs to
know about the other.

**What Downshift does with it** (all read-only against Headroom):

| Command | Effect |
|---|---|
| `downshift context status` | prints `Companion available: headroom vX detected` when the binary is on PATH and not recorded, or `Companion: headroom (recorded as active)` |
| `downshift context enable` | enables the native provider and, if Headroom is detected, records it as companion (prints the wrap command per harness) |
| `downshift context enable headroom` | records the companion only; refuses with the install command when the binary is absent |
| `downshift context disable headroom` | clears the record; `downshift context disable` clears provider and companion |
| `downshift doctor` | a `companion` row: ok when recorded and present; warn with the exact fix when detected but not recorded, or recorded but missing; silent (verbose: ok) when absent |
| `downshift context providers` / `context doctor` | capability and per-harness table below, plus the privacy note |

**Per harness** (Headroom README compatibility table, read 2026-10-17; the
command is Headroom's, run it yourself):

| Harness | Headroom support | Command |
|---|---|---|
| Claude Code | full | `headroom wrap claude` · undo `headroom unwrap claude` |
| Codex | full | `headroom wrap codex` · undo `headroom unwrap codex` |
| Cursor | manual | `headroom wrap cursor` starts the proxy and prints base URLs for Cursor's settings |
| Antigravity | not listed | any OpenAI-compatible client can target `headroom proxy`; not verified here |
| KiroCrew | not listed | not verified here |

**Install** (Headroom's own instructions): `uv tool install --python 3.13
"headroom-ai[all]"` or `pip install "headroom-ai[all]"`. The npm package is
the SDK only and has no CLI.

**Privacy.** Headroom's anonymous beacon is on by default upstream; set
`HEADROOM_BEACON=off` or `DO_NOT_TRACK=1` to turn it off. Downshift sends
nothing and stores no prompt, with or without a companion.

**The portable `headroom-skill`** (a re-implementation of the techniques as
agent rules plus Python helpers, Apache 2.0) works by adding rules to
`CLAUDE.md` or `AGENTS.md` in every project. That is always-loaded context in
every session, the cost the [audit skill](../../skills/downshift-audit/SKILL.md)
measures. Prefer the binary; if you use the skill, keep it in one project and
measure.

## Claude Code mods Downshift recommends

Claude Code **mods** (released 2026-10-01 with Claude Code 2.1.287) are
plugins with a hooks module (`hooks/hooks.json` listing `modules`) whose
functions run inside Claude Code: they can guard a tool call or a prompt,
add a `/command`, and draw a band above the prompt or a pane. Hooks run
wherever the plugin loads, including `claude -p`; drawing works in the
terminal, the Desktop app's Code tab and the VS Code extension, not in
Cursor or Codex. Reference: [Mods overview](https://code.claude.com/docs/en/plugins/mods/overview).
Four are recommended here: two from Anthropic (You should know, Token
Weather), one Anthropic sample that reviews edits (Replay Theater) and one
third-party learning aid (reflect-mod). Three more ship inside the Downshift
plugin itself (see the exception above).

### You should know (built into Claude Code, off by default)

`cc-plugin-you-should-know@builtin` is Anthropic's own mod, shipped inside
Claude Code and off until you turn it on. While Claude works through a long
task, a side agent rereads the conversation from time to time and, when it
finds one thing you might have missed (a trade-off Claude chose without
saying so, a result that may be off), shows a short note above the prompt:

```text
✦ Heads up · There was a change to how folder sharing starts in June, which may have bumped usage metrics artificially.
  1: Learn more   2: Knew this already   0: Dismiss
```

The side agent is a separate request to the session's model with no tools:
it cannot read files, run commands or change what Claude does, and the main
agent never sees a note unless you send it with "Chat in main session".

Turn it on at the Claude Code prompt, then open a new session:

```text
/plugin enable cc-plugin-you-should-know@builtin
```

`/plugin disable cc-plugin-you-should-know@builtin` turns it off. If the
enable command answers that the plugin is not installed, your session does
not list it: it needs a direct connection with a claude.ai account or a
Console key (not an LLM gateway or an `apiKeyHelper`) and the feature flags
a session with `DISABLE_TELEMETRY` does not fetch.

**It spends usage, and Downshift says so.** Each check is a background
request to the session's model: the conversation is read from the prompt
cache on the same terms as Claude's next request, and the mod's own
instructions (several thousand tokens) plus the reply are billed in full,
every check. That counts against the 5-hour and 7-day windows the Downshift
statusline shows. Downshift's own rule, doubt never spends money, is about
routing; this mod is a quality aid you pay for, so turn it on for long
tasks where a missed trade-off costs more than the checks, and off for
mechanical runs. `downshift setup` enables it unless you set the key yourself, `downshift doctor` reports it, and the plugin alone does not. Recent notes and
topics marked as known live in a file under `~/.claude/plugins/store/`
whose name starts with `cc-plugin-you-should-know`; Claude Code deletes it
after `cleanupPeriodDays` unused.

### Token Weather (sample mod, install from the playground)

Anthropic's DevRel team shares three sample mods in
[claude-code-playground/claude-code/mods](https://github.com/anthropics/claude-code-playground/tree/main/claude-code/mods),
Apache 2.0, as-is and unsupported.
[Token Weather](https://github.com/anthropics/claude-code-playground/tree/main/claude-code/mods/token-weather)
is the one Downshift recommends. One module, about 120 lines, no
dependencies. After each main-loop turn it calls `$.session.usage()` and
draws one line above the prompt:

```text
 ☂  Showers  67% of context  134.4k / 200k   last turns ▁▂█  ▲ +98.3k last turn
```

A weather word for how full the context window is (☀ Clear under 25%, ☁
Cloudy, ☂ Showers, ☇ Storm, ↯ Compact soon from 90%), the percentage and
tokens of the window, a block chart of the last 12 turns and what the last
turn added. The figures are the ones Claude Code itself reports, so they are
real, not estimated.

**How the two fit.** Token Weather answers "how full is my main session?"
above the prompt. Downshift answers "what did my subagents run on, and what
did that cost?" below it: the statusline `setup` installs shows the quota
windows and the session's routing summary (`◆ downshift · 6 subagents · 4 on
haiku · saved $0.31`, an estimate), and `/downshift` or `downshift session`
prints the tree. Neither reads the other; nothing competes, because Downshift
draws no `AbovePrompt` band. A second mod that draws that band would compete
with Token Weather, not with Downshift.

**Install** (Anthropic's instructions; verified on Claude Code 2.1.302,
`claude plugin validate` passes):

```bash
git clone https://github.com/anthropics/claude-code-playground.git
cd claude-code-playground/claude-code/mods
claude plugin validate ./token-weather     # read hooks/token-weather.mjs first
claude --plugin-dir ./token-weather        # one session, or:
claude plugin marketplace add ./
claude plugin install token-weather@claude-code-playground-mods --scope user
```

Open a new session; the band appears after the first response. The other two
sample mods, `blast-radius` (holds a risky shell command and shows what it
would change, with Proceed and Cancel) and `replay-theater` (steps through
the last turn's edits), install the same way and are unrelated to routing.

**What Downshift does with it.** Nothing, by the rule above: it does not
install, detect or configure a mod, and `downshift setup` never touches
`enabledPlugins`. This section is the whole integration.

**Before you install any mod.** A hooks module runs inside Claude Code with
Claude Code's access, unsandboxed. Read the module; Token Weather's calls are
`$.session.usage`, `$.ui.resolve` and `$.ui.invalidate`, which `claude plugin
validate` lists. Pin the commit you read if you install from a clone.

### Replay Theater (sample mod, review the last turn's edits)

[Replay Theater](https://github.com/anthropics/claude-code-playground/tree/main/claude-code/mods/replay-theater),
the third Anthropic sample, records each Edit, Write and MultiEdit call
during a turn (file, text before and after) without blocking or changing
it. When the turn ends a hint appears above the prompt
(`▶ Replay: 5 edits (press r)  [ Replay ]`); `/replay` or the button opens
a pane that steps through the edits one diff at a time, with Prev, Next and
Close. It calls no model and sends nothing. Downshift recommends it because
its own `downshift session --tree` says which model did the work, and this
shows what the work was; the two together answer "was the cheaper model
good enough?" in a minute. Same clone and install as Token Weather, plugin
name `replay-theater`.

### reflect-mod (claude-reflect, third-party, spends a small model call)

[claude-reflect](https://github.com/BayramAnnakov/claude-reflect) (MIT) is
a self-learning aid: it captures the corrections you make to Claude and
turns them into rules in `CLAUDE.md`. It comes in two forms, and its author
says to install only one, because with both every correction is captured
twice. The mod form, `reflect-mod` (Claude Code 2.1.286+), checks each
short prompt you send (500 characters or less, not `ok`, `continue` or a
slash command) with one small model call, Haiku by default, and shows the
rules it finds in a band above the prompt; nothing is written until you
press Save, which adds a bullet inside a marked section of
`~/.claude/CLAUDE.md` or `./CLAUDE.md`. `/reflect-queue` lists what it
found, `/reflect-pause` turns it off, `remember: <rule>` saves a rule
without the model check.

```bash
claude plugin marketplace add bayramannakov/claude-reflect
claude plugin install reflect-mod@claude-reflect-marketplace
```

**Why Downshift recommends it, and the cost.** Every rule it saves is
always-loaded context in later sessions, the cost the
[audit skill](../../skills/downshift-audit/SKILL.md) measures, so keep the
section short and prune it. Each check is a Haiku-sized request on your
own account; it is small, but it is usage, and on a subscription it counts
in the windows the statusline shows. The Python plugin form (`/reflect`,
reviewed in batch, also syncs `AGENTS.md`, works for Codex and Cursor) spends
nothing until you run it. Downshift neither installs nor detects either
form.

## claude-code-setup (pattern, not a dependency)

Anthropic's official
[claude-code-setup](https://github.com/anthropics/claude-plugins-official/tree/main/plugins/claude-code-setup)
plugin scans a repository and recommends hooks, skills, MCP servers,
subagents and commands. It is read-only and Claude Code only. Downshift does
not integrate with it; it follows the same contract: `downshift setup
--dry-run` shows the diff, `setup` asks before writing, `setup --undo`
restores the backup, and the [audit skill](../../skills/downshift-audit/SKILL.md)
never writes.

## task-observer (optional misroute collector)

[task-observer](https://github.com/rebelytics/one-skill-to-rule-them-all),
CC BY 4.0, records corrections you make during a session and proposes skill
updates for your approval. It costs tokens at session start rather than
saving them, so it is not shipped here. One use fits Downshift: when you
correct a subagent that failed because its model was too small, the recorded
observation is the raw material for a `misroute:` issue
([CONTRIBUTING.md](../../CONTRIBUTING.md)) or a reviewed `required_tier`
label. Keep the task text out of the issue; `downshift try "<task>"` output
and the expected tier are enough.

## Cost and usage skills (overlap, not companions)

Skills such as `usage-limit-reducer` or `claude-cost-optimizer` read local
transcripts and suggest, among other rules, "use a cheaper model for simple
tasks". That is the rule Downshift applies deterministically at spawn time,
so they overlap rather than complement. Their fixed-context advice (smaller
`CLAUDE.md`, fewer always-loaded tools) is covered by section 3 of the
[audit skill](../../skills/downshift-audit/SKILL.md). Their dollar figures
are list-price estimates from transcripts; on a subscription plan there is no
per-token invoice, the same caveat `downshift stats` prints.

## Related

- [when-to-use.md](when-to-use.md): Downshift versus gateways and SDKs
- [design/context-optimization.md](../design/context-optimization.md): the native compressor, providers and companions
- [reference/harness-matrix.md](../reference/harness-matrix.md): what each harness honors
