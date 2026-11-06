<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Design note: extracting setup from `cmd/downshift` (audit 4.4)

**Status:** slices 1 and 2a landed; slices 2b and 3 done. Slice 2 was split in two because it passed 1,500 changed lines. What stays in `cmd/downshift` after slice 3, and why, is listed below. Behaviour does not change in any slice.

`cmd/downshift` is package `main` (about 17.7k lines). About 40 of its files
(`setup*.go`, about 5.5k lines) are `downshift setup`. They move to
`internal/setup` in three PRs, leaves first. Each PR compiles, passes
`make check` and can merge alone.

## What setup needs from `main` (measured)

Compiling every non-test `setup*.go` file as its own package leaves only
these names undefined:

| Name | Defined in | Used by | Plan |
|------|-----------|---------|------|
| `planCrewRegistry` | `kirocrew_registry.go` | `setup_plan_targets.go` | moved in slice 3 (`crew_registry.go`); doctor's `checkCrewRegistry` stays in `cmd` and calls `setup.StripDownshiftCrewHooks` |
| `planPolicyInit`, `policyInitTarget` | `policy_init.go` | `setup.go`, `setup_write.go` | moved in slice 3 (`policy_init.go`, exported `PlanPolicyInit`): it is a pure planner over `routing/policy`, so no `Options` field is needed; `runPolicyInit` stays in `cmd` |
| `pluginEnabled` | `doctor_hooks_report.go` | plan, plugin, skills, mods | moves to `internal/setup`; doctor calls the export |
| `ownerAppRunningState`, `ownerAppWarning`, ... | `owner_app*.go` (build tags) | `setup_mcp.go` | moved in slice 3, build tags intact; doctor uses `OwnerAppState`, `OwnerAppRunningState`, `OwnerAppFix` |
| `buildDate` | `main.go` | `setup_prune.go` | `Options.BuildDate`, filled by `cmd`'s `setupRun` |
| `harnessRegistry` | `main.go` | `setup_smoke.go` | not needed: the smoke check stays in `cmd` (see below) |

Everything else setup reads from the process (HOME, binary, writers, clock,
`PATH` directories, the plugin installer) already travels in `setupOptions`.
Setup never imports `cmd`; `core` never imports setup (archtest).

The other direction, `main` code that uses setup: `doctor*` (`setupTargets`,
`setupTarget`, `setupHook`, `setupMcp`, `setupHarnessFile`, `mcpServerName`,
`mcpJSONPath`, `mcpTOMLBlock`, `mcpBlockBounds`, `readClaudeEntry`,
`isDownshiftCommand`, `isWrapperCommand`, `pluginRegistryRel`), `policy_*`
(`setupOptions`, `setupPlan`, `applyPlans`, `setupDisplay`, `setupConfirm`,
`setupBackupInfix`), `default_entry.go`, `main.go`, `uninstall.go`
(`runSetup`, `setupTargets`, `setupBackupInfix`) and `report_clear.go`
(`isInteractiveTerminal`). Each becomes a named export of `internal/setup`.

## Slices

| # | Package | Files from `cmd/downshift` | Points |
|---|---------|----------------------------|--------|
| 1 | `internal/setup/configfile` | `setup_json.go` (split into `object.go`, `object_json.go`, `textdiff.go`), `setup_mcp_json_doc.go` (`members.go`), `setup_write_atomic.go` (`atomic.go`), `setup_ownership.go`; `asObj` from `setup_merge.go` | 3 |
| 2a | `internal/setup` | `setup_target_types` (`Target`, `Hook`, `Mcp`, ..., plus the MCP layout names), `setup_targets` (`Targets`), `setupOptions` and `setupExec` (`Options`, `Exec`, `StatePath`), `setupPlan` (`Plan` with exported fields, `Changed`, `Display`), the steering text and markers (with `assets/`), the backup name (`BackupInfix`, `LatestBackup`), `setup_linked_path`, `setup_linked_targets`, `setup_binpath` (with `GuardHookBinary`), `setup_harness_files` | 5 |
| 2b | `internal/setup` | `setup_merge`, `setup_merge_ensure` (with `ownsCommand` and `wrapperPath` from `setup_wrapper`), `setupWrite` and `pruneOlderBackups` (`setupUndo` waits for slice 3), `setup_summary` (`Summary`, `Confirm`, `PrintNotes`, `DiffName`; `setupDiscoverModels` moved to `setup_session_models.go`, since discovery imports `core`), the MCP constants from `setup_mcp` (`McpServerName`, `McpServeArgs`, the marker, `McpSameArgs`), `setup_mcp_json_layout`, `setup_mcp_json`, `setup_mcp_json_hint`, `setup_mcp_toml` (with `mergeMarkedBlock` from `setup_steering`), and `pluginEnabled` | 5 |
| 3 | `internal/setup` | `setup_plan` (`plan_target.go`), `setup_plan_targets`, `planCrewRegistry` from `kirocrew_registry` (`crew_registry.go`), `planPolicyInit` (`policy_init.go`), `setup_apply`, `setup_mcp`, `setup_mcp_undo`, `setup_undo`, `setup_skills`, `setup_skills_plugin`, `setup_steering` (planner, `steering_plan.go`), `setup_mods`, `setup_mods_profile`, `setup_plugin`, `setup_plugin_refresh`, `setup_wrapper`, `setup_workers` (planner), `setup_binaries`, `setup_prune`, `owner_app*`, and the body of `setupRun` (`Run`, `run.go`); the unit and end-to-end tests of that code moved with it | 5 |

Slice 1 moved what has no dependency on setup types: it knows no target,
plan or option. Slices 2 and 3 split on side effects: slice 2 computes plans
and writes config files; slice 3 runs processes (`claude`, plugin install,
smoke), copies or removes binaries and drives the terminal UI.

Tests move with the code they test. After slice 3 the `newSetupEnv` home
fixture exists twice: `internal/setup/run_test.go` drives `setup.Run`, and
`cmd/downshift/setup_env_test.go` drives `setupRun` for the tests that need
the flag parser, the TUI, policy init or doctor. Folding the two into one
exported test helper is a follow-up.

## What stays in `cmd/downshift`, and why

| Code | Reason |
|------|--------|
| `setup.go` (`runSetup`, `setupRun`, `reportStateMigration`, `wantsSetupWizard`, usage), `setup_args.go` | argument parsing and the call; `setupRun` fills `Options.BuildDate`, `DiscoverModels` and `EnsureSessionModels` and calls `setup.Run` |
| `setup_session_models.go` (`setupDiscoverModels`, `setupEnsureSessionModels`) | imports `internal/core` and `internal/discovery`; injected as `Options` funcs |
| `setup_smoke.go` | imports `core`, `catalog` and `telemetry`; only the TUI report calls it |
| `setup_tui*.go`, `setup_workers.go` (`workersChoice`) | import `internal/console`, which archtest allows only from `cmd` (edge-only) |
| `terminal.go` (`isInteractiveTerminal`) | uses `github.com/mattn/go-isatty`; internal packages are standard library or in-module only |
| `kirocrew_registry.go` (`checkCrewRegistry`), `doctor_owner_app_mcp.go` | doctor checks; they call the setup exports |

## Risks

- Slice 2a renamed `setupTarget`, `setupPlan` and `setupOptions` at every call
  site (`setup.Target`, `setup.Plan`, `setup.Options`) and exported the
  fields of `Plan`. Slice 2b and 3 diffs are smaller: they move code, not types.
- `setupUndo` reaches into apply-side path lists (`skillPathsFor`,
  `modPathsFor`, `wrapperPathsFor`), so it moved in slice 3 (`setup.Undo`), not with the
  rest of `setup_write.go`.
- Slice 2a moved the embedded steering text and `assets/` to `internal/setup`; slice 3 moved the planner that writes it.
- `owner_app*.go` carries build tags (`_darwin`); keep the tag pair intact.
- Every moved file keeps the 300-line and 10-function limits; a file in the
  archtest function baseline gets its new path, never a higher ceiling.
- `internal/setup/*` is in the OS-sensitive classifier
  (`scripts/ci-classify-changes.sh`), so a change there runs `test-os`; since
  slice 3 the macOS and Windows steps of `test-os` run the same `-run` subset of
  `./internal/setup/...` that they run of `./cmd/downshift/`.
- `internal/setup` imports `routing/policy` and `contextopt`, which import
  `core`; the package never imports `core` itself.
