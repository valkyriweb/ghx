package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHostUsesGHHost(t *testing.T) {
	t.Setenv("GH_HOST", "ghe.example.com")

	if got := resolveHost(); got != "ghe.example.com" {
		t.Fatalf("resolveHost() = %q, want ghe.example.com", got)
	}
}

func TestResolveHostFromGHConfigUsesSingleHost(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "hosts.yml"), []byte("ghe.example.com:\n  oauth_token: redacted\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := resolveHostFromGHConfig(); got != "ghe.example.com" {
		t.Fatalf("resolveHostFromGHConfig() = %q, want ghe.example.com", got)
	}
}

func TestParseHostFromURL(t *testing.T) {
	cases := map[string]string{
		"git@ghe.example.com:owner/repo.git":       "ghe.example.com",
		"ssh://git@ghe.example.com/owner/repo.git": "ghe.example.com",
		"https://ghe.example.com/owner/repo.git":   "ghe.example.com",
		"https://github.com/owner/repo.git":        "github.com",
	}
	for input, want := range cases {
		if got := parseHostFromURL(input); got != want {
			t.Fatalf("parseHostFromURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveHostFromGHConfigPrefersGitHubDotComWhenConfigured(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "hosts.yml"), []byte("ghe.example.com:\n  oauth_token: redacted\ngithub.com:\n  oauth_token: redacted\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := resolveHostFromGHConfig(); got != "github.com" {
		t.Fatalf("resolveHostFromGHConfig() = %q, want github.com", got)
	}
}

func TestCacheKeyIsolatesAuthIdentity(t *testing.T) {
	args := []string{"api", "user"}
	first := CacheKey(ExecContext{
		Host:      "github.com",
		Repo:      "owner/repo",
		Branch:    "main",
		TokenHash: tokenHash("account-one-token"),
	}, args)
	second := CacheKey(ExecContext{
		Host:      "github.com",
		Repo:      "owner/repo",
		Branch:    "main",
		TokenHash: tokenHash("account-two-token"),
	}, args)

	if first == second {
		t.Fatal("cache keys for different auth identities must differ")
	}
}

func TestTokenHashIsFullSHA256Fingerprint(t *testing.T) {
	got := tokenHash("secret-token")
	if len(got) != 64 {
		t.Fatalf("tokenHash() length = %d, want 64", len(got))
	}
	if strings.Contains(got, "secret-token") {
		t.Fatal("tokenHash() exposed the raw token")
	}
	if got != tokenHash("secret-token") {
		t.Fatal("tokenHash() is not deterministic")
	}
}
