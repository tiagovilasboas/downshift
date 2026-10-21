# Code Intelligence (DCI)

Downshift Code Intelligence extracts structural context from your codebase and
injects it into agent prompts automatically. Instead of the agent reading files
to understand your code, DCI provides the relevant symbols, callers, and blast
radius upfront.

## The Problem

When an agent needs to understand code, it typically:

1. Reads the file you mentioned
2. Finds imports and follows them
3. Reads more files to understand types
4. Searches for usages across the codebase
5. Builds a mental model from all this text

This exploration consumes thousands of tokens and still misses context.

## The Solution

DCI indexes your codebase once and answers structural questions instantly:

```
"What calls this function?" → 3 callers in 2 files
"What does this type look like?" → struct definition + methods
"What's the blast radius?" → 12 files could be affected
```

The agent gets precise answers without reading file after file.

## How It Works

### Indexing

DCI builds a lightweight index of your codebase:

```bash
downshift dci enable
```

The index captures:
- **Symbols** — functions, types, constants, variables
- **References** — who calls what, who imports what
- **Structure** — packages, modules, inheritance

Indexing runs in the background and updates incrementally on file changes.

### Query

When enabled, DCI intercepts agent prompts and enriches them with relevant
context before they reach the model:

```
Original prompt:
  "Refactor the PaymentService to use the new gateway"

DCI-enriched prompt:
  "Refactor the PaymentService to use the new gateway

   [Context: PaymentService]
   - Defined in: internal/payment/service.go:45
   - Methods: Process, Refund, Validate
   - Called by: CheckoutHandler, SubscriptionRenewer
   - Depends on: Gateway interface, Transaction type

   [Context: Gateway interface]
   - Defined in: internal/payment/gateway.go:12
   - Implementations: StripeGateway, PayPalGateway
   - Methods: Charge, Refund, Validate"
```

The model sees the structure without reading the files.

## Commands

### Enable/Disable

```bash
downshift dci enable   # Start indexing, enable injection
downshift dci disable  # Stop injection (index preserved)
```

### Query Blast Radius

See what a change might affect:

```bash
downshift dci impact internal/payment/service.go:PaymentService
```

Output:
```
PaymentService (internal/payment/service.go:45)
├── Called by:
│   ├── CheckoutHandler (internal/api/checkout.go:78)
│   ├── SubscriptionRenewer (internal/jobs/renew.go:34)
│   └── RefundProcessor (internal/jobs/refund.go:22)
├── Depends on:
│   ├── Gateway (internal/payment/gateway.go:12)
│   ├── Transaction (internal/payment/types.go:8)
│   └── Logger (pkg/log/logger.go:15)
└── Blast radius: 12 files

Changes to PaymentService may require updates in:
  - internal/api/checkout.go
  - internal/api/checkout_test.go
  - internal/jobs/renew.go
  - internal/jobs/renew_test.go
  ...
```

### Inspect Context

Preview what DCI would inject for a prompt:

```bash
downshift dci inspect "refactor PaymentService"
```

Shows the exact context that would be added.

### Benchmark

Measure indexing performance:

```bash
downshift dci benchmark
```

## Supported Languages

| Language | Symbols | References | Blast Radius |
|----------|---------|------------|--------------|
| Go | ✅ | ✅ | ✅ |
| TypeScript | ✅ | ✅ | ✅ |
| JavaScript | ✅ | ✅ | ✅ |
| Python | ✅ | ✅ | ✅ |
| Rust | ✅ | ✅ | ✅ |
| PHP | ✅ | ✅ | ✅ |
| Java | ✅ | ✅ | ✅ |
| C# | ✅ | ✅ | ✅ |

## Token Savings

DCI reduces tokens in two ways:

1. **Fewer file reads** — The agent doesn't need to explore to understand
2. **Precise context** — Only relevant symbols, not entire files

Typical savings: 30-50% fewer tokens on refactoring and cross-file changes.

## Privacy

DCI runs entirely locally:
- Index stored in `~/.downshift/dci/`
- No code sent to external services
- No telemetry beyond opt-in usage stats

## Configuration

```bash
# Set languages to index (default: auto-detect)
downshift config set dci.languages "go,typescript,python"

# Set directories to exclude
downshift config set dci.exclude "vendor,node_modules,dist"

# Set max index size (default: 100MB)
downshift config set dci.max_index_size "200MB"
```

## Troubleshooting

### Index not updating

```bash
downshift dci benchmark  # Check indexer health
downshift dci disable && downshift dci enable  # Force reindex
```

### Context not injected

Verify DCI is enabled and the file type is supported:

```bash
downshift doctor --verbose
```

### Large codebase slow to index

Exclude generated code and vendored dependencies:

```bash
downshift config set dci.exclude "vendor,node_modules,dist,generated"
```

---

## How It Compares

| Approach | Tokens | Accuracy | Latency |
|----------|--------|----------|---------|
| Agent reads files | High | Variable | Slow |
| RAG/embeddings | Medium | Variable | Medium |
| **DCI structural index** | **Low** | **High** | **Fast** |

DCI provides deterministic, accurate context at a fraction of the token cost.
