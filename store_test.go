package agentmem

import (
	"testing"
	"time"
)

func TestEntryExpired(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		expired bool
	}{
		{
			name:    "zero TTL never expires",
			entry:   Entry{Key: "a", Value: []byte("v"), TTL: 0, CreatedAt: time.Now().Add(-24 * 365 * time.Hour)},
			expired: false,
		},
		{
			name:    "negative TTL never expires",
			entry:   Entry{Key: "b", Value: []byte("v"), TTL: -1, CreatedAt: time.Now()},
			expired: false,
		},
		{
			name:    "fresh entry not expired",
			entry:   Entry{Key: "c", Value: []byte("v"), TTL: time.Hour, CreatedAt: time.Now()},
			expired: false,
		},
		{
			name:    "old entry is expired",
			entry:   Entry{Key: "d", Value: []byte("v"), TTL: time.Nanosecond, CreatedAt: time.Now().Add(-time.Second)},
			expired: true,
		},
		{
			name:    "exactly at TTL boundary",
			entry:   Entry{Key: "e", Value: []byte("v"), TTL: time.Millisecond, CreatedAt: time.Now().Add(-time.Second)},
			expired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.Expired(); got != tt.expired {
				t.Errorf("Expired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestEntryFields(t *testing.T) {
	now := time.Now()
	e := Entry{
		Key:       "test-key",
		Value:     []byte("test-value"),
		Metadata:  map[string]string{"role": "user"},
		CreatedAt: now,
		UpdatedAt: now,
		TTL:       time.Hour,
		Priority:  5,
	}

	if e.Key != "test-key" {
		t.Errorf("Key = %q, want %q", e.Key, "test-key")
	}
	if string(e.Value) != "test-value" {
		t.Errorf("Value = %q, want %q", e.Value, "test-value")
	}
	if e.Metadata["role"] != "user" {
		t.Errorf("Metadata[role] = %q, want %q", e.Metadata["role"], "user")
	}
	if e.Priority != 5 {
		t.Errorf("Priority = %d, want 5", e.Priority)
	}
}
