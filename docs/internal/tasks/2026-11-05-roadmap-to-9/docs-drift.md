<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# DR: Docs-versus-code drift guard

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Part of the [roadmap to 9](README.md).

## Problem

The docs can name commands, flags, environment variables, MCP tools and tests
that the code no longer has, and nothing fails.

Guards that exist today:

- the `internal/docscheck` tests;
- `limits/parity_test.go`, which compares only the status column;
- the execwrap `families_inventory_test.go`;
- `help_test.go`;
- `TestShippedTextNamesOnlyRegisteredTools`.

Drift found on main (2026-11-05):

| Where | Says | Code says |
|-------|------|-----------|
| `docs/reference/cli.md:486` | `DOWNSHIFT_LOG_LEVEL` | No code reads it |
| `docs/reference/cli.md:411-418` | 4 MCP tools | 13 registered: `providers`, `playbook`, `status`, `apply`, `plan`, `retrieve` and 7 worker tools |
| `README.md:218` | "eleven" tools | 13 |
| `docs/reference/cli.md:409` | `--mcp` | Deprecated |
| `docs/reference/cli.md` | No section for `uninstall`, `policy`, `retrieve`, `quota`, `accept`, `config`, `loop`, `models list/pull/check/discover` | All exist |
| `docs/reference/cli.md:295` | `DOWNSHIFT_FORCE_COMPACT` | Not yet checked |
| `CHANGELOG.md:42` | Quotes `TestTokenBudget` | Not yet checked |

QF-1 in the [README](README.md#quick-fixes) fixes these rows by hand. The tasks
below stop the drift from coming back.

## Tasks

| ID | Title | Pts | Depends on | Phase |
|----|-------|-----|------------|-------|
| DR-02 | Guard: every test a doc cites exists | 2 | none | 1 |
| DR-03 | Guard: every documented default endpoint matches the code | 2 | none | 1 |
| DR-04 | Guard: every documented `DOWNSHIFT_*` variable is read by the code | 2 | none | 1 |
| DR-05 | Guard: documented MCP tool list and count match the registry | 2 | none | 1 |
| DR-12 | `devcheck` runs docscheck when only docs change | 1 | none | 1 |
| DR-01 | Command registry as data | 3 | none | 2 |
| DR-06 | Guard: `cli.md` has a section for every command | 3 | DR-01 | 2 |
| DR-07 | Guard: every documented flag exists on its command | 3 | DR-01 | 2 |
| DR-08 | Guard: harness matrix matches `HarnessCapabilities` | 3 | none | 2 |
| DR-09 | Hidden `downshift docs-manifest` prints JSON | 3 | DR-01 | 3 |
| DR-10 | Release attaches the manifest | 2 | DR-09 | 3 |
| DR-11 | Site CI checks its pages against the manifest | 5 | DR-10, ST-5 | 4 |

All guards live in `internal/docscheck`, one file per guard, each under 300
lines. They read the repo's Markdown and the Go registries, and they never
shell out to the binary.

### DR-02: Cited-test guard (2)

**Design**

- Scan `docs/**/*.md`, `README.md` and `CHANGELOG.md` for `Test[A-Z]\w+`.
- Each name must exist as `func TestX(` in some `_test.go` file.
- History sections can be allowlisted with an inline marker, for example
  `<!-- docscheck:history -->`.

**Acceptance**

- `CHANGELOG.md:42` fails if `TestTokenBudget` is missing, or the change adds
  the marker on purpose.

### DR-03: Endpoint guard (2)

**Design**

- Collect `localhost:<port>` and `http://…` defaults from the docs.
- Compare them with the constants in `internal/gateway` and the other default
  endpoints.
- Allow other values only through a short allowlist, for example example URLs.

**Acceptance**

- The old `:3000` 9router port, if it came back to the docs, fails the test.

### DR-04: Environment-variable guard (2)

**Design**

- Collect `DOWNSHIFT_[A-Z_]+` from the docs and from the Go sources
  (`os.Getenv`, `LookupEnv`, plus any constant).
- A variable in the docs that the code never reads fails.
- A variable the code reads that no doc names is a warning in the first PR and
  a failure after QF-1.

**Acceptance**

- `DOWNSHIFT_LOG_LEVEL` fails until QF-1 removes or implements it.

### DR-05: MCP tools guard (2)

**Design**

- Read the tool registry: the same source as
  `TestShippedTextNamesOnlyRegisteredTools`.
- The tool list in `cli.md` and the count words ("eleven", "13") in
  `README.md` must match it.
- A count written as a word is parsed, or the docs stop spelling counts out.

**Acceptance**

- `README.md:218` fails with "eleven".

### DR-12: devcheck selects docscheck (1)

**Design**

- A change that touches only `*.md` still runs
  `go test ./internal/docscheck` under `make check`.

### DR-01: Command registry as data (3)

**Design**

- The root command table (name, summary, flags, hidden) becomes a slice that
  help, usage and the guards read.
- The dispatch code does not change.
- Without this task, DR-06 and DR-07 would have to parse help text.

**Acceptance**

- `help_test.go` still passes.
- The registry lists every dispatched command, checked by a test.

### DR-06: cli.md command coverage (3)

**Design**

- Every non-hidden command in the registry has a `##` or `###` heading in
  `cli.md`.
- The commands that are missing today get sections in the same PR, or a
  tracked allowlist that QF-1 empties.

### DR-07: Flags guard (3)

**Design**

- Every `--flag` written next to a command in `cli.md` exists on that command
  in the registry.
- Deprecated flags (`--mcp`) must sit under a "Deprecated" label.

### DR-08: Harness matrix parity (3)

**Design**

- Extend `limits/parity_test.go` beyond the status column: each capability
  cell in `harness-matrix.md` must match `core.AllHarnessCaps()`.
- The cell vocabulary is fixed: yes, no, partial plus a note.

**Acceptance**

- Flipping a capability in code without updating the matrix fails.

### DR-09: docs-manifest command (3)

**Design**

- `downshift docs-manifest` is hidden and prints JSON: version, commands,
  flags, environment variables, MCP tools, default endpoints and the harness
  capabilities.
- It reads the same registries as the guards, and its output is deterministic.

**Tests**

- A golden JSON file, refreshed with `-update`.

### DR-10: Publish the manifest (2)

**Design**

- The release workflow attaches `docs-manifest.json` to each GitHub release.

**Acceptance**

- The v0.7.3 release has the asset.

### DR-11: Site manifest check (5)

**Design**

- `downshift-site` CI downloads the manifest for the version it documents.
- It fails when a page names a command, flag, variable, tool or port the
  manifest lacks.
- It depends on ST-5: the check reads whichever file is the source of truth.

**Acceptance**

- Putting back the `:3000` port, or `omniroute`, in a site page fails site CI.

## Note on the site HTML

One research pass said the site's static HTML is generated from the MDX. The
platform pass checked and found nothing that generates it: the two are edited
by hand and drift apart. This plan follows the platform finding. See
[site-and-pane.md](site-and-pane.md), ST-5.
