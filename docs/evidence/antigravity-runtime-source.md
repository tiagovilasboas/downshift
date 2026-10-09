# Antigravity native discovery and quota evidence

Verified on 2026-10-09 at approximately 13:51–13:53 UTC. This evidence separates
the desktop app, the native Antigravity CLI, parent model selection, and
`invoke_subagent` model tiers.

## Verified native source

The installed desktop app was Antigravity 2.21.1. Its Electron launcher and
HostBridge do not supply a supported model-listing command. The HostBridge
protobuf describes application update status and application update actions;
it is not a model or quota API. The app's separate language server uses a
dynamic HTTPS port and a CSRF-protected internal connection. This investigation
did not extract credentials or call that internal API.

The separate official **Antigravity CLI**, executable `agy`, supplies a usable
read-only source. The official installer was read without executing it. Its
Darwin ARM64 manifest identified CLI 1.3.2. The release payload's SHA-512 matched
the manifest before the binary was placed at
`/tmp/downshift-antigravity-native/agy`. No shell profile, application bundle,
global executable, or authentication settings were changed.

The downloaded CLI used its own existing authentication. No login command,
authentication prompt, keychain extraction, cookie read, or token read was
performed by Downshift.

| Native command | Result | Meaning |
| --- | --- | --- |
| `agy --version` | `1.3.2`, exit 0 | Version identity |
| `agy --help` | Includes `models`, `--output-format`, `--print` | CLI command identity |
| `agy --output-format json models` | `SUCCESS`, exit 0, 18 model rows | Parent picker availability |
| `agy -p /model --output-format json` | `SUCCESS`, exit 0 | Current parent selection |
| `agy -p /usage --output-format json` | `SUCCESS`, exit 0, 2 groups, 4 quota buckets | Current account quota |
| `agy -p /help --output-format json` | `SUCCESS`, exit 0 | Native read-only slash command support |

All three structured model/usage commands reported an empty conversation ID,
zero turns and zero token usage. The models command can emit a progress message
on stderr; JSON is on stdout. With CLI 1.3.2 the output-format flag must precede
the `models` subcommand. `agy models --output-format json` failed with exit 1.

The [official installation guide](https://antigravity.google/docs/cli/install/)
identifies `agy` and its native authentication. The
[official changelog](https://antigravity.google/docs/changelog/) documents
structured output for `models` and read-only `/usage` print invocations that do
not start an agent turn or spend quota. The
[quota guide](https://antigravity.google/docs/cli/commands/usage/) documents the
backend refresh, and the
[headless guide](https://antigravity.google/docs/cli/headless/) distinguishes
these native slash commands from model responses.

## Observed structured contracts

Parent availability:

```text
status: SUCCESS
command.name: models
command.data.models[]: { id: string, label: string }
```

Current parent model:

```text
status: SUCCESS
command.name: model
command.data: { id: string, label: string, effort: string, is_default: boolean }
```

Account quota:

```text
status: SUCCESS
command.name: usage
command.data.groups[]:
  name: string
  description: string
  buckets[]:
    id: string
    name: string
    description: string (optional)
    window: string
    remaining_fraction: number
    reset_time: RFC3339 string
```

The runtime quota groups were Gemini and third-party models. Each had weekly
and five-hour buckets. The parent list supplied exact picker IDs, but neither
payload contained a structural group-to-model association. Group descriptions
contained human-readable model-family names. Those descriptions are not an
exact ID membership contract and must not be parsed into routing coverage.

## Remaining routing boundary

The [official subagent schema](https://antigravity.google/docs/subagents)
documents `inherit`, `flash` and `pro` as subagent tiers. Parent picker IDs are
different identities. The native listing does not establish that those exact
IDs are valid `invoke_subagent.Subagents[].Model` values, nor does it resolve
the tiers to an account quota group. `flash_lite` is not documented in that
schema. A model name in a parent picker is therefore insufficient evidence to
rewrite a subagent tier.

Downshift must keep quota status **unknown** for those subagent aliases until
the provider supplies an exact structural association, or the executing hook
exports the selected runtime model and pool membership. A discovered list must
not introduce parent IDs as accepted subagent overrides. An unknown third-party
pool must not be treated as shared quota for the entire Antigravity harness.

The concrete supported operator export is:

```sh
agy --output-format json models > antigravity-models.json
agy -p /usage --output-format json > antigravity-usage.json
```

These commands obtain current native evidence; they do not solve the missing
alias/pool association. Refresh and import belong outside tool hooks. Hooks
must continue to read a bounded, expiring cache without starting a native CLI,
network refresh, or authentication flow. No recurring job was installed.

## Follow-up execution on 2026-10-09

The same verified CLI remained at `/tmp/downshift-antigravity-native/agy`.
It was outside `PATH`; an unqualified `agy: command not found` did not mean the
binary had disappeared. Running its absolute path again returned version
`1.3.2`, `SUCCESS` for 18 parent models, and `SUCCESS` for two quota groups with
four buckets. The group schema still contained names/descriptions/buckets and
no structural model membership. No profile or authentication change was needed.

The [native executor observation](antigravity-executor-ack.md) independently
resolves one real rewritten `flash_lite` child to `gemini-3.5-flash-lite` on
Antigravity 2.21.1. That observation does not fill the quota membership gap or
make this CLI listing a discovery source wired into Downshift.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
| --- | --- | --- | --- |
| Parent/subagent identity | This contract and official subagent schema: inferencial | Parser tests rejecting implicit pool/alias association: computacional | architecture fitness |
| Quota evidence | Native read-only export contract: inferencial | Native exit code, JSON command identity, finite fractions, reset times and cache expiry: computacional | behaviour |
| Command source | Separate `agy` CLI identity from app and `agyo`: inferencial | Verified native help/version and SHA-512 release check: computacional | maintainability |

Native command execution was verified locally. No pre-commit hook or CI job is
claimed by this evidence document; repository tests are the integration sensor
once the corresponding parser is wired.
