.PHONY: build test lint vet check check-full check-fast check-changed test-changed test-staged vet-staged staticcheck-staged test-mods hooks hooks-uninstall install build-all clean report e2e bench bench-baseline

BINARY=downshift
# DEVCHECK is the gate binary behind check-full and check-fast. Built, not go run, so a
# failure prints no extra "exit status 1" line; dist/ is git-ignored.
DEVCHECK=dist/devcheck$(shell go env GOEXE)

# Version stamp, same main.build* variables as .goreleaser.yml. Falls back to
# "dev" outside a git checkout. Untagged repos get a "main-" prefix.
VERSION := $(shell git describe --tags --dirty 2>/dev/null || (d=$$(git describe --always --dirty 2>/dev/null) && echo main-$$d) || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X main.buildVersion=$(VERSION) -X main.buildCommit=$(COMMIT) -X main.buildDate=$(DATE)"

## build: compile the downshift binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/downshift

## test: run tests with race detector and coverage
test:
	go test -race -cover ./...

## vet: run go vet
vet:
	go vet ./...

## check-fast: gofmt and go vet for host, windows and linux, run concurrently by cmd/devcheck (no tests; quick loop, also run by the pre-push hook)
check-fast:
	@go build -o $(DEVCHECK) ./cmd/devcheck && $(DEVCHECK) -fast

## check: local gate for the files you changed: gofmt, vet (host, windows, linux), go test (cached) on the packages changed since origin/main plus dependents (./... when unsure), all at once; instant when the tree is unchanged since the last green run; CI runs the authoritative full selection
check:
	@go build -o $(DEVCHECK) ./cmd/devcheck && $(DEVCHECK) -changed

## check-full: the whole repository: cmd/devcheck runs the check-fast stages, then go test -count=1 ./... (no cache); slow, run on request
check-full:
	@go build -o $(DEVCHECK) ./cmd/devcheck && $(DEVCHECK)

# Packages a change needs, the same selection CI uses (scripts/ci-test-packages.sh):
# changed packages and their dependents, ./... when unsure, empty when no Go test applies.
CHANGED_PKGS = base=$$(git merge-base origin/main HEAD 2>/dev/null); \
	pkgs=$$(GITHUB_EVENT_NAME=pull_request GITHUB_BASE_SHA=$$base bash scripts/ci-test-packages.sh) || exit 1
STAGED_PKGS = pkgs=$$(bash scripts/staged-packages.sh) || exit 1
STATICCHECK = GOTOOLCHAIN="$$(go env GOVERSION)" go run honnef.co/go/tools/cmd/staticcheck@2026.2.2
# Subprocess deadlines in tests stretch by this factor (internal/testenv); the hooks
# run while other worktrees load the machine.
HOOK_ENV = DOWNSHIFT_TEST_TIMEOUT_SCALE=$${DOWNSHIFT_TEST_TIMEOUT_SCALE:-4}
# At most 4 test binaries at once per hook run, so parallel worktrees share the CPUs.
HOOK_TEST = $(HOOK_ENV) go test -count=1 -p $${DOWNSHIFT_HOOK_TEST_P:-4}

## check-changed: gofmt, then go vet (host, windows, linux) on the changed packages and their dependents; run by the pre-push hook
check-changed:
	@echo "[1/2] gofmt -l ."
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; echo "gofmt: format the files above (gofmt -w <file>)"; exit 1; fi
	@$(CHANGED_PKGS); \
	if [ -z "$$pkgs" ]; then echo "[2/2] go vet: no Go packages changed"; exit 0; fi; \
	echo "[2/2] go vet (host, GOOS=windows, GOOS=linux) $$(echo $$pkgs)"; \
	go vet $$pkgs && GOOS=windows GOARCH=amd64 go vet $$pkgs && GOOS=linux GOARCH=amd64 go vet $$pkgs

## test-changed: go test -count=1 on the packages changed since origin/main and their dependents, the CI selection (./... when unsure); run by the pre-push hook
test-changed:
	@$(CHANGED_PKGS); \
	if [ -z "$$pkgs" ]; then echo "[test-changed] no Go packages to test"; exit 0; fi; \
	echo "[test-changed] $$(echo $$pkgs)"; \
	$(HOOK_TEST) $$pkgs

## test-mods: the Claude Code mod's own tests (claude plugin test), only when mods/ changed since origin/main; run by the pre-push hook
test-mods:
	@base=$$(git merge-base origin/main HEAD 2>/dev/null); \
	if [ -n "$$base" ] && git diff --quiet "$$base" HEAD -- mods/; then echo "[test-mods] mods/ unchanged"; exit 0; fi; \
	if ! command -v claude >/dev/null 2>&1; then echo "[test-mods] claude CLI not on PATH; skipped"; exit 0; fi; \
	claude plugin test mods/downshift-agents

## test-staged: go test -count=1 on the packages of the staged .go files only (fast); run by the pre-commit hook
test-staged:
	@$(STAGED_PKGS); \
	if [ -z "$$pkgs" ]; then echo "[test-staged] no staged Go packages"; exit 0; fi; \
	echo "[test-staged] $$(echo $$pkgs)"; \
	$(HOOK_TEST) $$pkgs

## vet-staged: go vet on the packages of the staged .go files; run by the pre-commit hook
vet-staged:
	@$(STAGED_PKGS); [ -z "$$pkgs" ] || go vet $$pkgs

## staticcheck-staged: staticcheck on the packages of the staged .go files; run by the pre-commit hook
staticcheck-staged:
	@$(STAGED_PKGS); [ -z "$$pkgs" ] || $(STATICCHECK) $$pkgs

## hooks: install the lefthook git hooks (lefthook.yml); lefthook is pinned as a go.mod tool, so no global install
hooks:
	@git config --local --unset core.hooksPath 2>/dev/null || true
	go tool lefthook install
	@echo "git hooks installed (pre-commit, pre-push). Skip once: LEFTHOOK=0 git commit / git push"

## hooks-uninstall: remove the lefthook git hooks
hooks-uninstall:
	go tool lefthook uninstall

## install: install downshift into GOBIN
install:
	go install $(LDFLAGS) ./cmd/downshift

## build-all: cross-compile for macOS, Linux, Windows
build-all:
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  ./cmd/downshift
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  ./cmd/downshift
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   ./cmd/downshift
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   ./cmd/downshift
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/downshift

## report: print eval-outcome block (maintainer: full suite in downshift-labs; update docs/benchmark/report.md for public)
report:
	go run ./cmd/downshift eval-outcome --report

## bench: token-savings benchmark over the compressor corpus; rewrites docs/reference/bench.md (DOWNSHIFT_BENCH_RTK=1 adds rtk columns)
bench:
	@DOWNSHIFT_BENCH_WRITE=1 go test -count=1 -run TestBenchReport -v ./internal/compressor

## bench-baseline: rewrite internal/compressor/testdata/bench/baseline.tsv, the floor TestBenchGuard enforces
bench-baseline:
	@DOWNSHIFT_BENCH_BASELINE=1 go test -count=1 -run TestBenchBaselineWrite ./internal/compressor

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

## help: list targets
help:
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## e2e: build, then run the end-to-end battery against ./downshift in a temp state dir
e2e: build
	DOWNSHIFT_BIN=./$(BINARY) python3 -I scripts/e2e/battery.py
