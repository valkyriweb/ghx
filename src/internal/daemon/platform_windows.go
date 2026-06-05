//go:build windows

package daemon

import (
	"os"
	"os/signal"
)

// removeStaleSocket is a no-op on Windows; named pipes are managed by the OS.
func removeStaleSocket(_ string) {}

// acquireSingletonLock opens the lock file with exclusive sharing so a second
// daemon cannot open it while this one runs. The handle is released when the
// process exits. Falls back to the dial-check guard in Run if it can't lock.
func acquireSingletonLock(lockPath string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		// Another process holds it exclusively.
		return nil, errDaemonAlreadyRunning
	}
	return func() {
		f.Close()
		os.Remove(lockPath)
	}, nil
}

// setSocketPermissions is a no-op on Windows; named pipes use security descriptors.
func setSocketPermissions(_ string) error { return nil }

// notifyShutdownSignals registers platform-appropriate signals for graceful shutdown.
// On Windows, only os.Interrupt is supported. The shutdown IPC command can also be used.
func notifyShutdownSignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt)
}
