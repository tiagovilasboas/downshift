# Adding Compressors

Guide for adding new output compressors to Downshift.

## When to Add

Add a compressor when:
- A command produces verbose, repetitive output
- The essential information can be summarized concisely
- `downshift learning suggestions` shows it as a candidate

## Structure

Compressors live in `internal/compressor/`:

```go
// internal/compressor/mycommand.go

package compressor

import "regexp"

var (
    reMyCommandHeader = regexp.MustCompile(`...`)
    reMyCommandPass   = regexp.MustCompile(`...`)
)

func compressMyCommand(output []byte, exit int) []byte {
    // Extract essential info
    // Return compact summary
}
```

## Registration

Add to the compressor registry:

```go
// internal/compressor/registry.go

var compressors = []Compressor{
    // ... existing
    {Match: matchMyCommand, Compress: compressMyCommand},
}
```

## Guidelines

### Preserve Information

Good:
```
✓ PASS · 54 packages · 0.156s
```

Bad:
```
✓ OK
```

### Expand on Failure

```go
if exit != 0 {
    // Include failure details
    return append(summary, failureLines...)
}
```

### Test Thoroughly

```go
// internal/compressor/mycommand_test.go

func TestCompressMyCommand_Pass(t *testing.T) { ... }
func TestCompressMyCommand_Fail(t *testing.T) { ... }
func TestCompressMyCommand_Empty(t *testing.T) { ... }
```

## Example: go test

```go
func compressGoTest(output []byte, exit int) []byte {
    lines := bytes.Split(output, []byte("\n"))
    
    var passed, failed int
    var duration string
    
    for _, line := range lines {
        if m := reGoTestOK.FindSubmatch(line); m != nil {
            passed++
            duration = string(m[1])
        }
        if m := reGoTestFail.FindSubmatch(line); m != nil {
            failed++
        }
    }
    
    if exit == 0 {
        return fmt.Appendf(nil, "✓ PASS · %d packages · %s", passed, duration)
    }
    return fmt.Appendf(nil, "✗ FAIL · %d passed, %d failed", passed, failed)
}
```

## Testing Locally

```bash
# Build with your changes
make build

# Test compression
echo "your test output" | ./downshift context compress
```

## Submit PR

1. Add compressor + tests
2. Update `docs/reference/supported-tools.md`
3. Run `make test`
4. Submit PR with example output
