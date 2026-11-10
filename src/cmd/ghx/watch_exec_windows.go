//go:build windows

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

func execWatchWithRetry(ghPath string, args []string) {
	code := runWithTransientRetry(func() (int, []byte) {
		path, err := exec.LookPath(ghPath)
		if err != nil {
			return 1, []byte(err.Error())
		}
		var captured bytes.Buffer
		cmd := exec.Command(path, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = io.MultiWriter(os.Stdout, &captured)
		cmd.Stderr = io.MultiWriter(os.Stderr, &captured)
		err = cmd.Run()
		if err == nil {
			return 0, captured.Bytes()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), captured.Bytes()
		}
		return 1, append(captured.Bytes(), []byte(err.Error())...)
	}, timeSleep, watchRetryNotice)
	os.Exit(code)
}
