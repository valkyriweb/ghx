package main

import "time"

// timeSleep is the production sleeper. Tests pass their own function into
// runWithTransientRetry and never call this.
func timeSleep(d time.Duration) {
	time.Sleep(d)
}
