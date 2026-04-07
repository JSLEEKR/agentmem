package agentmem

import (
	"path/filepath"
	"testing"
)

func TestTesterTestRecall(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestRecall()

	if suite.Name != "Recall" {
		t.Errorf("suite name = %q, want Recall", suite.Name)
	}
	if len(suite.Results) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Passed() {
		for _, r := range suite.Results {
			if !r.Passed {
				t.Errorf("failed: %s — %s", r.Name, r.Error)
			}
		}
	}
}

func TestTesterTestStaleness(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestStaleness()

	if suite.Name != "Staleness" {
		t.Errorf("suite name = %q, want Staleness", suite.Name)
	}
	if len(suite.Results) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Passed() {
		for _, r := range suite.Results {
			if !r.Passed {
				t.Errorf("failed: %s — %s", r.Name, r.Error)
			}
		}
	}
}

func TestTesterTestPruning(t *testing.T) {
	s := NewMemStore(WithMaxSize(10))
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestPruning()

	if suite.Name != "Pruning" {
		t.Errorf("suite name = %q, want Pruning", suite.Name)
	}
	if len(suite.Results) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Passed() {
		for _, r := range suite.Results {
			if !r.Passed {
				t.Errorf("failed: %s — %s", r.Name, r.Error)
			}
		}
	}
}

func TestTesterTestPruningNonPrunable(t *testing.T) {
	// Use a store that does not implement PrunableStore
	s := &nonPrunableStore{MemStore: NewMemStore()}
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestPruning()

	if suite.Passed() {
		t.Error("expected pruning suite to fail for non-prunable store")
	}
}

// nonPrunableStore wraps MemStore but shadows Prune/Count to break PrunableStore.
type nonPrunableStore struct {
	*MemStore
}

// Shadow Prune so it no longer satisfies PrunableStore.
func (s *nonPrunableStore) Prune() {}

// Shadow Count so it no longer satisfies PrunableStore.
func (s *nonPrunableStore) Count() {}

var _ Store = (*nonPrunableStore)(nil)

func TestTesterTestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tester-persist.json")
	s := NewMemStore(WithFilePath(path))
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestPersistence()

	if suite.Name != "Persistence" {
		t.Errorf("suite name = %q, want Persistence", suite.Name)
	}
	if len(suite.Results) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Passed() {
		for _, r := range suite.Results {
			if !r.Passed {
				t.Errorf("failed: %s — %s", r.Name, r.Error)
			}
		}
	}
}

func TestTesterTestPersistenceNonPersistable(t *testing.T) {
	s := &nonPersistableStore{MemStore: NewMemStore()}
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestPersistence()

	if suite.Passed() {
		t.Error("expected persistence suite to fail for non-persistable store")
	}
}

type nonPersistableStore struct {
	*MemStore
}

// Shadow Save/Load to break PersistableStore.
func (s *nonPersistableStore) Save() {}
func (s *nonPersistableStore) Load() {}

var _ Store = (*nonPersistableStore)(nil)

func TestTesterTestConcurrency(t *testing.T) {
	s := NewMemStore()
	defer s.Close()

	tester := NewTester(s)
	suite := tester.TestConcurrency()

	if suite.Name != "Concurrency" {
		t.Errorf("suite name = %q, want Concurrency", suite.Name)
	}
	if len(suite.Results) == 0 {
		t.Fatal("expected results")
	}
	if !suite.Passed() {
		for _, r := range suite.Results {
			if !r.Passed {
				t.Errorf("failed: %s — %s", r.Name, r.Error)
			}
		}
	}
}

func TestTesterRunAll(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "run-all.json")
	s := NewMemStore(WithFilePath(path), WithMaxSize(10))
	defer s.Close()

	tester := NewTester(s)
	suites := tester.RunAll()

	if len(suites) != 5 {
		t.Fatalf("expected 5 suites, got %d", len(suites))
	}

	names := map[string]bool{}
	for _, s := range suites {
		names[s.Name] = true
	}
	expected := []string{"Recall", "Staleness", "Pruning", "Persistence", "Concurrency"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing suite: %s", name)
		}
	}
}

func TestTestSuiteMethods(t *testing.T) {
	suite := TestSuite{
		Name: "test",
		Results: []TestResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: false, Error: "fail"},
			{Name: "c", Passed: true},
		},
	}

	if suite.Passed() {
		t.Error("suite should not pass")
	}
	if suite.PassCount() != 2 {
		t.Errorf("PassCount = %d, want 2", suite.PassCount())
	}
	if suite.FailCount() != 1 {
		t.Errorf("FailCount = %d, want 1", suite.FailCount())
	}
}

func TestTestSuiteAllPassed(t *testing.T) {
	suite := TestSuite{
		Name: "all-pass",
		Results: []TestResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: true},
		},
	}
	if !suite.Passed() {
		t.Error("suite should pass")
	}
}

func TestTestSuiteEmpty(t *testing.T) {
	suite := TestSuite{Name: "empty"}
	if !suite.Passed() {
		t.Error("empty suite should pass")
	}
	if suite.PassCount() != 0 {
		t.Error("empty suite pass count should be 0")
	}
}
