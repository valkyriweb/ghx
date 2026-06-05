//go:build !windows

package daemon

import (
	"os"
	"os/signal"
	"syscall"
)

// removeStaleSocket removes a leftover socket file from a previous run.
func removeStaleSocket(path string) {
	os.Remove(path)
}

// acquireSingletonLock takes an exclusive, non-blocking lock on lockPath so only
// one daemon can run at a time. The OS releases the lock automatically when the
// process exits (including a crash), so there is no stale-lock problem. Returns
// a release func, or an error if another daemon already holds the lock.
func acquireSingletonLock(lockPath string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, errDaemonAlreadyRunning
		}
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		os.Remove(lockPath)
	}, nil
}

// setSocketPermissions restricts socket file access to the owner.
func setSocketPermissions(path string) error {
	return os.Chmod(path, 0600)
}

// notifyShutdownSignals registers platform-appropriate signals for graceful shutdown.
func notifyShutdownSignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
}
