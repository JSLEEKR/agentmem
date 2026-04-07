package agentmem

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Report aggregates results from tests, benchmarks, and checks.
type Report struct {
	GeneratedAt time.Time     `json:"generated_at"`
	TestSuites  []TestSuite   `json:"test_suites,omitempty"`
	Benchmarks  *BenchSuite   `json:"benchmarks,omitempty"`
	HealthCheck *CheckReport  `json:"health_check,omitempty"`
	Summary     ReportSummary `json:"summary"`
}

// ReportSummary provides a quick overview of results.
type ReportSummary struct {
	TotalTests  int  `json:"total_tests"`
	PassedTests int  `json:"passed_tests"`
	FailedTests int  `json:"failed_tests"`
	AllPassed   bool `json:"all_passed"`
	Healthy     bool `json:"healthy"`
}

// NewReport creates a Report from the given results.
func NewReport(suites []TestSuite, bench *BenchSuite, check *CheckReport) Report {
	r := Report{
		GeneratedAt: time.Now(),
		TestSuites:  suites,
		Benchmarks:  bench,
		HealthCheck: check,
	}

	for _, s := range suites {
		r.Summary.TotalTests += len(s.Results)
		r.Summary.PassedTests += s.PassCount()
		r.Summary.FailedTests += s.FailCount()
	}
	r.Summary.AllPassed = r.Summary.FailedTests == 0

	if check != nil {
		r.Summary.Healthy = check.Healthy
	} else {
		r.Summary.Healthy = true
	}

	return r
}

// WriteJSON writes the report as JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteText writes a human-readable report.
func (r *Report) WriteText(w io.Writer) error {
	var sb strings.Builder

	sb.WriteString("=== Agent Memory Test Report ===\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	// Test results
	if len(r.TestSuites) > 0 {
		sb.WriteString("--- Test Results ---\n")
		for _, suite := range r.TestSuites {
			status := "PASS"
			if !suite.Passed() {
				status = "FAIL"
			}
			sb.WriteString(fmt.Sprintf("\n[%s] %s (%s)\n", status, suite.Name, suite.Duration.Truncate(time.Millisecond)))
			for _, result := range suite.Results {
				icon := "+"
				if !result.Passed {
					icon = "x"
				}
				sb.WriteString(fmt.Sprintf("  [%s] %s (%s)", icon, result.Name, result.Duration.Truncate(time.Microsecond)))
				if result.Error != "" {
					sb.WriteString(fmt.Sprintf(" — %s", result.Error))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	// Benchmark results
	if r.Benchmarks != nil {
		sb.WriteString("--- Benchmark Results ---\n")
		sb.WriteString(fmt.Sprintf("Suite: %s (%s)\n\n", r.Benchmarks.Name, r.Benchmarks.Duration.Truncate(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("%-10s %10s %12s %12s %12s %12s %12s\n",
			"Operation", "Ops", "Avg", "P50", "P95", "P99", "Ops/sec"))
		sb.WriteString(strings.Repeat("-", 82) + "\n")
		for _, b := range r.Benchmarks.Results {
			sb.WriteString(fmt.Sprintf("%-10s %10d %12s %12s %12s %12s %12.0f\n",
				b.Name, b.Operations,
				b.AvgLatency.Truncate(time.Microsecond),
				b.P50Latency.Truncate(time.Microsecond),
				b.P95Latency.Truncate(time.Microsecond),
				b.P99Latency.Truncate(time.Microsecond),
				b.OpsPerSec))
		}
		sb.WriteString("\n")
	}

	// Health check
	if r.HealthCheck != nil {
		sb.WriteString("--- Health Check ---\n")
		status := "HEALTHY"
		if !r.HealthCheck.Healthy {
			status = "UNHEALTHY"
		}
		sb.WriteString(fmt.Sprintf("Store: %s — %s\n", r.HealthCheck.StoreName, status))
		sb.WriteString(fmt.Sprintf("Entries: %d\n", r.HealthCheck.TotalEntries))
		sb.WriteString(fmt.Sprintf("Errors: %d, Warnings: %d\n", r.HealthCheck.ErrorCount(), r.HealthCheck.WarningCount()))

		if len(r.HealthCheck.Findings) > 0 {
			sb.WriteString("\nFindings:\n")
			for _, f := range r.HealthCheck.Findings {
				key := ""
				if f.EntryKey != "" {
					key = fmt.Sprintf(" [%s]", f.EntryKey)
				}
				sb.WriteString(fmt.Sprintf("  [%s] %s: %s%s\n", f.Severity, f.Category, f.Description, key))
			}
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("--- Summary ---\n")
	sb.WriteString(fmt.Sprintf("Tests: %d/%d passed\n", r.Summary.PassedTests, r.Summary.TotalTests))
	if r.Summary.AllPassed {
		sb.WriteString("Result: ALL TESTS PASSED\n")
	} else {
		sb.WriteString(fmt.Sprintf("Result: %d TESTS FAILED\n", r.Summary.FailedTests))
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
