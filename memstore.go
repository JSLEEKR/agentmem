package agentmem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MemStore is an in-memory implementation of Store, PrunableStore, and PersistableStore.
// It is safe for concurrent use. This serves as both a reference implementation
// and the default backend for testing.
type MemStore struct {
	mu       sync.RWMutex
	entries  map[string]Entry
	closed   bool
	filePath string // optional path for persistence
	maxSize  int    // max entries before pruning (0 = unlimited)
}

// MemStoreOption configures a MemStore.
type MemStoreOption func(*MemStore)

// WithFilePath sets the file path for persistence operations.
func WithFilePath(path string) MemStoreOption {
	return func(s *MemStore) {
		s.filePath = path
	}
}

// WithMaxSize sets the maximum number of entries before pruning is needed.
func WithMaxSize(n int) MemStoreOption {
	return func(s *MemStore) {
		s.maxSize = n
	}
}

// NewMemStore creates a new in-memory store with the given options.
func NewMemStore(opts ...MemStoreOption) *MemStore {
	s := &MemStore{
		entries: make(map[string]Entry),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *MemStore) Put(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put: %w", err)
	}
	if entry.Key == "" {
		return ErrEmptyKey
	}
	if entry.Value == nil {
		return ErrNilValue
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	s.entries[entry.Key] = entry
	return nil
}

func (s *MemStore) Get(ctx context.Context, key string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, fmt.Errorf("get: %w", err)
	}
	if key == "" {
		return Entry{}, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return Entry{}, ErrStoreClosed
	}

	entry, ok := s.entries[key]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return entry, nil
}

func (s *MemStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if key == "" {
		return ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	if _, ok := s.entries[key]; !ok {
		return ErrNotFound
	}
	delete(s.entries, key)
	return nil
}

func (s *MemStore) List(ctx context.Context) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	result := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result, nil
}

// Search performs a simple byte-similarity search using normalized edit distance.
// For production vector stores, implement the Store interface with your own search.
func (s *MemStore) Search(ctx context.Context, query []byte, limit int) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var results []SearchResult
	for _, e := range s.entries {
		score := byteSimilarity(query, e.Value)
		results = append(results, SearchResult{Entry: e, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *MemStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// Prune removes expired entries and, if maxSize is set, removes lowest-priority
// entries until the store is within capacity. Returns the count of removed entries.
func (s *MemStore) Prune(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("prune: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStoreClosed
	}

	removed := 0

	// Phase 1: remove expired entries
	for key, entry := range s.entries {
		if entry.Expired() {
			delete(s.entries, key)
			removed++
		}
	}

	// Phase 2: enforce max size by removing lowest-priority entries
	if s.maxSize > 0 && len(s.entries) > s.maxSize {
		entries := make([]Entry, 0, len(s.entries))
		for _, e := range s.entries {
			entries = append(entries, e)
		}
		// Sort by priority ascending (lowest first = remove first)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Priority != entries[j].Priority {
				return entries[i].Priority < entries[j].Priority
			}
			return entries[i].CreatedAt.Before(entries[j].CreatedAt)
		})

		excess := len(s.entries) - s.maxSize
		for i := 0; i < excess; i++ {
			delete(s.entries, entries[i].Key)
			removed++
		}
	}

	return removed, nil
}

// Count returns the number of entries in the store.
func (s *MemStore) Count(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, ErrStoreClosed
	}

	return len(s.entries), nil
}

// persistData is the serialization format for MemStore.
type persistData struct {
	Entries []Entry `json:"entries"`
}

// Save writes the store contents to the configured file path.
func (s *MemStore) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if s.filePath == "" {
		return fmt.Errorf("save: no file path configured")
	}

	s.mu.RLock()
	entries := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}
	closed := s.closed
	s.mu.RUnlock()

	if closed {
		return ErrStoreClosed
	}

	data := persistData{Entries: entries}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("save: marshal: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("save: mkdir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "agentmem-*.tmp")
	if err != nil {
		return fmt.Errorf("save: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath) // clean up on failure
	}()

	if _, err := tmpFile.Write(raw); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("save: write: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("save: close: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("save: rename: %w", err)
	}
	return nil
}

// Load reads the store contents from the configured file path.
func (s *MemStore) Load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	if s.filePath == "" {
		return fmt.Errorf("load: no file path configured")
	}

	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("load: read: %w", err)
	}

	var data persistData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("load: unmarshal: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	s.entries = make(map[string]Entry, len(data.Entries))
	for _, e := range data.Entries {
		s.entries[e.Key] = e
	}
	return nil
}

// byteSimilarity computes a similarity score between two byte slices.
// Returns a value between 0.0 (completely different) and 1.0 (identical).
func byteSimilarity(a, b []byte) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Use substring containment + length ratio for a fast approximation
	var matchScore float64

	if bytes.Equal(a, b) {
		return 1.0
	}
	if bytes.Contains(b, a) || bytes.Contains(a, b) {
		shorter := math.Min(float64(len(a)), float64(len(b)))
		longer := math.Max(float64(len(a)), float64(len(b)))
		matchScore = shorter / longer
	} else {
		// Count common byte pairs (bigrams)
		aBigrams := makeBigrams(a)
		bBigrams := makeBigrams(b)
		intersection := 0
		for bg, countA := range aBigrams {
			if countB, ok := bBigrams[bg]; ok {
				if countA < countB {
					intersection += countA
				} else {
					intersection += countB
				}
			}
		}
		total := len(a) - 1 + len(b) - 1
		if total > 0 {
			matchScore = float64(2*intersection) / float64(total)
		}
	}
	return matchScore
}

func makeBigrams(data []byte) map[[2]byte]int {
	m := make(map[[2]byte]int)
	for i := 0; i < len(data)-1; i++ {
		key := [2]byte{data[i], data[i+1]}
		m[key]++
	}
	return m
}
