package executor

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestExecute_WorkDir(t *testing.T) {
	dir := t.TempDir()

	// Use pwd (or cd on Windows) to verify the subprocess runs in the specified directory.
	var result *Result
	if runtime.GOOS == "windows" {
		result = Execute(context.Background(), "cmd", []string{"/C", "cd"}, dir, nil)
	} else {
		result = Execute(context.Background(), "pwd", nil, dir, nil)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != dir {
		t.Errorf("expected workdir %q, got %q", dir, got)
	}
}

func TestExecute_EmptyWorkDir(t *testing.T) {
	// Empty workDir should inherit the current process's working directory.
	cwd, _ := os.Getwd()

	var result *Result
	if runtime.GOOS == "windows" {
		result = Execute(context.Background(), "cmd", []string{"/C", "cd"}, "", nil)
	} else {
		result = Execute(context.Background(), "pwd", nil, "", nil)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != cwd {
		t.Errorf("expected cwd %q, got %q", cwd, got)
	}
}

func TestExecute_UsesProvidedEnv(t *testing.T) {
	result := Execute(
		context.Background(),
		os.Args[0],
		[]string{"-test.run=TestExecuteEnvHelperProcess", "--"},
		"",
		append(os.Environ(), "GO_WANT_EXECUTOR_ENV_HELPER=1", "GHX_EXECUTOR_ENV=client-token"),
	)

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != "client-token" {
		t.Fatalf("expected subprocess to see provided env, got %q", got)
	}
}

func TestExecuteEnvHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_EXECUTOR_ENV_HELPER") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("GHX_EXECUTOR_ENV"))
	os.Exit(0)
}

func TestExecute_RelativeWorkDir_Ignored(t *testing.T) {
	// Relative paths should be ignored (not set as cmd.Dir).
	cwd, _ := os.Getwd()

	var result *Result
	if runtime.GOOS == "windows" {
		result = Execute(context.Background(), "cmd", []string{"/C", "cd"}, "relative/path", nil)
	} else {
		result = Execute(context.Background(), "pwd", nil, "relative/path", nil)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Stderr)
	}
	got := strings.TrimSpace(string(result.Stdout))
	if got != cwd {
		t.Errorf("relative path should be ignored; expected cwd %q, got %q", cwd, got)
	}
}
