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
		{"api", "/repos/o/r/issues", "-f", "title=x"},
		{"api", "-X", "GET", "search/issues", "-F", "q=@-"},
		{"api", "--field=q=@-", "-X", "GET", "search/issues"},
	}
	for _, args := range cases {
		class := allowlist.NewClassifier(nil).Classify(args)
		if !shouldBypassPipedStdin(class, args, false) {
			t.Fatalf("expected %v to bypass when stdin is a pipe", args)
		}
	}
	// A command that reads stdin bypasses even from a terminal: the daemon
	// protocol does not forward stdin.
	for _, args := range [][]string{
		{"api", "--input", "-", "/repos/o/r"},
		{"api", "-X", "GET", "search/issues", "-F", "q=@-"},
	} {
		class := allowlist.NewClassifier(nil).Classify(args)
		if !shouldBypassPipedStdin(class, args, true) {
			t.Fatalf("expected %v to bypass when it reads stdin", args)
		}
	}
}

func TestLiteralDashDoesNotSkipInvalidation(t *testing.T) {
	args := []string{"pr", "edit", "8", "--body", "-"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if class.Type != allowlist.Mutation {
		t.Fatalf("type=%d, want mutation", class.Type)
	}
	if shouldBypassPipedStdin(class, args, true) {
		t.Fatal("literal --body - must reach the daemon so the cached view is invalidated")
	}
	raw := []string{"api", "-X", "GET", "/user", "-f", "title=@-"}
	rawClass := allowlist.NewClassifier(nil).Classify(raw)
	if shouldBypassPipedStdin(rawClass, raw, false) {
		t.Fatal("-f title=@- is a literal, not stdin")
	}
}

func TestExplicitGETWithQueryFieldsUsesDaemon(t *testing.T) {
	args := []string{"api", "--method=GET", "search/issues", "-F", "q=is:open"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if class.Type != allowlist.Cacheable {
		t.Fatalf("type=%d, want cacheable", class.Type)
	}
	if shouldBypassPipedStdin(class, args, false) {
		t.Fatal("a piped explicit GET must use the daemon")
	}
}
