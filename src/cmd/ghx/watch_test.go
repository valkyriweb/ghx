package main

import (
	"testing"
	"time"
)

// The 2026-11-10 failure: gh pr checks --watch exited 1 on a TCP read timeout.
// A later poll of the same watch must be allowed to succeed.
func TestWatchRetriesTransientTimeoutThenSucceeds(t *testing.T) {
	calls := 0
	sleeps := 0
	code := runWithTransientRetry(func() (int, []byte) {
		calls++
		if calls == 1 {
			return 1, []byte(`Post "https://api.github.com/graphql": read tcp 192.168.3.52:58972->20.87.245.6:443: read: operation timed out`)
		}
		return 0, nil
	}, func(time.Duration) { sleeps++ }, nil)
	if code != 0 || calls != 2 || sleeps != 1 {
		t.Fatalf("code=%d calls=%d sleeps=%d, want 0, 2, 1", code, calls, sleeps)
	}
}

func TestWatchDoesNotRetryFailedChecks(t *testing.T) {
	calls := 0
	code := runWithTransientRetry(func() (int, []byte) {
		calls++
		return 1, nil
	}, func(time.Duration) {
		t.Fatal("failed checks must not be retried")
	}, nil)
	if code != 1 || calls != 1 {
		t.Fatalf("code=%d calls=%d, want 1, 1", code, calls)
	}
}

func TestWatchStopsAfterTransientCap(t *testing.T) {
	calls := 0
	code := runWithTransientRetry(func() (int, []byte) {
		calls++
		return 1, []byte("read: operation timed out")
	}, func(time.Duration) {}, nil)
	if code != 1 || calls != maxTransientWatchRetries+1 {
		t.Fatalf("code=%d calls=%d, want 1 and %d", code, calls, maxTransientWatchRetries+1)
	}
}
