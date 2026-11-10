package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brunoborges/ghx/src/internal/executor"
)

// maxTransientWatchRetries bounds how many transport faults a long poll will
// absorb before the failure is returned. One timeout then a success is the
// case agents hit; a dead network must still surface.
const maxTransientWatchRetries = 8

// runWithTransientRetry runs a watch child until it exits for a reason other
// than a transport fault. sleep and onRetry are injectable so the retry
// decision can be tested without a network or a clock.
func runWithTransientRetry(run func() (int, []byte), sleep func(time.Duration), onRetry func(attempt int)) int {
	backoff := time.Second
	for attempt := 1; ; attempt++ {
		code, output := run()
		if code == 0 || !executor.IsTransientNetwork(output) {
			return code
		}
		if attempt > maxTransientWatchRetries {
			return code
		}
		if onRetry != nil {
			onRetry(attempt)
		}
		sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func reportWatchRetry(attempt int) {
	fmt.Fprintf(os.Stderr, "ghx: transient network error, retrying watch (%d/%d)\n", attempt, maxTransientWatchRetries)
}

var watchRetryNotice = reportWatchRetry
