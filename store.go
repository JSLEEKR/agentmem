// Package agentmem provides a testing framework for AI agent memory systems.
// It validates memory persistence, tests recall accuracy, detects staleness/corruption,
// benchmarks retrieval latency, and verifies pruning logic.
package agentmem

import (
	"context"
	"time"
)

// Entry represents a single memory entry in an agent memory store.
type Entry struct {
	// Key is the unique identifier for this memory.
	Key string `json:"key"`
	// Value is the stored content.
	Value []byte `json:"value"`
	// Metadata holds arbitrary key-value pairs associated with the entry.
	Metadata map[string]string `json:"metadata,omitempty"`
	// CreatedAt is when the entry was first stored.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the entry was last modified.
	UpdatedAt time.Time `json:"updated_at"`
	// TTL is the time-to-live for this entry. Zero means no expiration.
	TTL time.Duration `json:"ttl,omitempty"`
	// Priority indicates importance for pruning decisions (higher = more important).
	Priority int `json:"priority,omitempty"`
}

// Expired returns true if the entry has a TTL set and it has expired.
func (e *Entry) Expired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.CreatedAt) > e.TTL
}

// SearchResult represents a result from a similarity search.
type SearchResult struct {
	Entry Entry   `json:"entry"`
	Score float64 `json:"score"` // similarity score, higher = more similar
}

// Store defines the interface that any agent memory backend must implement
// to be testable with agentmem.
type Store interface {
	// Put stores an entry. If an entry with the same key exists, it is overwritten.
	Put(ctx context.Context, entry Entry) error

	// Get retrieves an entry by key. Returns ErrNotFound if not present.
	Get(ctx context.Context, key string) (Entry, error)

	// Delete removes an entry by key. Returns ErrNotFound if not present.
	Delete(ctx context.Context, key string) error

	// List returns all entries in the store.
	List(ctx context.Context) ([]Entry, error)

	// Search performs a similarity search against entry values.
	// Returns results sorted by descending score. Limit <= 0 means no limit.
	Search(ctx context.Context, query []byte, limit int) ([]SearchResult, error)

	// Close releases any resources held by the store.
	Close() error
}

// PrunableStore extends Store with pruning capabilities.
type PrunableStore interface {
	Store

	// Prune removes entries based on the store's pruning strategy.
	// Returns the number of entries removed.
	Prune(ctx context.Context) (int, error)

	// Count returns the total number of entries in the store.
	Count(ctx context.Context) (int, error)
}

// PersistableStore extends Store with persistence capabilities.
type PersistableStore interface {
	Store

	// Save persists the store's current state to durable storage.
	Save(ctx context.Context) error

	// Load restores the store's state from durable storage.
	Load(ctx context.Context) error
}
