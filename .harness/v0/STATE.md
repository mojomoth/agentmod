# STATE — current implementation state

Last updated: 2026-06-10 (iteration: Phase 1 task 1 — Go skeleton)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 started.
- Go skeleton LANDED and green: `go.mod` (module
  `github.com/agentmod/agentmod`, go 1.26, zero deps — toml deferred per
  D009), thin `main.go`, `internal/cli` dispatcher with `--version`/
  `version`/`help`/unknown-command handling, exit-code constants
  (0/1/2/3 per IMPLEMENTATION_PLAN §3), table tests + exit-code contract
  test. `gofmt -l` clean, `go vet` clean, `go test ./...` PASSES.

## Toolchain baseline (verified on this machine, 2026-06-10)
- go 1.26.2 darwin/arm64 · claude 2.1.170 · codex-cli 0.137.0 · opencode 1.4.3
- jq present (guard hook depends on it, with a fail-safe fallback).

## Global-home baseline (CHECKS.md §2 — compare against this)
Recorded 2026-06-10, all timestamps PREDATE this repo (no writes by us):
```
drwxr-xr-x 23 jeongyounglee staff  736  6월  6 10:18:56 2026 ~/.claude
drwxr-xr-x 34 jeongyounglee staff 1088  6월  5 20:51:55 2026 ~/.codex
drwxr-xr-x 10 jeongyounglee staff  320  4월 30 13:53:29 2026 ~/.config/opencode
```
`~/.claude/skills` baseline contains the user's own pre-existing gstack
entries: `gstack`, `gstack-upgrade`, `open-gstack-browser` (D010). These are
NOT a violation; only new entries/mtime changes caused by our work are.

## Failing tests
None. All checks green as of this iteration's end.

## Exact next step
Phase 1, next unchecked TASKS.md item: `internal/project` — upward discovery
of `.agentmod/agentmod.toml` (found in cwd; found in ancestor; nearest-wins
with nested projects; not found; stops at filesystem root) + tests
(TEST_MATRIX T01). Use temp dirs; no real agent homes involved.

## Cautions for the next iteration
- Guard blocks shell output-redirection (`>>`) to absolute paths under $HOME
  even inside the repo — use the Write/Edit tools for project files instead
  of `cat >>`.
- Do not reinstall mattpocock skills; verify via `skills-lock.json` only.
- `.agentmod/` is gitignored — never commit it.
- Tests must inject fake homes via parameters/env vars consumed by OUR code —
  never reassign the real `HOME` for the parent process, never touch real
  global agent homes (guard blocks it).
- Add `github.com/BurntSushi/toml` only when `internal/config` lands (D009).
