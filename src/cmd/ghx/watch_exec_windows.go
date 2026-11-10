//go:build windows

package main

import (
	"os"
	"os/exec"
)

func execWatchWithRetry(ghPath string, args []string) {
	code := runWithTransientRetry(func() (int, []byte) {
		path, err := exec.LookPath(ghPath)
		if err != nil {
			return 1, []byte(err.Error())
		}
		cmd, stderr := prepareWatchCmd(path, args)
		err = cmd.Run()
		if err == nil {
			return 0, stderr()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), stderr()
		}
		return 1, append(stderr(), []byte(err.Error())...)
	}, timeSleep, watchRetryNotice)
	os.Exit(code)
}
