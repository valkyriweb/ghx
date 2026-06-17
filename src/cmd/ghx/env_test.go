package main

import "testing"

func TestDaemonRequestEnvFiltersCacheRelevantVars(t *testing.T) {
	env := daemonRequestEnv([]string{
		"GH_TOKEN=token",
		"GITHUB_TOKEN=github-token",
		"GH_HOST=github.example.com",
		"GH_REPO=owner/repo",
		"GH_CONFIG_DIR=/tmp/gh-config",
		"NO_COLOR=1",
		"PATH=/usr/bin",
		"GH_PROMPT_DISABLED=1",
	})

	want := []string{
		"GH_TOKEN=token",
		"GITHUB_TOKEN=github-token",
		"GH_HOST=github.example.com",
		"GH_REPO=owner/repo",
		"GH_CONFIG_DIR=/tmp/gh-config",
	}
	if len(env) != len(want) {
		t.Fatalf("env length = %d, want %d: %#v", len(env), len(want), env)
	}
	for i := range want {
		if env[i] != want[i] {
			t.Fatalf("env[%d] = %q, want %q", i, env[i], want[i])
		}
	}
}
