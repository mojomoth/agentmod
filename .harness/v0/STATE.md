# STATE — current implementation state

Last updated: 2026-06-10 (iteration: Phase 1 task 3 — internal/config)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 in progress (3 of 4 items done).
- Go skeleton LANDED and green: `go.mod` (module
  `github.com/agentmod/agentmod`, go 1.26, zero deps — toml deferred per
  D009), thin `main.go`, `internal/cli` dispatcher with `--version`/
  `version`/`help`/unknown-command handling, exit-code constants
  (0/1/2/3 per IMPLEMENTATION_PLAN §3), table tests + exit-code contract
  test.
- `internal/project` LANDED and green (T01 ✅): `Discover(startDir)` walks
  upward to filesystem root for `.agentmod/agentmod.toml`, nearest-wins,
  lexical (no symlink resolution), marker must be a regular file (D011).
  Exposes `Project{Root, AgentmodDir, ConfigPath}`, `ErrNotFound`,
  `DirName`/`ConfigFileName` constants. 7 tests, all temp-dir based.
- `internal/config` LANDED and green (T02 ✅): schema v1 per
  IMPLEMENTATION_PLAN §6, `Default()`/`Parse()`/`Load()`/`Validate()`/
  `Marshal()`. Overlay-on-defaults loading, unknown keys rejected, hard
  rejects for change_home/schema_version/mode/include_sessions (D012).
  `github.com/BurntSushi/toml v1.6.0` added to go.mod (D004/D009 fulfilled —
  this remains the ONLY dependency). 13 test funcs incl. §13 defaults pin,
  partial-override, round-trip, Load error paths.
  `gofmt -l` clean, `go vet` clean, `go test ./...` PASSES.

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
Phase 1, last unchecked item: `agentmod status` — active/inactive output
(TEST_MATRIX T03; spec FABLE_PLAN §24). Wire a `status` subcommand into
`internal/cli`: use `project.Discover(cwd)` + `config.Load(ConfigPath)`;
inactive → say so (exit 0 per §24 — re-check §24 wording before assuming);
active → print project root, `.agentmod` root, per-agent homes derived from
config (claude/codex/opencode/node enabled flags). Keep env-var *truth*
reporting minimal until the shell hook exists — status can only report what
WOULD be routed plus whether AGENTMOD_ACTIVE is actually set. Inject cwd,
env lookup, and stdout for tests (no real installs needed).

## Cautions for the next iteration
- Guard blocks shell output-redirection (`>>`) to absolute paths under $HOME
  even inside the repo — use the Write/Edit tools for project files instead
  of `cat >>`.
- Do not reinstall mattpocock skills; verify via `skills-lock.json` only.
- `.agentmod/` is gitignored — never commit it.
- Tests must inject fake homes via parameters/env vars consumed by OUR code —
  never reassign the real `HOME` for the parent process, never touch real
  global agent homes (guard blocks it).
- BurntSushi/toml is now IN go.mod — it stays the only dependency (D004).
- `config.Load` errors already name the file; don't re-wrap with the path.
