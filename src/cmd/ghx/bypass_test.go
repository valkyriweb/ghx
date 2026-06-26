package main

import "testing"

func TestShouldBypassDaemonForRunWatch(t *testing.T) {
	if !shouldBypassDaemon([]string{"run", "watch", "123", "--repo", "owner/repo"}) {
		t.Fatal("expected gh run watch to bypass daemon")
	}
}

func TestShouldBypassDaemonDoesNotBypassOtherRunCommands(t *testing.T) {
	cases := [][]string{
		{"run", "view", "123"},
		{"run", "list", "--repo", "owner/repo"},
		{"pr", "checks", "123"},
		{"run"},
		nil,
	}

	for _, args := range cases {
		if shouldBypassDaemon(args) {
			t.Fatalf("expected %v not to bypass daemon", args)
		}
	}
}
