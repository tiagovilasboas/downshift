<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Output Cutting Candidate Mapping & Comprehensive Roadmap

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

This document comprehensively maps developer CLI tools for native output cutting in `downshift exec`. It evaluates token waste, invocation frequency, fail-open safety invariants, and execution phases across unit tests, E2E, build systems, linters, package managers, ORMs, and version control noise.

---

## 1. Architectural Invariants

Every candidate must satisfy the following four core principles:

1. **Strict Fail-Open Safety:** An exit code != 0, unhandled exception, assertion error, compiler error, traceback, or non-empty stderr stream MUST preserve 100% of the raw, uncompressed output verbatim. A developer agent must never hallucinate or fail to diagnose a bug due to squelched error details.
2. **Zero Shell Layer:** Execution bypasses `/bin/sh` or `/bin/zsh`. Binaries are resolved via `exec.LookPath` and invoked directly with `os/exec.Command`, using a strict argument whitelist (`safeByte`).
3. **Deterministic In-Memory Compression:** No remote APIs, local LLM calls, or background daemons. Parsing uses pre-compiled regular expressions and bounded byte buffers (< 2 MB) executing in under 1ms.
4. **ROI Focus (80/20 Rule):** Target repetitive test runners, compiler checks, progress bars, dependency trees, and ASCII art generators that inflate multi-turn cache debt across long agentic sessions.

---

## 2. Comprehensive Candidate Matrix

### Category A: Test Runners & E2E Suites
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`go test`** | 300 – 2,500 tokens (package list, durations, cached tags) | `Go test: PASS (N packages ok)` | **~85%** | `FAIL`, `--- FAIL`, `panic:`, exit != 0 | **Landed (v0.1)** |
| **`go test -v`** | 1,000 – 5,000 tokens (one line per test, package summaries) | `Go test: PASS (N tests across M packages)` | **~90%** | `FAIL`, `--- FAIL`, `panic:`, unrecognized output, exit != 0 | **Phase 1 (In Progress)** |
| **`git status`** | 100 – 400 tokens (boilerplate branch info) | `On branch X (working tree clean)` | **~65%** | Modified files, untracked, exit != 0 | **Landed (v0.1)** |
| **`pytest`** | 500 – 3,000 tokens (rootdir, session starts, dots, duration) | `Pytest: PASS (N passed in Xs)` | **~95%** | `FAILED`, `ERROR`, `Traceback`, exit != 0 | **Landed (PR #267)** |
| **`vitest`** | 500 – 2,500 tokens (test files, runner headers, transform times) | `Vitest: PASS (N passed across M files in Xs)` | **~86%** | `FAIL`, `Error:`, `AssertionError`, exit != 0 | **Landed (PR #267)** |
| **`playwright test`** | 2,000 – 6,000 tokens (workers, browser matrix, step timings, HTML links) | `Playwright: PASS (N passed across B browsers in Xs)` | **~97%** | `failed`, `flaky`, timeout, retry, trace, `Error:`, exit != 0 | **Phase 1 (In Progress)** |
| **`cargo test`** | 1,000 – 3,000 tokens (compilation banners, individual `test ... ok`) | `Cargo test: PASS (N passed in Xs)` | **~92%** | `FAILED`, `error[E...]`, warning, panic, exit != 0 | **Phase 1 (In Progress)** |
| **`jest`** | 1,000 – 3,500 tokens (spec trees, test suites summary) | `Jest: PASS (N passed across M suites in Xs)` | **~90%** | `FAIL`, `Error:`, exit != 0 | **Phase 2** |
| **`cypress run`** | 2,500 – 6,500 tokens (spec tables, video links, browser launch) | `Cypress: PASS (N passed across M specs in Xs)` | **~96%** | `failed`, `AssertionError`, exit != 0 | **Phase 2** |
| **`mvn test`** | 3,000 – 8,000 tokens (`[INFO]` downloads, plugin resolutions, build logs) | `Maven: PASS (N tests run, 0 failures in Xs)` | **~98%** | `BUILD FAILURE`, `FAILURE!`, exceptions, exit != 0 | **Phase 3** |
| **`gradle test`** | 2,500 – 6,000 tokens (task execution tree, daemon banners) | `Gradle: PASS (N tests passed in Xs)` | **~97%** | `BUILD FAILED`, task failure, exit != 0 | **Phase 3** |
| **`flutter test`** | 1,000 – 3,500 tokens (engine init, simulator status, test lines) | `Flutter: PASS (N tests passed in Xs)` | **~92%** | `Some tests failed`, exceptions, exit != 0 | **Phase 4** |
| **`phpunit` / `pest`** | 500 – 3,000 tokens (per-test progress, suite summaries) | `PHP tests: PASS (N tests, 0 failures)` | **~90%** | Failure/error details, exceptions, exit != 0 | **Phase 3** |

---

### Category B: Typecheckers, Compilers & Linters
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`tsc --noEmit`** | 500 – 2,000 tokens (or clean 0 bytes / header noise) | `TSC: PASS (0 errors)` | **~95%** | `error TS...`, `Found N errors`, exit != 0 | **Phase 1 (In Progress)** |
| **`mypy` / `pyright`** | 600 – 2,500 tokens (file-by-file checks, environment warnings) | `Mypy: PASS (no issues found in N source files)` | **~90%** | `Found N errors`, type mismatch, exit != 0 | **Phase 2** |
| **`eslint` / `biome`** | 800 – 3,000 tokens (dozens of non-blocking style/import warnings) | `ESLint: PASS (0 errors, W warnings)` | **~85%** | `X errors`, exit != 0 | **Phase 3** |
| **`ruff check`** | 500 – 2,000 tokens (rule descriptions, code fix snippets) | `Ruff: PASS (All checks passed)` | **~90%** | Syntax/rule errors, exit != 0 | **Phase 3** |
| **`golangci-lint run`** | 1,000 – 4,000 tokens (multi-linter diagnostic trees) | `golangci-lint: PASS (no issues)` | **~92%** | Linter violations, exit != 0 | **Phase 3** |
| **`phpstan analyse`** | 500 – 3,000 tokens (file progress, memory/time footer) | `PHPStan: PASS (N files, 0 errors)` | **~90%** | Any diagnostic, baseline error, internal error, exit != 0 | **Phase 3** |

---

### Category C: Formatters & Pretty-Printers
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`prettier --check`** | 1,500 – 4,000 tokens (lists every unchanged file checked) | `Prettier: PASS (N files checked, clean)` | **~99%** | `Code style issues found`, exit != 0 | **Phase 2** |
| **`black --check`** / **`ruff format --check`** | 400 – 1,500 tokens (lists all checked Python files) | `Format: PASS (N files left unchanged)` | **~95%** | `would be reformatted`, exit != 0 | **Phase 2** |
| **`pint --test`** | 300 – 1,500 tokens (file-by-file style results) | `Pint: PASS (N files clean)` | **~90%** | Any file needing a fix, formatter error, exit != 0 | **Phase 3** |

---

### Category D: Build Bundlers & ORM ASCII Art
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`next build`** | 2,000 – 4,500 tokens (massive ASCII route tables, chunk sizes, symbols) | `Next.js Build: SUCCESS (N routes compiled in Xs)` | **~96%** | `Failed to compile`, build errors, exit != 0 | **Phase 2** |
| **`prisma generate` / `migrate`** | 800 – 2,000 tokens (ASCII pyramid banner, telemetry notes) | `Prisma: SUCCESS (Client generated for schema)` | **~95%** | `Error: P...`, migration failed, exit != 0 | **Phase 3** |
| **`alembic upgrade`** | 400 – 1,200 tokens (revision migration logs) | `Alembic: SUCCESS (Upgraded to revision REV)` | **~85%** | Migration errors, exit != 0 | **Phase 4** |

---

### Category E: Package Managers & Dependency Installs
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`pnpm install`** | 1,000 – 3,500 tokens (progress tickers, peer dependency warnings, audit) | `pnpm: SUCCESS (added N packages in Xs)` | **~92%** | ERESOLVE, lockfile error, exit != 0 | **Phase 4** |
| **`npm install`** | 1,500 – 4,500 tokens (ASCII trees, deprecation notices, fund notes) | `npm: SUCCESS (added N packages in Xs)` | **~94%** | npm ERR!, peer conflict, exit != 0 | **Phase 4** |
| **`pip install` / `uv sync`** | 800 – 3,000 tokens (wheel collection, cache hits, build wheels) | `pip/uv: SUCCESS (installed N packages)` | **~90%** | Resolution error, exit != 0 | **Phase 4** |
| **`composer install`** | 1,000 – 4,000 tokens (package operations, autoload generation, scripts) | `Composer: SUCCESS (dependencies installed)` | **~90%** | Script output, warning, package conflict, exit != 0 | **Phase 4** |

---

### Category F: Code Generation & Query Tooling
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`go generate ./...`** | 500 – 3,000 tokens (generator output and package progress) | `go generate: SUCCESS (N packages)` | **~85%** | Unrecognized generator output, stderr, warning, or non-zero exit | **Phase 3** |
| **`sqlc generate`** | 300 – 2,000 tokens (generation notices and package output) | `sqlc: SUCCESS (queries generated)` | **~90%** | Any diagnostic, schema/query error, warning, stderr, or non-zero exit | **Phase 3** |

### Category G: Security Audits & Code Search Bounds
| Tool / Command | Typical Raw Output | Compacted Target | Token Reduction | Fail-Open Trigger | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`npm audit` / `pip-audit` / `cargo audit`** | 2,000 – 5,000 tokens (massive tabular dependency paths for devDependencies) | `Audit: N vulnerabilities (X low, Y mod, Z high)` | **~90%** | Scan failure, exit != 0 | **Phase 4** |
| **`git diff` (Lockfile Filter)** | 3,000 – 20,000 tokens (generated lockfile diffs: `package-lock.json`, `pnpm-lock.yaml`, `go.sum`, `Cargo.lock`) | Squelch lockfile diffs to `diff: package-lock.json (N lines changed, squelched)` | **~95%** | Error reading git diff, exit != 0 | **Phase 5** |
| **`ripgrep` (`rg`)** | 3,000 – 15,000 tokens (unconstrained searches dumping 500+ matches) | Grouped hits: top 3 per file + total count summary | **~80%** | Regex errors, exit != 0 | **Phase 5** |

---

## 3. Deliberately Excluded Tools (Anti-Patterns)

The following tools are explicitly excluded from `downshift exec` wrapping to prevent agent hallucinations and diagnostic blind spots:

| Tool | Reason for Exclusion |
| :--- | :--- |
| **`docker` / `docker compose`** | Output contains critical runtime state, port allocations, container IDs, and intermittent crash logs. Squelching risks hiding subtle initialization deadlocks. |
| **`kubectl`** | Cluster state, pod events, and status columns are context-sensitive. Omitting output causes agents to miss pending pods or restart counts. |
| **`psql` / database CLIs** | Query results, row counts, and schema inspection columns must remain intact. Compacting query output breaks data-driven reasoning. |
| **`curl` / `wget`** | Raw response bodies and HTTP status headers are actively inspected by agents debugging APIs. Truncation causes parser errors. |
| **`aws` / `gcloud`** | Cloud CLI JSON envelopes are parsed directly by developer agents for resource provisioning. Filtering risks cutting critical resource properties. |

---

## 4. Execution Roadmap Summary

- **Phase 1 (In Progress):** `playwright` (E2E), `cargo test` (Rust), `tsc` (TypeScript typecheck), and verbose `go test`.
- **Phase 2 (Web & Unit):** `jest`, `cypress`, `next build`, `prettier --check`, `black/ruff format --check`, `mypy`.
- **Phase 3 (Enterprise & Linters):** `mvn test`, `gradle test`, `prisma generate/migrate`, `eslint`, `ruff check`, `golangci-lint`, PHP test/type/format tools, `go generate`, and `sqlc`.
- **Phase 4 (Deps & Audits):** `pnpm install`, `npm install`, `pip/uv`, `npm/cargo/pip audit`, `alembic`, `flutter test`.
- **Phase 5 (Noise & Search Bounds):** `git diff` lockfile squelcher, `ripgrep` (`rg`) hierarchical match bounds.
