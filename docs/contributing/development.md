# Development Setup

Guide for contributing to Downshift.

## Prerequisites

- Go 1.27+
- Make

## Clone and Build

```bash
git clone https://github.com/tiagovilasboas/downshift.git
cd downshift
make build
```

## Run Tests

```bash
make test          # All tests
make test-race     # With race detector
make test-cover    # With coverage
```

## Project Structure

```
cmd/downshift/     CLI entry point
internal/
  adapt/           Adaptive tier memory
  catalog/         Model catalog
  classify/        Task classifier
  codeintel/       DCI (code intelligence)
  compressor/      Output compaction
  core/            Routing engine
  telemetry/       Local observability
  ...
docs/              Documentation
```

## Code Style

```bash
make fmt           # Format code
make lint          # Run linters
make vet           # Go vet
```

## Adding a Feature

1. Create a branch: `git checkout -b feat/my-feature`
2. Write tests first
3. Implement the feature
4. Run `make test`
5. Submit PR

## See Also

- [Adding Compressors](compressors.md)
- [Architecture Overview](../architecture/design.md)
