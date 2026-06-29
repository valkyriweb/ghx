package daemon

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunoborges/ghx/src/internal/allowlist"
	"github.com/brunoborges/ghx/src/internal/cache"
	"github.com/brunoborges/ghx/src/internal/config"
	execctx "github.com/brunoborges/ghx/src/internal/context"
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

func TestHandler_ForwardsRequestEnvToGH(t *testing.T) {
	cfg := &config.Config{GHPath: os.Args[0]}
	c := cache.New(100)
	cl := allowlist.NewClassifier(nil)
	s := metrics.New()
	h := NewHandler(cfg, c, cl, s)

	resp := h.Handle(&protocol.Request{
		Type: protocol.TypeExec,
		Args: []string{"-test.run=TestHandlerEnvHelperProcess", "--"},
		Env:  append(os.Environ(), "GO_WANT_HANDLER_ENV_HELPER=1", "GHX_HANDLER_ENV=client-token"),
	})

	if resp.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", resp.ExitCode, resp.Stderr)
	}
	got := strings.TrimSpace(string(resp.Stdout))
	if got != "client-token" {
		t.Fatalf("expected gh subprocess to see request env, got %q", got)
	}
}

func TestHandler_DoesNotCacheFailedReads(t *testing.T) {
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

func TestHandlerEnvHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HANDLER_ENV_HELPER") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("GHX_HANDLER_ENV"))
	os.Exit(0)
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
