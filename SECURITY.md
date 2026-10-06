<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: BUSL-1.1 -->

# Security Policy

## Reporting vulnerabilities

This project runs as a PreToolUse hook inside AI coding agents (Claude Code, Cursor, Codex, Antigravity, KiroCrew). If you discover a security vulnerability, please report it privately to avoid immediate public disclosure.

**Do not open a public GitHub issue for security vulnerabilities.**

### Reporting process

1. Email your report to the project maintainer: visit [github.com/tiagovilasboas](https://github.com/tiagovilasboas) for contact details.
2. Include:
   - A clear description of the vulnerability.
   - Steps to reproduce (if applicable).
   - Potential impact.
   - Suggested fix (if you have one).
3. Allow **7 days** for an initial response, and **30 days** for a fix and release.

## Scope

Security vulnerabilities in `harness-downshift` include:

- **Privilege escalation** — routing logic that bypasses or circumvents hook execution controls.
- **Model & token injection** — unauthorized mutation of `updated_input.model` or `reasoning_effort` fields.
- **Event log leakage** — paths, prompts, or API keys exposed in `~/.harness-downshift/events.jsonl`.
- **Catalog injection** — malformed or untrusted catalog entries that cause denial of service or bypass safety guards.
- **Hook runtime isolation** — failure to run isolated from the parent harness process, or unexpected process access.

## Out of scope

- Vulnerabilities in upstream harnesses (Claude Code, Cursor, Codex, etc.) that ignore the hook's rewrite.
- Vulnerabilities in LLM providers' APIs (Anthropic, OpenRouter, etc.).
- Configuration mistakes (e.g., insecure file permissions on `~/.harness-downshift/`).
- Dependency vulnerabilities in `go` stdlib or the build toolchain (report to Go team).

## Fixed and disclosed vulnerabilities

None reported or fixed as of 2026-10-06. See [releases](https://github.com/tiagovilasboas/harness-downshift/releases) for patch history.

## Building and testing

```bash
# Run the full test suite with `-race`
go test -race ./...

# Run security checks
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Format code
gofmt -w .

# Lint (install golangci-lint)
golangci-lint run
```

## Code review

All pull requests undergo:
- Automated testing (race detector, coverage).
- Manual code review (routing logic, catalog handling, event serialization).
- Security checks (govulncheck, gofmt, vet).

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow.

## Dependencies

`harness-downshift` has **zero external Go dependencies**. The binary is statically linked and safe to download and run offline.

```bash
# View dependency graph
go mod graph
# Output: empty (no external modules)
```

Python tooling (LangGraph orchestration, test helpers) may have dependencies—see `orchestration/pyproject.toml` and `tests/requirements.txt`.

## License implications

`harness-downshift` is published under BUSL-1.1 (Business Source License 1.1). Free for non-commercial use; commercial use requires a written licence.

Security patches are released for all versions and are available to all users, regardless of commercial licensing status.

---

**Last updated:** 2026-10-06  
**Maintainer:** Tiago de Carvalho Vilas Boas
