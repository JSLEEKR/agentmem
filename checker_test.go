package agentmem

import (
	"context"
	"testing"
	"time"
)

func TestCheckerHealthyStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.Put(ctx, Entry{Key: "a", Value: []byte("hello"), TTL: time.Hour})
	s.Put(ctx, Entry{Key: "b", Value: []byte("world"), Metadata: map[string]string{"k": "v"}})

	checker := NewChecker(s, "test-store")
	report := checker.Check(ctx)

	if !report.Healthy {
		t.Error("expected healthy store")
		for _, f := range report.Findings {
			t.Logf("finding: [%s] %s: %s", f.Severity, f.Category, f.Description)
		}
	}
	if report.TotalEntries != 2 {
		t.Errorf("total entries = %d, want 2", report.TotalEntries)
	}
	if report.StoreName != "test-store" {
		t.Errorf("store name = %q", report.StoreName)
	}
}

func TestCheckerStaleEntries(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.Put(ctx, Entry{
		Key:       "stale",
		Value:     []byte("old"),
		TTL:       time.Nanosecond,
		CreatedAt: time.Now().Add(-time.Second),
	})

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	if report.WarningCount() == 0 {
		t.Error("expected staleness warnings")
	}

	found := false
	for _, f := range report.Findings {
		if f.Category == "staleness" && f.EntryKey == "stale" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected staleness finding for 'stale' key")
	}
}

func TestCheckerCorruptedEntries(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	// Entry with empty value - need to bypass validation
	s.mu.Lock()
	s.entries["empty-val"] = Entry{Key: "empty-val", Value: []byte{}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.mu.Unlock()

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	if report.ErrorCount() == 0 {
		t.Error("expected corruption error for empty value")
	}
}

func TestCheckerZeroTimestamp(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.mu.Lock()
	s.entries["zero-time"] = Entry{Key: "zero-time", Value: []byte("v")}
	s.mu.Unlock()

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "corruption" && f.EntryKey == "zero-time" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected corruption finding for zero timestamp")
	}
}

func TestCheckerFutureTimestamp(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.mu.Lock()
	s.entries["future"] = Entry{
		Key:       "future",
		Value:     []byte("v"),
		CreatedAt: time.Now().Add(time.Hour),
		UpdatedAt: time.Now().Add(time.Hour),
	}
	s.mu.Unlock()

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "corruption" && f.EntryKey == "future" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected corruption finding for future timestamp")
	}
}

func TestCheckerUpdatedBeforeCreated(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.mu.Lock()
	s.entries["bad-order"] = Entry{
		Key:       "bad-order",
		Value:     []byte("v"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now().Add(-time.Hour),
	}
	s.mu.Unlock()

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "corruption" && f.EntryKey == "bad-order" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected corruption finding for updated_at before created_at")
	}
}

func TestCheckerEmptyMetadataKey(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.Put(ctx, Entry{
		Key:      "meta-empty-key",
		Value:    []byte("v"),
		Metadata: map[string]string{"": "value"},
	})

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "metadata" && f.EntryKey == "meta-empty-key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected metadata finding for empty key")
	}
}

func TestCheckerEmptyMetadataValue(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.Put(ctx, Entry{
		Key:      "meta-empty-val",
		Value:    []byte("v"),
		Metadata: map[string]string{"key": ""},
	})

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "metadata" && f.Severity == SeverityInfo {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected info finding for empty metadata value")
	}
}

func TestCheckerEmptyStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	checker := NewChecker(s, "empty")
	report := checker.Check(ctx)

	if !report.Healthy {
		t.Error("empty store should be healthy")
	}
	if report.TotalEntries != 0 {
		t.Errorf("total = %d, want 0", report.TotalEntries)
	}
}

func TestCheckReportCounts(t *testing.T) {
	report := CheckReport{
		Findings: []CheckFinding{
			{Severity: SeverityError},
			{Severity: SeverityError},
			{Severity: SeverityWarning},
			{Severity: SeverityInfo},
		},
	}

	if report.ErrorCount() != 2 {
		t.Errorf("errors = %d, want 2", report.ErrorCount())
	}
	if report.WarningCount() != 1 {
		t.Errorf("warnings = %d, want 1", report.WarningCount())
	}
}

func TestCheckerNegativeTTL(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()
	defer s.Close()

	s.mu.Lock()
	s.entries["neg-ttl"] = Entry{
		Key:       "neg-ttl",
		Value:     []byte("v"),
		TTL:       -time.Second,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.mu.Unlock()

	checker := NewChecker(s, "test")
	report := checker.Check(ctx)

	found := false
	for _, f := range report.Findings {
		if f.Category == "corruption" && f.EntryKey == "neg-ttl" {
			found = true
		}
	}
	if !found {
		t.Error("expected finding for negative TTL")
	}
}
