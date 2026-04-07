package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JSLEEKR/agentmem"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "test":
		exitCode := runTests(args)
		os.Exit(exitCode)
	case "bench":
		exitCode := runBench(args)
		os.Exit(exitCode)
	case "check":
		exitCode := runCheck(args)
		os.Exit(exitCode)
	case "version":
		fmt.Printf("agentmem v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`agentmem — Testing framework for AI agent memory systems

Usage:
  agentmem <command> [options]

Commands:
  test     Run memory store tests (persistence, recall, staleness, pruning, concurrency)
  bench    Run performance benchmarks (put, get, delete, search, list)
  check    Run health checks on a memory store file
  version  Print version
  help     Print this help

Test Options:
  --store <path>    Path to memory store file (creates temp if not specified)
  --max-size <n>    Maximum store size for pruning tests (default: 10)
  --suite <name>    Run specific suite: recall, staleness, pruning, persistence, concurrency
  --json            Output results as JSON
Bench Options:
  --ops <n>         Number of operations per benchmark (default: 1000)
  --value-size <n>  Size of values in bytes (default: 256)
  --pre-fill <n>    Number of entries to pre-fill (default: 100)
  --json            Output results as JSON

Check Options:
  --store <path>    Path to memory store file (required)
  --json            Output results as JSON

Examples:
  agentmem test                         # Run all tests with temp store
  agentmem test --suite recall --json   # Run recall tests, JSON output
  agentmem bench --ops 5000             # Benchmark with 5000 ops
  agentmem check --store ./memory.json  # Health check a store file`)
}

func parseFlag(args []string, flag string) (string, []string) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return args[i+1], remaining
		}
	}
	return "", args
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func runTests(args []string) int {
	storePath, args := parseFlag(args, "--store")
	maxSizeStr, args := parseFlag(args, "--max-size")
	suiteName, args := parseFlag(args, "--suite")
	jsonOutput := hasFlag(args, "--json")

	maxSize := 10
	if maxSizeStr != "" {
		if n, err := fmt.Sscanf(maxSizeStr, "%d", &maxSize); n != 1 || err != nil || maxSize < 1 {
			fmt.Fprintf(os.Stderr, "error: --max-size must be a positive integer\n")
			return 1
		}
	}

	var opts []agentmem.MemStoreOption
	opts = append(opts, agentmem.WithMaxSize(maxSize))

	if storePath == "" {
		tmpDir, err := os.MkdirTemp("", "agentmem-test-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating temp dir: %v\n", err)
			return 1
		}
		defer os.RemoveAll(tmpDir)
		storePath = filepath.Join(tmpDir, "store.json")
	}
	opts = append(opts, agentmem.WithFilePath(storePath))

	store := agentmem.NewMemStore(opts...)
	defer store.Close()

	tester := agentmem.NewTester(store)

	var suites []agentmem.TestSuite
	if suiteName != "" {
		switch strings.ToLower(suiteName) {
		case "recall":
			suites = append(suites, tester.TestRecall())
		case "staleness":
			suites = append(suites, tester.TestStaleness())
		case "pruning":
			suites = append(suites, tester.TestPruning())
		case "persistence":
			suites = append(suites, tester.TestPersistence())
		case "concurrency":
			suites = append(suites, tester.TestConcurrency())
		default:
			fmt.Fprintf(os.Stderr, "unknown suite: %s\n", suiteName)
			return 1
		}
	} else {
		suites = tester.RunAll()
	}

	report := agentmem.NewReport(suites, nil, nil)

	if jsonOutput {
		if err := report.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing JSON: %v\n", err)
			return 1
		}
	} else {
		if err := report.WriteText(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
			return 1
		}
	}

	if !report.Summary.AllPassed {
		return 1
	}
	return 0
}

func runBench(args []string) int {
	opsStr, args := parseFlag(args, "--ops")
	valueSizeStr, args := parseFlag(args, "--value-size")
	preFillStr, args := parseFlag(args, "--pre-fill")
	jsonOutput := hasFlag(args, "--json")

	config := agentmem.DefaultBenchConfig()
	if opsStr != "" {
		if n, err := fmt.Sscanf(opsStr, "%d", &config.Operations); n != 1 || err != nil || config.Operations < 1 {
			fmt.Fprintf(os.Stderr, "error: --ops must be a positive integer\n")
			return 1
		}
	}
	if valueSizeStr != "" {
		if n, err := fmt.Sscanf(valueSizeStr, "%d", &config.ValueSize); n != 1 || err != nil || config.ValueSize < 1 {
			fmt.Fprintf(os.Stderr, "error: --value-size must be a positive integer\n")
			return 1
		}
	}
	if preFillStr != "" {
		if n, err := fmt.Sscanf(preFillStr, "%d", &config.PreFill); n != 1 || err != nil || config.PreFill < 0 {
			fmt.Fprintf(os.Stderr, "error: --pre-fill must be a non-negative integer\n")
			return 1
		}
	}

	store := agentmem.NewMemStore()
	defer store.Close()

	bencher := agentmem.NewBencher(store, config)
	bench := bencher.RunAll()

	report := agentmem.NewReport(nil, &bench, nil)

	if jsonOutput {
		if err := report.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing JSON: %v\n", err)
			return 1
		}
	} else {
		if err := report.WriteText(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
			return 1
		}
	}
	return 0
}

func runCheck(args []string) int {
	storePath, args := parseFlag(args, "--store")
	jsonOutput := hasFlag(args, "--json")

	if storePath == "" {
		fmt.Fprintln(os.Stderr, "error: --store flag is required for check command")
		return 1
	}

	store := agentmem.NewMemStore(agentmem.WithFilePath(storePath))
	defer store.Close()

	ctx := context.Background()
	if err := store.Load(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error loading store: %v\n", err)
		return 1
	}

	checker := agentmem.NewChecker(store, filepath.Base(storePath))
	checkReport := checker.Check(ctx)

	report := agentmem.NewReport(nil, nil, &checkReport)

	if jsonOutput {
		if err := report.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing JSON: %v\n", err)
			return 1
		}
	} else {
		if err := report.WriteText(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
			return 1
		}
	}

	if !checkReport.Healthy {
		return 1
	}
	return 0
}
