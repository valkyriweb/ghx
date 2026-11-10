//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/brunoborges/ghx/src/internal/config"
)

// processAlive reports whether a process with the given PID currently exists.
// EPERM means the process exists but we lack permission to signal it; treat it
// as alive (we own ghxd, so this is unlikely in practice).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// terminateProcess asks a process to shut down gracefully (SIGTERM).
func terminateProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// killProcess forcibly terminates a wedged process (SIGKILL).
func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// startDaemon launches ghxd as a background process.
func startDaemon(cfg *config.Config) error {
	ghxdPath, err := findGHXD()
	if err != nil {
		return err
	}

	cmd := exec.Command(ghxdPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	return cmd.Start()
}

// execDirect runs gh directly, replacing the current process (bypass daemon entirely).
func execDirect(ghPath string, args []string) {
	path, err := exec.LookPath(ghPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: gh not found: %v\n", err)
		os.Exit(1)
	}
	allArgs := append([]string{ghPath}, args...)
	syscall.Exec(path, allArgs, os.Environ())
}

// runDirectAndAfter runs gh with inherited stdio, then runs after once the
// child exits. The wrapper is needed when a direct mutation reads stdin so the
// daemon can invalidate its cache after the mutation completes.
func runDirectAndAfter(ghPath string, args []string, after func()) int {
	path, err := exec.LookPath(ghPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: gh not found: %v\n", err)
		return 1
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ghx: %v\n", err)
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case sig := <-sigs:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		case err := <-done:
			after()
			if err == nil {
				return 0
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					return 128 + int(status.Signal())
				}
				return exitErr.ExitCode()
			}
			fmt.Fprintf(os.Stderr, "ghx: %v\n", err)
			return 1
		}
	}
}

// execReplace replaces the current process with the given binary.
func execReplace(path string, argv []string, env []string) {
	syscall.Exec(path, argv, env)
}
