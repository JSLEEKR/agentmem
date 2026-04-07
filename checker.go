package agentmem

import (
	"context"
	"fmt"
	"time"
)

// CheckSeverity indicates the severity of a check finding.
type CheckSeverity string

const (
	SeverityInfo    CheckSeverity = "info"
	SeverityWarning CheckSeverity = "warning"
	SeverityError   CheckSeverity = "error"
)

// CheckFinding represents a single finding from a health check.
type CheckFinding struct {
	Severity    CheckSeverity `json:"severity"`
	Category    string        `json:"category"`
	Description string        `json:"description"`
	EntryKey    string        `json:"entry_key,omitempty"`
}

// CheckReport holds the results of a health check.
type CheckReport struct {
	StoreName   string         `json:"store_name"`
	CheckedAt   time.Time      `json:"checked_at"`
	Duration    time.Duration  `json:"duration"`
	TotalEntries int           `json:"total_entries"`
	Findings    []CheckFinding `json:"findings"`
	Healthy     bool           `json:"healthy"`
}

// ErrorCount returns the number of error-severity findings.
func (r *CheckReport) ErrorCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-severity findings.
func (r *CheckReport) WarningCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			n++
		}
	}
	return n
}

// Checker runs health checks against a Store.
type Checker struct {
	store     Store
	storeName string
}

// NewChecker creates a new Checker.
func NewChecker(store Store, name string) *Checker {
	return &Checker{store: store, storeName: name}
}

// Check runs all health checks and returns a report.
func (c *Checker) Check(ctx context.Context) CheckReport {
	start := time.Now()
	report := CheckReport{
		StoreName: c.storeName,
		CheckedAt: start,
		Healthy:   true,
	}

	entries, err := c.store.List(ctx)
	if err != nil {
		report.Findings = append(report.Findings, CheckFinding{
			Severity:    SeverityError,
			Category:    "accessibility",
			Description: fmt.Sprintf("failed to list entries: %v", err),
		})
		report.Healthy = false
		report.Duration = time.Since(start)
		return report
	}

	report.TotalEntries = len(entries)

	// Check for stale entries
	c.checkStaleness(entries, &report)

	// Check for corruption indicators
	c.checkCorruption(entries, &report)

	// Check for duplicate metadata anomalies
	c.checkMetadataAnomalies(entries, &report)

	// Check accessibility of each entry
	c.checkAccessibility(ctx, entries, &report)

	// Determine overall health
	report.Healthy = report.ErrorCount() == 0
	report.Duration = time.Since(start)
	return report
}

func (c *Checker) checkStaleness(entries []Entry, report *CheckReport) {
	staleCount := 0
	for _, e := range entries {
		if e.Expired() {
			staleCount++
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityWarning,
				Category:    "staleness",
				Description: fmt.Sprintf("entry expired %s ago (TTL: %s)", time.Since(e.CreatedAt.Add(e.TTL)).Truncate(time.Millisecond), e.TTL),
				EntryKey:    e.Key,
			})
		}
	}

	if staleCount > 0 {
		report.Findings = append(report.Findings, CheckFinding{
			Severity:    SeverityWarning,
			Category:    "staleness",
			Description: fmt.Sprintf("%d of %d entries are stale (%.1f%%)", staleCount, len(entries), float64(staleCount)/float64(len(entries))*100),
		})
	}
}

func (c *Checker) checkCorruption(entries []Entry, report *CheckReport) {
	for _, e := range entries {
		// Check for empty values
		if len(e.Value) == 0 {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityError,
				Category:    "corruption",
				Description: "entry has empty value",
				EntryKey:    e.Key,
			})
		}

		// Check for zero timestamps
		if e.CreatedAt.IsZero() {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityWarning,
				Category:    "corruption",
				Description: "entry has zero created_at timestamp",
				EntryKey:    e.Key,
			})
		}

		// Check for future timestamps
		if e.CreatedAt.After(time.Now().Add(time.Minute)) {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityWarning,
				Category:    "corruption",
				Description: "entry has future created_at timestamp",
				EntryKey:    e.Key,
			})
		}

		// Check for updated_at before created_at
		if !e.UpdatedAt.IsZero() && e.UpdatedAt.Before(e.CreatedAt) {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityWarning,
				Category:    "corruption",
				Description: "entry updated_at is before created_at",
				EntryKey:    e.Key,
			})
		}

		// Check for empty key (shouldn't happen, but worth checking)
		if e.Key == "" {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityError,
				Category:    "corruption",
				Description: "entry has empty key",
			})
		}

		// Check for negative TTL
		if e.TTL < 0 {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityError,
				Category:    "corruption",
				Description: fmt.Sprintf("entry has negative TTL: %s", e.TTL),
				EntryKey:    e.Key,
			})
		}
	}
}

func (c *Checker) checkMetadataAnomalies(entries []Entry, report *CheckReport) {
	// Check for suspiciously large metadata
	for _, e := range entries {
		if len(e.Metadata) > 100 {
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityWarning,
				Category:    "metadata",
				Description: fmt.Sprintf("entry has %d metadata keys (unusually high)", len(e.Metadata)),
				EntryKey:    e.Key,
			})
		}

		// Check for empty metadata keys or values
		for k, v := range e.Metadata {
			if k == "" {
				report.Findings = append(report.Findings, CheckFinding{
					Severity:    SeverityWarning,
					Category:    "metadata",
					Description: "entry has empty metadata key",
					EntryKey:    e.Key,
				})
			}
			if v == "" {
				report.Findings = append(report.Findings, CheckFinding{
					Severity:    SeverityInfo,
					Category:    "metadata",
					Description: fmt.Sprintf("entry has empty metadata value for key %q", k),
					EntryKey:    e.Key,
				})
			}
		}
	}
}

func (c *Checker) checkAccessibility(ctx context.Context, entries []Entry, report *CheckReport) {
	inaccessible := 0
	for _, e := range entries {
		_, err := c.store.Get(ctx, e.Key)
		if err != nil {
			inaccessible++
			report.Findings = append(report.Findings, CheckFinding{
				Severity:    SeverityError,
				Category:    "accessibility",
				Description: fmt.Sprintf("entry listed but not accessible via Get: %v", err),
				EntryKey:    e.Key,
			})
		}
	}

	if inaccessible > 0 {
		report.Findings = append(report.Findings, CheckFinding{
			Severity:    SeverityError,
			Category:    "accessibility",
			Description: fmt.Sprintf("%d entries are inaccessible", inaccessible),
		})
	}
}
