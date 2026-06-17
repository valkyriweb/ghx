package context

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExecContext holds resolved execution context for building cache keys.
type ExecContext struct {
	Host      string `json:"host"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	TokenHash string `json:"token_hash"`
}

// Resolve gathers execution context from the current working directory and environment.
func Resolve(ghPath string) ExecContext {
	ctx := ExecContext{}

	// Host: GH_HOST env var or default from gh config.
	ctx.Host = resolveHost()

	// Repo: GH_REPO env var or from git remote
	if repo := os.Getenv("GH_REPO"); repo != "" {
		ctx.Repo = repo
	} else {
		ctx.Repo = resolveRepoFromGit()
	}

	// Branch: current git branch
	ctx.Branch = resolveCurrentBranch()

	// Auth: hash of the current auth token
	ctx.TokenHash = resolveTokenHash(ghPath, ctx.Host)

	return ctx
}

func resolveHost() string {
	if host := os.Getenv("GH_HOST"); host != "" {
		return host
	}
	if host := resolveHostFromGit(); host != "" {
		return host
	}
	if host := resolveHostFromGHConfig(); host != "" {
		return host
	}
	return "github.com"
}

func resolveHostFromGit() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseHostFromURL(strings.TrimSpace(string(out)))
}

func parseHostFromURL(url string) string {
	if strings.HasPrefix(url, "git@") {
		withoutPrefix := strings.TrimPrefix(url, "git@")
		if idx := strings.Index(withoutPrefix, ":"); idx != -1 {
			return withoutPrefix[:idx]
		}
	}
	if strings.HasPrefix(url, "ssh://git@") {
		withoutPrefix := strings.TrimPrefix(url, "ssh://git@")
		if idx := strings.Index(withoutPrefix, "/"); idx != -1 {
			return withoutPrefix[:idx]
		}
	}
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(url, prefix) {
			withoutPrefix := strings.TrimPrefix(url, prefix)
			if idx := strings.Index(withoutPrefix, "/"); idx != -1 {
				return withoutPrefix[:idx]
			}
			return withoutPrefix
		}
	}
	return ""
}

func resolveHostFromGHConfig() string {
	configDir := os.Getenv("GH_CONFIG_DIR")
	if configDir == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			configDir = filepath.Join(xdg, "gh")
		} else if home := os.Getenv("HOME"); home != "" {
			configDir = filepath.Join(home, ".config", "gh")
		}
	}
	if configDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, "hosts.yml"))
	if err != nil {
		return ""
	}
	hosts := map[string]any{}
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		return ""
	}
	if _, ok := hosts["github.com"]; ok {
		return "github.com"
	}
	if len(hosts) == 1 {
		for host := range hosts {
			return host
		}
	}
	return ""
}

func resolveRepoFromGit() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseRepoFromURL(strings.TrimSpace(string(out)))
}

// parseRepoFromURL extracts "owner/repo" from a git remote URL.
func parseRepoFromURL(url string) string {
	// Handle SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		if idx := strings.Index(url, ":"); idx != -1 {
			path := url[idx+1:]
			path = strings.TrimSuffix(path, ".git")
			return path
		}
	}

	// Handle HTTPS: https://github.com/owner/repo.git
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	return ""
}

func resolveCurrentBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func resolveTokenHash(ghPath, host string) string {
	if token := envTokenForHost(host); token != "" {
		return hashToken(token)
	}

	cmd := exec.Command(ghPath, "auth", "token", "--hostname", host)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return ""
	}
	return hashToken(token)
}

func envTokenForHost(host string) string {
	if host != "" && host != "github.com" {
		if token := os.Getenv("GH_ENTERPRISE_TOKEN"); token != "" {
			return token
		}
		if token := os.Getenv("GITHUB_ENTERPRISE_TOKEN"); token != "" {
			return token
		}
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return ""
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:8]) // first 8 bytes is enough for keying
}

// CacheKey builds a deterministic cache key from the execution context and command args.
func CacheKey(ctx ExecContext, args []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "host=%s\n", ctx.Host)
	fmt.Fprintf(h, "repo=%s\n", ctx.Repo)
	fmt.Fprintf(h, "branch=%s\n", ctx.Branch)
	fmt.Fprintf(h, "token=%s\n", ctx.TokenHash)
	fmt.Fprintf(h, "args=%s\n", strings.Join(args, "\x00"))
	return fmt.Sprintf("%x", h.Sum(nil))
}
