# Changelog

Fork of `brunoborges/ghx`. See `UPSTREAM.md` for provenance.

## Unreleased

### Fixed
- Forward filtered client auth/context environment to daemon-executed `gh`
  subprocesses, so per-call auth like `GH_TOKEN`/`GITHUB_TOKEN` is honored by a
  long-lived daemon instead of inheriting the daemon's startup identity.
- Apply `GHX_TTL`, `GHX_SOCKET`, and `GHX_GH_PATH` overrides even when
  `~/.ghx/config.yaml` does not exist.

## v1.6.0 — 2026-06-05

First fork release. Based on upstream `main` @ `7d70f48` (post-v1.5.2 `src/`
restructure, unreleased upstream).

### Fixed
- Daemon connection deadlines are now scoped per phase (read / handle / write),
  so long `gh` calls (auth device flow, large project queries, slow network) no
  longer trip `write response: i/o timeout` / `broken pipe`.
- `gh auth` (and stdin-piped commands) bypass the proxy and run direct — never
  cached or time-bounded.
- `ghx xdaemon stop/restart` reap a wedged daemon robustly: graceful shutdown →
  SIGTERM → SIGKILL, then clear stale socket/PID files.
- Eliminated the start bind race: a `flock` singleton lock ensures only one
  daemon runs, so a second start can't steal a live socket and orphan the first
  (which kept holding TCP :9847).

### Added
- `scripts/verify` (local gate) and `scripts/release` (one-command fork release
  + Homebrew tap update).
- `AGENTS.md`, `UPSTREAM.md`, this changelog.
