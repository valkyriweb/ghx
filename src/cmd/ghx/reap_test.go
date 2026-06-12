package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/brunoborges/ghx/src/internal/client"
	"github.com/brunoborges/ghx/src/internal/config"
)

func tmpCfg(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{
		SocketPath: filepath.Join(dir, "ghxd.sock"),
		PIDFile:    filepath.Join(dir, "ghxd.pid"),
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}
	// PID 0 / negative are never alive.
	if processAlive(0) || processAlive(-1) {
		t.Fatal("non-positive PIDs must not be reported alive")
	}
	// A very high, almost-certainly-unused PID should be dead.
	if processAlive(0x7FFFFFF0) {
		t.Fatal("bogus PID should not be alive")
	}
}

func TestReadPIDFile(t *testing.T) {
	cfg := tmpCfg(t)

	if got := readPIDFile(cfg); got != 0 {
		t.Fatalf("missing PID file should read 0, got %d", got)
	}

	os.WriteFile(cfg.PIDFile, []byte("  4242\n"), 0600)
	if got := readPIDFile(cfg); got != 4242 {
		t.Fatalf("expected 4242, got %d", got)
	}

	os.WriteFile(cfg.PIDFile, []byte("not-a-number"), 0600)
	if got := readPIDFile(cfg); got != 0 {
		t.Fatalf("garbage PID file should read 0, got %d", got)
	}
}

func TestReapDaemonNothingRunning(t *testing.T) {
	cfg := tmpCfg(t)
	// Stale files pointing at a dead PID — reap should clear them and report
	// nothing was running.
	if runtime.GOOS != "windows" {
		os.WriteFile(cfg.SocketPath, []byte{}, 0600)
	}
	os.WriteFile(cfg.PIDFile, []byte(strconv.Itoa(0x7FFFFFF0)), 0600)

	if reapDaemon(cfg) {
		t.Fatal("reapDaemon should report false when no live daemon exists")
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(cfg.SocketPath); !os.IsNotExist(err) {
			t.Fatal("stale socket file should be removed")
		}
	}
	if _, err := os.Stat(cfg.PIDFile); !os.IsNotExist(err) {
		t.Fatal("stale PID file should be removed")
	}
}

func TestDaemonGone(t *testing.T) {
	cfg := tmpCfg(t)
	cl := client.New(cfg.SocketPath)
	// No socket, dead PID -> gone.
	if !daemonGone(cl, 0x7FFFFFF0) {
		t.Fatal("should be gone with no socket and dead PID")
	}
	// Live PID -> not gone (socket still absent, but PID alive).
	if daemonGone(cl, os.Getpid()) {
		t.Fatal("should not be gone while PID is alive")
	}
}
