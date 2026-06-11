# STATE — current implementation state

Last updated: 2026-06-11 (iteration: Phase 6 slice 4 — restore portability
pass: guard-hook rewrite + MCP absolute-path warnings, D044; T27 ✅)

## Where things stand
- Phase 0 (harness) COMPLETE. Phase 1 COMPLETE. Phase 2 COMPLETE (init +
  both shell hooks + rc editor + env-hygiene integration tests + the
  first-session diagnosis). Phase 3 COMPLETE (six doctor slices + guard
  claude-bash + guard wiring T17 + auth copy-on-consent T15). Phase 4
  COMPLETE (install gstack clone + --force + pollution verification +
  distinct error reporting; T18 ✅). Phase 5 COMPLETE: slice 1 (.amod
  writer) + slice 2 (default exclusion engine, T20 ✅) + slice 3 (secret
  scan + REDACTION.md, T21 ✅) + slice 4 (HANDOFF.md + RESTORE.md docs,
  D037; T19 ✅) + slice 5 (git state metadata + --allow-dirty, D039;
  T22 ✅) + slice 6 (inspect/verify/list + pack alias, D040; T23 ✅).
  Phase 6 IN PROGRESS: slice 1 (restore validation layer PlanRestore,
  D041; T24) + slice 2 (pre-restore backup BackupAgentmod, D042; T25) +
  slice 3 (extraction: Restore + `handoff restore` cli, D043; T24+T25 ✅,
  T26 🟡) + slice 4 (portability pass: guard rewrite + MCP warnings,
  D044; T27 ✅) done; remaining: post-restore notices/doctor + unpack
  alias, then the new doctor-MCP-finding follow-up item.
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
- init guard wiring LANDED and green (T17 ✅, D027): new
  `internal/cli/claudesettings.go` (`ensureClaudeGuardHook`), called by
  runInit between the opencode stub and ensureGitignore; init output gained
  a `Claude guard:` line. Read D027 before touching settings.json code.
  - `Env` gained `Executable func() (string, error)` (osEnv = os.Executable;
    fakeEnv returns fixed `/fake/bin/agentmod` → every init test exercises
    wiring). Hook command = `shellQuote(filepath.Clean(bin)) + " guard
    claude-bash"`, matcher "Bash". Clean is load-bearing: os.Executable can
    return `…/proj/../agentmod` spellings (caught in binary smoke).
  - Merge mirrors the rc editor's discipline: absent → create;
    present-and-correct → ZERO writes (bytes preserved); stale binary path →
    repaired in place via the "guard claude-bash" ownership marker (no dup
    entries); missing-from-existing → appended with all user keys preserved
    (stdlib re-marshal, keys sorted); whitespace-only = `{}`; invalid JSON /
    wrong-typed hooks keys → hard error, zero writes. Unresolvable binary →
    skip line, no file, exit 0. Project `.claude/settings.json` never
    touched (tested byte-identical).
  - Tests: 9 funcs in claudesettings_test.go; TestInitReinitNeverOverwrites'
    stray file moved off claude/settings.json (now a MANAGED file) to
    claude/user-notes.md; TestInitSecondRunIsNoOp asserts the
    already-configured line and still proves byte-identical re-init.
  - Follow-up task added to TASKS.md Phase 3: doctor finding for guard
    wired/stale-binary state (IMPLEMENTATION_PLAN §11 "re-resolved by
    doctor"; deliberately NOT part of T17, see D027).
- auth copy-on-consent LANDED and green (T15 ✅, D028): new
  `internal/cli/auth.go` (`bootstrapAuth`/`bootstrapOneAuth`/`authPrompter`/
  `copyAuthFile`), called by runInit after the hook-activation notice; init
  output gained aligned `Claude auth:` / `Codex auth:` lines. Read D028
  before touching auth code.
  - Decision ladder per agent (first match wins): darwin Claude → Keychain
    note, NO file flow; local auth present → left untouched, no prompt;
    HOME unset / global absent / global non-regular → remedy line, no
    prompt; NonInteractive → never reads stdin, never copies; else `[y/N]`
    prompt on stdout reading env.Stdin (D026). Only explicit y/yes copies;
    EOF/nil-stdin/anything-else declines; decline is exit 0.
  - Copy = ReadFile global + O_CREATE|O_EXCL write, mode 0600. One shared
    bufio.Reader across both prompts (partial final line counts).
  - Shared strings: `claudeReloginRemedy`/`codexReloginRemedy` consts now
    in doctor.go (agentHomeFindings uses them too);
    `globalClaudeDirName`/`globalCodexDirName` in auth.go.
  - Config deliberately not consulted (D027 pattern): explicit consent is
    the gate, not claude.enabled.
  - Phase 5 NOTE: consent-copied targets `claude/.credentials.json` +
    `codex/auth.json` (rel to .agentmod/) MUST be on the T20 exclusion
    list — D028 records this.
  - 10 test funcs in auth_test.go (all fake-HOME via injected Env;
    sk-FAKE fixture values). No existing test needed changes.
- doctor Claude-guard wiring finding LANDED and green (Phase 3 final item ✅,
  D029): `guardFinding(agentmodDir, env)` in doctor.go, inside-project only,
  read-only. Read D029 (+D021/D027) before touching guard-state code.
  - claudesettings.go grew the read-only half: `guardHookEntries(pre)`
    (shared marker-walker; ensureClaudeGuardHook rewritten on it, write
    behavior unchanged, T17 tests untouched) + `inspectGuardHook(path,
    desired)` → FileAbsent/Missing/Stale/Current + found command.
  - Severities: wired-current ok; file absent / hook absent → warn (re-run
    init); stale command → warn naming found AND expected commands;
    unparseable / wrong-typed file → error (writer's own hard-error
    strings); env.Executable nil-or-erroring → ok "hook present … binary
    path not verified" (file-absent/hook-missing warns still fire — they
    need no binary). Not gated on claude.enabled (D027).
  - doctor_test.go: mkLayout now writes a guard-wired settings.json for
    fakeBinPath (`writeGuardSettings`/`guardSettingsPath` helpers — reuse
    them), matching init's guarantee; layout tests deleting claude/ moved
    to os.RemoveAll. 5 new test funcs + AllHealthy/fresh-machine
    assertions extended.
- `agentmod install gstack` LANDED and green (Phase 4 slice 1 ✅, D030, T18
  🟡): new `internal/cli/install.go` (runInstall + installGstack), wired
  into dispatcher + usage. Read D030 before touching installer code.
  - Lives in internal/cli, NOT internal/installer (D030: needs
    gstackRelProject/exit codes/Env — single consumer, no new package).
  - Flow: require project (ErrNotFound → exit 2 naming 'agentmod init');
    target = agentmodDir/gstackRelProject (REUSED from doctor.go per D025);
    Lstat-exists → "already installed … remove that directory to reinstall"
    exit 1 (never clobber; --force is the NEXT slice and is currently
    rejected by arg validation); git via real exec.LookPath (documented:
    install EXECUTES git, unlike doctor's injected-Env statBinaryOnPath);
    clone source = AGENTMOD_GSTACK_SOURCE via env.LookupEnv if set, else
    https://github.com/garrytan/gstack; MkdirAll skills dir → MkdirTemp
    sibling ".gstack-clone-*" → `git clone -- <src> <tmp>` with
    GIT_TERMINAL_PROMPT=0 → os.Rename onto target (atomic, same fs);
    deferred RemoveAll cleans temp on failure. `.git` kept in the clone.
  - 5 test funcs in install_test.go (fixture repo via local `git init` +
    commit with GIT_CONFIG_GLOBAL/SYSTEM masked to os.DevNull + identity
    flags, t.Skip without git): clone happy path (SKILL.md bytes, .git dir,
    stdout lines, no temp leftovers), outside-project exit 2,
    already-installed abort with sentinel-file untouched, clone-failure
    cleanup (no target, empty skills dir), arg-validation table ×3 proving
    nothing is created before validation passes. Helpers
    `makeGstackFixtureRepo`/`runGitFixture` — reuse for the remaining
    Phase 4 slices and T30 scenarios.
  - Binary smoke in /tmp passed: install → doctor "gstack (project):
    installed" ok line → second run aborts exit 1. (Doctor's global-gstack
    warn fired on the dev machine's own pre-existing install — D010,
    expected, not a violation.)
- install gstack `--force` LANDED and green (Phase 4 slice 2 ✅, D031):
  runInstall arg loop now accepts `--force` (anything else → "unsupported
  argument %q (only --force is supported)" before any FS work);
  installGstack(agentmodDir, force, …). Read D030+D031 before touching
  installer code.
  - Replace order: clone to the sibling temp dir FIRST (unchanged code
    path), then swap — old install renamed to a `.gstack-old-*` sibling,
    clone renamed in, old RemoveAll'd. Failed clone returns before the
    existing install is touched. Rename-in failure restores the old copy
    (or preserves it and prints its path). Non-force abort message now says
    "re-run with --force to replace it".
  - macOS gotcha (cost one red run): Darwin rename(2) refuses an existing
    dir destination even when EMPTY — MkdirTemp only reserves the
    `.gstack-old-*` name; os.Remove it before renaming onto it.
  - Tests: TestInstallArgValidation grew to ×4 (`--frobnicate` and
    `--force extra` both rejected, nothing created); 3 new funcs —
    ForceReplacesExisting (sentinel gone, fixture SKILL.md in, skills dir
    contains only "gstack"), ForceWithoutExisting (plain install, no
    "Replacing" line), ForceCloneFailureKeepsOld (old install byte-intact,
    no leftovers). AlreadyInstalled now also asserts the --force hint.
    Usage text in cli.go mentions --force.
  - Binary smoke in /tmp passed: install v1 → no-force abort exit 1 →
    --force → SKILL.md is v2, old marker gone, no stray entries.
- install gstack global pollution verification LANDED and green (Phase 4
  slice 3 ✅, D032): `snapshotGlobalSkills`/`diffListings`/
  `verifyGlobalSkillsUnchanged` in install.go. Read D030–D032 before
  touching installer code.
  - Snapshot of `$HOME/.claude/skills` (injected-Env HOME only) is the
    FIRST thing installGstack does; compared after the swap, before the
    success paragraph. Delta → stderr VIOLATION naming added/removed
    entries + manual-removal/bug-report instructions, exit 1, success
    paragraph suppressed, local install left in place. Absent dir = empty
    listing — only a DELTA violates, never existence (D010 safe). HOME
    unset / unreadable dir → stdout "Global skills check: skipped (…)",
    exit 0. New stdout line on every install ("…: <dir> unchanged" when
    clean).
  - Violation path is tested END-TO-END with no production test hook:
    fake HOME's `.claude/skills` is a SYMLINK to the project-local skills
    dir, so the legitimate install appears as a new "global" entry.
    7 new test funcs/tables in install_test.go (diffListings ×7 +
    slicesEqual helper, unchanged-line w/ pre-existing global entries,
    delta violation, ENOTDIR skip, removed-entry direct); existing happy
    path grew the HOME-unset skip-line assertion.
  - Binary smoke in /tmp passed: install against real HOME printed
    "Global skills check: /Users/…/.claude/skills unchanged" (read-only),
    exit 0.
- install gstack distinct error reporting LANDED and green (Phase 4 final
  slice ✅, D032→D033, T18 ✅): read D030–D033 before touching installer code.
  - Clone failure now appends a two-line hint after the forwarded git
    output: check network/source reachability + the
    `AGENTMOD_GSTACK_SOURCE=<url-or-path>` override. Decision (D033): git's
    CombinedOutput IS the diagnosis (it distinguishes DNS vs not-found vs
    auth itself); agentmod does not classify. Only product change this
    iteration — everything else was already distinct.
  - 2 new test funcs in install_test.go: GitMissing (t.Setenv PATH to an
    empty temp dir — install uses the REAL PATH per D030; asserts the
    distinct needs-git message + nothing created) and
    SetupFailureSkillsBlocked (regular FILE at claude/skills → ENOTDIR;
    asserts "not a directory" + path on stderr, exit 1, blocker untouched —
    note the failure fires at the initial Lstat, not MkdirAll; the test
    asserts the user-visible contract, see D033). CloneFailure test grew
    forwarding assertions (`fatal:` + `does not exist` — git's words, never
    ours) + hint-line assertions.
  - Binary smoke in /tmp passed: bogus local source → forwarded fatal line
    + both hint lines, exit 1.
- `.amod` writer + `agentmod handoff create` LANDED and green (Phase 5
  slice 1 ✅, D034, T19 🟡): new `internal/handoff` package (Create +
  Manifest/Inventory/InventoryEntry types, SchemaVersion=1, member-name
  constants) + `internal/cli/handoff.go` (runHandoff dispatch +
  runHandoffCreate), wired into dispatcher + usage. Read D034 before
  touching snapshot code.
  - Zip layout: manifest.json / inventory.json / checksums.txt at root,
    payload under `payload/.agentmod/...` (project-root-relative,
    forward-slash) so later slices add `payload/.claude/...` etc. without
    a format break. REDACTION/HANDOFF/RESTORE members NOT written yet —
    separate TASKS items, T19 stays 🟡 until they land.
  - Payload = all of .agentmod/ EXCEPT snapshots/ (structural: it is the
    output dir) and the output/temp file itself. NO policy exclusions yet:
    a slice-1 snapshot MAY contain auth files — the exclusion engine (next
    task, T20) must land before handoff is described as secret-safe.
  - Inventory: every non-dir payload member (zip name incl. payload/
    prefix, size, sha256 of member content, octal mode, symlink_target),
    sorted; dirs in zip only (empty dirs restore). Symlinks = zip symlink
    entries whose content IS the target; sha256 is of the target string so
    hash-checking is uniform. Irregular files (fifo) → hard error.
    checksums.txt = sha256sum format over manifest/inventory/payload
    members; `shasum -a 256 -c` verified it in the binary smoke.
  - Deterministic (all mtimes = CreatedAt; byte-identical re-create
    tested) and atomic (`.amod-partial-*` temp + rename; failure leaves
    nothing). Existing output → refuse. Output keeps 0600 (deliberate —
    snapshots may carry sessions; D034).
  - `Env` gained `Now func() time.Time` (osEnv = time.Now; fakeEnv = fixed
    fakeNow 2026-06-11T12:30:45Z in status_test.go). Default output
    `.agentmod/snapshots/<base>-<utc-stamp>.amod`; nil Now falls back to
    time.Now. restore/inspect/verify/list subcommands answer "not
    implemented yet" (exit 1).
  - Tests: 11 funcs in internal/handoff/handoff_test.go (fixture tree w/
    exec bit + symlink + empty dir + pre-existing snapshot; member set,
    manifest fields, inventory↔payload match, zip modes, checksums
    coverage, determinism, refuse-existing, missing-.agentmod, unreadable
    leaves-no-partial, fifo refusal incl. mkfifo-shellout helper) + 6 in
    internal/cli/handoff_test.go (default name w/ fixed clock, --output,
    same-clock collision refusal, outside-project exit 2, arg-validation
    table ×8 creating nothing, nil-Now). Binary smoke in /tmp passed:
    init → create → unzip -l layout → shasum -c OK → refuse-overwrite
    exit 1, no partial files.
- default exclusion engine LANDED and green (Phase 5 slice 2 ✅, D035,
  T20 ✅): new `internal/handoff/exclude.go` (`Rule{ID, Reason, Matches}`,
  `ExcludedEntry`, `DefaultRules()`, `snapshotsExclusion`) wired into the
  writeSnapshot walk; `CreateOptions.Rules` (nil = defaults; empty non-nil
  = policy off, pinned escape hatch) + `Result.Excluded` (path + rule +
  reason, walk order, pruned dirs once with trailing "/"). Read D035
  before touching exclusion/redaction code.
  - Rules: auth by NAME at any depth (.credentials.json, auth.json,
    credentials.json, credentials — D028 provenance-independent), *.env +
    .env.*, ssh keys (id_* families + .pub + *.pem), credential dirs
    (.ssh/.aws/.azure/.gcloud/.kube/.gnupg/.docker), *.keychain(-db),
    .git (dir OR worktree file), node_modules dirs, tmp/.tmp dirs,
    .cache dirs + path-anchored routing cache targets (new layout consts
    NodeNPMCacheDir/NodePnpmDir/NodeBunDir; routing.Vars refactored onto
    them, no behavior change). Sessions/logs deliberately stay IN; no
    fuzzy token matching (T21's content scan owns that); no source-code
    rule yet (payload is .agentmod-only — structurally absent).
  - Rule check precedes the member-kind switch: an excluded fifo is
    dropped, not the irregular-file error. Structural snapshots/ skip is
    recorded as `snapshots-output` when the dir exists.
  - CLI prints each excluded path + rule ID under "excluded by default
    policy:" (count line is singular/plural-correct; "nothing" when 0).
  - Tests: 5 funcs in new exclude_test.go (38-case rule table,
    hostile-fixture end-to-end w/ exact Excluded map, prune-once +
    payload count, empty-Rules escape hatch, determinism) + cli
    TestHandoffCreateReportsPolicyExclusions; default-output test grew
    the structural-line assertions. Binary smoke in /tmp passed (auth/
    .env/npm-cache excluded + named on stdout, zip clean, exit 0).
  - Docs-slice note (D035): gstack clone loses .git (reinstall via
    --force); node/bin npm symlinks dangle (lib/node_modules excluded) —
    HANDOFF.md/RESTORE.md must mention both.
- secret-candidate scan + REDACTION.md LANDED and green (Phase 5 slice 3 ✅,
  D036, T21 ✅): new `internal/handoff/scan.go` (ScanFinding, scanPatterns,
  scanContent) + `internal/handoff/redaction.go` (RedactionName,
  renderRedaction), wired into writeSnapshot. Read D034–D036 before
  touching snapshot/scan/redaction code.
  - §12 order pinned: only KEPT files are scanned (private key inside an
    excluded .env → zero findings, tested). Regular-file zip path now
    os.ReadFile (scanned bytes == packed bytes); unreadable-file error
    still names the path.
  - Patterns: private-key (the ONLY hard one), aws-access-key-id,
    github-token, sk-token (20+ chars so sk-FAKE-fixture stays clean),
    api-key/token/secret with required `[:=]` assignment context (prose
    and tokenizer-style names never warn). One finding per (file, pattern),
    first match line only; matched bytes never recorded anywhere.
  - Hard findings refuse creation after the walk with one error listing
    every path/line/pattern + the --allow-findings remedy; temp-file defer
    keeps refusal atomic. `CreateOptions.AllowFindings` packs them, marked
    HARD in REDACTION.md and on stdout. Warn findings never block.
  - REDACTION.md = root zip member between inventory.json and
    checksums.txt coverage; renders Excluded (path — ruleID: reason) +
    findings (path/line/pattern only); explicit sentences for both empty
    states; deterministic (byte-identical re-create re-tested with
    findings present).
  - CLI: `handoff create [--output PATH] [--allow-findings]`; stdout
    gained "secret scan: clean/N candidate finding(s)" + per-finding
    lines; unsupported-arg message names both flags (old assertion still
    matches — substring).
  - Tests: 7 funcs in new scan_test.go + RedactionName in the member-set
    test + 2 new cli funcs + "secret scan: clean" in the default-output
    test. Binary smoke in /tmp passed: refusal exit 1 → --allow-findings
    exit 0 with both findings printed → REDACTION.md correct → shasum -c
    all OK (now incl. REDACTION.md).
- HANDOFF.md + RESTORE.md docs members LANDED and green (Phase 5 slice 4 ✅,
  D037, T19 ✅): new `internal/handoff/docs.go` (HandoffDocName/
  RestoreDocName, renderHandoffDoc/renderRestoreDoc, countNoun pluralizer),
  wired into writeSnapshot. Read D034–D037 before touching snapshot/docs
  code.
  - Both are root zip members; member + checksums.txt order is manifest,
    inventory, REDACTION.md, HANDOFF.md, RESTORE.md, payload (checksums.txt
    written last). Renderers are pure functions over data already in scope
    (createdAt/version/platform, Base(Clean(ProjectRoot)) as project name,
    populated *Result) — determinism test auto-covers them.
  - HANDOFF.md: identity ¶, payload size, restore pointer + honest
    "creating build does not implement restore yet" note, "What is missing"
    (exclusion count or explicit nothing-sentence, scan clean/N summary,
    auth-never-travels, D035's gstack-.git and node/bin-dangling notes).
  - RESTORE.md: 4 steps (install → init → handoff restore w/ safety
    properties named → doctor), re-login section, macOS Keychain note,
    reinstall section (gstack --force, npm globals).
  - Canonical re-login remedies MOVED to internal/handoff (exported
    ClaudeReloginRemedy/CodexReloginRemedy); doctor.go's unexported consts
    are aliases now, so doctor/init/auth wording can never drift from the
    RESTORE.md packed into snapshots. cli imports handoff — no cycle. No
    cli test changed (strings identical).
  - Tests: 3 funcs in new docs_test.go (two end-to-end content-anchor
    tests incl. verbatim-remedy comparison, one renderer unit test pinning
    empty/singular/plural "What is missing" states); member-set test grew
    the two names; checksums + determinism tests auto-extended. Binary
    smoke in /tmp passed: init → create → unzip -p both docs correct →
    `shasum -a 256 -c` all OK incl. both new members.
- git state metadata + --allow-dirty gate LANDED and green (Phase 5
  slice 5 ✅, D039, T22 ✅): new `internal/cli/gitstate.go`
  (collectGitState/gitOutput/summarizeStatus/redactRemoteURL/gitIdentity);
  Manifest gained `Git *GitState` (omitempty — nil ⇒ key ABSENT, D034
  tolerate-absence; GitState carries branch/head/dirty/status_summary/
  remote_url/source_included). Run 2's orphaned working-tree struct edit
  was finished by this iteration. Read D030+D034+D039 before touching
  git-metadata code.
  - Split: internal/handoff stays exec-free (CreateOptions.Git is plain
    data); the cli EXECUTES git (D030 exception, real LookPath + real env
    + GIT_TERMINAL_PROMPT=0 + GIT_OPTIONAL_LOCKS=0 so status never
    refreshes the index). Binary absent / not a repo → `git: metadata
    omitted (<note>)` on stdout + no manifest key, never a failure.
  - Dirty (untracked counts; `-c status.showUntrackedFiles=normal` defeats
    user display config) + no `--allow-dirty` → stderr refusal naming the
    summary + flag, exit 1, NOTHING written (gate sits before Create).
    With the flag stdout marks `DIRTY (…) — packed anyway (--allow-dirty)`.
  - Redaction: scheme:// remotes lose the ENTIRE userinfo (user:token,
    token-only, and ssh:// git@ alike — manifest documents WHERE, not a
    dialable URL); scp-like git@host:path unchanged; port preserved.
    source_included is always false in MVP but explicitly spelled out.
  - Tests: 7 funcs in new gitstate_test.go (tables for redact ×8 +
    porcelain ×7; PATH-masked git-missing; non-repo; clean+remote;
    dirty+detached; unborn branch) + 3 in handoff_test.go (omitted note,
    refuse-then-allow, clean manifest) + readSnapshotManifest helper +
    TestCreateManifestGitState (internal/handoff). cli usage text grew
    --allow-dirty. Binary smoke in /tmp passed (all four shapes incl.
    manifest JSON via unzip -p).
- handoff inspect/verify/list + pack/unpack aliases LANDED and green
  (Phase 5 slice 6 ✅, D040, T23 ✅): new `internal/handoff/read.go`
  (`Open` → `Snapshot{Manifest, Inventory, Redaction, Members,
  PayloadDirs}` + `(*Snapshot).Verify() *VerifyResult`); cli grew
  runHandoffInspect/Verify/List, the shared `listSnapshotFiles` lister
  (status.recentHandoff refactored onto it — same pick on mtime ties),
  and top-level `pack` (≡ handoff create, same flags) + `unpack`
  (explicit stub until Phase 6). Read D040 (+D034) before touching
  read/restore code — restore MUST build on Open/Verify.
  - Open: structural gate only (zip + six §21 root members + parseable
    manifest/inventory), NO hashing, NO schema-version gate (caller
    decides: inspect warns, Verify records a problem, restore will
    refuse). Verify: re-hash all content members vs checksums.txt +
    inventory↔payload cross-check both directions (presence/size/sha/
    mode/symlink-target-hash); read failures are problems, never aborts.
  - Exit codes: verify problems OR invalid snapshot → 3 (ExitValidation);
    unstat-able path → 1 (typo ≠ validation verdict); inspect informs
    (0/1, prints REDACTION.md verbatim — it IS the redaction summary).
    inspect/verify need NO project; list requires one (exit 2).
  - Tests: 9 funcs in new internal/handoff/read_test.go (incl. the
    rewriteSnapshot mutate/drop/add/fix-checksums tamper helper — REUSE
    for T24 malicious restore fixtures) + 12 new cli funcs + reworked
    arg-validation rows ("not implemented" rows now only restore).
    Binary smoke in /tmp passed: create→list→inspect (git line + redacted
    remote)→verify exit 0→pack→garbage verify exit 3→unpack stub exit 1.
  - Smoke gotcha: shell redirects INTO the repo (`> create.out`) make the
    worktree dirty and trip the D039 gate — redirect smoke output outside
    the project.
- restore validation layer LANDED and green (Phase 6 slice 1 ✅, D041,
  T24 🟡): new `internal/handoff/validate.go` — `(*Snapshot).PlanRestore()
  (*RestorePlan, []string)` returns ALL path-safety problems in one pass
  or the extraction plan (`PlanEntry{ZipName, RelPath, Mode, Target}` in
  Dirs/Files/Links, each sorted by RelPath). Read D034+D040+D041 before
  touching restore code; the extraction slice EXECUTES this plan.
  - Validation-only slice (decided in D041): `handoff restore`/`unpack`
    stay not-implemented stubs until extraction lands. Restore pipeline
    will be Open → Verify → PlanRestore, refusing on any problem from
    either; PlanRestore never hashes (Verify's job) but DOES gate schema
    version (read.go's new shared `schemaProblem()` — Verify refactored
    onto it, wording unchanged, no test churn).
  - Refusals: non-root-member outside payload/ (smuggled `../evil`),
    backslash/absolute/`C:`-drive names, non-canonical paths (Clean
    fixpoint), `..` escapes named "zip-slip", first element must be
    `.agentmod` (schema-v1 whitelist), §21 protected elements
    (.git/.ssh/.aws/.docker) anywhere below it, duplicate payload paths,
    irregular member types, symlink targets that are empty/absolute/
    backslashed/>4096 bytes/lexically escaping `.agentmod/`. Setuid/
    setgid/sticky are STRIPPED (Mode().Perm()), not refused. Links must
    be extracted LAST (field doc pins Dirs → Files → Links).
  - Tests: 11 funcs in new validate_test.go — hostile symlink targets
    made by MUTATING the fixture link's content via rewriteSnapshot w/
    fixChecksums (content IS the target, D034); new `addZipMember`
    helper appends one member with an explicit mode (fifo, setuid,
    extra symlinks) which rewriteSnapshot's fixed-mode extras cannot;
    `wantNoPlan` asserts nil-plan + named problem. Clean fixture pins
    the exact Dirs/Files/Links sets incl. modes + link target.
- pre-restore backup LANDED and green (Phase 6 slice 2 ✅, D042, T25 🟡):
  new `internal/handoff/backup.go` — `BackupAgentmod(projectRoot, now)
  (string, error)` renames `.agentmod` to `.agentmod.backup-<utc-stamp>`
  (stamp format = default snapshot stamp, always UTC; `now` injected per
  D034). Read D034+D040+D041+D042 before touching restore code; pipeline
  pinned Open → Verify → PlanRestore → BackupAgentmod → extract.
  - Rename, not copy (D042): atomic, contents never read, rollback =
    rename back. Absent `.agentmod` → ("", nil) no-op; occupied backup
    name → refusal, nothing moved; stray regular FILE at `.agentmod`
    backed up as-is. Exported `BackupPrefix` for the restore cli.
  - Gitignore decision SETTLED in D042: extraction slice must add
    `.agentmod.backup-*/` via ensureGitignore (generalized to take an
    entry) when a backup was actually created — untracked backups trip
    the D039 dirty gate on the next create. init stays untouched.
  - 5 test funcs in backup_test.go (tree-intact + rename-back rollback,
    no-op, collision refusal w/ source+occupant untouched, stray-file,
    UTC stamp under KST clock). Library-only slice: restore/unpack stubs
    unchanged, no cli surface change, no binary smoke needed.
- restore extraction LANDED and green (Phase 6 slice 3 ✅, D043, T24 ✅
  T25 ✅ T26 🟡): `internal/handoff/restore.go` (`(*Snapshot).Restore` +
  extractPlan/writeFileMember — the library half was a prior iteration's
  orphaned uncommitted work, verified sound and adopted per the T06
  precedent) + `runHandoffRestore` in cli/handoff.go, wired into the
  handoff dispatcher + usage. Read D034+D040–D043 before touching restore
  code.
  - cli pipeline: project required (exit 2, RESTORE.md step order) →
    Stat (typo → exit 1) → Open/Verify/PlanRestore (any problem → exit 3,
    all listed) — all BEFORE any disk move (refused restores provably
    create no backup, cli-tested) → Restore = BackupAgentmod + extract
    Dirs(0700, recorded modes chmodded deepest-first after content) →
    Files(O_CREATE|O_EXCL + explicit Chmod, umask-proof exec bits) →
    Links last. Extraction failure → automatic rollback (RemoveAll
    partial + rename backup back), error says "rolled back"; rollback
    failure names BOTH paths. layout.Subdirs() recreated after extract
    (snapshots/ never travels) — doctor reports a complete tree.
  - ensureGitignore GENERALIZED to (dir, entry) (covers-check derives the
    4 spellings from the entry; init passes gitignoreEntry, zero behavior
    change). `gitignoreBackupEntry` = `.agentmod.backup-*/`, ensured only
    when a backup was created; gitignore failure after successful restore
    = stderr warning, exit stays 0. unpack deliberately STILL a stub
    (TASKS assigns the alias to the notices slice; its message stays
    true). docs.go honesty: "does not implement restore yet" notes
    REMOVED from both renderers (+ docs_test anchors updated); create's
    closing line now points at verify/restore.
  - Tests: 7 funcs in new internal/handoff/restore_test.go
    (digestTree/diffDigests/backupEntries/pipelineForRestore helpers —
    reuse for T30): fresh-root round trip (modes 0600/0700/0755, symlink
    + resolution, empty dirs, snapshots/ recreated, target root gains
    ONLY .agentmod), backup-of-existing, nil-plan refusal, ghost-member
    rollback to byte-identical tree, fresh-root failure leaves nothing,
    duplicate-entry O_EXCL no-overwrite rollback, snapshot-inside-own-
    snapshots/ (zip fd survives the backup rename). 8 new cli funcs:
    round trip w/ deterministic backup name + .gitignore "added", skip
    outside git repo, same-clock second restore refused, outside-project
    exit 2, missing file exit 1, garbage + tampered exit 3 with
    assertNoBackupOrLoss, arg table reworked (restore rows ×2). Binary
    smoke in /tmp passed: A→B restore (marker traveled, backup held old
    tree, .gitignore created with pattern, git status clean of backup) →
    doctor "all 6 directories present" → garbage refusal exit 3 no
    backup → unpack stub exit 1.
- restore portability pass LANDED and green (Phase 6 slice 4 ✅, D044,
  T27 ✅): new `internal/cli/portability.go` (`reportPortability` +
  `scanRestoredConfigs`/`classifyAbsoluteToken`/`collectStrings`/
  `localAgentmodEquivalent`/`isWindowsAbsPath`/`restoredConfigRelPaths`),
  called by runHandoffRestore after the gitignore step. Read D044 (+D029/
  D041/D043) before touching portability code.
  - Separators + exec bits needed NO new code (D041 refusals + FromSlash
    + D043 umask-proof chmod) — T27's matrix row records the split.
  - The ONE rewrite: ensureClaudeGuardHook re-runs after every successful
    restore, repairing the guard hook command from the source machine's
    binary to this machine's (or writing settings.json fresh when the
    snapshot had none). Wiring failure = stderr warning, exit stays 0.
  - Everything else warn-only: known config files (claude/settings.json,
    claude/.claude.json, codex/config.toml, opencode/opencode.json) are
    string-walked (no MCP schema assumptions, §31); whitespace-tokenized,
    quote-trimmed tokens classify as Windows-spelling / foreign-.agentmod
    (warning names the local equivalent) / inside-this-.agentmod-missing /
    nonexistent-here; paths that resolve locally and relative paths stay
    silent; guardHookMarker strings exempt; warnings deduped + sorted;
    user-owned files NEVER re-marshaled (D044 records why, incl. that the
    manifest deliberately lacks the source project root).
  - cli importing BurntSushi/toml directly is new (still the only module
    dependency, D004 intact); collectStrings needs a []map[string]any
    case because BurntSushi decodes arrays-of-tables that way.
  - Tests: 4 funcs in new portability_test.go (classify table ×13,
    local-equivalent unit, four-file scanner end-to-end w/ exact warning
    set + dedup + exemption + sorted order + unparseable-file fallback,
    absent-files-clean) + cli TestHandoffRestoreRewritesGuardAndWarnsForeignPaths
    (settings.json rewritten before/after, codex/config.toml byte-intact,
    warn line, exit 0); round-trip test grew the fresh-wire + clean-
    portability line assertions. Binary smoke in /tmp passed (foreign
    settings.json rewritten, 2 codex warnings w/ local-equivalent hint,
    doctor "wired with the current binary"; doctor exit 3 in the smoke
    subshell is the documented in-project-vars-unset warning, not a bug).
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
(2026-06-11: same pattern again — `~/.codex` mtime now 6월 11 01:11, the
other two homes and the skills list unchanged; no agentmod artifacts.
Later same day: `~/.codex` mtime 6월 11 02:28, same verdict. And again
16:44, then 19:15 — skills list + other two homes still match baseline.)

## Failing tests
None. All checks green as of this iteration's end.

## Exact next step
Phase 6, fifth item: "post-restore doctor + re-login notices; unpack
alias (+ tests)" — flips T26 ✅ and completes Phase 6's TASKS list
(the new doctor-MCP-finding follow-up item comes after). Notes:
- Print the re-login remedies after restore
  (handoff.ClaudeReloginRemedy/CodexReloginRemedy — already exported
  from internal/handoff, identical strings to RESTORE.md/doctor), plus
  the macOS Keychain note where env.GOOS == "darwin" (D025 pattern).
- Run or point at doctor (FABLE_PLAN §18 "Run doctor after restore" —
  decide run-inline vs print-the-command and record it; runDoctor takes
  (stdout, stderr, env) so inline is feasible, but mind its exit-3
  in-project-vars-unset warning firing in fresh shells, see D044 smoke
  note).
- Wire `unpack` as a TRUE alias of `handoff restore` (cli.go top-level
  case currently prints the stub message; mirror the `pack` alias).
- Mention deleting the backup once verified (D042/D043) and the D043
  known wrinkle (init-before-`git init` → .gitignore holds only the
  backup pattern; re-run init).
- Insertion point: runHandoffRestore between reportPortability and the
  current closing line (which the notices likely replace/extend).

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
