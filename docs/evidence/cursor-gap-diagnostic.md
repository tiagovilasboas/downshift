# Cursor runtime gap diagnostic — 2026-10-09

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Initial observations at approximately **2026-10-09 14:37 UTC** against repository commit
`827a688fb354b4411eb626d4ede21f62c1a7b3cb` and installed Cursor **3.24.9**.
The initial pass was read-only. Follow-up implementation and opt-in runtime
validation continued through **14:49 UTC**. No credentials were read, private
settings changed, provider turn submitted, or executor acknowledgement claimed.

## Follow-up result

The native availability consumer is implemented and passes regression tests:
hook list → configured native export → discovery cache → operator file.
A complete export retains exact provider IDs and its original observation,
expires after five minutes, and rejects any future timestamp. Configured invalid
or expired input holds the session without consulting a fallback. Native
candidates require resolver metadata for their actual tier and cost; provider
order and pool membership never supply a capability ranking. Exhausted pools
cannot make a downshift select a stronger or more expensive equal-tier model.

**The real export source remains blocked on Cursor 3.24.9.** The bridge was
packaged locally, installed, and confirmed as version 0.0.1. Its command appeared
in a fresh editor host. Execution failed at the sanitized `transport` stage.
Declaring `enabledApiProposals: ["cursor"]` and opening another host with
`--enable-proposed-api tiagovilasboas.downshift-cursor-quota-bridge` produced the
same failure before either RPC, and no `cursor-usage.json` was created.

The installed public `out/vs/workbench/api/node/extensionHostProcess.js` explains
the boundary: the proposal check rejects `cursor` for non-built-in extensions
before consulting enabled proposals. Its diagnostic explicitly says this
proposal is only available for built-in extensions. The exact-ID launch flag
does not remove that restriction. No vendor modification, built-in impersonation,
token extraction, or alternate private endpoint was attempted.

`go test ./...` and all five decoder tests passed. The locally packaged VSIX
passed ZIP integrity validation and installed-extension listing. These establish
software and installation evidence, not a successful native collection. The
globally installed Downshift binary was not replaced, so its future hooks were
not claimed to use this new consumer.

Cleanup restored the prior extension state: the experimental extension was
uninstalled and absence was rechecked through the installed-extension listing.
No native export exists. The temporary `cursorbridge` window, containing no
user tabs, was closed; the `harness-downshift` window with its restored user chat
was preserved. No launch flags were written to persistent configuration.

## Evidence and limits

| Surface | Actual observation | What it establishes |
|---|---|---|
| `cursor-agent --version` | `2026.09.10-fd3934a` | Installed CLI version only. |
| `cursor-agent models` | Exit 1, `Authentication required` | CLI discovery cannot recover this account's models in the current execution environment. No login was attempted. |
| Cursor Agents → Plan & Usage | Cursor Models **60% used**; Other Models **100% used**; reset displayed **Oct 26** | Native host can display the two usage pools. The CLI failure does not establish that the desktop host is unauthenticated. This display does not provide exact model-to-pool IDs or a machine-readable export. |
| Cursor Agents → Models | Model display labels and enabled/disabled switches are visible | Desktop model selection exists. Display names and switches are not a complete exact-ID discovery contract and do not establish included credit. No switch was changed. |
| Local extension/globalStorage directory names | No `*downshift*` entry in the standard Cursor extension or globalStorage directories | The experimental bridge is not present in those standard locations. Alternative extension paths were not searched. |
| Current process environment | `DOWNSHIFT_CURSOR_NATIVE_FILE` absent | This diagnostic process has no configured native export path. The environment of a future Cursor hook was not inspected. |

Account identifiers, pricing, screenshots, private configuration contents, and
raw provider payloads are deliberately excluded from this evidence document.
Pool percentages are dated observations, not a reusable balance source.

## Native bridge contract

The checked-in [experimental bridge](../../internal/cursorbridge/extension/extension.cjs)
uses the host-owned `vscode.cursor.connectTransport`, requests
`DashboardService.GetCurrentPeriodUsage` and `AiService.AvailableModels`, then
writes a bounded local export with `observed_at`. It does not use the CLI login.

A bounded inspection of the installed public application bundle
`Cursor.app/Contents/Resources/app/extensions/cursor-always-local/dist/main.js`
confirmed the following protobuf descriptors are present:

- `GetCurrentPeriodUsageResponse`: `billing_cycle_end` field 2, `plan_usage`
  field 3, `enabled` field 6, `auto_bucket_models` field 13.
- `PlanUsage`: `auto_percent_used` field 12 and `api_percent_used` field 13,
  both optional doubles.
- `AvailableModelsRequest`: `exclude_max_named_models` field 3,
  `use_model_parameters` field 5, `use_react_model_picker` field 11. These match
  the bridge's encoded request flags.
- `AvailableModelsResponse`: repeated model messages in field 2. Model fields
  used by the decoder are present: name 1, supports_agent 5, degradation_status
  6, is_user_added 23, is_hidden 35, default_disabled_in_admin_allowlist 43.

This establishes a schema match with the installed bundle, **not a successful
authenticated native request**, completeness of the user's picker, or a stable
vendor-supported extension API. The bridge remains experimental.

## Integration gap found in the initial pass

Before the follow-up implementation, installing/exporting the bridge did not remove the manual
availability fallback:

1. The bridge exports `available_models` and `available_models_complete: true`.
2. [Quota parsing](../../internal/quota/native.go) uses those IDs to compute
   Other Models membership but returns a quota snapshot, not a session list.
3. [Session resolution](../../internal/core/session.go) reads hook lists,
   discovered cache, then `session-models.json`. It does not recover availability
   from the native export.
4. The [Cursor adapter](../../internal/adapters/cursor/cursor.go) resolves the
   session before attaching native quota.

The follow-up implementation closes this consumer gap, with tests for precedence,
freshness, provider-order independence, catalog-unknown models, quality floors,
and quota-induced upgrades. The vendor's built-in-only transport restriction
still prevents eliminating the handwritten list using this bridge in the actual
installed host.

The initial pass also found inconsistent native/cache precedence documentation.
`docs/install.md` and the `LoadNative` comment have since been corrected to
match `docs/quota.md` and `SessionList.WithUsageQuota`: the configured native
source precedes the canonical cache. This documentation finding is closed.

## Next executable sequence

1. Obtain a vendor-supported export/collector surface that provides exact
   model IDs and pool membership, or verify a future host explicitly allows this
   bridge. Current desktop UI percentages do not meet that machine contract.
2. Only after that source is available, capture a current export. The tested
   native consumer already preserves exact IDs, source observation time,
   five-minute expiry, and authoritative failure behavior.
3. Invoke `Downshift: Export Cursor Quota (Experimental)` and verify the actual
   export has fresh timestamps, exact dynamic membership, both pool percentages,
   and a complete available-model list. Never derive membership from Grok or
   Composer display names.
4. Point the Cursor hook to that export and verify an actual hook invocation
   consumes it without an operator model list. Demonstrate that exhausted Other
   Models are rejected and that a fresh model with included quota can be selected
   only when the catalog provides its tier.
5. Correlate emitted rewrite with the actual child model using the
   [rewrite-honored protocol](rewrite-honored-protocol.md). A valid file, a passing
   parser test, or `updated_input` proves no executor acknowledgement by itself.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| Keep host authentication distinct from CLI authentication | Evidence and source boundaries above (inferencial) | Separate CLI exit and real native UI/request observations (computacional output with inferencial assessment) | Architecture fitness / behaviour |
| Bind exact availability and quota dynamically | Native schema, precedence, freshness contract (inferencial) | Parser/quota/session gates and native-availability regression (computacional); real export check blocked by built-in-only API | Behaviour / architecture fitness |
| Avoid false actuation claims | Rewrite-honored protocol (inferencial) | Correlated hook decision and actual child execution evidence (computacional; not performed here) | Behaviour |

The local diagnostic supplies evidence for the IDE surface. CI regression checks
and prompt-free quota/source telemetry must cover the same contracts outside
this conversation; no new continuous collector or honored rewrite was verified
by this diagnostic.
