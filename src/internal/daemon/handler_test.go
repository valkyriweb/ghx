package daemon

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brunoborges/ghx/src/internal/allowlist"
	"github.com/brunoborges/ghx/src/internal/authenv"
	"github.com/brunoborges/ghx/src/internal/cache"
	"github.com/brunoborges/ghx/src/internal/config"
	execctx "github.com/brunoborges/ghx/src/internal/context"
	"github.com/brunoborges/ghx/src/internal/executor"
	"github.com/brunoborges/ghx/src/internal/metrics"
	"github.com/brunoborges/ghx/src/internal/protocol"
)

func TestSanitizeCmdKey(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "two args", args: []string{"pr", "list"}, want: "pr_list"},
		{name: "many args", args: []string{"api", "-H", "Authorization: token secret", "/repos"}, want: "api_-H"},
		{name: "single arg", args: []string{"auth"}, want: "auth"},
		{name: "empty args", args: nil, want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCmdKey(tt.args)
			if got != tt.want {
				t.Errorf("sanitizeCmdKey(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestHandler_DoesNotCacheFailedReads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh is a #!/bin/sh script; not executable on windows")
	}
	dir := t.TempDir()
	countFile := dir + "/count"
	fakeGH := dir + "/gh"
	if err := os.WriteFile(fakeGH, []byte(`#!/bin/sh
count=0
if [ -f "`+countFile+`" ]; then
	count=$(cat "`+countFile+`")
fi
count=$((count + 1))
printf '%s' "$count" > "`+countFile+`"
if [ "$count" -eq 1 ]; then
	echo 'HTTP 401: Bad credentials' >&2
	exit 1
fi
echo 'ok'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{GHPath: fakeGH, TTL: 60 * time.Second}
	c := cache.New(100)
	cl := allowlist.NewClassifier(nil)
	s := metrics.New()
	h := NewHandler(cfg, c, cl, s)
	req := &protocol.Request{
		Type: protocol.TypeExec,
		Args: []string{"pr", "view", "123", "--json", "number"},
		Context: execctx.ExecContext{
			Host:      "github.com",
			Repo:      "owner/repo",
			Branch:    "main",
			TokenHash: "token",
		},
	}

	first := h.Handle(req)
	if first.ExitCode == 0 {
		t.Fatalf("expected first read to fail")
	}

	second := h.Handle(req)
	if second.ExitCode != 0 {
		t.Fatalf("expected second read to retry instead of returning cached failure, got %d: %s", second.ExitCode, second.Stderr)
	}
	if got := strings.TrimSpace(string(second.Stdout)); got != "ok" {
		t.Fatalf("expected second read stdout ok, got %q", got)
	}
	if got := strings.TrimSpace(mustReadFile(t, countFile)); got != "2" {
		t.Fatalf("expected fake gh to execute twice, got count %s", got)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHandler_GHPath_AtomicAccess(t *testing.T) {
	cfg := &config.Config{GHPath: "/usr/bin/gh"}
	c := cache.New(100)
	cl := allowlist.NewClassifier(nil)
	s := metrics.New()

	h := NewHandler(cfg, c, cl, s)

	// Initial value from config
	if got := h.GHPath(); got != "/usr/bin/gh" {
		t.Errorf("GHPath() = %q, want %q", got, "/usr/bin/gh")
	}

	// Update via SetGHPath
	h.SetGHPath("/opt/homebrew/bin/gh")
	if got := h.GHPath(); got != "/opt/homebrew/bin/gh" {
		t.Errorf("GHPath() after set = %q, want %q", got, "/opt/homebrew/bin/gh")
	}

	// Concurrent reads and writes — verifies no data race under -race
	paths := []string{
		"/usr/local/bin/gh",
		"/opt/homebrew/bin/gh",
		"/home/user/.ghx/bin/gh",
		"/snap/bin/gh",
	}

	const goroutines = 20
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				// Writer
				for j := range iterations {
					h.SetGHPath(paths[j%len(paths)])
				}
			} else {
				// Reader
				for range iterations {
					got := h.GHPath()
					// Value must always be a valid path we've set
					valid := false
					for _, p := range paths {
						if got == p {
							valid = true
							break
						}
					}
					if !valid && got != "/opt/homebrew/bin/gh" {
						t.Errorf("GHPath() returned unexpected value: %q", got)
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestHandler_NoCacheUsesEachClientsAuthEnvironment(t *testing.T) {
	h := newTestHandler()
	h.execute = func(_ context.Context, _ string, _ []string, _ string, env authenv.Environment) *executor.Result {
		login := "configured-account"
		if token := env["GH_TOKEN"]; token != "" {
			login = token
		}
		return &executor.Result{Stdout: []byte(login)}
	}

	tests := []struct {
		name string
		env  authenv.Environment
		want string
	}{
		{name: "first token", env: authenv.Environment{"GH_TOKEN": "account-one"}, want: "account-one"},
		{name: "second token", env: authenv.Environment{"GH_TOKEN": "account-two"}, want: "account-two"},
		{name: "configured account without token", want: "configured-account"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := h.Handle(&protocol.Request{
				Type:    protocol.TypeExec,
				Args:    []string{"api", "user"},
				AuthEnv: tt.env,
				NoCache: true,
			})
			if got := string(resp.Stdout); got != tt.want {
				t.Fatalf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandler_CacheKeysIsolateAuthIdentity(t *testing.T) {
	h := newTestHandler()
	var calls int
	h.execute = func(_ context.Context, _ string, _ []string, _ string, env authenv.Environment) *executor.Result {
		calls++
		return &executor.Result{Stdout: []byte(env["GH_TOKEN"])}
	}

	request := func(token, tokenHash string) *protocol.Response {
		return h.Handle(&protocol.Request{
			Type:    protocol.TypeExec,
			Args:    []string{"api", "user"},
			AuthEnv: authenv.Environment{"GH_TOKEN": token},
			Context: execctx.ExecContext{Host: "github.com", TokenHash: tokenHash},
		})
	}

	first := request("account-one", "hash-one")
	second := request("account-two", "hash-two")
	firstAgain := request("account-one", "hash-one")

	if got := string(first.Stdout); got != "account-one" {
		t.Fatalf("first stdout = %q", got)
	}
	if got := string(second.Stdout); got != "account-two" {
		t.Fatalf("second stdout = %q", got)
	}
	if got := string(firstAgain.Stdout); got != "account-one" || !firstAgain.Cached {
		t.Fatalf("repeated first response = %q, cached = %v", got, firstAgain.Cached)
	}
	if calls != 2 {
		t.Fatalf("executions = %d, want 2 isolated identities", calls)
	}
}

func TestHandler_SingleflightIsolatesAuthIdentity(t *testing.T) {
	h := newTestHandler()
	started := make(chan string, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	h.execute = func(_ context.Context, _ string, _ []string, _ string, env authenv.Environment) *executor.Result {
		calls.Add(1)
		token := env["GH_TOKEN"]
		started <- token
		<-release
		return &executor.Result{Stdout: []byte(token)}
	}

	requests := []*protocol.Request{
		{
			Type:    protocol.TypeExec,
			Args:    []string{"api", "user"},
			AuthEnv: authenv.Environment{"GH_TOKEN": "account-one"},
			Context: execctx.ExecContext{Host: "github.com", TokenHash: "hash-one"},
		},
		{
			Type:    protocol.TypeExec,
			Args:    []string{"api", "user"},
			AuthEnv: authenv.Environment{"GH_TOKEN": "account-two"},
			Context: execctx.ExecContext{Host: "github.com", TokenHash: "hash-two"},
		},
	}

	var wg sync.WaitGroup
	responses := make([]*protocol.Response, len(requests))
	for i, req := range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses[i] = h.Handle(req)
		}()
	}

	seen := make(map[string]bool)
	for range requests {
		select {
		case token := <-started:
			seen[token] = true
		case <-time.After(time.Second):
			close(release)
			wg.Wait()
			t.Fatal("requests with different auth identities were coalesced")
		}
	}
	close(release)
	wg.Wait()

	if !seen["account-one"] || !seen["account-two"] {
		t.Fatalf("executed identities = %v", seen)
	}
	if calls.Load() != 2 {
		t.Fatalf("executions = %d, want 2", calls.Load())
	}
	for i, want := range []string{"account-one", "account-two"} {
		if got := string(responses[i].Stdout); got != want {
			t.Fatalf("response %d = %q, want %q", i, got, want)
		}
	}
}

func newTestHandler() *Handler {
	cfg := &config.Config{GHPath: "gh", MaxCacheEntries: 100, TTL: 30 * time.Second}
	return NewHandler(cfg, cache.New(100), allowlist.NewClassifier(nil), metrics.New())
}

// A trust fault must reap the daemon exactly once, no matter how many calls hit it.
func TestExecGHReapsDaemonOnTrustFailure(t *testing.T) {
	h := newTestHandler()
	h.execute = func(context.Context, string, []string, string, authenv.Environment) *executor.Result {
		return &executor.Result{
			ExitCode: 1,
			Stderr:   []byte("tls: failed to verify certificate: x509: OSStatus -26276"),
		}
	}
	reaped := 0
	h.SetOnTrustFailure(func() { reaped++ })

	h.execGH([]string{"api", "/rate_limit"}, "", nil)
	h.execGH([]string{"api", "graphql"}, "", nil)

	if reaped != 1 {
		t.Fatalf("trust failure reaped daemon %d times, want exactly 1", reaped)
	}
}

// An ordinary failure must never reap the daemon -- a 401 is a credential problem.
func TestExecGHDoesNotReapOnOrdinaryFailure(t *testing.T) {
	h := newTestHandler()
	h.execute = func(context.Context, string, []string, string, authenv.Environment) *executor.Result {
		return &executor.Result{ExitCode: 1, Stderr: []byte("HTTP 401: Bad credentials")}
	}
	h.SetOnTrustFailure(func() { t.Fatal("ordinary failure must not reap the daemon") })

	h.execGH([]string{"api", "/rate_limit"}, "", nil)
}
