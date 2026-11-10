//go:build unix

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

// execWatchWithRetry runs a long poll under ghx so a transient TCP timeout
// does not end the watch. The child stays in this process group so an external
// timeout that signals the group stops both. A signal delivered only to ghx
// is forwarded to the child, then ghx exits — the child must not keep polling.
func execWatchWithRetry(ghPath string, args []string) {
	var mu sync.Mutex
	var child *exec.Cmd

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		sig := <-sigs
		mu.Lock()
		cmd := child
		mu.Unlock()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
		if s, ok := sig.(syscall.Signal); ok {
			os.Exit(128 + int(s))
		}
		os.Exit(1)
	}()

	code := runWithTransientRetry(func() (int, []byte) {
		path, err := exec.LookPath(ghPath)
		if err != nil {
			return 1, []byte(err.Error())
		}
		cmd, stderr := prepareWatchCmd(path, args)
		mu.Lock()
		child = cmd
		mu.Unlock()
		err = cmd.Run()
		mu.Lock()
		child = nil
		mu.Unlock()
		if err == nil {
			return 0, stderr()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				return 128 + int(status.Signal()), nil
			}
			return exitErr.ExitCode(), stderr()
		}
		return 1, append(stderr(), []byte(err.Error())...)
	}, timeSleep, watchRetryNotice)
	os.Exit(code)
}
