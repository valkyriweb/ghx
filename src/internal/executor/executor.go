package executor

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result holds the output of a gh command execution.
type Result struct {
	Stdout   []byte        `json:"stdout"`
	Stderr   []byte        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ms"`
}

// Execute runs a gh command with the given arguments and returns its output.
// If workDir is non-empty and absolute, the command runs in that directory.
// If env is non-nil, the subprocess overlays that request environment onto the
// daemon's baseline environment after removing auth/context variables.
func Execute(ctx context.Context, ghPath string, args []string, workDir string, env []string) *Result {
	start := time.Now()

	cmd := exec.CommandContext(ctx, ghPath, args...)
	if workDir != "" && filepath.IsAbs(workDir) {
		cmd.Dir = workDir
	}
	if env != nil {
		cmd.Env = overlayRequestEnv(os.Environ(), env)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: 0,
		Duration: time.Since(start),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Stderr = append(result.Stderr, []byte("\nghx: "+err.Error())...)
		}
	}

	return result
}

func overlayRequestEnv(base []string, request []string) []string {
	scrub := map[string]bool{
		"GH_TOKEN":      true,
		"GITHUB_TOKEN":  true,
		"GH_HOST":       true,
		"GH_REPO":       true,
		"GH_CONFIG_DIR": true,
	}
	merged := make([]string, 0, len(base)+len(request))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && scrub[key] {
			continue
		}
		merged = append(merged, entry)
	}
	merged = append(merged, request...)
	return merged
}

// IsBinaryNotFound reports whether the given gh binary path cannot be found or resolved.
func IsBinaryNotFound(ghPath string) bool {
	_, err := exec.LookPath(ghPath)
	return err != nil && (errors.Is(err, exec.ErrNotFound) || errors.Is(err, exec.ErrDot) || os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist))
}
