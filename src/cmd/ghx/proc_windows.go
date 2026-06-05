//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/brunoborges/ghx/src/internal/config"
)

// startDaemon launches ghxd as a background process.
// On Windows, CREATE_NEW_PROCESS_GROUP is used instead of Setsid.
func startDaemon(cfg *config.Config) error {
	ghxdPath, err := findGHXD()
	if err != nil {
		return err
	}

	cmd := exec.Command(ghxdPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200} // CREATE_NEW_PROCESS_GROUP

	return cmd.Start()
}

// execDirect runs gh directly (bypass daemon entirely).
// Windows has no exec-replace, so we run the command and exit.
func execDirect(ghPath string, args []string) {
	path, err := exec.LookPath(ghPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: gh not found: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}

// processAlive reports whether a process with the given PID currently exists.
// On Windows, os.FindProcess opens a real handle and fails for unknown PIDs.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p
	return true
}

// terminateProcess stops a process. Windows has no SIGTERM, so this is a hard kill.
func terminateProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// killProcess forcibly terminates a process.
func killProcess(pid int) error {
	return terminateProcess(pid)
}

// execReplace emulates exec-replace on Windows by running the command and exiting.
func execReplace(path string, argv []string, env []string) {
	cmd := exec.Command(path, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}
