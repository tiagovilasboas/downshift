# Bench baseline

`internal/compressor/testdata/bench/baseline.tsv` is the floor for the
compaction benchmark: per family, the fixture count, raw bytes and reduced bytes
(integers), plus `@lossless`, `@lossy` and `@total` sums. `TestBenchGuard` in
`internal/compressor` is never skipped. It re-runs the bench corpus with rtk off
and the CCR column ignored, and fails when:

- a family's saving falls more than 2.0 points below its row **and** its reduced
  size grew by more than 8 bytes (the byte floor protects one-fixture families);
- a family has fewer fixtures than its row, or vanished;
- a family compacts but has no row;
- `@lossless`, `@lossy` or `@total` lose more than 0.5 points.

## Update the baseline

```
make bench-baseline   # rewrites baseline.tsv
make bench            # rewrites docs/reference/bench.md
```

Commit both files; the TSV change is the reviewable diff. An improvement or a
new fixture needs no more than that. Running `make bench` twice must leave
`git diff` empty (the rtk columns are off by default: rtk output is not stable).

## Lowering a floor

Owner decision BN-D1: a lowered family or aggregate floor needs a stated reason
in the PR text **and** a line in `CHANGELOG.md`. The CHANGELOG line is the audit
trail once the PR closes. A lower saving with no reason is a regression, not a
baseline update.

## Line endings

`.gitattributes` marks `internal/compressor/testdata/**` as `-text`, so git never
rewrites fixture bytes. `TestCorpusNoUnintendedCR` allows a carriage return only
in the fixtures listed in `intendedCR` (CRLF transcripts, terminal progress).
