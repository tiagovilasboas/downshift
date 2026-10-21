# Compaction measurement: corpus results

Date: 2026-10-21
Commit: TBD (to be filled after commit)
Command: `go test -v -run TestMeasureCorpusCompaction ./internal/compressor/`

## Summary

| Metric | Value |
|--------|-------|
| Corpus files | 248 |
| Files compacted | 26 (10.5%) |
| Files unchanged | 222 (89.5%) |
| Overall bytes before | 62,309 |
| Overall bytes after | 47,532 |
| Overall reduction | 23.7% |
| Compacted files reduction | 82.3% |

## What this measures

The corpus contains synthetic test fixtures representing various command outputs.
Many fixtures are intentionally unchanged (failures, edge cases, non-compactable
formats) to test the conservative compaction policy.

The **82.3% reduction on compacted files** is the relevant figure for clean,
success-case outputs. The **23.7% overall** includes all test fixtures, many
designed to stay raw.

## Top 10 compacted

| File | Before | After | Reduction |
|------|--------|-------|-----------|
| flutter_pass_compact.txt | 2,113 | 55 | 97.4% |
| dart_pass_compact.txt | 2,001 | 55 | 97.3% |
| cargo_pass_clean.txt | 767 | 38 | 95.0% |
| pytest_pass_clean.txt | 679 | 34 | 95.0% |
| playwright_pass_clean.txt | 422 | 37 | 91.2% |
| tsc_pass_clean.txt | 183 | 21 | 88.5% |
| cypress_pass_clean.txt | 910 | 105 | 88.5% |
| maven_pass_package.txt | 2,170 | 251 | 88.4% |
| maven_pass_test.txt | 2,043 | 241 | 88.2% |
| vitest_pass_clean.txt | 358 | 49 | 86.3% |

## What this does NOT measure

- Real-world command frequency (which commands run most often)
- Token cost (this measures bytes, not LLM tokens)
- Actual billing impact (which depends on your usage patterns)

## Reproducing

```bash
cd /path/to/downshift
go test -v -run TestMeasureCorpusCompaction ./internal/compressor/
```

## Evidence classification

This is **Rung B** evidence per [evidence.md](../guides/evidence.md): measured on
synthetic fixtures, not real-world traffic or billing data.
