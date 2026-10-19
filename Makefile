.PHONY: build test lint vet install build-all clean report sim

BINARY=downshift
LDFLAGS=-ldflags "-s -w"

## build: compile the downshift binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/downshift

## test: run tests with race detector and coverage
test:
	go test -race -cover ./...

## sim: offline accounting simulation with per-scenario ESTIMATE tables (no model call)
sim:
	go test ./cmd/downshift -run TestSimulation -count=1 -v

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
