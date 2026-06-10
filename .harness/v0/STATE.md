# STATE — current implementation state

Last updated: 2026-06-10 (iteration: Phase 2 task 2 — init `.gitignore` handling, T07)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 COMPLETE. Phase 2 items 1–2 LANDED.
- Go skeleton LANDED and green: `go.mod` (module
  `github.com/agentmod/agentmod`, go 1.26), thin `main.go`, `internal/cli`
  dispatcher with `--version`/`version`/`help`/unknown-command handling,
  exit-code constants (0/1/2/3 per IMPLEMENTATION_PLAN §3).
- `internal/project` LANDED and green (T01 ✅): `Discover(startDir)` upward
  walk, nearest-wins, lexical, marker must be a regular file (D011).
- `internal/config` LANDED and green (T02 ✅): schema v1,
  `Default()`/`Parse()`/`Load()`/`Validate()`/`Marshal()`, overlay-on-defaults,
  unknown keys rejected, hard rejects per D012. BurntSushi/toml v1.6.0 is the
  ONLY dependency (D004/D009).
- `agentmod status` LANDED and green (T03 ✅): `internal/cli/status.go`.
  - `Env` struct (Getwd + LookupEnv) injected through new unexported
    `run(args, stdout, stderr, env)`; public `Run` wraps it with `osEnv()`.
    Future commands needing cwd/env should take the same `Env`.
  - Inactive (§24): "AgentMod: inactive" + not-found + global-defaults lines,
    exit 0 (inactive is an answer, not an error; exit 2 stays reserved for
    commands that REQUIRE a project).
  - Active (§24): project root, agentmod root, Claude/Codex/OpenCode/Node
    paths from IMPLEMENTATION_PLAN §4 layout (constants currently live in
    status.go — extract to a shared layout/routing package when init lands),
    disabled agents show `disabled (<key> = false)`, XDG opt-in annotated.
  - "Shell routing" line: reports AGENTMOD_ACTIVE truth — not applied /
    applied / applied-for-different-root (stale) via AGENTMOD_PROJECT_ROOT.
  - "Recent handoff": newest `*.amod` by mtime in `.agentmod/snapshots/`,
    else "none". Broken config → exit 1, error on stderr (Load names file).
  - 10 test funcs in status_test.go, all temp-dir/fake-Env based.
- `agentmod init` core LANDED and green (T04 ✅): `internal/cli/init.go` +
  new shared `internal/layout` package (status.go refactored onto it).
  - Always inits at cwd; nested-under-existing-project prints a shadowing
    notice and proceeds (D013). Re-init = fill missing dirs only.
  - Never overwrites: agentmod.toml (= Marshal(Default())) and the
    opencode.json stub are written via O_CREATE|O_EXCL (`writeIfAbsent`);
    pre-existing files stay byte-identical (tested). `.agentmod` as a
    regular file → error, never deleted.
  - `layout.Subdirs()` = claude codex opencode node snapshots logs (NO
    opencode/xdg — opt-in mode creates that later).
  - Currently REJECTS all arguments ("init takes no arguments yet") —
    flags (--no-shell-hook/--yes) are the T06 iteration; .gitignore editing
    (T07) and rc-hook install (T08) are separate Phase 2 items, so init
    output deliberately says nothing about them yet.
  - 6 test funcs in init_test.go (fresh, re-init no-clobber incl. stray
    user file, .agentmod-is-a-file, nested warn, re-init-at-root no warn,
    arg rejection).
- init `.gitignore` handling LANDED and green (T07 ✅): new
  `internal/cli/gitignore.go` (`ensureGitignore`), wired into runInit after
  the opencode stub; init output gained a `.gitignore:` line. Semantics in
  D014: append-with-byte-preservation; dedup accepts
  `.agentmod[/ ]`/`/.agentmod[/]` trimmed (comments/negations don't count);
  missing file created only inside a git repo (lexical upward `.git` walk,
  any file type, no exec), else "skipped (not a git repository; re-run init
  after 'git init')"; an EXISTING .gitignore is extended even outside a
  repo; `.gitignore`-as-a-directory is a hard error. 10 test funcs in
  gitignore_test.go (created/appended/newline-glue/dedup-variants/non-git
  skip/non-git-existing-file extend/ancestor repo/worktree `.git` file/
  second-run no-op/dir error). Second-run no-op already covers the
  .gitignore slice of T05.
- `gofmt -l` clean, `go vet` clean, `go test ./...` PASSES (all packages).

## Toolchain baseline (verified on this machine, 2026-06-10)
- go 1.26.2 darwin/arm64 · claude 2.1.170 · codex-cli 0.137.0 · opencode 1.4.3
- jq present (guard hook depends on it, with a fail-safe fallback).

## Global-home baseline (CHECKS.md §2 — compare against this)
Recorded 2026-06-10, all timestamps PREDATE this repo (no writes by us);
re-verified unchanged this iteration:
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
Phase 2, third item: init idempotency guarantee tests (TEST_MATRIX T05).
Much is already covered piecemeal (re-init no-clobber in init_test.go,
.gitignore second-run no-op in gitignore_test.go); T05 wants one holistic
test: run init twice in a git repo, snapshot the FULL tree state (every
file's bytes + the dir set under cwd and .agentmod) after run 1 and assert
run 2 changes nothing and its stdout reports all-already-present
("already initialized", "left untouched" ×2, "already covers"). The
"no dup rc block" slice of T05 cannot be tested until the rc editor (T08)
exists — mark T05 🟡 with a note, or tick it and fold the rc slice into
T08's row (decide; rc-block idempotency is listed in T08 anyway).
Implementation should need NO product-code changes — if run 2 mutates
anything, that's a bug to fix this iteration. Then move to init flags
(--no-shell-hook, --yes) per TASKS.md.

## Cautions for the next iteration
- Guard blocks shell output-redirection (`>>`) to absolute paths under $HOME
  even inside the repo — use the Write/Edit tools for project files instead
  of `cat >>`.
- Do not reinstall mattpocock skills; verify via `skills-lock.json` only.
- `.agentmod/` is gitignored — never commit it.
- Tests must inject fake homes via parameters/env vars consumed by OUR code —
  never reassign the real `HOME` for the parent process, never touch real
  global agent homes (guard blocks it).
- BurntSushi/toml stays the ONLY dependency (D004).
- `config.Load` errors already name the file; don't re-wrap with the path.
- Substring assertions: beware "inactive" CONTAINS "active" — assert on
  "AgentMod: active" style anchored strings (bit us once in status tests).
