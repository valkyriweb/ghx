package main

import (
	"os"
	"testing"

	"github.com/brunoborges/ghx/src/internal/allowlist"
	execctx "github.com/brunoborges/ghx/src/internal/context"
	"github.com/brunoborges/ghx/src/internal/protocol"
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

func TestCacheableReadsDoNotReadStdin(t *testing.T) {
	args := []string{"pr", "view", "8", "--json", "number"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if class.Type != allowlist.Cacheable {
		t.Fatalf("type=%d, want cacheable", class.Type)
	}
	if commandReadsStdin(args) {
		t.Fatal("a cacheable read with no input flag must not be treated as a stdin reader")
	}
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if stdinNeedsDirectExecution(args, stdin) {
		t.Fatal("a cacheable read with terminal-like stdin should use the daemon")
	}
}

func TestPipedStdinAlwaysUsesDirectPath(t *testing.T) {
	stdin, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	defer stdin.Close()

	tests := []struct {
		args []string
		want allowlist.CommandType
	}{
		{args: []string{"gist", "create"}, want: allowlist.Mutation},
		{args: []string{"unrecognized", "command"}, want: allowlist.Passthrough},
		{args: []string{"pr", "view", "8", "--json", "number"}, want: allowlist.Cacheable},
	}
	classifier := allowlist.NewClassifier(nil)
	for _, tt := range tests {
		if got := classifier.Classify(tt.args).Type; got != tt.want {
			t.Fatalf("Classify(%v) = %d, want %d", tt.args, got, tt.want)
		}
		if !stdinNeedsDirectExecution(tt.args, stdin) {
			t.Errorf("piped stdin for %v must use direct execution", tt.args)
		}
	}
}

func TestStdinReadersBypassDaemon(t *testing.T) {
	cases := [][]string{
		{"api", "--input", "-", "/repos/o/r"},
		{"api", "--input=-", "/repos/o/r"},
		{"issue", "create", "--body-file", "-"},
		{"api", "-X", "GET", "search/issues", "-F", "q=@-"},
		{"api", "--field=q=@-", "-X", "GET", "search/issues"},
		{"secret", "set", "TOKEN"},
		{"variable", "set", "BUILD"},
	}
	for _, args := range cases {
		if !commandReadsStdin(args) {
			t.Fatalf("expected %v to read stdin", args)
		}
	}

	for _, args := range [][]string{
		{"secret", "set", "TOKEN", "--body", "value"},
		{"variable", "set", "BUILD", "-b=value"},
		{"pr", "edit", "8", "--body", "-"},
		{"api", "-X", "GET", "/user", "-f", "title=@-"},
	} {
		if commandReadsStdin(args) {
			t.Fatalf("%v does not read stdin", args)
		}
	}
}

func TestMutationsWithoutPipedInputUseDaemon(t *testing.T) {
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	for _, args := range [][]string{
		{"pr", "edit", "8", "--title", "updated"},
		{"pr", "comment", "8", "--body", "note"},
	} {
		class := allowlist.NewClassifier(nil).Classify(args)
		if class.Type != allowlist.Mutation {
			t.Fatalf("type=%d for %v, want mutation", class.Type, args)
		}
		if stdinNeedsDirectExecution(args, stdin) {
			t.Fatalf("mutation %v with terminal-like stdin should use daemon invalidation", args)
		}
	}
}

func TestLiteralDashDoesNotSkipInvalidation(t *testing.T) {
	args := []string{"pr", "edit", "8", "--body", "-"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if class.Type != allowlist.Mutation {
		t.Fatalf("type=%d, want mutation", class.Type)
	}
	if commandReadsStdin(args) {
		t.Fatal("literal --body - must reach the daemon so the cached view is invalidated")
	}
}

func TestExplicitGETWithQueryFieldsUsesDaemon(t *testing.T) {
	args := []string{"api", "--method=GET", "search/issues", "-F", "q=is:open"}
	class := allowlist.NewClassifier(nil).Classify(args)
	if class.Type != allowlist.Cacheable {
		t.Fatalf("type=%d, want cacheable", class.Type)
	}
	if commandReadsStdin(args) {
		t.Fatal("an explicit GET query field does not read stdin")
	}
}

type recordingDaemonClient struct {
	running bool
	request *protocol.Request
}

func (c *recordingDaemonClient) IsRunning() bool { return c.running }
func (c *recordingDaemonClient) Send(req *protocol.Request) (*protocol.Response, error) {
	c.request = req
	return &protocol.Response{}, nil
}

func TestInvalidateRepoCacheAfterDirectCommand(t *testing.T) {
	ctx := execctx.ExecContext{Host: "github.com", Repo: "owner/repo"}
	cl := &recordingDaemonClient{running: true}
	if err := invalidateRepoCacheAfterDirectCommand(cl, ctx); err != nil {
		t.Fatal(err)
	}
	if cl.request == nil || cl.request.Type != protocol.TypeFlush || cl.request.Context != ctx {
		t.Fatalf("request=%+v, want repo-scoped cache flush for %+v", cl.request, ctx)
	}

	stopped := &recordingDaemonClient{}
	if err := invalidateRepoCacheAfterDirectCommand(stopped, ctx); err != nil {
		t.Fatal(err)
	}
	if stopped.request != nil {
		t.Fatalf("stopped daemon received request: %+v", stopped.request)
	}
}

func TestDirectCommandRunnerCallsAfterOnFailure(t *testing.T) {
	const helperEnv = "GHX_DIRECT_RUNNER_TEST_EXIT"
	if os.Getenv(helperEnv) == "1" {
		os.Exit(7)
	}
	t.Setenv(helperEnv, "1")

	invalidated := false
	code := runDirectAndAfter(os.Args[0], []string{"-test.run=^TestDirectCommandRunnerCallsAfterOnFailure$"}, func() {
		invalidated = true
	})
	if code != 7 {
		t.Fatalf("exit code=%d, want 7", code)
	}
	if !invalidated {
		t.Fatal("post-run cache invalidation was not called")
	}
}
