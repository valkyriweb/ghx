package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/brunoborges/ghx/src/internal/executor"
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

// A failed check whose name contains a transport marker exits 1. That text is
// a listing on stdout. The retry decision sees stderr only, so it must not restart.
func TestWatchDoesNotRetryListingThatMentionsTimeout(t *testing.T) {
	listing := []byte("build\tfail\t0\toperation timed out\n")
	if !executor.IsTransientNetwork(listing) {
		t.Fatal("the fixture must be a marker the old combined scan would have retried")
	}
	calls := 0
	code := runWithTransientRetry(func() (int, []byte) {
		calls++
		return 1, nil // stderr empty; listing is not passed in
	}, func(time.Duration) {
		t.Fatal("a check listing must not be retried")
	}, nil)
	if code != 1 || calls != 1 {
		t.Fatalf("code=%d calls=%d, want 1, 1", code, calls)
	}
}

func TestWatchStdoutStaysOnCallerTerminal(t *testing.T) {
	cmd, diag := prepareWatchCmd("gh", []string{"pr", "checks", "--watch"})
	if cmd.Stdout != os.Stdout {
		t.Fatal("watch stdout must stay the caller terminal")
	}
	if cmd.Stderr == os.Stderr {
		t.Fatal("stderr must be wrapped so only a bounded tail is kept")
	}
	if len(diag()) != 0 {
		t.Fatal("diagnostic starts empty")
	}
}

func TestWatchDiagnosticKeepsTail(t *testing.T) {
	b := &tailBuffer{max: 48}
	_, _ = b.Write(bytes.Repeat([]byte("x"), 200))
	_, _ = b.Write([]byte("read: operation timed out"))
	got := b.Bytes()
	if len(got) != 48 {
		t.Fatalf("len=%d, want 48", len(got))
	}
	if !executor.IsTransientNetwork(got) {
		t.Fatalf("tail lost the error: %q", got)
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
