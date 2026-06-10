# STATE — current implementation state

Last updated: 2026-06-10 (Phase 0 — harness scaffold session)

## Where things stand
- Phase 0 (harness) is COMPLETE: git repo initialized; all harness docs
  written; `loop.sh` ready (bounded, DONE-verified); PreToolUse guard wired
  via `.claude/settings.json` and smoke-tested (16/16 cases pass);
  skills installed project-locally; `IMPLEMENTATION_PLAN.md` written.
- Phase 1 (Go skeleton) is NOT STARTED. There is no `go.mod` yet —
  `go build` / `go test` will fail until the first Phase 1 task lands.

## Toolchain baseline (verified on this machine, 2026-06-10)
- go 1.26.2 darwin/arm64 · claude 2.1.170 · codex-cli 0.137.0 · opencode 1.4.3
- jq present (guard hook depends on it, with a fail-safe fallback).

## Global-home mtime baseline (CHECKS.md §2)
Record the output of `ls -ldT ~/.claude ~/.codex ~/.config/opencode` on your
first check this iteration and compare on later iterations. Baseline at
scaffold time: untouched by this project (no writes performed).

## Failing tests
None exist yet (no Go code). First failure to expect: nothing builds until
`go.mod` + `main.go` land.

## Exact next step
Phase 1, first task in TASKS.md: create `go.mod`
(module github.com/agentmod/agentmod, go 1.26, dep BurntSushi/toml),
`main.go` with the subcommand dispatcher skeleton and `--version`, plus a
first trivial test so `go test ./...` is green from the very first commit.

## Cautions for the next iteration
- Do not reinstall mattpocock skills; verify via `skills-lock.json` only.
- `.agentmod/` is gitignored — never commit it (it will appear once we
  dogfood `agentmod init` here).
- Tests must inject fake homes via parameters/env vars consumed by OUR code —
  never reassign the real `HOME` in a way that affects the parent process,
  and never touch real global agent homes (guard blocks it).
