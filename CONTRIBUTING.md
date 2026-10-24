# Contributing to Downshift

Thank you for your interest in contributing to Downshift! This guide explains how
to add new compressors, run tests, and submit changes.

## Development Setup

```bash
# Clone
git clone https://github.com/tiagovilasboas/downshift.git
cd downshift

# Build
go build -o downshift ./cmd/downshift

# Gate before declaring a change done (see Before you push below)
make check

# Run tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/compressor/...
```

### Requirements

- Go 1.27+
- Git
- Make (optional, for convenience commands)

### Before you push

`make check` is the gate before you declare a change done. It runs gofmt, `go vet`
for the host, Windows and Linux (which type-checks test files for those OSes
without running them), then `go test -count=1 ./...`. Run `go test` alone and you
miss the Windows check: in the last 80 CI runs, 11 of 12 failures were the Windows
job, from test files that call helpers defined behind `//go:build !windows`.

- `make check-fast`: the same checks without the test run, for quick loops.
- `make hooks`: opt in to `.githooks/pre-push`, which runs `make check-fast` on push.
  Skip it once with `git push --no-verify`; undo with `git config --unset core.hooksPath`.
- Windows: the hook needs Git Bash and `make`. Otherwise run `make check-fast` by hand.

---

## Adding a New Compressor

Compressors transform verbose command output into compact summaries. Follow these
steps to add support for a new tool.

### Step 1: Create the Compressor File

Create a new file in `internal/compressor/`:

```go
// internal/compressor/mytool.go
package compressor

import (
	"regexp"
	"strings"
)

// mytoolCompressor compacts output from `mytool` commands.
type mytoolCompressor struct{}

func (c *mytoolCompressor) Name() string {
	return "mytool"
}

func (c *mytoolCompressor) Match(cmd string) bool {
	// Match commands like "mytool test", "mytool build"
	return strings.HasPrefix(cmd, "mytool ")
}

func (c *mytoolCompressor) Compact(output string, exitCode int) (string, bool) {
	// Return early if nothing to compact
	if len(output) < 100 {
		return output, false
	}

	// Parse and compact the output
	// Return (compacted, true) if compacted
	// Return (output, false) if unchanged
	
	if exitCode == 0 {
		// Success case: extract summary
		return c.compactSuccess(output)
	}
	
	// Failure case: preserve error details
	return c.compactFailure(output)
}

func (c *mytoolCompressor) compactSuccess(output string) (string, bool) {
	// Example: extract pass count and duration
	// ✓ PASS · 42 tests · 1.234s
	return "✓ PASS · summary here", true
}

func (c *mytoolCompressor) compactFailure(output string) (string, bool) {
	// Preserve failure details for debugging
	// Only compact redundant parts, keep error messages
	return output, false
}
```

### Step 2: Register the Compressor

Add your compressor to the registry in `internal/compressor/registry.go`:

```go
func init() {
	Register(&mytoolCompressor{})
}
```

Or in `internal/compressor/detect.go` if using format detection:

```go
case "mytool":
	return &mytoolCompressor{}
```

### Step 3: Add Test Fixtures

Create test fixtures in `internal/compressor/testdata/`:

```
internal/compressor/testdata/
├── mytool_pass_clean.txt      # Clean success output
├── mytool_pass_verbose.txt    # Verbose success output
├── mytool_fail_simple.txt     # Simple failure
└── mytool_fail_complex.txt    # Complex failure with stack
```

### Step 4: Write Tests

Create `internal/compressor/mytool_test.go`:

```go
package compressor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMytoolCompressor_Match(t *testing.T) {
	c := &mytoolCompressor{}
	
	tests := []struct {
		cmd  string
		want bool
	}{
		{"mytool test", true},
		{"mytool build", true},
		{"othertool test", false},
	}
	
	for _, tt := range tests {
		if got := c.Match(tt.cmd); got != tt.want {
			t.Errorf("Match(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestMytoolCompressor_Compact(t *testing.T) {
	c := &mytoolCompressor{}
	
	// Load fixture
	input, _ := os.ReadFile(filepath.Join("testdata", "mytool_pass_clean.txt"))
	
	output, changed := c.Compact(string(input), 0)
	
	if !changed {
		t.Error("expected compaction")
	}
	
	if len(output) >= len(input) {
		t.Errorf("output should be smaller: %d >= %d", len(output), len(input))
	}
}
```

### Step 5: Update Documentation

Add your tool to `docs/reference/supported-tools.md`:

```markdown
### MyTool

| Command | Compaction |
|---------|------------|
| `mytool test` | ✅ Pass/fail summary |
| `mytool build` | ✅ Build summary |
```

---

## Compressor Design Guidelines

### DO

- **Preserve exit codes** — Never change the exit code meaning
- **Keep failure details** — Errors, stack traces, file:line references
- **Be conservative** — When in doubt, don't compact
- **Test edge cases** — Empty output, partial output, unicode
- **Measure reduction** — Add to corpus and run measurement

### DON'T

- **Remove error context** — The agent needs to debug failures
- **Compact already-compact output** — Check length first
- **Assume output format** — Tools change between versions
- **Use LLM for compaction** — Must be deterministic

### Output Format Convention

```
✓ PASS · {count} {unit} · {duration}
✗ FAIL · {count} failed · {summary}
⚠ WARN · {message}
```

---

## Running the Test Suite

```bash
# All tests
go test ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test -v ./internal/compressor/...

# Run corpus measurement
go test -v -run TestMeasureCorpusCompaction ./internal/compressor/
```

---

## Architecture Overview

```
cmd/downshift/          # CLI entry point
internal/
├── classify/           # Task complexity classification (Naive Bayes)
├── compressor/         # Output compaction (this is where you add tools)
├── core/               # Shared types and interfaces
├── gateway/            # Harness integration (hooks)
├── routing/            # Model routing decisions
├── session/            # Session state management
└── telemetry/          # Metrics and observability
```

### Key Interfaces

```go
// Compressor transforms verbose output to compact form
type Compressor interface {
    Name() string
    Match(cmd string) bool
    Compact(output string, exitCode int) (string, bool)
}

// Classifier determines task complexity tier
type Classifier interface {
    Classify(prompt string) (Tier, Confidence, error)
}

// Router selects the appropriate model for a task
type Router interface {
    Route(task Task) (Model, error)
}
```

---

## Pull Request Process

1. **Fork and branch** — Create a feature branch from `main`
2. **Make changes** — Follow the guidelines above
3. **Test locally** — Run `go test ./...` and ensure all pass
4. **Update docs** — Add to supported-tools.md if adding a compressor
5. **Submit PR** — Use a clear title: `feat(compressor): add mytool support`

### Commit Message Format

```
type(scope): description

feat(compressor): add pytest verbose mode support
fix(gateway): handle empty hook response
docs(readme): clarify installation steps
test(classify): add edge case for empty prompts
```

### PR Checklist

- [ ] Gate passes locally (`make check`)
- [ ] New compressor has test fixtures
- [ ] Documentation updated
- [ ] No breaking changes (or clearly marked)

---

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/tiagovilasboas/downshift/issues)
- **Discussions**: [GitHub Discussions](https://github.com/tiagovilasboas/downshift/discussions)

---

## License

By contributing, you agree that your contributions will be licensed under the
Apache 2.0 License.
