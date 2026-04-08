# agentmem

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-128-brightgreen?style=for-the-badge)](https://github.com/JSLEEKR/agentmem)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue?style=for-the-badge)]()

Testing framework for AI agent memory systems. Validates memory persistence, tests recall accuracy, detects staleness and corruption, benchmarks retrieval latency, and verifies pruning logic.

## Why This Exists

AI agents rely on memory systems to maintain context across conversations, store tool outputs, and track user preferences. These memory stores are critical infrastructure, but they are rarely tested systematically. When memory breaks — stale entries that should have expired, corrupted data that silently returns wrong values, pruning that deletes important memories, or concurrent access that races — agents produce wrong answers and users lose trust.

agentmem provides a structured testing framework that validates any agent memory backend. Define your store interface, point agentmem at it, and get immediate feedback on persistence correctness, recall accuracy, staleness detection, pruning behavior, concurrency safety, and performance characteristics.

Whether you are building a simple key-value memory for a chatbot, a vector store for RAG retrieval, or a file-based memory for Claude Code, agentmem tells you if your memory system actually works.

## Install

```bash
go install github.com/JSLEEKR/agentmem/cmd/agentmem@latest
```

Or as a library:

```bash
go get github.com/JSLEEKR/agentmem
```

## Quick Start

### CLI Usage

```bash
# Run all memory store tests
agentmem test

# Run specific test suite
agentmem test --suite recall
agentmem test --suite persistence
agentmem test --suite staleness
agentmem test --suite pruning
agentmem test --suite concurrency

# Run with JSON output
agentmem test --json

# Run performance benchmarks
agentmem bench

# Benchmark with custom parameters
agentmem bench --ops 5000 --value-size 512

# Health check an existing memory store file
agentmem check --store ./agent-memory.json

# Health check with JSON output
agentmem check --store ./agent-memory.json --json
```

### Library Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/JSLEEKR/agentmem"
)

func main() {
    // Create a memory store (or use your own implementation)
    store := agentmem.NewMemStore(
        agentmem.WithFilePath("./memory.json"),
        agentmem.WithMaxSize(1000),
    )
    defer store.Close()

    // Run all tests
    tester := agentmem.NewTester(store)
    suites := tester.RunAll()

    // Run benchmarks
    bencher := agentmem.NewBencher(store, agentmem.DefaultBenchConfig())
    bench := bencher.RunAll()

    // Run health check
    checker := agentmem.NewChecker(store, "my-agent-memory")
    check := checker.Check(context.Background())

    // Generate report
    report := agentmem.NewReport(suites, &bench, &check)
    report.WriteText(os.Stdout)

    if !report.Summary.AllPassed {
        fmt.Fprintf(os.Stderr, "%d tests failed\n", report.Summary.FailedTests)
        os.Exit(1)
    }
}
```

### Testing Your Own Memory Store

Implement the `Store` interface to test any backend:

```go
package mystore

import (
    "context"
    "github.com/JSLEEKR/agentmem"
)

type MyVectorStore struct {
    // your vector store fields
}

func (s *MyVectorStore) Put(ctx context.Context, entry agentmem.Entry) error {
    // store the entry
    return nil
}

func (s *MyVectorStore) Get(ctx context.Context, key string) (agentmem.Entry, error) {
    // retrieve by key
    return agentmem.Entry{}, nil
}

func (s *MyVectorStore) Delete(ctx context.Context, key string) error {
    // delete by key
    return nil
}

func (s *MyVectorStore) List(ctx context.Context) ([]agentmem.Entry, error) {
    // list all entries
    return nil, nil
}

func (s *MyVectorStore) Search(ctx context.Context, query []byte, limit int) ([]agentmem.SearchResult, error) {
    // vector similarity search
    return nil, nil
}

func (s *MyVectorStore) Close() error {
    return nil
}
```

Then test it:

```go
func TestMyStore(t *testing.T) {
    store := &MyVectorStore{}
    defer store.Close()

    tester := agentmem.NewTester(store)

    // Run recall tests
    recall := tester.TestRecall()
    if !recall.Passed() {
        t.Fatalf("recall tests failed")
    }

    // Run concurrency tests
    conc := tester.TestConcurrency()
    if !conc.Passed() {
        t.Fatalf("concurrency tests failed")
    }
}
```

## Architecture

```
agentmem/
├── store.go          # Core interfaces: Store, PrunableStore, PersistableStore
├── errors.go         # Sentinel errors: ErrNotFound, ErrStoreClosed, etc.
├── memstore.go       # Reference in-memory implementation with file persistence
├── tester.go         # Test framework: persistence, recall, staleness, pruning, concurrency
├── bench.go          # Benchmark framework: put/get/delete/search/list latency
├── checker.go        # Health checker: staleness, corruption, metadata, accessibility
├── report.go         # Report generation: JSON and human-readable output
└── cmd/agentmem/
    └── main.go       # CLI: test, bench, check commands
```

### Core Interfaces

**Store** — The base interface every memory backend must implement:
- `Put(ctx, entry)` — Store an entry (upsert semantics)
- `Get(ctx, key)` — Retrieve by key
- `Delete(ctx, key)` — Remove by key
- `List(ctx)` — List all entries
- `Search(ctx, query, limit)` — Similarity search
- `Close()` — Release resources

**PrunableStore** — Extends Store with pruning:
- `Prune(ctx)` — Remove expired/low-priority entries
- `Count(ctx)` — Count total entries

**PersistableStore** — Extends Store with persistence:
- `Save(ctx)` — Write to durable storage
- `Load(ctx)` — Read from durable storage

### Test Suites

| Suite | What It Tests |
|-------|---------------|
| **Recall** | Exact recall, overwrite, not-found, search, binary values, large values |
| **Staleness** | TTL expiration, zero-TTL immortality, staleness scanning |
| **Pruning** | Expired entry removal, max-size enforcement, priority preservation |
| **Persistence** | Save/load roundtrip, metadata survival, empty store handling |
| **Concurrency** | Concurrent put/get, put/delete, list-during-writes, concurrent search |

### Benchmark Operations

| Operation | What It Measures |
|-----------|-----------------|
| **put** | Write latency for new entries |
| **get** | Read latency for existing entries |
| **delete** | Deletion latency |
| **search** | Similarity search latency |
| **list** | Full scan latency |

Each benchmark reports: operations, min/max/avg latency, P50/P95/P99 percentiles, operations per second.

### Health Checks

| Category | Severity | What It Detects |
|----------|----------|-----------------|
| **staleness** | warning | Expired entries still in store |
| **corruption** | error | Empty values, zero timestamps, future timestamps, negative TTL |
| **corruption** | warning | Updated-before-created timestamp anomaly |
| **metadata** | warning | Empty metadata keys, unusually large metadata |
| **metadata** | info | Empty metadata values |
| **accessibility** | error | Entries that List returns but Get cannot retrieve |

## Entry Model

```go
type Entry struct {
    Key       string            // Unique identifier
    Value     []byte            // Stored content (any format)
    Metadata  map[string]string // Arbitrary key-value pairs
    CreatedAt time.Time         // First stored timestamp
    UpdatedAt time.Time         // Last modified timestamp
    TTL       time.Duration     // Time-to-live (0 = no expiration)
    Priority  int               // Importance for pruning (higher = keep)
}
```

## Report Formats

### Human-Readable (default)

```
=== Agent Memory Test Report ===
Generated: 2026-04-07T10:00:00Z

--- Test Results ---

[PASS] Recall (1ms)
  [+] exact_recall (100us)
  [+] overwrite_recall (50us)
  [+] not_found_recall (10us)

--- Benchmark Results ---
Operation       Ops          Avg          P50          P95          P99      Ops/sec
----------------------------------------------------------------------------------
put            1000          5us          4us         12us         25us      200000
get            1000          2us          1us          5us         10us      500000

--- Health Check ---
Store: my-memory — HEALTHY
Entries: 150
Errors: 0, Warnings: 2

--- Summary ---
Tests: 20/20 passed
Result: ALL TESTS PASSED
```

### JSON

```bash
agentmem test --json | jq '.summary'
```

```json
{
  "total_tests": 20,
  "passed_tests": 20,
  "failed_tests": 0,
  "all_passed": true,
  "healthy": true
}
```

## CLI Reference

### `agentmem test`

Run memory store validation tests.

| Flag | Description | Default |
|------|-------------|---------|
| `--store <path>` | Path to store file | temp directory |
| `--max-size <n>` | Max store size for pruning tests | 10 |
| `--suite <name>` | Run specific suite | all |
| `--json` | JSON output | text |

### `agentmem bench`

Run performance benchmarks.

| Flag | Description | Default |
|------|-------------|---------|
| `--ops <n>` | Operations per benchmark | 1000 |
| `--value-size <n>` | Value size in bytes | 256 |
| `--pre-fill <n>` | Pre-fill entry count | 100 |
| `--json` | JSON output | text |

### `agentmem check`

Run health checks on an existing memory store.

| Flag | Description | Default |
|------|-------------|---------|
| `--store <path>` | Path to store file | required |
| `--json` | JSON output | text |

## Use Cases

1. **CI/CD validation** — Run `agentmem test` in your pipeline to catch memory regressions
2. **Performance tracking** — Run `agentmem bench --json` and compare P95 latency across releases
3. **Production health** — Run `agentmem check --store /path/to/agent/memory.json` to detect corruption
4. **Custom backend testing** — Implement the Store interface and reuse all test suites
5. **Concurrency verification** — Validate thread-safety before deploying multi-agent systems

## Dependencies

Zero external dependencies. Built entirely on Go standard library.

## License

MIT
