# Changelog

Fork of `brunoborges/ghx`. See `UPSTREAM.md` for provenance.

## Unreleased

### Fixed
- Run commands directly when stdin is not a character device, or when an explicit
  stdin flag indicates an input read. This preserves input for `gh gist create`
  and unknown commands, then clears that repository's cached results. Cacheable
  reads with character-device stdin and no stdin flags still use the daemon.
- Cache completed `gh pr checks` reads, including exit 8 (pending) and exit 1
  (a check failed). Transport and auth failures are still not cached.
- Keep a cached read usable for 45s past its TTL so a poll that sleeps for the
  TTL does not miss on every iteration.
- Retry `gh pr checks --watch` and `gh run watch` after a transient network
  timeout on stderr instead of exiting. The watch listing stays on the
  terminal and is not scanned. Semantic failures are not retried.
- Do not cache `gh api` calls that are POSTs by default (`-f`, `-F`, `--input`,
  `graphql`) when `--method` is omitted. The last `--method` wins. An explicit
  GET is still cached, except when a magic `-F` value is read from a file.
- Update the Homebrew tap release audit snapshot alongside the GHX formula.
- Reject a release without its version heading in this changelog before building
  or publishing.
- Make detached daemon start/restart wait until `ghxd` accepts control requests,
  preventing `gh xdaemon restart && gh xcache flush` from racing a freshly
  spawned daemon.
- Bypass the daemon for all watch-style invocations, including
  `gh pr checks --watch`, so long-running live status commands do not die on IPC
  deadlines or get misread as CI/auth failures.
- Do not cache failed read responses; transient GitHub/API errors such as
  `HTTP 401: Bad credentials` are retried by the next caller instead of replayed
  from cache.
- Forward filtered client auth/context environment to daemon-executed `gh`
  subprocesses, so per-call auth like `GH_TOKEN`/`GITHUB_TOKEN` is honored by a
  long-lived daemon instead of inheriting the daemon's startup identity.
- Apply `GHX_TTL`, `GHX_SOCKET`, and `GHX_GH_PATH` overrides even when
  `~/.ghx/config.yaml` does not exist.

## v1.7.1 — 2026-08-17

### Fixed
- Stand down on TLS trust failure instead of wedging subsequent `gh` calls
  ([#5](https://github.com/valkyriweb/ghx/pull/5)).

Published release: [v1.7.1](https://github.com/valkyriweb/ghx/releases/tag/v1.7.1).

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
