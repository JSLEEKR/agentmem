package agentmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// BenchResult holds the result of a single benchmark.
type BenchResult struct {
	Name       string        `json:"name"`
	Operations int           `json:"operations"`
	TotalTime  time.Duration `json:"total_time"`
	MinLatency time.Duration `json:"min_latency"`
	MaxLatency time.Duration `json:"max_latency"`
	AvgLatency time.Duration `json:"avg_latency"`
	P50Latency time.Duration `json:"p50_latency"`
	P95Latency time.Duration `json:"p95_latency"`
	P99Latency time.Duration `json:"p99_latency"`
	OpsPerSec  float64       `json:"ops_per_sec"`
}

// BenchSuite holds a collection of benchmark results.
type BenchSuite struct {
	Name      string        `json:"name"`
	Results   []BenchResult `json:"results"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
}

// BenchConfig configures benchmark parameters.
type BenchConfig struct {
	// Operations is the number of operations per benchmark.
	Operations int
	// ValueSize is the size of values in bytes.
	ValueSize int
	// PreFill is the number of entries to pre-fill before benchmarking.
	PreFill int
}

// DefaultBenchConfig returns a reasonable default configuration.
func DefaultBenchConfig() BenchConfig {
	return BenchConfig{
		Operations: 1000,
		ValueSize:  256,
		PreFill:    100,
	}
}

// Bencher runs benchmarks against a Store implementation.
type Bencher struct {
	store  Store
	config BenchConfig
}

// NewBencher creates a new Bencher.
func NewBencher(store Store, config BenchConfig) *Bencher {
	return &Bencher{store: store, config: config}
}

func (b *Bencher) makeValue(size int) []byte {
	val := make([]byte, size)
	for i := range val {
		val[i] = byte(i % 256)
	}
	return val
}

func computeStats(name string, latencies []time.Duration) BenchResult {
	n := len(latencies)
	if n == 0 {
		return BenchResult{Name: name}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var total time.Duration
	for _, l := range latencies {
		total += l
	}

	opsPerSec := 0.0
	if total > 0 {
		opsPerSec = float64(n) / total.Seconds()
	}

	return BenchResult{
		Name:       name,
		Operations: n,
		TotalTime:  total,
		MinLatency: latencies[0],
		MaxLatency: latencies[n-1],
		AvgLatency: total / time.Duration(n),
		P50Latency: latencies[int(math.Floor(float64(n)*0.50))],
		P95Latency: latencies[int(math.Floor(float64(n)*0.95))],
		P99Latency: latencies[int(math.Floor(float64(n)*0.99))],
		OpsPerSec:  opsPerSec,
	}
}

// BenchPut benchmarks Put operations.
func (b *Bencher) BenchPut() BenchResult {
	ctx := context.Background()
	val := b.makeValue(b.config.ValueSize)
	latencies := make([]time.Duration, 0, b.config.Operations)

	for i := 0; i < b.config.Operations; i++ {
		key := fmt.Sprintf("bench-put-%d", i)
		start := time.Now()
		_ = b.store.Put(ctx, Entry{Key: key, Value: val})
		latencies = append(latencies, time.Since(start))
	}

	return computeStats("put", latencies)
}

// BenchGet benchmarks Get operations.
func (b *Bencher) BenchGet() BenchResult {
	ctx := context.Background()
	val := b.makeValue(b.config.ValueSize)

	// Pre-fill
	for i := 0; i < b.config.Operations; i++ {
		_ = b.store.Put(ctx, Entry{Key: fmt.Sprintf("bench-get-%d", i), Value: val})
	}

	latencies := make([]time.Duration, 0, b.config.Operations)
	for i := 0; i < b.config.Operations; i++ {
		key := fmt.Sprintf("bench-get-%d", i)
		start := time.Now()
		_, _ = b.store.Get(ctx, key)
		latencies = append(latencies, time.Since(start))
	}

	return computeStats("get", latencies)
}

// BenchDelete benchmarks Delete operations.
func (b *Bencher) BenchDelete() BenchResult {
	ctx := context.Background()
	val := b.makeValue(b.config.ValueSize)

	// Pre-fill
	for i := 0; i < b.config.Operations; i++ {
		_ = b.store.Put(ctx, Entry{Key: fmt.Sprintf("bench-del-%d", i), Value: val})
	}

	latencies := make([]time.Duration, 0, b.config.Operations)
	for i := 0; i < b.config.Operations; i++ {
		key := fmt.Sprintf("bench-del-%d", i)
		start := time.Now()
		_ = b.store.Delete(ctx, key)
		latencies = append(latencies, time.Since(start))
	}

	return computeStats("delete", latencies)
}

// BenchSearch benchmarks Search operations.
func (b *Bencher) BenchSearch() BenchResult {
	ctx := context.Background()
	val := b.makeValue(b.config.ValueSize)

	// Pre-fill
	for i := 0; i < b.config.PreFill; i++ {
		_ = b.store.Put(ctx, Entry{Key: fmt.Sprintf("bench-search-%d", i), Value: val})
	}

	ops := b.config.Operations / 10 // search is more expensive
	if ops < 10 {
		ops = 10
	}
	latencies := make([]time.Duration, 0, ops)
	query := b.makeValue(b.config.ValueSize / 2)

	for i := 0; i < ops; i++ {
		start := time.Now()
		_, _ = b.store.Search(ctx, query, 10)
		latencies = append(latencies, time.Since(start))
	}

	return computeStats("search", latencies)
}

// BenchList benchmarks List operations.
func (b *Bencher) BenchList() BenchResult {
	ctx := context.Background()
	val := b.makeValue(b.config.ValueSize)

	// Pre-fill
	for i := 0; i < b.config.PreFill; i++ {
		_ = b.store.Put(ctx, Entry{Key: fmt.Sprintf("bench-list-%d", i), Value: val})
	}

	ops := b.config.Operations / 10
	if ops < 10 {
		ops = 10
	}
	latencies := make([]time.Duration, 0, ops)

	for i := 0; i < ops; i++ {
		start := time.Now()
		_, _ = b.store.List(ctx)
		latencies = append(latencies, time.Since(start))
	}

	return computeStats("list", latencies)
}

// RunAll runs all benchmarks and returns results.
func (b *Bencher) RunAll() BenchSuite {
	start := time.Now()
	suite := BenchSuite{
		Name:      "Memory Store Benchmark",
		StartedAt: start,
	}

	suite.Results = append(suite.Results,
		b.BenchPut(),
		b.BenchGet(),
		b.BenchDelete(),
		b.BenchSearch(),
		b.BenchList(),
	)

	suite.Duration = time.Since(start)
	return suite
}
