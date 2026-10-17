# cache-tax (bundled mod)

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

This directory is a vendored copy of **cache-tax**, a Claude Code mod by
Karan Bansal, MIT License ([LICENSE](LICENSE)). Owner decision 2026-10-17:
the Downshift plugin carries it by default, so a plugin install gets the
prompt-cache guard and `/keepwarm` without a second marketplace.

| | |
|---|---|
| Upstream | https://github.com/karanb192/cache-tax |
| Version | 2.2.1 |
| Commit | `06cac47ec125a41a4aeeb78a6b006bec8cc6d6ed` (2026-10-04) |
| Files | `register.ts`, `status-icons.ts` byte-identical to upstream `hooks/`; `register.test.ts` is upstream `tests/register.test.ts` with its two imports pointed at this directory |
| Loaded by | [`../hooks.json`](../hooks.json), `"modules": ["./cache-tax/register.ts"]` |
| Needs | Claude Code 2.1.287 or later (mods on by default); a one-hour prompt cache for the keep-warm timer |

## What it does

- **Cold-send guard (on by default).** When the session's context is at
  least 50,000 tokens and the last request is more than an hour old, an
  ordinary message is dropped once with its estimated rewrite price
  (`cache-tax: the prompt cache went cold 2h00m ago. Sending this re-writes
  up to 200,502 tokens at $20/MTok = $4.01 ...`). Sending it again goes
  through. `/cache-tax guard warn` shows the price and sends.
- **Keep-warm (you arm it).** `/keepwarm` (6 h), `/keepwarm 90m`,
  `/keepwarm always`, `/keepwarm off`. Inside the window, after 50 idle
  minutes one tool-less fork with a fixed one-line prompt reads the cache so
  it is not rewritten; it stops itself when a readback shows the cache was
  already gone. A detected cold write arms three hours on its own.
- **`/cache-tax`**: the card (state, context, cold price, window, this
  session's cold writes). A band above the prompt while a window is armed.

**It spends usage when it pings.** A ping is a cache read plus a few
uncached tokens plus the model's one-word answer, billed at list prices on
an API key and counted in the plan windows on a subscription. The guard
itself sends nothing. Prices are the mod's own table (list prices,
September 2026), not Downshift's catalog; Downshift's `stats` does not count
pings.

## Privacy

Of each prompt the mod reads only whether the first non-blank character is a
slash; the text is passed on untouched, never kept and never logged. It
persists, in Claude Code's plugin store, the keep-warm deadline and ping
period keyed by session id, plus the global always switch and guard mode.
Nothing leaves the machine except the fork, an API request over the
session's own transcript. Full threat model: upstream README, "What it can
reach".

## Both forms installed

If `cache-tax@claude-code-mods` (the upstream marketplace install) is also
enabled, two copies guard every cold send and both register `/keepwarm`.
`downshift doctor` warns and names the fix (`/plugin uninstall
cache-tax@claude-code-mods`). The settings-hook form
(`cache-tax@claude-code-hooks`) is detected by the mod itself at session
start.

## Update procedure

Re-vendor, never edit in place:

```bash
git clone --depth 1 https://github.com/karanb192/cache-tax.git /tmp/cache-tax
cp /tmp/cache-tax/hooks/register.ts /tmp/cache-tax/hooks/status-icons.ts hooks/cache-tax/
cp /tmp/cache-tax/LICENSE hooks/cache-tax/LICENSE
sed -e "s#'../hooks/status-icons'#'./status-icons'#" -e "s#'../hooks/register'#'./register'#" \
  /tmp/cache-tax/tests/register.test.ts > hooks/cache-tax/register.test.ts
claude plugin validate . && claude plugin test .
```

Then update the version and commit in this file and in [NOTICE](../../NOTICE),
and read the upstream diff for new calls (`claude plugin validate` lists
them) before committing.

## Verify

```bash
claude plugin validate .     # hooks: ... calls: $.model.fork (via ping) ... env reads: NO_COLOR
claude plugin test .         # 56 tests, mock clock and engine
```
