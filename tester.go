package agentmem

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TestResult represents the outcome of a single test.
type TestResult struct {
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// TestSuite holds a collection of test results.
type TestSuite struct {
	Name      string        `json:"name"`
	Results   []TestResult  `json:"results"`
	StartedAt time.Time    `json:"started_at"`
	Duration  time.Duration `json:"duration"`
}

// Passed returns true if all tests in the suite passed.
func (s *TestSuite) Passed() bool {
	for _, r := range s.Results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// PassCount returns the number of passing tests.
func (s *TestSuite) PassCount() int {
	n := 0
	for _, r := range s.Results {
		if r.Passed {
			n++
		}
	}
	return n
}

// FailCount returns the number of failing tests.
func (s *TestSuite) FailCount() int {
	return len(s.Results) - s.PassCount()
}

// Tester runs test suites against a Store implementation.
type Tester struct {
	store Store
}

// NewTester creates a new Tester for the given store.
func NewTester(store Store) *Tester {
	return &Tester{store: store}
}

// runTest executes a named test function and records the result.
func (t *Tester) runTest(name string, fn func(context.Context) error) TestResult {
	ctx := context.Background()
	start := time.Now()
	err := fn(ctx)
	dur := time.Since(start)

	result := TestResult{
		Name:     name,
		Passed:   err == nil,
		Duration: dur,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// TestPersistence runs persistence tests. The store must implement PersistableStore.
func (t *Tester) TestPersistence() TestSuite {
	start := time.Now()
	suite := TestSuite{Name: "Persistence", StartedAt: start}

	ps, ok := t.store.(PersistableStore)
	if !ok {
		suite.Results = append(suite.Results, TestResult{
			Name:   "store_implements_persistable",
			Passed: false,
			Error:  "store does not implement PersistableStore interface",
		})
		suite.Duration = time.Since(start)
		return suite
	}

	suite.Results = append(suite.Results, t.runTest("save_load_roundtrip", func(ctx context.Context) error {
		// Store some entries
		entries := []Entry{
			{Key: "persist-1", Value: []byte("hello world"), Priority: 5},
			{Key: "persist-2", Value: []byte("agent memory"), Priority: 10,
				Metadata: map[string]string{"type": "test"}},
			{Key: "persist-3", Value: []byte("persistence check"), TTL: time.Hour},
		}
		for _, e := range entries {
			if err := t.store.Put(ctx, e); err != nil {
				return fmt.Errorf("put %s: %w", e.Key, err)
			}
		}

		// Save
		if err := ps.Save(ctx); err != nil {
			return fmt.Errorf("save: %w", err)
		}

		// Clear and reload
		for _, e := range entries {
			_ = t.store.Delete(ctx, e.Key)
		}

		if err := ps.Load(ctx); err != nil {
			return fmt.Errorf("load: %w", err)
		}

		// Verify
		for _, e := range entries {
			got, err := t.store.Get(ctx, e.Key)
			if err != nil {
				return fmt.Errorf("get %s after load: %w", e.Key, err)
			}
			if string(got.Value) != string(e.Value) {
				return fmt.Errorf("value mismatch for %s: got %q, want %q", e.Key, got.Value, e.Value)
			}
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("metadata_survives_roundtrip", func(ctx context.Context) error {
		entry := Entry{
			Key:      "meta-persist",
			Value:    []byte("with-metadata"),
			Metadata: map[string]string{"role": "assistant", "source": "chat"},
			Priority: 7,
		}
		if err := t.store.Put(ctx, entry); err != nil {
			return fmt.Errorf("put: %w", err)
		}
		if err := ps.Save(ctx); err != nil {
			return fmt.Errorf("save: %w", err)
		}
		_ = t.store.Delete(ctx, entry.Key)
		if err := ps.Load(ctx); err != nil {
			return fmt.Errorf("load: %w", err)
		}
		got, err := t.store.Get(ctx, entry.Key)
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		if got.Metadata["role"] != "assistant" || got.Metadata["source"] != "chat" {
			return fmt.Errorf("metadata mismatch: got %v", got.Metadata)
		}
		if got.Priority != 7 {
			return fmt.Errorf("priority mismatch: got %d, want 7", got.Priority)
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("empty_store_save_load", func(ctx context.Context) error {
		// Should handle empty store gracefully
		if err := ps.Save(ctx); err != nil {
			return fmt.Errorf("save empty: %w", err)
		}
		if err := ps.Load(ctx); err != nil {
			return fmt.Errorf("load empty: %w", err)
		}
		return nil
	}))

	suite.Duration = time.Since(start)
	return suite
}

// TestRecall runs recall accuracy tests.
func (t *Tester) TestRecall() TestSuite {
	start := time.Now()
	suite := TestSuite{Name: "Recall", StartedAt: start}

	suite.Results = append(suite.Results, t.runTest("exact_recall", func(ctx context.Context) error {
		entries := map[string]string{
			"recall-1": "The capital of France is Paris",
			"recall-2": "Go was created at Google",
			"recall-3": "Memory testing is important",
		}
		for k, v := range entries {
			if err := t.store.Put(ctx, Entry{Key: k, Value: []byte(v)}); err != nil {
				return fmt.Errorf("put %s: %w", k, err)
			}
		}
		for k, v := range entries {
			got, err := t.store.Get(ctx, k)
			if err != nil {
				return fmt.Errorf("get %s: %w", k, err)
			}
			if string(got.Value) != v {
				return fmt.Errorf("recall mismatch for %s: got %q, want %q", k, got.Value, v)
			}
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("overwrite_recall", func(ctx context.Context) error {
		key := "recall-overwrite"
		if err := t.store.Put(ctx, Entry{Key: key, Value: []byte("v1")}); err != nil {
			return fmt.Errorf("put v1: %w", err)
		}
		if err := t.store.Put(ctx, Entry{Key: key, Value: []byte("v2")}); err != nil {
			return fmt.Errorf("put v2: %w", err)
		}
		got, err := t.store.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		if string(got.Value) != "v2" {
			return fmt.Errorf("expected v2, got %q", got.Value)
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("not_found_recall", func(ctx context.Context) error {
		_, err := t.store.Get(ctx, "nonexistent-key-12345")
		if err == nil {
			return fmt.Errorf("expected error for missing key")
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("search_recall", func(ctx context.Context) error {
		entries := []Entry{
			{Key: "search-1", Value: []byte("machine learning algorithms")},
			{Key: "search-2", Value: []byte("machine learning models")},
			{Key: "search-3", Value: []byte("database optimization")},
		}
		for _, e := range entries {
			if err := t.store.Put(ctx, e); err != nil {
				return fmt.Errorf("put %s: %w", e.Key, err)
			}
		}
		results, err := t.store.Search(ctx, []byte("machine learning"), 2)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("expected search results, got none")
		}
		if len(results) > 2 {
			return fmt.Errorf("expected at most 2 results, got %d", len(results))
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("binary_value_recall", func(ctx context.Context) error {
		val := []byte{0x00, 0xFF, 0x01, 0xFE, 0x02, 0xFD}
		if err := t.store.Put(ctx, Entry{Key: "binary-recall", Value: val}); err != nil {
			return fmt.Errorf("put: %w", err)
		}
		got, err := t.store.Get(ctx, "binary-recall")
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		if len(got.Value) != len(val) {
			return fmt.Errorf("length mismatch: got %d, want %d", len(got.Value), len(val))
		}
		for i := range val {
			if got.Value[i] != val[i] {
				return fmt.Errorf("byte mismatch at %d: got %x, want %x", i, got.Value[i], val[i])
			}
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("large_value_recall", func(ctx context.Context) error {
		val := make([]byte, 1024*1024) // 1MB
		for i := range val {
			val[i] = byte(i % 256)
		}
		if err := t.store.Put(ctx, Entry{Key: "large-recall", Value: val}); err != nil {
			return fmt.Errorf("put: %w", err)
		}
		got, err := t.store.Get(ctx, "large-recall")
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		if len(got.Value) != len(val) {
			return fmt.Errorf("length mismatch: got %d, want %d", len(got.Value), len(val))
		}
		return nil
	}))

	suite.Duration = time.Since(start)
	return suite
}

// TestStaleness runs staleness detection tests.
func (t *Tester) TestStaleness() TestSuite {
	start := time.Now()
	suite := TestSuite{Name: "Staleness", StartedAt: start}

	suite.Results = append(suite.Results, t.runTest("expired_entry_detection", func(ctx context.Context) error {
		entry := Entry{
			Key:       "stale-1",
			Value:     []byte("expires soon"),
			TTL:       time.Nanosecond,
			CreatedAt: time.Now().Add(-time.Second),
		}
		if !entry.Expired() {
			return fmt.Errorf("expected entry to be expired")
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("not_expired_entry", func(ctx context.Context) error {
		entry := Entry{
			Key:       "fresh-1",
			Value:     []byte("still fresh"),
			TTL:       time.Hour,
			CreatedAt: time.Now(),
		}
		if entry.Expired() {
			return fmt.Errorf("expected entry to not be expired")
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("zero_ttl_never_expires", func(ctx context.Context) error {
		entry := Entry{
			Key:       "no-ttl",
			Value:     []byte("forever"),
			TTL:       0,
			CreatedAt: time.Now().Add(-24 * 365 * time.Hour),
		}
		if entry.Expired() {
			return fmt.Errorf("zero TTL entry should never expire")
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("staleness_scan", func(ctx context.Context) error {
		// Add mix of fresh and stale entries
		fresh := Entry{Key: "scan-fresh", Value: []byte("fresh"), TTL: time.Hour}
		stale := Entry{
			Key:       "scan-stale",
			Value:     []byte("stale"),
			TTL:       time.Nanosecond,
			CreatedAt: time.Now().Add(-time.Second),
		}
		if err := t.store.Put(ctx, fresh); err != nil {
			return fmt.Errorf("put fresh: %w", err)
		}
		if err := t.store.Put(ctx, stale); err != nil {
			return fmt.Errorf("put stale: %w", err)
		}

		entries, err := t.store.List(ctx)
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}

		staleCount := 0
		for _, e := range entries {
			if e.Expired() {
				staleCount++
			}
		}
		if staleCount == 0 {
			return fmt.Errorf("expected at least one stale entry")
		}
		return nil
	}))

	suite.Duration = time.Since(start)
	return suite
}

// TestPruning runs pruning verification tests. The store must implement PrunableStore.
func (t *Tester) TestPruning() TestSuite {
	start := time.Now()
	suite := TestSuite{Name: "Pruning", StartedAt: start}

	ps, ok := t.store.(PrunableStore)
	if !ok {
		suite.Results = append(suite.Results, TestResult{
			Name:   "store_implements_prunable",
			Passed: false,
			Error:  "store does not implement PrunableStore interface",
		})
		suite.Duration = time.Since(start)
		return suite
	}

	suite.Results = append(suite.Results, t.runTest("prune_expired", func(ctx context.Context) error {
		// Add expired entries
		for i := 0; i < 5; i++ {
			e := Entry{
				Key:       fmt.Sprintf("prune-expired-%d", i),
				Value:     []byte("expired"),
				TTL:       time.Nanosecond,
				CreatedAt: time.Now().Add(-time.Second),
			}
			if err := t.store.Put(ctx, e); err != nil {
				return fmt.Errorf("put: %w", err)
			}
		}
		// Add non-expired entries
		for i := 0; i < 3; i++ {
			e := Entry{
				Key:   fmt.Sprintf("prune-fresh-%d", i),
				Value: []byte("fresh"),
				TTL:   time.Hour,
			}
			if err := t.store.Put(ctx, e); err != nil {
				return fmt.Errorf("put: %w", err)
			}
		}

		removed, err := ps.Prune(ctx)
		if err != nil {
			return fmt.Errorf("prune: %w", err)
		}
		if removed < 5 {
			return fmt.Errorf("expected at least 5 removed, got %d", removed)
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("prune_preserves_fresh", func(ctx context.Context) error {
		key := "prune-keep-this"
		if err := t.store.Put(ctx, Entry{Key: key, Value: []byte("keep me"), TTL: time.Hour}); err != nil {
			return fmt.Errorf("put: %w", err)
		}
		if _, err := ps.Prune(ctx); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
		_, err := t.store.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("entry was incorrectly pruned: %w", err)
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("prune_respects_priority", func(ctx context.Context) error {
		// This test is for stores with maxSize
		ms, ok := t.store.(*MemStore)
		if !ok || ms.maxSize <= 0 {
			return nil // skip for non-MemStore or no maxSize
		}

		// Fill beyond capacity with mixed priorities
		for i := 0; i < ms.maxSize+5; i++ {
			e := Entry{
				Key:      fmt.Sprintf("priority-%d", i),
				Value:    []byte(fmt.Sprintf("entry-%d", i)),
				Priority: i,
			}
			if err := t.store.Put(ctx, e); err != nil {
				return fmt.Errorf("put: %w", err)
			}
		}

		if _, err := ps.Prune(ctx); err != nil {
			return fmt.Errorf("prune: %w", err)
		}

		count, err := ps.Count(ctx)
		if err != nil {
			return fmt.Errorf("count: %w", err)
		}
		if count > ms.maxSize {
			return fmt.Errorf("expected at most %d entries, got %d", ms.maxSize, count)
		}

		// Highest priority entries should survive
		highKey := fmt.Sprintf("priority-%d", ms.maxSize+4)
		if _, err := t.store.Get(ctx, highKey); err != nil {
			return fmt.Errorf("high priority entry was pruned: %w", err)
		}
		return nil
	}))

	suite.Duration = time.Since(start)
	return suite
}

// TestConcurrency runs concurrent access tests.
func (t *Tester) TestConcurrency() TestSuite {
	start := time.Now()
	suite := TestSuite{Name: "Concurrency", StartedAt: start}

	suite.Results = append(suite.Results, t.runTest("concurrent_put_get", func(ctx context.Context) error {
		const goroutines = 20
		const opsPerGoroutine = 50

		var wg sync.WaitGroup
		errCh := make(chan error, goroutines*2)

		// Writers
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					key := fmt.Sprintf("conc-%d-%d", id, j)
					err := t.store.Put(ctx, Entry{
						Key:   key,
						Value: []byte(fmt.Sprintf("value-%d-%d", id, j)),
					})
					if err != nil {
						errCh <- fmt.Errorf("put %s: %w", key, err)
						return
					}
				}
			}(i)
		}

		// Wait for writes
		wg.Wait()

		// Readers
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					key := fmt.Sprintf("conc-%d-%d", id, j)
					got, err := t.store.Get(ctx, key)
					if err != nil {
						errCh <- fmt.Errorf("get %s: %w", key, err)
						return
					}
					expected := fmt.Sprintf("value-%d-%d", id, j)
					if string(got.Value) != expected {
						errCh <- fmt.Errorf("value mismatch for %s", key)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			return err
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("concurrent_put_delete", func(ctx context.Context) error {
		const goroutines = 10
		const ops = 100

		var wg sync.WaitGroup
		errCh := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < ops; j++ {
					key := fmt.Sprintf("cd-%d-%d", id, j)
					if err := t.store.Put(ctx, Entry{Key: key, Value: []byte("temp")}); err != nil {
						errCh <- fmt.Errorf("put: %w", err)
						return
					}
					_ = t.store.Delete(ctx, key)
				}
			}(i)
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			return err
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("concurrent_list_during_writes", func(ctx context.Context) error {
		const writers = 5
		const readers = 5
		const ops = 50

		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		errCh := make(chan error, writers+readers)

		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < ops; j++ {
					if ctx2.Err() != nil {
						return
					}
					_ = t.store.Put(ctx2, Entry{
						Key:   fmt.Sprintf("cl-%d-%d", id, j),
						Value: []byte("data"),
					})
				}
			}(i)
		}

		for i := 0; i < readers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < ops; j++ {
					if ctx2.Err() != nil {
						return
					}
					_, err := t.store.List(ctx2)
					if err != nil && ctx2.Err() == nil {
						errCh <- fmt.Errorf("list: %w", err)
						return
					}
				}
			}()
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			return err
		}
		return nil
	}))

	suite.Results = append(suite.Results, t.runTest("concurrent_search", func(ctx context.Context) error {
		// Pre-fill
		for i := 0; i < 20; i++ {
			_ = t.store.Put(ctx, Entry{
				Key:   fmt.Sprintf("csearch-%d", i),
				Value: []byte(fmt.Sprintf("search content %d", i)),
			})
		}

		const goroutines = 10
		var wg sync.WaitGroup
		errCh := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := t.store.Search(ctx, []byte("search content"), 5)
				if err != nil {
					errCh <- err
				}
			}()
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			return err
		}
		return nil
	}))

	suite.Duration = time.Since(start)
	return suite
}

// RunAll runs all test suites and returns the results.
func (t *Tester) RunAll() []TestSuite {
	return []TestSuite{
		t.TestRecall(),
		t.TestStaleness(),
		t.TestPruning(),
		t.TestPersistence(),
		t.TestConcurrency(),
	}
}
