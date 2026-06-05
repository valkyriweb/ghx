# UPSTREAM.md — ghx fork provenance

This repo is a **fork** of [`brunoborges/ghx`](https://github.com/brunoborges/ghx),
a caching proxy daemon for the GitHub CLI (`gh`). Divergence is intentional.

| | |
|---|---|
| **Upstream** | https://github.com/brunoborges/ghx |
| **Fork** | https://github.com/valkyriweb/ghx (`origin`) |
| **Forked from** | `7d70f48` — "Move Go source into src/ folder and docs into specs/ folder (#12)" (upstream `main`, post-v1.5.2, unreleased upstream) |
| **Distribution** | Fork releases on `valkyriweb/ghx` + Homebrew tap `valkyriweb/homebrew-tap` (Formula/ghx.rb) |

## Why the fork

Upstream `gh auth refresh` and other long/interactive `gh` calls tripped the
daemon's connection deadline, flooding logs with `i/o timeout` / `broken pipe`
and forcing fallback to direct `gh`. `xdaemon stop/restart` also left orphaned
half-dead daemons (stale socket + a SIGTERM-ignoring process holding TCP :9847).
These are reliability fixes worth running now rather than waiting on an upstream
release.

## Divergence from upstream `main` (`7d70f48`)

Local commits on top of the fork point:

- **`fix: scope daemon connection deadlines + bypass proxy for gh auth`** —
  per-phase deadlines (10s read / clear across `Handle()` / 30s write); `gh auth`
  and stdin-piped commands run direct instead of through the daemon.
- **`fix: robust daemon stop/restart + eliminate the start bind race`** —
  graceful → SIGTERM → SIGKILL reap; `flock` singleton lock so only one daemon
  can bind; stale socket/PID cleanup. (`src/internal/daemon/{server,platform_unix,platform_windows}.go`,
  `src/cmd/ghx/{main,proc_unix,proc_windows}.go`, tests added.)
- **`ci(release): publish from this fork + its homebrew tap`** — release
  workflow points at `${{ github.repository }}` and `valkyriweb/homebrew-tap`.

## Refresh from upstream

```bash
git fetch upstream
git log --oneline HEAD..upstream/main        # what's new upstream
git rebase upstream/main                      # reapply fork patches on top
# resolve any conflicts (the src/ restructure already landed), then:
./scripts/verify
```

Cherry-pick interesting upstream commits explicitly; never auto-merge the whole
of upstream. If upstream eventually releases these same daemon fixes, retire the
fork divergence and consume upstream releases directly (update the tap formula
URLs back to `brunoborges/ghx`).

## Upstreaming

These fixes are candidates for an upstream PR to `brunoborges/ghx`. If accepted
and released upstream, prefer dropping the fork distribution and pointing the tap
back at upstream to minimize maintenance.
