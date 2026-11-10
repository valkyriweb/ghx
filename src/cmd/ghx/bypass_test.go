package main

import (
	"testing"

	"github.com/brunoborges/ghx/src/internal/allowlist"
)

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

func TestPipedStdinDoesNotBypassCacheableReads(t *testing.T) {
	args := []string{"pr", "view", "8", "--json", "number"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if shouldBypassPipedStdin(class, args, false) {
		t.Fatal("a piped cacheable read must use the daemon")
	}
	if shouldBypassPipedStdin(class, args, true) {
		t.Fatal("a terminal stdin must not take the pipe bypass")
	}
}

func TestPipedStdinStillBypassesStdinReadersAndMutations(t *testing.T) {
	cases := [][]string{
		{"api", "--input", "-", "/repos/o/r"},
		{"api", "--input=-", "/repos/o/r"},
		{"issue", "create", "--body-file", "-"},
		{"pr", "comment", "8", "--body", "note"},
		{"api", "graphql", "-f", "query={viewer{login}}"},
	}
	for _, args := range cases {
		class := allowlist.NewClassifier(nil).Classify(args)
		if !shouldBypassPipedStdin(class, args, false) {
			t.Fatalf("expected %v to bypass when stdin is a pipe", args)
		}
	}
}
