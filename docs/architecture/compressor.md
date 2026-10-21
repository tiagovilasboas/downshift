# Compressor Pipeline

The compressor transforms verbose command output into compact summaries while
preserving all essential information.

## Pipeline Stages

```
Command Output → Detect Format → Apply Compressor → Emit Summary
                      ↓                  ↓
               Unknown format     Learning system
               (pass-through)     (observe pattern)
```

## Format Detection

Each compressor registers patterns it can handle:

```go
type Compressor interface {
    Match(output []byte) bool
    Compress(output []byte, exitCode int) []byte
}
```

Detection is fast (regex-based) and runs in order of specificity.

## Compressor Types

### Test Runners

Go, Rust, Python, Node.js, PHP, etc.

```
Input:  50 lines of "ok pkg/..." 
Output: "✓ PASS · 50 packages · 0.156s"
```

### Build Tools

Cargo, Go build, npm, etc.

```
Input:  100 lines of compilation
Output: "✓ BUILD · 0 warnings · 2.3s"
```

### Git

Status, log, diff, etc.

```
Input:  20 lines of status output
Output: "✓ clean working tree"
```

## Exit Code Handling

Compressors preserve exit codes and expand failure output:

```
Exit 0 → Compact summary
Exit 1 → Summary + failure details + last N lines
```

## Learning Integration

Unknown formats are observed by the learning system:

1. Command completes with unrecognized output
2. Learning system records the signature and the output's shape (counts, never text)
3. After N observations, surfaces as suggestion

See [Compressor Learning](../features/learning.md).

## Adding Compressors

See [Contributing Compressors](../contributing/compressors.md).
