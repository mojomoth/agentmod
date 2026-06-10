# STATE — current implementation state

Last updated: 2026-06-11 (iteration: Phase 3 — guard claude-bash engine +
CLI, T16; Env.Stdin injection)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 COMPLETE. Phase 2 COMPLETE (init +
  both shell hooks + rc editor + env-hygiene integration tests + the
  first-session diagnosis). Phase 3 IN PROGRESS: all five doctor slices +
  guard claude-bash done; guard wiring into settings.json (T17) + auth
  copy-on-consent remain.
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
- `agentmod hook zsh` LANDED and green (T09 ✅, D017): new
  `internal/shellhook` package (`Zsh()` returns the script) +
  `internal/cli/hook.go` (runHook; zsh supported, bash → "not implemented
  yet", others rejected). Wired into dispatcher + usage. Script: pure-zsh
  `_agentmod_find_root` upward walk, `_agentmod_hook` on precmd+chpwd
  (dedup-guarded registration), transitions eval `agentmod env`; failed-root
  cache + A→broken-B fallback deactivate + missing-binary warn-once — full
  contract in D017 (read it AND D016 before touching hook/env code).
  - 9 test funcs in hook_test.go: command table, script-content anchors,
    `zsh -n` syntax gate, cd-in/out (vars+PATH set/restored, HOME untouched,
    OPENCODE_CONFIG set, XDG unset in partial mode), nested nearest-wins,
    precmd new-shell activation (interactive zsh), missing-binary warn-once,
    broken-config error-once + old-project deactivation, double-eval
    single-registration. All run a REAL `zsh -f` (t.Skip if absent) with the
    test binary impersonating agentmod via a TestMain dispatch on
    AGENTMOD_TEST_RUN_MAIN=1 + a /bin/sh wrapper on the child PATH — reuse
    this harness for the bash hook (T10) and integration tests (T11).
  - macOS gotcha (cost one red run): zsh resolves its STARTING dir physically
    (/var→/private/var), so start-inside-project assertions need
    filepath.EvalSymlinks; plain `cd` keeps the logical path.
- `agentmod hook bash` LANDED and green (T10 ✅, D018): `shellhook.Bash()` +
  hook.go bash case wired (usage/error strings now say "zsh, bash").
  bash-3.2-clean script; PROMPT_COMMAND-only registration (no chpwd in
  bash) with `case ";$PC;"` dedup that preserves the user's existing
  PROMPT_COMMAND; same failed-root cache / warn-once / A→broken-B
  deactivate contract as zsh. 7 test funcs in new hook_bash_test.go
  (contents, `bash -n` gate, cd in/out, nested nearest-wins, interactive
  PROMPT_COMMAND new-shell activation, warn-once, broken-config-once,
  eval-twice-registers-once+keeps-user-entry) reusing hook_test.go's
  fakeAgentmodBin/childEnv/lineAfter harness; requireBash prefers
  /bin/bash so macOS tests real 3.2. Non-interactive bash never fires
  PROMPT_COMMAND, so those tests call `_agentmod_hook` explicitly and one
  `-i` run proves the registration path (stderr ignored there: forced-
  interactive bash without tty prints prompts + job-control notice).
  Known limitation for README/doctor: hook inert in non-interactive bash
  scripts (same as direnv).
- rc fenced-block editor LANDED and green (T08 ✅, D019): new
  `internal/cli/rcfile.go` (`ensureShellHook`/`shellHookTarget`/
  `ensureRCBlock`/`rcBlockFor`/`abbrevHome`), wired into runInit after
  ensureGitignore; the `Shell hook:` placeholder line is gone — it now
  reports installed/updated/already-installed in `~/.zshrc` (home
  abbreviated) or a skip reason. Full contract in D019 (read it before
  touching rc code): block = `command -v agentmod … && eval "$(agentmod
  hook <shell>)"` between `# >>> agentmod >>>`/`# <<< agentmod <<<`;
  zsh → ${ZDOTDIR:-$HOME}/.zshrc; bash → existing .bashrc > existing
  .bash_profile > create .bashrc; SHELL/HOME unset or unsupported shell →
  skip (exit 0); corrupt fence → hard error, zero writes. rc paths derive
  ONLY from injected Env, so tests never go near real rc files.
  10 new test funcs in rcfile_test.go (install/append-glue/idempotent/
  stale-update-in-place/--no-shell-hook-enforcement/skips table/bash rc
  selection ×3/ZDOTDIR/corrupt-fence ×2/zsh+bash -n block syntax).
  init_test.go's TestInitDefaultShellHookLine updated for the new skip
  wording ("not installed yet" placeholder no longer exists).
- scripted-shell env-hygiene tests LANDED and green (T11 ✅): new
  `internal/cli/integration_test.go`, `TestHookScriptedSessionEnvHygiene`
  with a {zsh, bash} table (`shellCases()`: per-shell `run` + `cd` snippet —
  bash appends an explicit `_agentmod_hook` since PROMPT_COMMAND never fires
  non-interactively, D018). One scripted session per shell does
  in→out→in→A→B(switch)→A→out and proves FABLE_PLAN §7 "perfect inverse":
  - Full `env | sort` dumps between `===ENV0===`/`===ENDENV0===` markers
    (parser `envSection` filters PWD/OLDPWD/SHLVL/`_`), diffed as maps —
    catches lost/changed/leaked vars, PATH + HOME restoration, and any
    lingering `AGENTMOD*` in one assertion set.
  - Sentinel pre-existing values (with spaces, a single quote, and `$`) on
    ALL routed vars + XDG_CONFIG_HOME; asserted overridden inside, restored
    mid-trip and after; `AGENTMOD_SAVED_CLAUDE_CONFIG_DIR` equals the USER
    original even right after the A→B switch (D016: saves never capture our
    own routing). Quoting round-trip now automated (was manual smoke only).
  - PATH checkpoints inside A/B count entries via exact-match split:
    exactly one node/bin entry, always the CURRENT project's, zero dups
    after repeated transitions.
  - No-shims: `snapshotTree` (reused from init_test.go) over projA, projB,
    fakeAgentmodBin dir, and the start dir before/after the session;
    `diffTrees` reports created/removed/changed entries.
  - Helpers added: `shellCases`/`sentinelEnv`/`envSection`/
    `countPathEntries`/`diffTrees` — reuse them for the Phase 8 scenario
    matrix (T30) instead of inventing new ones.
  - Gotcha that cost one red run: section markers must differ between the
    open and close lines for BOTH dumps — parser expects `===ENDENV0===`,
    not `===END0===`.
- init first-session diagnosis LANDED and green (T08a ✅, D020):
  `hookActivationNotice(res, projectRoot, env)` in rcfile.go, printed by
  runInit between the "Shell hook:" line and the closing status hint.
  `ensureShellHook` now returns `shellHookResult{Line, Action, Shell}`
  (Action gained rcSkipped) instead of a bare string — future callers
  (doctor) can reuse both. Matrix in D020: AGENTMOD_ACTIVE=1 → live
  message (same root "already routing" / other root "switches at next
  prompt", printed even under --no-shell-hook); not live + block present →
  first-session caveat (new terminal / exec $SHELL / one-shot eval), with
  the "already-loaded hook picks it up next prompt" hedge ONLY for
  rcUpdated/rcUnchanged (an rcInstalled block is brand new, hook provably
  not loaded); not live + skipped → silent (skip reason suffices, CI
  quiet). TestInitHookActivationNotice: 6-case table in rcfile_test.go,
  fakeEnv only. No existing test needed changes (all stdout assertions
  are substring-based).
- `agentmod doctor` slice 1 LANDED and green (Phase 3 item 1 ✅, D021):
  `internal/cli/doctor.go` + doctor_test.go (13 test funcs), wired into
  dispatcher + usage. Checks: project discovery/agentmod root, config
  validity (error finding, keeps going — does NOT exit like status),
  layout completeness (missing dirs warn / non-dir entry errors), shell
  type + rc-block state (read-only), routing env (§23 warnings:
  installed-but-inactive w/ D020 remedies, in-project-vars-unset,
  other-root stale, per-var drift vs routing.Vars). Exit 0 clean / 3 any
  finding / 1 plumbing. Read D021 before extending doctor.
  - Refactors made for it (no behavior change, all old tests untouched):
    rcfile.go gained `locateRCBlock`/`rcFenceError`/`inspectRCBlock`
    (ensureRCBlock now built on them); status.go gained
    `routingEnvState(env)` (shellRoutingState built on it; status now uses
    routing.EnvActive/EnvProjectRoot constants instead of literals).
  - Severity policy (D021): outside a project, not-installed/skip findings
    are ok-level so a fresh machine exits 0; identical conditions inside a
    project warn. Outside-project routing check is SKIPPED entirely —
    lingering-vars is the next task; don't print "ok" for it meanwhile.
- doctor slice 2 LANDED and green (Phase 3 item 2 ✅, D022): four §23
  warning families added to doctor.go, all read-only, exit contract
  unchanged. Read D021 + D022 before extending doctor.
  - "Routing env" outside a project is now the LINGERING audit (the slice-1
    skip is gone): bookkeeping vars / SAVED_* / routed values containing an
    `.agentmod` path element / `.agentmod` PATH entries → warn with the
    deactivate-eval remedy; user's own routed-name vars stay silent. New
    `routing.RoutedNames()` is the probe superset (single source = Vars).
  - "PATH" (inside a project): dup NodeBinDir entries warn; missing while
    active+cfgOK+node-enabled warns; foreign `.agentmod` entries warn;
    exactly-1-while-inactive is deliberately ok (routingFinding owns that).
  - "HOME" (always): AGENTMOD_SAVED_HOME present or HOME inside an
    `.agentmod` → warn; unset HOME is ok-level.
  - "Shims" (inside a project): scans node/bin only for
    claude/codex/opencode; symlink resolving inside .agentmod = ok
    (project-local npm install, named in detail); anything else warns.
    EvalSymlinks both sides (macOS /var). `hasAgentmodElement` is the
    shared points-into-an-agentmod-root test.
  - healthyVars (doctor_test.go) now includes PATH with the project's
    node/bin once — new tests that perturb PATH must start from it.
    9 new test funcs; TestDoctorAllHealthy asserts the three new ok lines;
    fresh-machine test now asserts the ok lingering line instead of
    absence of "Routing env".
- doctor slice 3 LANDED and green (Phase 3 item 3 ✅, D023): per-agent
  findings added to doctor.go, read-only, exit contract unchanged. Read
  D021+D022+D023 before extending doctor.
  - "Claude home"/"Codex home": auth file state (claude/.credentials.json,
    codex/auth.json — constants in doctor.go per §12/§15). Present → ok;
    ABSENT → ok too (D023: not in §23's must-warn list; fresh projects
    must not exit 3 forever) with §12's exact re-login instruction in the
    detail; present-but-not-regular-file → warn. Disabled agents → ok
    "routing disabled (<key>.enabled = false)"; broken config treats all
    agents enabled. Global-auth comparison + copy prompt belong to the
    auth copy-on-consent task, NOT doctor.
  - "OpenCode config": opencode/opencode.json present ok / missing warn
    (re-init recreates) / non-regular error. Partial-isolation session +
    merge-chain warnings are the NEXT task — not covered here.
  - "Agent binaries" (in AND out of project, always ok-level):
    `statBinaryOnPath` stat-only PATH walk (executable regular file);
    exec.LookPath rejected — it reads the real PATH, not injected Env.
  - doctor_test.go `mkLayout` now also writes the opencode.json stub
    (matching init's guarantee) — new doctor tests relying on a healthy
    fixture get it for free. 7 new test funcs; TestDoctorAllHealthy +
    fresh-machine test assert the new ok lines.
- doctor slice 4 LANDED and green (Phase 3 item 4 ✅, D024): the two §15.3
  OpenCode leak findings added to doctor.go, read-only, exit contract
  unchanged. Read D021–D024 before extending doctor.
  - "OpenCode sessions": warns ONLY when the global data dir
    `${XDG_DATA_HOME:-$HOME/.local/share}/opencode` exists (sessions really
    are accumulating globally); absent → same limitation at ok level
    ("nothing stored there yet"), so default-config fixtures stay exit 0.
    The opt-in remedy (opencode.xdg_full_isolation = true) is in BOTH
    details.
  - "OpenCode merge chain": global `${XDG_CONFIG_HOME:-$HOME/.config}/
    opencode/opencode.json` with ≥1 top-level key besides `$schema` → warn
    listing sorted keys; absent/empty/`{}`/schema-only → ok; unparseable
    (JSONC) / unreadable / non-regular → conservative warn. Strict-JSON
    parse via stdlib encoding/json (no new dependency).
  - Both skipped entirely (no line) when opencode disabled; both ok when
    xdg_full_isolation on; broken config = defaults. Global paths resolve
    from injected Env only. Helpers `globalOpencodeDataDir` /
    `globalOpencodeConfigPath` / `opencodeConfigKeys` — reuse for handoff
    exclusions later. 8 new test funcs in doctor_test.go (+ helpers
    `wantNoFinding`, `writeGlobalOpencodeConfig`); TestDoctorAllHealthy
    asserts the two new ok lines. XDG-opt-in test gotcha: healthyVars
    builds routing vars from config.Default(), so overlay
    `routing.Vars(agentmodDir, cfg)` on top when the project cfg enables
    XDG, or misroutedVars warns.
- doctor slice 5 LANDED and green (Phase 3 item 5 ✅, D025): macOS Keychain
  note + gstack global/project findings, read-only, exit contract unchanged.
  Read D021–D025 before extending doctor.
  - `Env` struct (status.go) gained `GOOS string`; osEnv() fills
    runtime.GOOS; fakeEnv leaves "" (= not-darwin) so all tests are
    host-independent — tests wanting darwin set env.GOOS explicitly. ALL
    future platform-conditional code must read env.GOOS, not runtime.GOOS.
  - "Claude auth (macOS)" (keychainFindings): ok-level §15.1 statement
    (shared Keychain, no per-project account isolation, no re-login) on
    darwin + in-project + claude enabled; otherwise no line at all
    (skip-when-moot, D024 pattern). Broken config = defaults.
  - "gstack (global)": warns whenever $HOME/.claude/skills/gstack exists
    (Lstat — file/symlink/dir all count), in AND out of project — no
    out-of-project downgrade, it is a real §23 pollution risk, not a
    fresh-machine default. HOME unset → ok "cannot locate". NOTE: doctor on
    THIS dev machine correctly warns (the user's own global gstack, D010).
  - "gstack (project)" (inside project only): installed / not-installed
    both ok ("agentmod install gstack" named as remedy — Phase 4 ships it);
    non-directory entry warns. Not gated on claude.enabled.
  - Path constants gstackRelGlobal/gstackRelProject in doctor.go — the
    Phase 4 installer MUST reuse them. 7 new test funcs in doctor_test.go;
    TestDoctorAllHealthy asserts the two new gstack ok lines (no Keychain
    line there — fakeEnv GOOS is "").
- `agentmod guard claude-bash` LANDED and green (T16 ✅, D026): new
  `internal/guard` package (pure `Decide(input, home) Decision`, stdlib
  only) + `internal/cli/guard.go` (runGuard), wired into dispatcher +
  usage. Read D026 before touching guard code; T17 (settings.json wiring)
  consumes this command as-is.
  - `Env` gained `Stdin io.Reader` (osEnv = os.Stdin; fakeEnv leaves nil =
    empty input). Future stdin-consuming commands (auth consent prompt)
    must read env.Stdin, never os.Stdin.
  - Deny modes per §3.1: default exit 2 + "agentmod guard: BLOCKED: …" on
    stderr; `--json` exit 0 + hookSpecificOutput/permissionDecision=deny
    JSON on stdout. Allow = silent exit 0 in both modes.
  - Rules: sudo; HOME= reassignment; and (global-home reference AND
    write-cmd | git-clone | redirect-targeting-global). Protected: the four
    agent homes in ~/$HOME/${HOME}//Users/x//home/x spellings + literal
    injected HOME. Reads never blocked; redirect rule is target-scoped
    (narrower than dev-harness guard — see D026). npm -g deliberately not
    blocked (routing already localizes it). Fail-safe: unparseable input
    denies only on raw global-path reference; stdin read errors decide on
    partial bytes.
  - Tests: guard_test.go in internal/guard (37-command table + custom-HOME
    + non-Bash + unparseable×5) and internal/cli (exit codes, JSON shape,
    silence on allow, nil/erroring stdin, usage errors). Binary smoke of
    all three modes done via /tmp fixture files (see caution below).
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
Phase 3 seventh item: "init wires guard into .agentmod/claude/settings.json"
(TASKS.md Phase 3 top unchecked, T17). Before writing code:
- Read FABLE_PLAN §17 placement paragraph (DECIDED: hook config goes in the
  ROUTED home's settings, `.agentmod/claude/settings.json` — never the
  project's `.claude/settings.json`) and D026 (the command contract being
  wired).
- init must create/merge `.agentmod/claude/settings.json` with a PreToolUse
  hook entry: matcher "Bash" → command invoking the guard. IMPLEMENTATION_
  PLAN §11 says reference the ABSOLUTE agentmod binary path (re-resolved by
  doctor if the binary moved — that doctor finding can be part of T17 or a
  follow-up; decide and record). Binary path discovery: os.Executable()
  equivalent must be injectable for tests (extend Env, following the
  Getwd/LookupEnv/Stdin pattern).
- Respect init's never-overwrite discipline (D013-era guarantees): if
  settings.json already exists, MERGE the hook entry in (or detect
  present-and-correct = no-op) without clobbering user keys; re-init stays
  idempotent (T05's snapshotTree test style). Decide JSON read-modify-write
  via stdlib encoding/json.
- Tests: fresh init writes the hook; re-init no-op; existing settings.json
  with user keys preserved; hook command points at the resolved binary;
  project `.claude/settings.json` NEVER touched.

## Cautions for the next iteration
- Guard blocks shell output-redirection (`>>`) to absolute paths under $HOME
  even inside the repo — use the Write/Edit tools for project files instead
  of `cat >>`. It also blocks heredocs/echo whose CONTENT merely mentions
  global agent paths alongside write-words or `git clone` (bit this
  iteration twice: a printf smoke fixture and a DECISIONS.md heredoc).
  Write fixture JSON to /tmp files with the Write tool and pipe those;
  append to harness docs with Edit, not `cat <<EOF`.
- Do not reinstall mattpocock skills; verify via `skills-lock.json` only.
- `.agentmod/` is gitignored — never commit it.
- Tests must inject fake homes via parameters/env vars consumed by OUR code —
  never reassign the real `HOME` for the parent process, never touch real
  global agent homes (guard blocks it).
- BurntSushi/toml stays the ONLY dependency (D004).
- `config.Load` errors already name the file; don't re-wrap with the path.
- Substring assertions: beware "inactive" CONTAINS "active" — assert on
  "AgentMod: active" style anchored strings (bit us once in status tests).
