package main

import "testing"

func TestShouldBypassDaemonForWatchCommands(t *testing.T) {
	cases := [][]string{
		{"run", "watch", "123", "--repo", "owner/repo"},
		{"pr", "checks", "123", "--watch", "--interval", "30"},
		{"pr", "checks", "123", "--watch=true"},
	}

	for _, args := range cases {
		if !shouldBypassDaemon(args) {
			t.Fatalf("expected %v to bypass daemon", args)
		}
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
