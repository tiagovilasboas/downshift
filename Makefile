.PHONY: build test lint vet install build-all clean report

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
