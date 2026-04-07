package agentmem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, opts ...MemStoreOption) *MemStore {
	t.Helper()
	return NewMemStore(opts...)
}

func TestMemStorePut(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	t.Run("basic put", func(t *testing.T) {
		err := s.Put(ctx, Entry{Key: "k1", Value: []byte("v1")})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
	})

	t.Run("put sets timestamps", func(t *testing.T) {
		before := time.Now()
		err := s.Put(ctx, Entry{Key: "k-time", Value: []byte("v")})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		got, _ := s.Get(ctx, "k-time")
		if got.CreatedAt.Before(before) {
			t.Error("CreatedAt should be >= now")
		}
		if got.UpdatedAt.Before(before) {
			t.Error("UpdatedAt should be >= now")
		}
	})

	t.Run("put preserves custom created_at", func(t *testing.T) {
		custom := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		err := s.Put(ctx, Entry{Key: "k-custom-time", Value: []byte("v"), CreatedAt: custom})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		got, _ := s.Get(ctx, "k-custom-time")
		if !got.CreatedAt.Equal(custom) {
			t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, custom)
		}
	})

	t.Run("put overwrites existing", func(t *testing.T) {
		s.Put(ctx, Entry{Key: "overwrite", Value: []byte("v1")})
		s.Put(ctx, Entry{Key: "overwrite", Value: []byte("v2")})
		got, _ := s.Get(ctx, "overwrite")
		if string(got.Value) != "v2" {
			t.Errorf("got %q, want %q", got.Value, "v2")
		}
	})

	t.Run("put empty key error", func(t *testing.T) {
		err := s.Put(ctx, Entry{Key: "", Value: []byte("v")})
		if !errors.Is(err, ErrEmptyKey) {
			t.Errorf("expected ErrEmptyKey, got %v", err)
		}
	})

	t.Run("put nil value error", func(t *testing.T) {
		err := s.Put(ctx, Entry{Key: "nil-val"})
		if !errors.Is(err, ErrNilValue) {
			t.Errorf("expected ErrNilValue, got %v", err)
		}
	})

	t.Run("put on closed store", func(t *testing.T) {
		s2 := newTestStore(t)
		s2.Close()
		err := s2.Put(ctx, Entry{Key: "k", Value: []byte("v")})
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("put with cancelled context", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		err := s.Put(ctx2, Entry{Key: "cancelled", Value: []byte("v")})
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("put with metadata", func(t *testing.T) {
		err := s.Put(ctx, Entry{
			Key:      "meta",
			Value:    []byte("v"),
			Metadata: map[string]string{"a": "1", "b": "2"},
		})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		got, _ := s.Get(ctx, "meta")
		if got.Metadata["a"] != "1" || got.Metadata["b"] != "2" {
			t.Errorf("metadata mismatch: %v", got.Metadata)
		}
	})
}

func TestMemStoreGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	s.Put(ctx, Entry{Key: "exists", Value: []byte("hello")})

	t.Run("get existing", func(t *testing.T) {
		got, err := s.Get(ctx, "exists")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(got.Value) != "hello" {
			t.Errorf("got %q, want %q", got.Value, "hello")
		}
	})

	t.Run("get not found", func(t *testing.T) {
		_, err := s.Get(ctx, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("get empty key", func(t *testing.T) {
		_, err := s.Get(ctx, "")
		if !errors.Is(err, ErrEmptyKey) {
			t.Errorf("expected ErrEmptyKey, got %v", err)
		}
	})

	t.Run("get on closed store", func(t *testing.T) {
		s2 := newTestStore(t)
		s2.Put(ctx, Entry{Key: "k", Value: []byte("v")})
		s2.Close()
		_, err := s2.Get(ctx, "k")
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("get with cancelled context", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.Get(ctx2, "exists")
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestMemStoreDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	t.Run("delete existing", func(t *testing.T) {
		s.Put(ctx, Entry{Key: "del-1", Value: []byte("v")})
		err := s.Delete(ctx, "del-1")
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = s.Get(ctx, "del-1")
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected entry to be deleted")
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		err := s.Delete(ctx, "missing-del")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete empty key", func(t *testing.T) {
		err := s.Delete(ctx, "")
		if !errors.Is(err, ErrEmptyKey) {
			t.Errorf("expected ErrEmptyKey, got %v", err)
		}
	})

	t.Run("delete on closed store", func(t *testing.T) {
		s2 := newTestStore(t)
		s2.Put(ctx, Entry{Key: "k", Value: []byte("v")})
		s2.Close()
		err := s2.Delete(ctx, "k")
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("delete with cancelled context", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		err := s.Delete(ctx2, "anything")
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestMemStoreList(t *testing.T) {
	ctx := context.Background()

	t.Run("list empty store", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		entries, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("list returns all entries sorted", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		s.Put(ctx, Entry{Key: "c", Value: []byte("3")})
		s.Put(ctx, Entry{Key: "a", Value: []byte("1")})
		s.Put(ctx, Entry{Key: "b", Value: []byte("2")})

		entries, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		if entries[0].Key != "a" || entries[1].Key != "b" || entries[2].Key != "c" {
			t.Errorf("entries not sorted: %v, %v, %v", entries[0].Key, entries[1].Key, entries[2].Key)
		}
	})

	t.Run("list on closed store", func(t *testing.T) {
		s := newTestStore(t)
		s.Close()
		_, err := s.List(ctx)
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("list with cancelled context", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.List(ctx2)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestMemStoreSearch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	s.Put(ctx, Entry{Key: "s1", Value: []byte("hello world")})
	s.Put(ctx, Entry{Key: "s2", Value: []byte("hello there")})
	s.Put(ctx, Entry{Key: "s3", Value: []byte("goodbye world")})

	t.Run("search finds similar", func(t *testing.T) {
		results, err := s.Search(ctx, []byte("hello"), 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results")
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		results, err := s.Search(ctx, []byte("hello"), 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("search zero limit returns all", func(t *testing.T) {
		results, err := s.Search(ctx, []byte("hello"), 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
	})

	t.Run("search exact match highest score", func(t *testing.T) {
		results, err := s.Search(ctx, []byte("hello world"), 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results")
		}
		if results[0].Entry.Key != "s1" {
			t.Errorf("expected s1 as top result, got %s", results[0].Entry.Key)
		}
		if results[0].Score != 1.0 {
			t.Errorf("expected score 1.0, got %f", results[0].Score)
		}
	})

	t.Run("search on closed store", func(t *testing.T) {
		s2 := newTestStore(t)
		s2.Close()
		_, err := s2.Search(ctx, []byte("q"), 10)
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("search with cancelled context", func(t *testing.T) {
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.Search(ctx2, []byte("q"), 10)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("search empty store", func(t *testing.T) {
		s2 := newTestStore(t)
		defer s2.Close()
		results, err := s2.Search(ctx, []byte("q"), 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestMemStoreClose(t *testing.T) {
	s := newTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Double close should not panic
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestMemStorePrune(t *testing.T) {
	ctx := context.Background()

	t.Run("prune removes expired", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()

		// Add expired entries
		for i := 0; i < 5; i++ {
			s.Put(ctx, Entry{
				Key:       fmt.Sprintf("expired-%d", i),
				Value:     []byte("old"),
				TTL:       time.Nanosecond,
				CreatedAt: time.Now().Add(-time.Second),
			})
		}
		// Add fresh entry
		s.Put(ctx, Entry{Key: "fresh", Value: []byte("new"), TTL: time.Hour})

		removed, err := s.Prune(ctx)
		if err != nil {
			t.Fatalf("Prune: %v", err)
		}
		if removed != 5 {
			t.Errorf("removed = %d, want 5", removed)
		}

		count, _ := s.Count(ctx)
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("prune enforces max size", func(t *testing.T) {
		s := newTestStore(t, WithMaxSize(5))
		defer s.Close()

		for i := 0; i < 10; i++ {
			s.Put(ctx, Entry{
				Key:      fmt.Sprintf("size-%d", i),
				Value:    []byte("data"),
				Priority: i,
			})
		}

		removed, err := s.Prune(ctx)
		if err != nil {
			t.Fatalf("Prune: %v", err)
		}
		if removed != 5 {
			t.Errorf("removed = %d, want 5", removed)
		}

		count, _ := s.Count(ctx)
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}

		// Highest priority should survive
		_, err = s.Get(ctx, "size-9")
		if err != nil {
			t.Error("highest priority entry was pruned")
		}
		// Lowest priority should be gone
		_, err = s.Get(ctx, "size-0")
		if !errors.Is(err, ErrNotFound) {
			t.Error("lowest priority entry should have been pruned")
		}
	})

	t.Run("prune on empty store", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		removed, err := s.Prune(ctx)
		if err != nil {
			t.Fatalf("Prune: %v", err)
		}
		if removed != 0 {
			t.Errorf("removed = %d, want 0", removed)
		}
	})

	t.Run("prune on closed store", func(t *testing.T) {
		s := newTestStore(t)
		s.Close()
		_, err := s.Prune(ctx)
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("prune with cancelled context", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.Prune(ctx2)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("prune priority tie broken by age", func(t *testing.T) {
		s := newTestStore(t, WithMaxSize(2))
		defer s.Close()

		old := time.Now().Add(-time.Hour)
		recent := time.Now()

		s.Put(ctx, Entry{Key: "old", Value: []byte("old"), Priority: 1, CreatedAt: old})
		s.Put(ctx, Entry{Key: "recent", Value: []byte("recent"), Priority: 1, CreatedAt: recent})
		s.Put(ctx, Entry{Key: "newest", Value: []byte("newest"), Priority: 1})

		s.Prune(ctx)
		count, _ := s.Count(ctx)
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
		// Old one should be pruned first
		_, err := s.Get(ctx, "old")
		if !errors.Is(err, ErrNotFound) {
			t.Error("oldest entry should have been pruned")
		}
	})
}

func TestMemStoreCount(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	s.Put(ctx, Entry{Key: "a", Value: []byte("1")})
	s.Put(ctx, Entry{Key: "b", Value: []byte("2")})

	count, err = s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestMemStorePersistence(t *testing.T) {
	ctx := context.Background()

	t.Run("save and load roundtrip", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test-store.json")

		s := newTestStore(t, WithFilePath(path))
		defer s.Close()

		s.Put(ctx, Entry{Key: "p1", Value: []byte("hello"), Metadata: map[string]string{"a": "1"}})
		s.Put(ctx, Entry{Key: "p2", Value: []byte("world"), Priority: 10})

		if err := s.Save(ctx); err != nil {
			t.Fatalf("Save: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file not created: %v", err)
		}

		// Load into new store
		s2 := newTestStore(t, WithFilePath(path))
		defer s2.Close()
		if err := s2.Load(ctx); err != nil {
			t.Fatalf("Load: %v", err)
		}

		got, err := s2.Get(ctx, "p1")
		if err != nil {
			t.Fatalf("Get p1: %v", err)
		}
		if string(got.Value) != "hello" {
			t.Errorf("p1 value = %q, want %q", got.Value, "hello")
		}
		if got.Metadata["a"] != "1" {
			t.Error("metadata lost")
		}

		got2, err := s2.Get(ctx, "p2")
		if err != nil {
			t.Fatalf("Get p2: %v", err)
		}
		if got2.Priority != 10 {
			t.Errorf("p2 priority = %d, want 10", got2.Priority)
		}
	})

	t.Run("save without path errors", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		err := s.Save(ctx)
		if err == nil {
			t.Error("expected error for no path")
		}
	})

	t.Run("load without path errors", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()
		err := s.Load(ctx)
		if err == nil {
			t.Error("expected error for no path")
		}
	})

	t.Run("load nonexistent file errors", func(t *testing.T) {
		s := newTestStore(t, WithFilePath("/nonexistent/path/store.json"))
		defer s.Close()
		err := s.Load(ctx)
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("save on closed store errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := newTestStore(t, WithFilePath(filepath.Join(tmpDir, "closed.json")))
		s.Close()
		err := s.Save(ctx)
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("load on closed store errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "for-load.json")
		// Create a valid file first
		s1 := newTestStore(t, WithFilePath(path))
		s1.Put(ctx, Entry{Key: "x", Value: []byte("y")})
		s1.Save(ctx)
		s1.Close()

		s2 := newTestStore(t, WithFilePath(path))
		s2.Close()
		err := s2.Load(ctx)
		if !errors.Is(err, ErrStoreClosed) {
			t.Errorf("expected ErrStoreClosed, got %v", err)
		}
	})

	t.Run("save with cancelled context", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := newTestStore(t, WithFilePath(filepath.Join(tmpDir, "cancel.json")))
		defer s.Close()
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		err := s.Save(ctx2)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("load with cancelled context", func(t *testing.T) {
		tmpDir := t.TempDir()
		s := newTestStore(t, WithFilePath(filepath.Join(tmpDir, "cancel.json")))
		defer s.Close()
		ctx2, cancel := context.WithCancel(ctx)
		cancel()
		err := s.Load(ctx2)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("load corrupted file errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "corrupt.json")
		os.WriteFile(path, []byte("not valid json{{{"), 0o644)

		s := newTestStore(t, WithFilePath(path))
		defer s.Close()
		err := s.Load(ctx)
		if err == nil {
			t.Error("expected error for corrupt file")
		}
	})

	t.Run("binary values survive roundtrip", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "binary.json")

		s := newTestStore(t, WithFilePath(path))
		defer s.Close()

		val := []byte{0x00, 0xFF, 0x01, 0xFE, 0x80}
		s.Put(ctx, Entry{Key: "bin", Value: val})
		s.Save(ctx)

		s2 := newTestStore(t, WithFilePath(path))
		defer s2.Close()
		s2.Load(ctx)

		got, err := s2.Get(ctx, "bin")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got.Value) != len(val) {
			t.Fatalf("value length mismatch")
		}
		for i := range val {
			if got.Value[i] != val[i] {
				t.Errorf("byte %d: got %x, want %x", i, got.Value[i], val[i])
			}
		}
	})
}

func TestMemStoreConcurrency(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	t.Run("concurrent puts", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("conc-put-%d", id)
				s.Put(ctx, Entry{Key: key, Value: []byte(fmt.Sprintf("v%d", id))})
			}(i)
		}
		wg.Wait()
	})

	t.Run("concurrent reads", func(t *testing.T) {
		s.Put(ctx, Entry{Key: "read-target", Value: []byte("data")})
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.Get(ctx, "read-target")
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent mixed operations", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(3)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("mix-%d", id)
				s.Put(ctx, Entry{Key: key, Value: []byte("v")})
			}(i)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("mix-%d", id)
				s.Get(ctx, key)
			}(i)
			go func() {
				defer wg.Done()
				s.List(ctx)
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent search", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.Search(ctx, []byte("search"), 5)
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent prune", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.Prune(ctx)
			}()
		}
		wg.Wait()
	})
}

func TestMemStoreOptions(t *testing.T) {
	t.Run("with file path", func(t *testing.T) {
		s := NewMemStore(WithFilePath("/tmp/test.json"))
		defer s.Close()
		if s.filePath != "/tmp/test.json" {
			t.Errorf("filePath = %q, want /tmp/test.json", s.filePath)
		}
	})

	t.Run("with max size", func(t *testing.T) {
		s := NewMemStore(WithMaxSize(42))
		defer s.Close()
		if s.maxSize != 42 {
			t.Errorf("maxSize = %d, want 42", s.maxSize)
		}
	})

	t.Run("default options", func(t *testing.T) {
		s := NewMemStore()
		defer s.Close()
		if s.filePath != "" {
			t.Error("default filePath should be empty")
		}
		if s.maxSize != 0 {
			t.Error("default maxSize should be 0")
		}
	})
}

func TestByteSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []byte
		minScore float64
		maxScore float64
	}{
		{"identical", []byte("hello"), []byte("hello"), 1.0, 1.0},
		{"empty both", []byte{}, []byte{}, 1.0, 1.0},
		{"empty a", []byte{}, []byte("hello"), 0.0, 0.0},
		{"empty b", []byte("hello"), []byte{}, 0.0, 0.0},
		{"substring", []byte("hello"), []byte("hello world"), 0.3, 1.0},
		{"completely different", []byte("aaaa"), []byte("zzzz"), 0.0, 0.1},
		{"similar", []byte("hello world"), []byte("hello earth"), 0.3, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := byteSimilarity(tt.a, tt.b)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("score = %f, want [%f, %f]", score, tt.minScore, tt.maxScore)
			}
		})
	}
}
