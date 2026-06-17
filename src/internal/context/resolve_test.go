package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHostUsesGHHost(t *testing.T) {
	t.Setenv("GH_HOST", "ghe.example.com")

	if got := resolveHost(); got != "ghe.example.com" {
		t.Fatalf("resolveHost() = %q, want ghe.example.com", got)
	}
}

func TestResolveHostUsesSingleGHConfigHost(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "hosts.yml"), []byte("ghe.example.com:\n  oauth_token: redacted\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := resolveHost(); got != "ghe.example.com" {
		t.Fatalf("resolveHost() = %q, want ghe.example.com", got)
	}
}

func TestResolveHostPrefersGitHubDotComWhenConfigured(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "hosts.yml"), []byte("ghe.example.com:\n  oauth_token: redacted\ngithub.com:\n  oauth_token: redacted\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := resolveHost(); got != "github.com" {
		t.Fatalf("resolveHost() = %q, want github.com", got)
	}
}

func TestResolveTokenHashUsesEnvTokenBeforeGHConfig(t *testing.T) {
	t.Setenv("GH_TOKEN", "token-a")
	t.Setenv("GITHUB_TOKEN", "token-b")

	got := resolveTokenHash("/path/that/must/not/run", "github.com")
	want := hashToken("token-a")
	if got != want {
		t.Fatalf("resolveTokenHash() = %q, want %q", got, want)
	}
}

func TestResolveTokenHashUsesGitHubTokenFallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token-b")

	got := resolveTokenHash("/path/that/must/not/run", "github.com")
	want := hashToken("token-b")
	if got != want {
		t.Fatalf("resolveTokenHash() = %q, want %q", got, want)
	}
}

func TestResolveTokenHashUsesEnterpriseTokenForEnterpriseHost(t *testing.T) {
	t.Setenv("GH_ENTERPRISE_TOKEN", "enterprise-token")
	t.Setenv("GH_TOKEN", "dotcom-token")

	got := resolveTokenHash("/path/that/must/not/run", "ghe.example.com")
	want := hashToken("enterprise-token")
	if got != want {
		t.Fatalf("resolveTokenHash() = %q, want %q", got, want)
	}
}
