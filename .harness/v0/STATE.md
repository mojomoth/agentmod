# STATE — current implementation state

Last updated: 2026-06-10 (iteration: Phase 2 task 5a — `agentmod env` transitions, T09a)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 COMPLETE. Phase 2 items 1–4 LANDED
  + the env half of item 5.
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
- init idempotency LANDED and green (T05 ✅): `TestInitSecondRunIsNoOp` in
  init_test.go + `snapshotTree` helper (WalkDir → map of dir-set + full file
  bytes under cwd, incl. .agentmod and .gitignore). Runs init twice in a fake
  git repo (a bare `.git` dir satisfies insideGitRepo — no git exec),
  asserts run 2 changes/creates/removes NOTHING and its stdout reports
  all-already-present ("already initialized", "all directories already
  present", "already covers .agentmod/", 2× "already present, left
  untouched"). NO product-code change was needed — re-init was already a
  true no-op. Decision: T05 ticked ✅; its "no dup rc block" slice is folded
  into T08's matrix row (rc editor doesn't exist yet; T08 already lists
  rc-block idempotency).
- init flags LANDED and green (T06 ✅): `parseInitFlags` + `initOptions`
  struct in init.go. NOTE: the code was written by a prior iteration that got
  rate-limited mid-task before committing; this iteration verified it green,
  did the bookkeeping, and committed it.
  - Accepted: `--no-shell-hook`, `--yes`, `--non-interactive` (last two are
    synonyms → opts.NonInteractive). Unknown flag / positional arg →
    ExitError naming the offender, and init does NOT start creating anything
    (parse happens before any FS work — tested).
  - Output gained a `Shell hook:` line: "skipped (--no-shell-hook)" vs
    "not installed yet (rc-file setup lands with 'agentmod hook zsh')".
    T08's rc editor must consume `initOptions` (already threaded through
    runInit) and replace that placeholder.
  - `TestInitFlagsBuildIdenticalTree` proves every flag combo builds a
    byte-identical tree to plain init (snapshotTree reuse). No-prompt needs
    no test: runInit has no stdin parameter, so no code path can read input.
  - Honest scope: rc-skip ENFORCEMENT is T08's matrix row; auth-never-copy
    is Phase 3's. Both flags are parsed, validated, reported, and threaded
    NOW so those tasks only consume them.
- `agentmod env` LANDED and green (T09a ✅, D016): new `internal/routing`
  package (Vars(agentmodDir,cfg) in stable order, NodeBinDir, bookkeeping
  var-name constants) + `internal/cli/env.go` (parseEnvFlags, envModel/opList
  in-memory env modeling, appendActivate/appendDeactivate, shellQuote,
  strip/prependPathEntry). Wired into dispatcher + usage. Contract details in
  D016 (read it before touching env/hook code). 13 test funcs in env_test.go
  + 4 in routing_test.go; real bash+zsh eval smoke (incl. quote-bearing
  values, PATH strip) passed manually this iteration.
  - Node var choices: NPM_CONFIG_PREFIX=.agentmod/node (so npm global bin ==
    node/bin, the one managed PATH entry), NPM_CONFIG_CACHE=node/npm-cache,
    PNPM_HOME=node/pnpm, BUN_INSTALL=node/bun. pnpm/bun global bins are NOT
    on PATH in MVP — list under README limitations (Phase 8).
- `.gitignore` (repo's own): added `.harness/v0/reports/*/*.log` — loop.sh
  logs moved into per-run subdirs (e.g. reports/run1-ratelimited/) were
  not matched by the original one-level pattern and polluted git status.
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

2026-06-10 audit note: `~/.codex` mtime drifted to `6월 10 22:22` — inspected
contents: churn is codex-cli's OWN runtime files from the user's interactive
use (history.jsonl, logs_2.sqlite*, config.toml, shell_snapshots, …); zero
agentmod-named artifacts. Not our work, not a violation. Expect this dir's
mtime to keep moving; audit by looking for agentmod-created entries, not by
mtime equality. `~/.claude` and `~/.config/opencode` unchanged from baseline.

## Failing tests
None. All checks green as of this iteration's end.

## Exact next step
Phase 2: `agentmod hook zsh` emitter (TASKS.md Phase 2; FABLE_PLAN §14,
IMPLEMENTATION_PLAN §7, D007, D016). The env half is DONE — the hook
only has to: (1) define a pure-zsh upward search for
`.agentmod/agentmod.toml` (no binary exec on the hot path), (2) on
precmd+chpwd detect transitions by comparing the found root against
$AGENTMOD_PROJECT_ROOT, (3) on transition eval
`agentmod env --shell zsh --activate <root>` / `--deactivate`
(switching needs only --activate — env self-deactivates first, D016),
(4) no-op with a one-time warning if the agentmod binary is missing.
`agentmod hook zsh` PRINTS the script to stdout; rc-file editing stays
T08. Test via scripted child zsh processes: `zsh -f` (skips user rc
files — never touch the real ones), built test binary prepended to the
child's PATH, script = eval "$(agentmod hook zsh)" + cd sequences +
env assertions; the child process env may carry a temp HOME but the
parent test process must never reassign the real HOME. bash
(PROMPT_COMMAND) is the item after.

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
