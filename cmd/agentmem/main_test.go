package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "agentmem.exe")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Join(getModuleRoot(t), "cmd", "agentmem")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func getModuleRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file location to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root")
		}
		dir = parent
	}
}

func TestCLIHelp(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "agentmem") {
		t.Error("help output missing 'agentmem'")
	}
	if !strings.Contains(output, "test") {
		t.Error("help output missing 'test' command")
	}
	if !strings.Contains(output, "bench") {
		t.Error("help output missing 'bench' command")
	}
	if !strings.Contains(output, "check") {
		t.Error("help output missing 'check' command")
	}
}

func TestCLIVersion(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "agentmem v") {
		t.Error("version output missing version string")
	}
}

func TestCLITestCommand(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "test").CombinedOutput()
	if err != nil {
		t.Fatalf("test: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Test Results") {
		t.Error("test output missing 'Test Results'")
	}
	if !strings.Contains(output, "ALL TESTS PASSED") {
		t.Error("test output missing 'ALL TESTS PASSED'")
	}
}

func TestCLITestJSON(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "test", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("test --json: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "\"summary\"") {
		t.Error("JSON output missing summary")
	}
}

func TestCLITestSuite(t *testing.T) {
	bin := buildBinary(t)

	suites := []string{"recall", "staleness", "concurrency"}
	for _, suite := range suites {
		t.Run(suite, func(t *testing.T) {
			out, err := exec.Command(bin, "test", "--suite", suite).CombinedOutput()
			if err != nil {
				t.Fatalf("test --suite %s: %v\n%s", suite, err, out)
			}
		})
	}
}

func TestCLITestUnknownSuite(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "test", "--suite", "nonexistent")
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for unknown suite")
	}
}

func TestCLIBenchCommand(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "bench", "--ops", "50").CombinedOutput()
	if err != nil {
		t.Fatalf("bench: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Benchmark Results") {
		t.Error("bench output missing 'Benchmark Results'")
	}
	if !strings.Contains(output, "put") {
		t.Error("bench output missing 'put' operation")
	}
}

func TestCLIBenchJSON(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "bench", "--ops", "50", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("bench --json: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "\"benchmarks\"") {
		t.Error("JSON output missing benchmarks")
	}
}

func TestCLICheckCommand(t *testing.T) {
	bin := buildBinary(t)

	// Create a store file
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "check-test.json")
	os.WriteFile(storePath, []byte(`{"entries":[{"key":"a","value":"aGVsbG8=","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`), 0o644)

	out, err := exec.Command(bin, "check", "--store", storePath).CombinedOutput()
	if err != nil {
		t.Fatalf("check: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Health Check") {
		t.Error("check output missing 'Health Check'")
	}
}

func TestCLICheckNoStore(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "check")
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for check without --store")
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "foobar")
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestCLINoArgs(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin)
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no args")
	}
}

func TestCLIHelpFlags(t *testing.T) {
	bin := buildBinary(t)

	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			out, err := exec.Command(bin, flag).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", flag, err, out)
			}
			if !strings.Contains(string(out), "agentmem") {
				t.Error("help output missing 'agentmem'")
			}
		})
	}
}

func TestCLITestWithStorePath(t *testing.T) {
	bin := buildBinary(t)
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "custom-store.json")

	out, err := exec.Command(bin, "test", "--store", storePath).CombinedOutput()
	if err != nil {
		t.Fatalf("test --store: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ALL TESTS PASSED") {
		t.Error("test should pass")
	}
}

func TestCLICheckJSON(t *testing.T) {
	bin := buildBinary(t)
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "check-json.json")
	os.WriteFile(storePath, []byte(`{"entries":[{"key":"a","value":"aGVsbG8=","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]}`), 0o644)

	out, err := exec.Command(bin, "check", "--store", storePath, "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("check --json: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "\"health_check\"") {
		t.Error("JSON output missing health_check")
	}
}
