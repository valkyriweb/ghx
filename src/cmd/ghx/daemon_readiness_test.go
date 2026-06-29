package main

import (
	"errors"
	"testing"
	"time"

	"github.com/brunoborges/ghx/src/internal/protocol"
)

type fakeDaemonClient struct {
	running      bool
	sendFailures int
	sends        int
}

func (c *fakeDaemonClient) IsRunning() bool {
	return c.running
}

func (c *fakeDaemonClient) Send(*protocol.Request) (*protocol.Response, error) {
	c.sends++
	if c.sendFailures > 0 {
		c.sendFailures--
		return nil, errors.New("daemon not ready")
	}
	return &protocol.Response{Stdout: []byte(`{"uptime":"0s"}`)}, nil
}

func TestWaitDaemonReadyRequiresControlResponse(t *testing.T) {
	client := &fakeDaemonClient{running: true, sendFailures: 2}

	if !waitDaemonReady(client, time.Second) {
		t.Fatal("expected daemon to become ready")
	}
	if client.sends != 3 {
		t.Fatalf("expected readiness to retry until control request succeeds, got %d sends", client.sends)
	}
}

func TestWaitDaemonReadyTimesOutWhenControlNeverResponds(t *testing.T) {
	client := &fakeDaemonClient{running: true, sendFailures: 1000}

	if waitDaemonReady(client, 25*time.Millisecond) {
		t.Fatal("expected daemon readiness to time out")
	}
	if client.sends == 0 {
		t.Fatal("expected readiness check to send a control request")
	}
}
