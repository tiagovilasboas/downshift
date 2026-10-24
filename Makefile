.PHONY: build test lint vet check check-fast test-changed hooks hooks-uninstall install build-all clean report e2e

BINARY=downshift

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

## check-fast: gofmt, then go vet for host, windows and linux (no tests; quick loop, also run by the pre-push hook)
check-fast:
	@echo "[1/4] gofmt -l ."
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; echo "gofmt: format the files above (gofmt -w <file>)"; exit 1; fi
	@echo "[2/4] go vet ./...  (host)"
	@go vet ./...
	@echo "[3/4] go vet ./...  (GOOS=windows: type-checks test files the Windows CI job compiles)"
	@GOOS=windows GOARCH=amd64 go vet ./...
	@echo "[4/4] go vet ./...  (GOOS=linux)"
	@GOOS=linux GOARCH=amd64 go vet ./...

## check: gate before declaring a change done; runs check-fast, then go test -count=1 ./... (no cache)
check: check-fast
	@echo "[5/5] go test -count=1 ./..."
	go test -count=1 ./...

## test-changed: go test -count=1 on the packages changed since origin/main and their dependents, the CI selection (./... when unsure); run by the pre-push hook
test-changed:
	@base=$$(git merge-base origin/main HEAD 2>/dev/null); \
	pkgs=$$(GITHUB_EVENT_NAME=pull_request GITHUB_BASE_SHA=$$base bash scripts/ci-test-packages.sh) || exit 1; \
	echo "[test-changed] $$(echo $$pkgs)"; \
	go test -count=1 $$pkgs

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

## report: print eval-outcome block (maintainer: full suite in downshift-labs; update benchmark/report.md for public)
report:
	go run ./cmd/downshift eval-outcome --report

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
