# AGENTS.md — ghx (valkyriweb fork)

Caching proxy daemon for the GitHub CLI. **This is a fork** of `brunoborges/ghx`
— see `UPSTREAM.md` for provenance, divergence, and the refresh workflow.

## Layout

- `src/cmd/ghx` — client CLI (parses flags, talks to the daemon, falls back to direct `gh`).
- `src/cmd/ghxd` — daemon entrypoint.
- `src/internal/daemon` — server, request handling, lifecycle, platform process/lock helpers.
- `src/internal/{cache,client,config,ipc,protocol,executor,ghcli,allowlist,metrics}` — supporting packages.
- `specs/` — design docs (ADR.md, DOCS.md). `agent-plugin/` — Claude/Codex plugin.

Pure Go, no CGO — cross-compiles cleanly.

## Gate

```bash
./scripts/verify        # gofmt + go vet + go test -race + build
```

Run it before every push. CI (`.github/workflows/ci.yml`) runs the same checks.

## Release

```bash
./scripts/release vX.Y.Z
```

Builds darwin/linux (amd64+arm64), publishes a GitHub release on the fork, and
updates the `valkyriweb/homebrew-tap` formula with fresh checksums. Versioning is
a simple monotonic bump on the fork line (do not assume parity with upstream tags).

The CI release workflow (`release.yml`) is **disabled** so it can't re-upload
differently-compiled binaries over a local release (sha mismatch). To switch to
CI-driven releases, set the `HOMEBREW_TAP_TOKEN` repo secret (PAT with push to
`valkyriweb/homebrew-tap`) and re-enable the workflow.

## Fleet

ghx is installed as the `gh` on PATH across Luke's machines and (planned) k8s
runtime images. Install state + rollout: `~/Projects/agent-scripts/TOOLS/ghx.md`
and the `ghx-rollout` docs under `infra/`. After a release, upgrade with
`brew upgrade valkyriweb/tap/ghx` (macOS) or the tarball install (Ubuntu).

## Daemon gotchas

- `ghxd` auto-starts on first `ghx` call; one daemon per user (flock singleton).
- State lives in `~/.ghx/`: `ghxd.sock`, `ghxd.pid`, `ghxd.lock`, `ghxd.log`, `config.yaml`, `bin/gh`.
- Recovery from a wedged daemon: `ghx xdaemon restart` (now reaps hard), or
  `pkill -9 -f ghxd && rm -f ~/.ghx/ghxd.{sock,pid,lock} && gh --version`.
