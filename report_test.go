package agentmem

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewReport(t *testing.T) {
	suites := []TestSuite{
		{
			Name: "s1",
			Results: []TestResult{
				{Name: "a", Passed: true},
				{Name: "b", Passed: false, Error: "fail"},
			},
		},
		{
			Name: "s2",
			Results: []TestResult{
				{Name: "c", Passed: true},
			},
		},
	}

	report := NewReport(suites, nil, nil)

	if report.Summary.TotalTests != 3 {
		t.Errorf("total = %d, want 3", report.Summary.TotalTests)
	}
	if report.Summary.PassedTests != 2 {
		t.Errorf("passed = %d, want 2", report.Summary.PassedTests)
	}
	if report.Summary.FailedTests != 1 {
		t.Errorf("failed = %d, want 1", report.Summary.FailedTests)
	}
	if report.Summary.AllPassed {
		t.Error("AllPassed should be false")
	}
	if !report.Summary.Healthy {
		t.Error("Healthy should be true when no check")
	}
}

func TestNewReportAllPassed(t *testing.T) {
	suites := []TestSuite{
		{
			Name:    "s1",
			Results: []TestResult{{Name: "a", Passed: true}},
		},
	}
	report := NewReport(suites, nil, nil)
	if !report.Summary.AllPassed {
		t.Error("AllPassed should be true")
	}
}

func TestNewReportWithCheck(t *testing.T) {
	check := &CheckReport{Healthy: false}
	report := NewReport(nil, nil, check)
	if report.Summary.Healthy {
		t.Error("should not be healthy")
	}
}

func TestReportWriteJSON(t *testing.T) {
	report := NewReport([]TestSuite{
		{
			Name:    "test",
			Results: []TestResult{{Name: "a", Passed: true, Duration: time.Millisecond}},
		},
	}, nil, nil)

	var buf bytes.Buffer
	if err := report.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Should be valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := decoded["summary"]; !ok {
		t.Error("JSON missing summary field")
	}
	if _, ok := decoded["test_suites"]; !ok {
		t.Error("JSON missing test_suites field")
	}
}

func TestReportWriteText(t *testing.T) {
	bench := &BenchSuite{
		Name: "bench",
		Results: []BenchResult{
			{
				Name: "put", Operations: 100,
				AvgLatency: time.Microsecond * 50,
				P50Latency: time.Microsecond * 45,
				P95Latency: time.Microsecond * 90,
				P99Latency: time.Microsecond * 95,
				OpsPerSec:  20000,
			},
		},
		Duration: time.Second,
	}

	check := &CheckReport{
		StoreName:    "test-store",
		TotalEntries: 10,
		Healthy:      true,
		Findings: []CheckFinding{
			{Severity: SeverityInfo, Category: "test", Description: "info finding"},
		},
	}

	report := NewReport(
		[]TestSuite{
			{
				Name: "suite",
				Results: []TestResult{
					{Name: "pass-test", Passed: true, Duration: time.Millisecond},
					{Name: "fail-test", Passed: false, Duration: time.Millisecond, Error: "something failed"},
				},
				Duration: 2 * time.Millisecond,
			},
		},
		bench,
		check,
	)

	var buf bytes.Buffer
	if err := report.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}

	output := buf.String()

	// Verify key sections are present
	sections := []string{
		"Agent Memory Test Report",
		"Test Results",
		"Benchmark Results",
		"Health Check",
		"Summary",
		"pass-test",
		"fail-test",
		"something failed",
		"put",
		"test-store",
		"HEALTHY",
	}
	for _, s := range sections {
		if !strings.Contains(output, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestReportWriteTextNoData(t *testing.T) {
	report := NewReport(nil, nil, nil)
	var buf bytes.Buffer
	if err := report.WriteText(&buf); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Summary") {
		t.Error("output missing Summary")
	}
	if !strings.Contains(output, "ALL TESTS PASSED") {
		t.Error("output missing ALL TESTS PASSED for empty suites")
	}
}

func TestReportWriteTextUnhealthy(t *testing.T) {
	check := &CheckReport{
		StoreName: "bad",
		Healthy:   false,
		Findings: []CheckFinding{
			{Severity: SeverityError, Category: "corruption", Description: "bad entry", EntryKey: "k1"},
		},
	}
	report := NewReport(nil, nil, check)
	var buf bytes.Buffer
	report.WriteText(&buf)
	output := buf.String()
	if !strings.Contains(output, "UNHEALTHY") {
		t.Error("output missing UNHEALTHY")
	}
	if !strings.Contains(output, "k1") {
		t.Error("output missing entry key")
	}
}

func TestReportWriteTextFailed(t *testing.T) {
	report := NewReport([]TestSuite{
		{
			Name:    "s",
			Results: []TestResult{{Name: "f", Passed: false, Error: "err"}},
		},
	}, nil, nil)
	var buf bytes.Buffer
	report.WriteText(&buf)
	output := buf.String()
	if !strings.Contains(output, "TESTS FAILED") {
		t.Error("output missing TESTS FAILED")
	}
}
