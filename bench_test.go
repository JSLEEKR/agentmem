package agentmem

import (
	"testing"
	"time"
)

func TestBencherBenchPut(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 100, ValueSize: 64, PreFill: 10})
	result := b.BenchPut()

	if result.Name != "put" {
		t.Errorf("name = %q, want put", result.Name)
	}
	if result.Operations != 100 {
		t.Errorf("ops = %d, want 100", result.Operations)
	}
	if result.AvgLatency < 0 {
		t.Error("avg latency should not be negative")
	}
	if result.OpsPerSec < 0 {
		t.Error("ops/sec should not be negative")
	}
}

func TestBencherBenchGet(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 100, ValueSize: 64, PreFill: 10})
	result := b.BenchGet()

	if result.Name != "get" {
		t.Errorf("name = %q, want get", result.Name)
	}
	if result.Operations != 100 {
		t.Errorf("ops = %d, want 100", result.Operations)
	}
}

func TestBencherBenchDelete(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 100, ValueSize: 64, PreFill: 10})
	result := b.BenchDelete()

	if result.Name != "delete" {
		t.Errorf("name = %q, want delete", result.Name)
	}
	if result.Operations != 100 {
		t.Errorf("ops = %d, want 100", result.Operations)
	}
}

func TestBencherBenchSearch(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 100, ValueSize: 64, PreFill: 20})
	result := b.BenchSearch()

	if result.Name != "search" {
		t.Errorf("name = %q, want search", result.Name)
	}
	if result.Operations <= 0 {
		t.Error("expected positive operations")
	}
}

func TestBencherBenchList(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 100, ValueSize: 64, PreFill: 20})
	result := b.BenchList()

	if result.Name != "list" {
		t.Errorf("name = %q, want list", result.Name)
	}
	if result.Operations <= 0 {
		t.Error("expected positive operations")
	}
}

func TestBencherRunAll(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	b := NewBencher(s, BenchConfig{Operations: 50, ValueSize: 32, PreFill: 10})
	suite := b.RunAll()

	if suite.Name != "Memory Store Benchmark" {
		t.Errorf("name = %q", suite.Name)
	}
	if len(suite.Results) != 5 {
		t.Errorf("expected 5 results, got %d", len(suite.Results))
	}
	if suite.Duration < 0 {
		t.Error("duration should be non-negative")
	}
	// Verify individual results have operations counted
	for _, r := range suite.Results {
		if r.Operations == 0 {
			t.Errorf("result %q has 0 operations", r.Name)
		}
	}
}

func TestDefaultBenchConfig(t *testing.T) {
	config := DefaultBenchConfig()
	if config.Operations != 1000 {
		t.Errorf("Operations = %d, want 1000", config.Operations)
	}
	if config.ValueSize != 256 {
		t.Errorf("ValueSize = %d, want 256", config.ValueSize)
	}
	if config.PreFill != 100 {
		t.Errorf("PreFill = %d, want 100", config.PreFill)
	}
}

func TestComputeStats(t *testing.T) {
	latencies := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	result := computeStats("test", latencies)
	if result.Name != "test" {
		t.Errorf("name = %q, want test", result.Name)
	}
	if result.Operations != 5 {
		t.Errorf("ops = %d, want 5", result.Operations)
	}
	if result.MinLatency != 1*time.Millisecond {
		t.Errorf("min = %v, want 1ms", result.MinLatency)
	}
	if result.MaxLatency != 5*time.Millisecond {
		t.Errorf("max = %v, want 5ms", result.MaxLatency)
	}
	if result.AvgLatency != 3*time.Millisecond {
		t.Errorf("avg = %v, want 3ms", result.AvgLatency)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	result := computeStats("empty", nil)
	if result.Operations != 0 {
		t.Errorf("ops = %d, want 0", result.Operations)
	}
}

func TestBenchResultPercentiles(t *testing.T) {
	latencies := make([]time.Duration, 100)
	for i := range latencies {
		latencies[i] = time.Duration(i+1) * time.Microsecond
	}
	result := computeStats("percentiles", latencies)

	// P50 should be around 50us
	if result.P50Latency < 45*time.Microsecond || result.P50Latency > 55*time.Microsecond {
		t.Errorf("P50 = %v, expected around 50us", result.P50Latency)
	}
	// P95 should be around 95us
	if result.P95Latency < 90*time.Microsecond || result.P95Latency > 100*time.Microsecond {
		t.Errorf("P95 = %v, expected around 95us", result.P95Latency)
	}
	// P99 should be around 99us
	if result.P99Latency < 95*time.Microsecond || result.P99Latency > 100*time.Microsecond {
		t.Errorf("P99 = %v, expected around 99us", result.P99Latency)
	}
}
