//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// execReplace replaces the current process with the given binary.
func execReplace(path string, argv []string, env []string) {
	syscall.Exec(path, argv, env)
}
