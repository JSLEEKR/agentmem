package agentmem

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestIntegrationFullWorkflow exercises the complete test-bench-check pipeline.
func TestIntegrationFullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "integration.json")
	ctx := context.Background()

	store := NewMemStore(WithFilePath(path), WithMaxSize(50))
	defer store.Close()

	// Phase 1: Populate store
	for i := 0; i < 30; i++ {
		entry := Entry{
			Key:      fmt.Sprintf("mem-%d", i),
			Value:    []byte(fmt.Sprintf("memory content for entry %d with some extra data", i)),
			Priority: i % 10,
			Metadata: map[string]string{
				"source": "test",
				"index":  fmt.Sprintf("%d", i),
			},
		}
		if i < 5 {
			entry.TTL = time.Nanosecond
			entry.CreatedAt = time.Now().Add(-time.Hour)
		} else {
			entry.TTL = time.Hour * 24
		}
		if err := store.Put(ctx, entry); err != nil {
			t.Fatalf("put: %v", err)
		}
	}

	// Phase 2: Run tests
	tester := NewTester(store)
	suites := tester.RunAll()
	for _, suite := range suites {
		if !suite.Passed() {
			for _, r := range suite.Results {
				if !r.Passed {
					t.Errorf("[%s] %s: %s", suite.Name, r.Name, r.Error)
				}
			}
		}
	}

	// Phase 3: Run benchmarks
	bencher := NewBencher(store, BenchConfig{
		Operations: 100,
		ValueSize:  128,
		PreFill:    20,
	})
	benchSuite := bencher.RunAll()
	for _, result := range benchSuite.Results {
		if result.OpsPerSec < 0 {
			t.Errorf("bench %s: ops/sec should not be negative", result.Name)
		}
	}

	// Phase 4: Run health check
	checker := NewChecker(store, "integration-store")
	checkReport := checker.Check(ctx)
	// We expect warnings for stale entries but not errors
	if checkReport.TotalEntries == 0 {
		t.Error("expected entries in store")
	}

	// Phase 5: Generate report
	report := NewReport(suites, &benchSuite, &checkReport)
	if report.Summary.TotalTests == 0 {
		t.Error("expected tests in report")
	}

	// Phase 6: Save and reload
	if err := store.Save(ctx); err != nil {
		t.Fatalf("save: %v", err)
	}
	store2 := NewMemStore(WithFilePath(path))
	defer store2.Close()
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}
	count, _ := store2.Count(ctx)
	if count == 0 {
		t.Error("store should have entries after load")
	}

	// Phase 7: Prune and verify
	removed, err := store.Prune(ctx)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed == 0 {
		t.Error("expected some entries to be pruned")
	}
}

func TestIntegrationConcurrentWorkload(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore(WithMaxSize(100))
	defer store.Close()

	const workers = 10
	const opsPerWorker = 100

	var wg sync.WaitGroup
	errCh := make(chan error, workers*4)

	// Writers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				err := store.Put(ctx, Entry{
					Key:   fmt.Sprintf("w%d-e%d", id, i),
					Value: []byte(fmt.Sprintf("worker %d entry %d", id, i)),
					TTL:   time.Hour,
				})
				if err != nil {
					errCh <- fmt.Errorf("writer %d put %d: %w", id, i, err)
					return
				}
			}
		}(w)
	}

	// Readers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				store.Get(ctx, fmt.Sprintf("w%d-e%d", id, i))
			}
		}(w)
	}

	// Listers
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				store.List(ctx)
			}
		}()
	}

	// Searchers
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				store.Search(ctx, []byte("worker"), 5)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count == 0 {
		t.Error("expected entries")
	}
}

func TestIntegrationPersistenceRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.json")
	ctx := context.Background()

	// Write entries with various types
	store := NewMemStore(WithFilePath(path))

	entries := []Entry{
		{Key: "text", Value: []byte("hello world"), Priority: 1},
		{Key: "binary", Value: []byte{0x00, 0xFF, 0x80, 0x7F}, Priority: 2},
		{Key: "unicode", Value: []byte("hello  unicode"), Priority: 3},
		{Key: "empty-meta", Value: []byte("no metadata"), Priority: 4},
		{Key: "rich-meta", Value: []byte("with metadata"), Priority: 5,
			Metadata: map[string]string{"a": "1", "b": "2", "c": "3"}},
		{Key: "with-ttl", Value: []byte("expiring"), Priority: 6, TTL: time.Hour},
		{Key: "large", Value: make([]byte, 10000), Priority: 7},
	}

	for _, e := range entries {
		if err := store.Put(ctx, e); err != nil {
			t.Fatalf("put %s: %v", e.Key, err)
		}
	}

	if err := store.Save(ctx); err != nil {
		t.Fatalf("save: %v", err)
	}
	store.Close()

	// Reload and verify
	store2 := NewMemStore(WithFilePath(path))
	defer store2.Close()
	if err := store2.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}

	for _, e := range entries {
		got, err := store2.Get(ctx, e.Key)
		if err != nil {
			t.Errorf("get %s: %v", e.Key, err)
			continue
		}
		if len(got.Value) != len(e.Value) {
			t.Errorf("%s value length: got %d, want %d", e.Key, len(got.Value), len(e.Value))
		}
		if got.Priority != e.Priority {
			t.Errorf("%s priority: got %d, want %d", e.Key, got.Priority, e.Priority)
		}
	}

	// Verify metadata
	rich, _ := store2.Get(ctx, "rich-meta")
	if rich.Metadata["a"] != "1" || rich.Metadata["b"] != "2" || rich.Metadata["c"] != "3" {
		t.Errorf("metadata mismatch: %v", rich.Metadata)
	}
}

func TestIntegrationPruningWorkflow(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore(WithMaxSize(5))
	defer store.Close()

	// Add 10 entries with different priorities
	for i := 0; i < 10; i++ {
		store.Put(ctx, Entry{
			Key:      fmt.Sprintf("prune-%d", i),
			Value:    []byte(fmt.Sprintf("data %d", i)),
			Priority: i * 10,
		})
	}

	count, _ := store.Count(ctx)
	if count != 10 {
		t.Fatalf("count = %d, want 10", count)
	}

	// Prune should reduce to maxSize
	removed, err := store.Prune(ctx)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 5 {
		t.Errorf("removed = %d, want 5", removed)
	}

	count, _ = store.Count(ctx)
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}

	// Top 5 priority entries should survive (priority 50-90)
	for i := 5; i < 10; i++ {
		_, err := store.Get(ctx, fmt.Sprintf("prune-%d", i))
		if err != nil {
			t.Errorf("high-priority entry %d was pruned", i)
		}
	}

	// Low priority entries should be gone
	for i := 0; i < 5; i++ {
		_, err := store.Get(ctx, fmt.Sprintf("prune-%d", i))
		if err == nil {
			t.Errorf("low-priority entry %d should have been pruned", i)
		}
	}
}

func TestIntegrationSearchAccuracy(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore()
	defer store.Close()

	// Store entries with known content
	store.Put(ctx, Entry{Key: "go-lang", Value: []byte("Go programming language by Google")})
	store.Put(ctx, Entry{Key: "rust-lang", Value: []byte("Rust programming language by Mozilla")})
	store.Put(ctx, Entry{Key: "python-lang", Value: []byte("Python programming language by PSF")})
	store.Put(ctx, Entry{Key: "cooking", Value: []byte("Best recipes for pasta and pizza")})
	store.Put(ctx, Entry{Key: "sports", Value: []byte("Football and basketball scores")})

	// Search for programming
	results, err := store.Search(ctx, []byte("programming language"), 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Programming entries should score higher than cooking/sports
	for _, r := range results {
		key := r.Entry.Key
		if key == "cooking" || key == "sports" {
			t.Errorf("non-programming entry %q should not be in top 3", key)
		}
	}
}

func TestIntegrationCheckerFindsIssues(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore()
	defer store.Close()

	// Add good entries
	store.Put(ctx, Entry{Key: "good-1", Value: []byte("fine"), TTL: time.Hour})
	store.Put(ctx, Entry{Key: "good-2", Value: []byte("also fine")})

	// Add stale entry
	store.Put(ctx, Entry{
		Key:       "stale-1",
		Value:     []byte("expired"),
		TTL:       time.Nanosecond,
		CreatedAt: time.Now().Add(-time.Hour),
	})

	checker := NewChecker(store, "mixed-store")
	report := checker.Check(ctx)

	if report.TotalEntries != 3 {
		t.Errorf("total = %d, want 3", report.TotalEntries)
	}
	if report.WarningCount() == 0 {
		t.Error("expected warnings for stale entry")
	}
}
