//go:build !windows

package daemon

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSingletonLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "ghxd.lock")

	release, err := acquireSingletonLock(lockPath)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}

	// A second acquisition must be refused while the first is held.
	if _, err := acquireSingletonLock(lockPath); !errors.Is(err, errDaemonAlreadyRunning) {
		t.Fatalf("second lock should report already-running, got: %v", err)
	}

	// After release, the lock is available again.
	release()
	release2, err := acquireSingletonLock(lockPath)
	if err != nil {
		t.Fatalf("re-acquire after release should succeed: %v", err)
	}
	release2()
}

func TestIsAlreadyRunning(t *testing.T) {
	if !IsAlreadyRunning(errDaemonAlreadyRunning) {
		t.Fatal("sentinel should be recognized")
	}
	if IsAlreadyRunning(errors.New("other")) {
		t.Fatal("unrelated error must not match")
	}
}
