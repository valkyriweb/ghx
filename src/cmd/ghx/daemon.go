package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/brunoborges/ghx/src/internal/client"
	"github.com/brunoborges/ghx/src/internal/config"
	"github.com/brunoborges/ghx/src/internal/ipc"
	"github.com/brunoborges/ghx/src/internal/protocol"
)

const (
	daemonStartTimeout = 5 * time.Second
	daemonStopTimeout  = 45 * time.Second
	daemonPollInterval = 50 * time.Millisecond
)

// Fork behaviour (valkyriweb): a daemon that does not drain within the stop
// timeout is force-killed via its recorded PID (SIGTERM, then SIGKILL), so a
// wedged daemon can never block stop/restart or leave two daemons running.
var (
	daemonShutdownTimeout = 2 * time.Second
	daemonSignalTimeout   = 2 * time.Second
)

type stopResult struct {
	found  bool // a daemon (listener, lock owner or live PID) existed
	forced bool // graceful drain timed out and the PID was signalled
}

func restartDaemon(cfg *config.Config) error {
	if _, err := stopDaemon(cfg, daemonStopTimeout); err != nil {
		return err
	}
	return startDaemon(cfg)
}

// stopDaemon asks the daemon to shut down and waits for in-flight requests to
// drain. If it is still alive after drainTimeout, it escalates SIGTERM ->
// SIGKILL against the PID file and then clears the socket and PID files.
func stopDaemon(cfg *config.Config, drainTimeout time.Duration) (stopResult, error) {
	cl := client.New(cfg.SocketPath)
	pid := readPIDFile(cfg.PIDFile)
	owned, err := daemonOwnsSocket(cfg.SocketPath)
	if err != nil {
		return stopResult{}, fmt.Errorf("check daemon ownership: %w", err)
	}
	running := cl.IsRunning()
	res := stopResult{found: running || owned || processAlive(pid)}
	if !res.found {
		return res, nil
	}
	if running {
		// Bounded: a wedged daemon may accept the connection but never reply.
		type sent struct {
			resp *protocol.Response
			err  error
		}
		done := make(chan sent, 1)
		go func() {
			resp, err := cl.Send(&protocol.Request{Type: protocol.TypeShutdown})
			done <- sent{resp, err}
		}()
		select {
		case r := <-done:
			if r.err == nil && r.resp != nil {
				os.Stdout.Write(r.resp.Stdout)
			}
		case <-time.After(daemonShutdownTimeout):
		}
	}
	drainErr := waitForDaemonStop(cfg.SocketPath, drainTimeout)
	if drainErr == nil && !processAlive(pid) {
		return res, nil
	}
	if !processAlive(pid) {
		if drainErr != nil {
			return res, fmt.Errorf("%w; no live daemon PID to force-kill", drainErr)
		}
		return res, nil
	}
	res.forced = true
	_ = terminateProcess(pid)
	if !waitProcessGone(pid, daemonSignalTimeout) {
		_ = killProcess(pid)
		if !waitProcessGone(pid, daemonSignalTimeout) {
			return res, fmt.Errorf("daemon (pid %d) survived SIGKILL", pid)
		}
	}
	os.Remove(cfg.SocketPath)
	os.Remove(cfg.PIDFile)
	return res, nil
}

// readPIDFile returns the daemon PID recorded on disk, or 0 if unavailable.
func readPIDFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid == os.Getpid() {
		return 0
	}
	return pid
}

func waitProcessGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(daemonPollInterval)
	}
	return true
}

func waitForDaemonStop(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	cl := client.New(path)
	for {
		owned, err := daemonOwnsSocket(path)
		if err != nil {
			return fmt.Errorf("check daemon ownership: %w", err)
		}
		// The listener also covers older daemons that do not hold a lock.
		if !owned && !cl.IsRunning() {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("daemon did not stop within %s (requests may still be draining)", timeout)
		}
		time.Sleep(min(daemonPollInterval, remaining))
	}
}

// startDaemon launches a detached daemon and confirms it can serve requests.
func startDaemon(cfg *config.Config) error {
	path, err := findGHXD()
	if err != nil {
		return err
	}
	return startDaemonCommand(cfg, exec.Command(path), daemonStartTimeout)
}

func startDaemonCommand(cfg *config.Config, cmd *exec.Cmd, timeout time.Duration) error {
	configureDetached(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch daemon: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(timeout)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(daemonPollInterval)
	defer ticker.Stop()
	var readinessErr error
	for {
		remaining := time.Until(deadline)
		if remaining > 0 {
			readinessErr = daemonReady(cfg.SocketPath, min(100*time.Millisecond, remaining))
			if readinessErr == nil {
				select {
				case err := <-exited:
					return daemonExitError(err)
				default:
					return nil
				}
			}
		}
		select {
		case err := <-exited:
			return daemonExitError(err)
		case <-timer.C:
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("daemon startup timed out; terminate launched daemon: %w", err)
			}
			<-exited
			return fmt.Errorf("daemon did not become ready within %s: %w", timeout, readinessErr)
		case <-ticker.C:
		}
	}
}

func daemonExitError(err error) error {
	if err != nil {
		return fmt.Errorf("daemon exited before becoming ready: %w", err)
	}
	return fmt.Errorf("daemon exited before becoming ready")
}

func daemonReady(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	conn, err := ipc.Dial(path, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	if err := protocol.WriteMessage(conn, &protocol.Request{Type: protocol.TypeStats}); err != nil {
		return err
	}
	var resp protocol.Response
	if err := protocol.ReadMessage(conn, &resp); err != nil {
		return err
	}
	if resp.Error != "" || resp.ExitCode != 0 {
		return fmt.Errorf("daemon readiness: exit code %d: %s", resp.ExitCode, resp.Error)
	}
	return nil
}
