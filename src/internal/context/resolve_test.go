package context

import "testing"

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
