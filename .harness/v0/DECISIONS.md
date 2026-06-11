# DECISIONS — significant design decisions

Append-only log. Each entry: context → decision → why. Inherited decisions
from FABLE_PLAN (§0, §12, §15, §17) are restated only where implementation
adds specifics.

## D001 — 2026-06-10 — Harness location & supersession
Harness lives at `.harness/v0/` per repo convention. FABLE_PLAN.md supersedes
GPT_PLAN.md; GPT_PLAN.md kept read-only for history.

## D002 — 2026-06-10 — loop.sh invocation & permissions
`claude -p "$(cat PROMPT.md)" --dangerously-skip-permissions` (claude
2.1.170 verified to support `-p`/`--print` and the flag). Rationale: an
unattended loop cannot answer permission prompts; safety comes from the
PreToolUse guard (`hooks/guard.sh`) wired in `.claude/settings.json`, which
blocks sudo/HOME/global-home writes regardless of permission mode.
Overridable via `AGENTMOD_LOOP_PERM_ARGS`. loop.sh additionally verifies
`go test ./...` before honoring a DONE sentinel and rewrites bogus DONE
claims to REJECTED (FABLE_PLAN §8 "prevent completion declarations" made
procedural, since a content-inspecting hook is brittle).

## D003 — 2026-06-10 — Harness guard implementation
Bash + jq (jq present at /opt/homebrew/bin/jq). Matches Bash, Write, Edit,
NotebookEdit. Fail-safe per §8: unparseable input denies only when raw text
references a global agent/credential path. 16-case smoke test passed
2026-06-10. The PRODUCT guard (`agentmod guard claude-bash`) will be Go, not
this script.

## D004 — 2026-06-10 — Go module & dependencies
Module `github.com/agentmod/agentmod` (rename trivially later if an org is
chosen). Dependencies: `github.com/BurntSushi/toml` only (TOML has no stdlib
codec). CLI is stdlib `flag` + a small hand-rolled subcommand dispatcher —
cobra adds deps and lock-in the project doesn't need. Zip via `archive/zip`,
checksums via `crypto/sha256`.

## D005 — 2026-06-10 — Skills
mattpocock/skills: already installed at `.claude/skills/` (verified via
skills-lock.json hashes) — not reinstalled. andrej-karpathy-skills: installed
project-locally only; selection rationale in `.harness/v0/skills/README.md`.
No global installation; guard blocks it anyway.

## D006 — 2026-06-10 — Env save/restore semantics (for Phase 2)
On activation the hook saves any pre-existing values of routed vars
(e.g. user's own global CLAUDE_CONFIG_DIR) into `AGENTMOD_SAVED_<VAR>` and
restores them on deactivation, rather than blind-unsetting. Blind unset would
break users who legitimately route their global config. AGENTMOD_* own vars
are simply unset. Verify exact mechanics in shell tests.

## D007 — 2026-06-10 — Hook performance strategy (for Phase 2)
The printed shell hook does the upward search for `.agentmod/agentmod.toml`
in pure shell on every prompt (cheap), and execs `agentmod env --shell <sh>`
only on activation/deactivation transitions. Keeps per-prompt overhead ~0 and
degrades gracefully if the binary disappears (hook no-ops with a one-time
warning).

## D008 — 2026-06-10 — Versions observed on this machine
go 1.26.2 darwin/arm64 · claude 2.1.170 · codex-cli 0.137.0 · opencode 1.4.3.
Runtime sanity checks in doctor should cite these as the verified baseline.

## D009 — 2026-06-10 — BurntSushi/toml deferred until internal/config
go.mod ships with zero requires for now: an unused `require` would be
stripped by `go mod tidy` and adds nothing. The dependency lands together
with `internal/config` (the first package that decodes TOML). D004 stands —
it remains the only planned dependency.

## D010 — 2026-06-10 — CHECKS.md §2 gstack grep: pre-existing install is baseline
`ls ~/.claude/skills | grep -i gstack` matches the developing user's own
pre-existing global gstack install (gstack, gstack-upgrade,
open-gstack-browser; global-home mtimes 2026-06-06/-05 and 2026-04-30 all
predate this repo). That is NOT pollution from this project. The check
compares against the baseline recorded in STATE.md; only gstack entries that
APPEAR after baseline are violations.

## D011 — 2026-06-10 — Project discovery is lexical; marker must be a regular file
`internal/project.Discover` makes startDir absolute WITHOUT resolving
symlinks (filepath.Abs, no EvalSymlinks): activation follows the path the
user is standing in, matching how the shell hook will see $PWD; resolving
symlinks could activate a project the user never visibly entered. The marker
is valid only when `.agentmod/agentmod.toml` is a regular file — a bare
`.agentmod/` directory or a directory named `agentmod.toml` does not
activate. Stat errors on ancestors (e.g. permissions) are treated as
"no project here" and the walk continues, rather than failing discovery.

## D012 — 2026-06-10 — Config load/validation semantics (internal/config)
Loading overlays the TOML document onto `Default()` (BurntSushi toml leaves
absent keys untouched), so a partial file keeps every §13 default — including
an absent `schema_version`, which is treated as current. Unknown keys are
REJECTED via `MetaData.Undecoded()`: within a schema version they can only be
typos, and a misspelled policy key silently reverting to a default is worse
than an error (cross-version compat is handled by schema_version itself).
Validation hard-rejects: `schema_version != 1`, `mode != "standard"`,
`isolation.change_home = true`, and `handoff.git.include_sessions = true`
(MVP has no encryption; error explains this, per IMPLEMENTATION_PLAN §6).
`snapshot.exclude_source`/`exclude_secrets` are *defaults*, not validated
hard-true: the Phase 5 exclusion engine enforces protected (secret/auth)
entries as never-removable regardless of config — documented on the struct.
Sentinels: `ErrSchemaVersion`, `ErrChangeHome`, `ErrSessionsNeedEncryption`.

## D013 — 2026-06-10 — `init` always targets cwd; nested init warns, then proceeds
`agentmod init` creates the project at the current directory, never
redirecting to an enclosing project (FABLE_PLAN §12 says "create .agentmod/",
and discovery is nearest-wins, so nesting is a supported concept per D011).
When cwd is strictly inside an existing project, init prints a notice that
the new project will shadow the outer one — running init in a subdirectory
by accident is likely, but refusing would block legitimate nesting and §12
defines no --force escape hatch. Re-init at an existing root is a quiet
no-op-plus-fill: missing layout dirs are created, existing files (config,
opencode.json, anything user-placed) are NEVER touched — enforced by
O_CREATE|O_EXCL writes (`writeIfAbsent`), not stat-then-write races.
`.agentmod` existing as a regular FILE is an error asking the user to move
it aside; init never deletes. The opencode.json stub is `{"$schema":
"https://opencode.ai/config.json"}` — an empty merge-chain layer (§3.3).
Layout names live in `internal/layout` (shared by status/init/future
routing); `layout.Subdirs()` excludes `opencode/xdg`, which only the opt-in
XDG full-isolation mode creates.

## D014 — 2026-06-10 — init .gitignore semantics (T07)
`ensureGitignore` (internal/cli/gitignore.go) appends `.agentmod/` to
`<cwd>/.gitignore`; user content is byte-preserved (a `\n` is prepended only
when the file lacks a trailing newline). **Dedup** accepts a trimmed line
equal to `.agentmod/`, `.agentmod`, `/.agentmod`, or `/.agentmod/` — all
ignore the directory from a root .gitignore; trimming is faithful because
git itself strips unescaped trailing whitespace. Commented (`# .agentmod/`)
and negated (`!.agentmod/`) lines do not count. **No-git-repo grace**
(FABLE_PLAN §12): when `.gitignore` is missing AND the directory is not in a
git repo, init skips with "skipped (not a git repository; re-run init after
'git init')" — creating a stray file in a non-repo would surprise; re-init
fills it later since re-init only fills gaps. But an EXISTING `.gitignore`
is extended even outside a repo: it signals git intent and protects a future
`git init` from committing `.agentmod/` (which can hold consent-copied
auth). **Repo detection** is a lexical upward walk for a `.git` entry of any
file type (dir = normal repo, file = worktree/submodule), never exec'ing
git — consistent with D011 discovery. `.gitignore` existing as a directory
is a hard error, like `.agentmod`-as-a-file in D013.

## D015 — 2026-06-10 — loop.sh rate-limit backoff (harness, run 2)
Run 1 hit the Claude session usage limit after iteration 7; iterations 8–25
each failed in ~1s ("You've hit your session limit · resets 7:50pm") and
burned the whole iteration budget in seconds, ending the run with exit 1.
Fix (commit 2cb5ed3): loop.sh now detects a rate-limited attempt (nonzero
exit + log < 2000 bytes + limit-message grep), sleeps 15 minutes, and retries
WITHOUT consuming an iteration, bounded by
AGENTMOD_LOOP_MAX_RATELIMIT_SLEEPS (default 48 ≈ 12h) — still never
unbounded. Run 1's garbage logs archived to reports/run1-ratelimited/.
Run 2 launched with AGENTMOD_LOOP_MAX_ITERS=60: ~36 tasks remained and run 1
averaged exactly one task per productive iteration; the 25 default was sized
before the task count was known.

## D016 — 2026-06-10 — `agentmod env` transition contract (T09a)
`agentmod env --shell <zsh|bash> (--activate ROOT | --deactivate)` prints
ONLY eval-able `export NAME='value'` / `unset NAME` lines (identical for both
shells; --shell exists for future fish/pwsh divergence). All logic lives in
Go, computed against the calling shell's real environment — the binary models
its own emitted mutations in memory (`envModel`), so emitted shell never
loops, branches, or re-derives state. Key semantics:
- **AGENTMOD_VARS** records the routed var names at activation; deactivation
  restores exactly that list, so it survives config edits/deletion while
  inside the project. Names from the (attacker-influencable) environment are
  validated as identifiers before being interpolated into shell code.
- **Save/restore (D006)**: pre-existing values saved to `AGENTMOD_SAVED_<VAR>`;
  absence of a SAVED var means "was unset" → deactivate unsets. Proven a
  perfect inverse by a round-trip test.
- **PATH** is strip/prepend, never save/restore: restoring a snapshot would
  clobber PATH edits the user made while inside. Single managed entry:
  `.agentmod/node/bin` (IMPLEMENTATION_PLAN §7). `NPM_CONFIG_PREFIX` is
  `.agentmod/node` so npm's global bin IS that entry; pnpm/bun global bins
  (`node/pnpm`, `node/bun/bin`) are NOT on PATH in MVP — README limitation.
- **Activate while active** (same or different root) = implicit deactivate
  first, computed in-memory, so switches never leak and saves never capture
  our own routing.
- **Failures keep stdout empty** (it gets eval'd): bad root → exit 2,
  broken config/flags → exit 1, errors on stderr only.
- env has NO filesystem side effects (XDG dirs under opencode/xdg are not
  created by env; that belongs to init/doctor when opt-in mode is on).
- Real-shell eval smoke (bash+zsh, quote-bearing values) verified manually
  this iteration; the scripted-shell integration suite remains its own task.

## D017 — 2026-06-10 — zsh hook contract + test-binary impersonation (T09)
`agentmod hook zsh` prints a self-contained script (internal/shellhook); rc
editing stays T08. Beyond D007/D016, the script decides:
- **Failed-root cache**: a root whose `env --activate` failed (broken config)
  is remembered in `_AGENTMOD_FAILED_ROOT` (typeset -g, NOT exported) and not
  retried while standing in it — one stderr error, not per-prompt spam.
  Leaving the project clears the cache, so re-entering retries (the fix-it
  path). After a failed switch A→broken-B the hook explicitly deactivates A:
  routing is only ever active for the project whose config actually loaded.
- **Missing binary**: warn once per shell (`_AGENTMOD_MISSING_WARNED`), then
  silently no-op; binary lookup happens only on transitions (whence -p).
- Functions use `emulate -L zsh`; registration appends to
  precmd_functions/chpwd_functions with an `(I)` dedup guard, so double-eval
  registers once. `print -r --`, quoted comparisons throughout.
- **Tests run a real zsh** (`zsh -f`, skipped if zsh absent) with the TEST
  BINARY impersonating agentmod: TestMain dispatches to cli.Run when
  AGENTMOD_TEST_RUN_MAIN=1, and a `#!/bin/sh` wrapper named `agentmod` on the
  child's PATH sets that var — no go-build at test time, no real install.
  Child env is fully explicit (throwaway HOME in the CHILD only). precmd is
  exercised via `zsh -f -i` with piped stdin (precmd fires before each
  prompt). Gotcha: zsh resolves its STARTING directory physically
  (/var→/private/var on macOS), so the precmd test compares against
  filepath.EvalSymlinks; `cd` keeps the logical path, matching D011.

## D018 — 2026-06-10 — bash hook contract (T10)
`agentmod hook bash` mirrors the zsh contract (D017) with bash-3.2-clean
shell (macOS /bin/bash): no associative arrays, no ${var,,}, `local` only
inside functions, `command -v` for binary lookup, `${dir%/*}` with an
empty→"/" guard for the upward walk.
- **Registration**: PROMPT_COMMAND alone (bash has no chpwd); it runs before
  every prompt, covering both cd and new-shell-inside-project. Treated as a
  SCALAR (bash 5.1 array PROMPT_COMMAND not supported in MVP). Dedup via
  `case ";${PROMPT_COMMAND-};" in *";_agentmod_hook;"*)`; appended as
  `;_agentmod_hook`, preserving any user entry. Double-eval appends once.
- **Consequence (document in README/doctor)**: in non-interactive bash
  scripts the hook never fires — same as direnv. Tests therefore call
  `_agentmod_hook` explicitly after cd in non-interactive scenarios and
  prove the PROMPT_COMMAND path with one forced-interactive (`-i`) run.
- **Tests prefer /bin/bash** when it exists so macOS CI exercises real 3.2,
  not a newer Homebrew bash earlier on PATH; forced-interactive bash without
  a tty prints prompts + a job-control notice on stderr, so that test
  asserts stdout only. bash resolves its starting dir physically when PWD is
  not inherited (same EvalSymlinks gotcha as zsh).

## D019 — 2026-06-10 — rc fenced-block editor (T08)
`agentmod init` installs the shell hook by editing the user's rc file with a
fenced block (FABLE_PLAN §12); code in `internal/cli/rcfile.go`.
- **Block content** (per shell): `# >>> agentmod >>>`, a managed-by comment,
  `command -v agentmod >/dev/null 2>&1 && eval "$(agentmod hook <shell>)"`,
  `# <<< agentmod <<<`. The `command -v` guard keeps shells quiet if the
  binary is later uninstalled or off PATH (rc files outlive binaries).
- **rc file choice**: zsh → `${ZDOTDIR:-$HOME}/.zshrc` (created if missing).
  bash → existing `~/.bashrc`, else existing `~/.bash_profile`, else create
  `~/.bashrc` (macOS login shells read .bash_profile; prefer whichever file
  the user actually maintains, never create a second one).
- **Shell detection**: basename of `$SHELL` via Env.LookupEnv. Unsupported
  shell, or SHELL/HOME unset → "skipped (…)" on the Shell hook line, exit 0
  — init never fails over exotic shells; it points at `agentmod hook`.
- **Editing semantics**: a marker is any line whose TrimSpace equals it.
  Absent → append (newline glue when the file lacks a trailing \n; create
  0644). Present and identical → zero writes. Present and stale → replace
  the fence in place; bytes outside the fence are byte-preserved (tested).
  Corrupt fence (begin without end, or >1 begin) → hard error naming the
  file, nothing written — guessing risks eating user config.
- **Test injection**: rc paths derive ONLY from Env (HOME/ZDOTDIR/SHELL), so
  tests run on throwaway homes and the dev guard never fires.
  `ensureShellHook` checks --no-shell-hook before computing any path
  (that's T06's enforcement row, now tested for real).

## D020 — 2026-06-11 — init first-session diagnosis (Phase 2 final item)
After the "Shell hook:" line, init prints `hookActivationNotice` (rcfile.go):
plain string logic on (rc outcome × AGENTMOD_* env), per FABLE_PLAN §12
"diagnose whether the hook is active; say so precisely".
- **Live detection**: AGENTMOD_ACTIVE=1 via the injected Env (same signal
  status.go reads). ROOT == cwd → "already routing this project". ROOT !=
  cwd → "live (routing <root>); switches to this project at your next
  prompt" — true because the hook fires per prompt and init's cwd is now
  the nearest root. Live messages print even under --no-shell-hook.
- **Not live + block present**: the first-session caveat (a process cannot
  modify its parent shell) with three remedies: new terminal, `exec $SHELL`,
  one-shot `eval "$(agentmod hook <shell>)"`. For rcUpdated/rcUnchanged
  (block predates this shell) add the hedge that an already-loaded hook
  picks the project up at the next prompt instead; for rcInstalled the
  block is brand new, so no hedge — we KNOW the hook isn't loaded.
- **Not live + skipped** (--no-shell-hook / exotic shell / no SHELL/HOME):
  no notice — the skip reason on the Shell hook line already says what to
  do; CI runs stay quiet.
- ensureShellHook now returns `shellHookResult{Line, Action, Shell}`
  (Action adds rcSkipped); the notice is table-tested in rcfile_test.go
  (6 cases) with fakeEnv only — no real shells.

## D021 — 2026-06-11 — doctor structure + exit semantics (Phase 3 slice 1)
`agentmod doctor` lives in `internal/cli/doctor.go` (NOT a separate
internal/doctor package yet — same precedent as init vs the planned
internal/initcmd; extract only if a non-CLI consumer appears). Structure:
a flat `[]finding{level, label, detail}` list; levels ok/warn/error; output
`%5s  Label: detail` lines + a summary line. Strictly READ-ONLY.
- **Exit codes**: 0 all-ok · 3 (ExitValidation) when ANY warn/error finding
  · 1 only for doctor's own plumbing (args, Getwd). Broken config is a
  FINDING (error level, stdout, keep checking) — unlike status, which
  exits 1 on stderr; doctor's job is to keep diagnosing past breakage.
- **Severity policy**: warn = degraded but recoverable (missing layout dirs,
  stale/missing hook, env drift); error = agentmod cannot work around it
  (corrupt fence, non-dir layout entry, unloadable config). Out-of-project
  context downgrades not-installed/skip findings to ok ("fresh machine must
  exit 0"); the SAME conditions inside a project warn.
- **Reuse, not forks**: rc inspection via new read-only
  `inspectRCBlock`/`locateRCBlock`/`rcFenceError` (ensureRCBlock rewritten
  on the same primitives — write path behavior unchanged); env
  classification via new `routingEnvState(env)` shared with
  status.shellRoutingState; shell/rc-path detection + skip wording reuse
  shellHookTarget verbatim; expected-var values come from routing.Vars, so
  doctor and `agentmod env` can never disagree.
- **Routing check depth**: when active for this root with a loadable
  config, every routing.Vars value is compared (unset or different ⇒ warn
  listing var names). PATH presence/dups deliberately EXCLUDED — that is
  the next doctor task. Outside a project the routing check is skipped
  entirely (lingering-vars warning belongs to that same next task; do not
  print a misleading "ok" meanwhile).

## D022 — 2026-06-11 — doctor slice 2: lingering / PATH / HOME / shims
Four §23 warnings added to doctor (read with D021; exit semantics unchanged):
- **Lingering (outside a project)** fills the slot D021 left skipped, under
  the same "Routing env" label: warn if any bookkeeping var is set, any
  `AGENTMOD_SAVED_<routed>` exists, any routed-name var's VALUE contains an
  `.agentmod` path element, or any PATH entry does. A routed-name var
  pointing elsewhere is the user's own setting — silence. The probe list is
  `routing.RoutedNames()` (new): Vars() with every agent + XDG enabled, so
  the superset can never drift from the emitter. Remedy: new terminal or
  `eval "$(agentmod env --shell <shell> --deactivate)"` (deactivate works
  off env bookkeeping alone, D016, so it is valid outside a project);
  shell part omitted when $SHELL is undetectable.
- **PATH (inside a project)**: exact-match count of this project's
  NodeBinDir — >1 warns (dups); 0 while active-for-this-root + cfgOK +
  node enabled warns (missing); any OTHER entry containing an `.agentmod`
  element warns (foreign project leak). Exactly-1-while-NOT-active is
  deliberately ok: routingFinding already warns about the inactive state,
  same root cause, one remedy (avoids double-counting).
- **HOME (always)**: agentmod never saves/sets HOME, so warn iff
  AGENTMOD_SAVED_HOME exists or HOME contains an `.agentmod` element;
  HOME-unset is ok-level (not our doing; shell-hook skip already covers it).
- **Shims (inside a project)**: scan ONLY node/bin (the one PATH dir we
  manage) for entries named claude/codex/opencode. Legit = symlink
  resolving inside .agentmod (npm bin layout for a project-local install →
  ok, named in the detail); everything else (script/regular file/dangling
  or escaping symlink) warns. EvalSymlinks both sides (macOS /var →
  /private/var). Which-style full-PATH scanning rejected as not cheap and
  full of false positives.
- `.agentmod`-element matching (`hasAgentmodElement`) is the shared
  "points into some agentmod root, whosever it is" test for HOME, lingering
  values, and PATH entries.

## D023 — 2026-06-11 — doctor slice 3: agent binaries + home/auth state
Three §23 subjects added (read with D021/D022; exit semantics unchanged):
- **Auth absence is ok-level, not a warn.** §23's must-warn list does not
  include auth; a fresh project legitimately has no auth yet, and warning
  would make every `agentmod init && agentmod doctor` exit 3 forever for
  agents the user never runs. The ok-detail carries §12's exact re-login
  instruction instead ("run 'codex login' inside this project"; for Claude,
  "running 'claude' inside this project" — hedged with "may ask" because on
  macOS Keychain auth makes the file legitimately absent; the explicit
  Keychain note is the next-next doctor task). Auth file names per §12/§15:
  `claude/.credentials.json`, `codex/auth.json` (constants in doctor.go).
  Auth path present but NOT a regular file → warn. Doctor stays read-only:
  copy-on-consent is the Phase 3 bootstrap task, which will also add the
  global-auth-exists comparison; this slice is local-state-only.
- **Disabled agents** report `routing disabled (<key>.enabled = false)` at
  ok (same wording family as status). Broken config (cfgOK=false) treats
  all agents as enabled — file checks don't need config, and silence on a
  broken-config machine would hide state.
- **OpenCode's subject is the config stub** (`opencode/opencode.json`), not
  auth — partial mode keeps auth/sessions global (§15.3; those warnings are
  the NEXT task). Missing stub → warn (re-init recreates it); a non-regular
  entry at that path → error (blocks routing). doctor_test's `mkLayout` now
  writes the stub, matching what init guarantees.
- **Binary presence** ("Agent binaries", in AND out of project) is a
  stat-only PATH walk (`statBinaryOnPath`: executable regular file;
  exec.LookPath unusable — reads the real PATH, not the injected Env's).
  Always ok-level: not every project uses all three agents.

## D024 — 2026-06-11 — doctor slice 4: OpenCode §15.3 isolation findings
Two inside-project findings (read with D021–D023; exit semantics unchanged),
both skipped entirely when `opencode.enabled = false` (no line printed), both
collapsed to ok when `opencode.xdg_full_isolation = true` (XDG roots are then
routed, so neither leak exists). Broken config = defaults (enabled, partial).
- **"OpenCode sessions" (partial-isolation warning)** warns ONLY when the
  global data dir `${XDG_DATA_HOME:-$HOME/.local/share}/opencode` (§3.3)
  EXISTS — evidence OpenCode is in use and sessions ARE accumulating
  globally. Absent dir → the same limitation is stated at ok level
  ("nothing stored there yet"). Rationale: §15.3's "must warn" taken
  unconditionally would make every default-config project exit 3 forever,
  which D021/D023 rejected for auth; conditioning on observed global data
  warns exactly when the leak is real while a fresh machine stays exit 0.
  The limitation text + opt-in remedy appear at BOTH levels, so doctor
  always states the §15.3 fact.
- **"OpenCode merge chain" (§23 must-warn)** inspects the global
  `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/opencode.json`. "Will leak" =
  JSON object with ≥1 top-level key besides `$schema` (warn lists the keys,
  sorted). Absent / empty file / `{}` / schema-only → ok. Unparseable
  (OpenCode tolerates JSONC; we only parse strict JSON via stdlib
  encoding/json — NOT a new dependency, D004 intact) or unreadable or
  non-regular → conservative warn ("review it manually"): doctor cannot
  prove it leaks nothing.
- Both resolve global paths from the injected Env (HOME / XDG_* lookups),
  never the process env — fully fakeable; HOME+XDG both unset → ok-level
  "cannot locate". Helpers: `globalOpencodeDataDir` /
  `globalOpencodeConfigPath` / `opencodeConfigKeys` in doctor.go; reuse
  them when the handoff exclusion engine needs the same paths.

## D025 — 2026-06-11 — doctor slice 5: Keychain note + gstack state; Env.GOOS
(read with D021–D024; exit semantics unchanged)
- **`Env` gained `GOOS string`** (osEnv sets runtime.GOOS): platform-gated
  findings read it, never runtime.GOOS directly — same injection philosophy
  as Getwd/LookupEnv. fakeEnv leaves it "" (= not-darwin), so every existing
  test is deterministic on any host; tests wanting darwin set it explicitly.
  Future platform-conditional code must use env.GOOS too.
- **Keychain note** (`keychainFindings`, label "Claude auth (macOS)"): on
  darwin + inside a project + claude routing enabled (broken config =
  defaults), an ok-level finding states §15.1 — auth lives in the shared
  system Keychain, no per-project account isolation is possible, no
  per-project re-login needed. ok because it is a platform fact, not a
  problem; absent entirely off darwin / outside a project / claude disabled
  (skip-when-moot, same pattern as D024's opencode findings).
- **gstack global risk** (label "gstack (global)", §23 must-warn): warns
  whenever `$HOME/.claude/skills/gstack` exists — in AND out of projects,
  no out-of-project downgrade: a global gstack affects every project, it is
  a real pollution condition, not a fresh-machine default (so D021's
  "fresh machine exits 0" still holds — fresh machines don't have it).
  Lstat, not Stat: a stray file or dangling symlink counts. HOME comes from
  injected Env only; HOME unset → ok "cannot locate".
- **gstack project state** (label "gstack (project)", inside project only):
  `.agentmod/claude/skills/gstack` dir → ok installed; absent → ok
  not-installed (installing gstack is optional; remedy names
  `agentmod install gstack`, which Phase 4 ships); non-directory → warn.
  Reported regardless of claude.enabled — what sits in the project-local
  skills dir is a fact either way. Path constants `gstackRelGlobal` /
  `gstackRelProject` in doctor.go — Phase 4's installer must reuse them.

## D026 — 2026-06-11 — guard claude-bash: engine, deny modes, fail-safe scope
- **Layout**: pure engine `guard.Decide(input []byte, home string) Decision`
  in new `internal/guard` (stdlib only); stdin/exit plumbing in
  `internal/cli/guard.go` (`runGuard`). `Env` gained `Stdin io.Reader`
  (osEnv = os.Stdin, fakeEnv leaves nil = empty input) — future
  stdin-consuming commands (auth copy-on-consent prompt) must read it, never
  os.Stdin directly.
- **Deny modes (§3.1, both implemented)**: default = exit 2 + reason on
  stderr ("agentmod guard: BLOCKED: …"); `--json` = exit 0 + stdout
  `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":
  "deny","permissionDecisionReason":…}}`. Allow is ALWAYS silent exit 0 (no
  "allow" JSON — no opinion is the safest allow). Exit 2 here is the hook
  protocol's number, not our exit-code table's ExitNotInProject; documented
  as local const exitGuardDeny.
- **Protected paths** (product scope, narrower than the dev-harness guard):
  the four global agent homes (claude/codex dot-homes, opencode config +
  XDG data dirs) in tilde / $HOME / ${HOME} / /Users/x / /home/x spellings
  PLUS the literal injected HOME value (catches /srv/home1-style homes).
  Credential dirs (.ssh etc.) stay harness-only. Boundary class after the
  home name so a ".claudette" path never matches; home == "/" is ignored
  (would match everything).
- **Deny rules**: (1) sudo at command position; (2) HOME= reassignment
  (boundary-guarded — a CODEX_HOME=/x prefix does NOT match); (3)
  global-home reference AND (command-position write cmd cp/mv/rm/mkdir/
  rmdir/touch/tee/ln/rsync/install/unzip/chmod/chown/truncate/dd | a git
  clone | redirection whose TARGET is a protected home). Redirect rule is
  target-scoped (per IMPLEMENTATION_PLAN §11), deliberately narrower than
  the harness guard's any-home-redirect rule: reading a global file
  redirected into the project, or redirecting to a non-agent file under
  home, is allowed. (?m) on command-position regexes so multiline Bash
  blocks are covered line by line.
- **npm -g deliberately NOT blocked**: inside an agentmod project
  NPM_CONFIG_PREFIX routes global installs into .agentmod/node — blocking
  them would fight our own routing (the dev-harness guard blocks it only
  because the dev repo has no routing).
- **Fail-safe (§17)**: unparseable/tool_name-less input → deny only if the
  raw bytes match the protected-path pattern, else allow; stdin read errors
  ignored (decide on the partial bytes); never block everything. Non-Bash
  tool_name → allow (the settings matcher is "Bash"; Write/Edit policing is
  Claude's own permission system's job, and T17 wires the matcher).

## D027 — 2026-06-11 — T17: init wires guard into .agentmod/claude/settings.json
- **Placement (FABLE_PLAN §17, restated)**: the PreToolUse hook lives in the
  ROUTED home's settings — `.agentmod/claude/settings.json` — and NEVER in
  the project's `.claude/settings.json` (git-shared; would impose the guard
  on non-agentmod users). A test locks project `.claude/settings.json`
  byte-identical through init.
- **Code**: `ensureClaudeGuardHook(agentmodDir, env)` in new
  `internal/cli/claudesettings.go`, called by runInit between the opencode
  stub and ensureGitignore; init output gained a `Claude guard:` line.
  `Env` gained `Executable func() (string, error)` (osEnv = os.Executable;
  fakeEnv returns the fixed `/fake/bin/agentmod`, so EVERY init test
  exercises wiring deterministically). Follow the Getwd/LookupEnv/Stdin
  pattern for future process-introspection needs.
- **Hook command**: `shellQuote(filepath.Clean(binPath)) + " guard
  claude-bash"`, matcher `"Bash"`, type `"command"`. Clean matters:
  os.Executable can return invocation-relative spellings
  (`…/proj/../agentmod` — caught by binary smoke). EvalSymlinks is
  deliberately NOT applied: a version-managed symlink (homebrew) is the
  stabler reference across upgrades.
- **Merge semantics (mirrors D019's rc discipline)**: file absent → created
  with exactly the hook config; hook present-and-correct → ZERO writes
  (user formatting preserved byte-identically); stale binary path →
  command rewritten in place, no duplicate entry (`guard claude-bash`
  substring is the ownership marker); hook absent from existing file →
  entry appended, all user keys preserved, but the file is re-marshaled
  (stdlib MarshalIndent, keys sorted — formatting loss only when a write
  is needed anyway). Whitespace-only file = empty object. Invalid JSON /
  non-object root / wrong-typed `hooks`/`hooks.PreToolUse` → hard error,
  zero writes (like D019's corrupt fence).
- **Unresolvable binary** (Executable nil or erroring): skip with an
  explanatory line, exit 0, no file written — init must not fail over an
  exotic os.Executable error, and a hook pointing nowhere would make every
  Claude Bash call error. Re-init later wires it. The doctor finding for
  wired/stale-binary guard state is a NEW Phase 3 task (added to TASKS.md),
  not part of T17.
- **Config not consulted**: init wires the guard even if claude.enabled =
  false — the hook only fires when Claude actually runs with the routed
  home, which is exactly when the guard should act; routing disablement is
  the hook's natural off switch.

## D028 — 2026-06-11 — T15: auth copy-on-consent lives in init (auth.go)
- **Placement**: `bootstrapAuth(agentmodDir, opts, stdout, env)` in new
  `internal/cli/auth.go`, called by runInit AFTER the Shell hook lines and
  hook-activation notice, BEFORE the closing status hint — prompts and the
  aligned `Claude auth:` / `Codex auth:` summary lines print together at
  the natural position. Doctor stays strictly read-only (D021/D023): it
  reports auth state; only init copies, and only with consent.
- **Decision ladder per agent** (first match wins): darwin Claude →
  Keychain note, no file flow at all (no prompt even if a global
  .credentials.json exists — the file is not how Claude authenticates on
  macOS); local auth present (Lstat, any type) → "already present, left
  untouched", no prompt; HOME unset → cannot-locate line + re-login remedy;
  global auth absent → "no global auth to copy" + remedy; global auth
  non-regular → refuse + remedy, no prompt; `--yes`/`--non-interactive` →
  "non-interactive mode never copies auth" + remedy, never reads stdin;
  else prompt `[y/N]` on stdout, read env.Stdin (D026). Only explicit
  y/yes (case-insensitive) consents — EOF, nil stdin, read errors, empty
  answers all decline. Decline is never an error: exit stays 0.
- **Copy mechanics**: read global file (read-only access to global homes is
  allowed; writes never), write local with O_CREATE|O_EXCL mode 0600
  (`copyAuthFile`). Copy failure → init exits 1 (like other init steps).
- **One shared bufio.Reader per init run** (`authPrompter`): consecutive
  prompts must not lose buffered input between them; a final partial line
  without trailing newline still counts as an answer.
- **Shared strings**: re-login remedies extracted to `claudeReloginRemedy`/
  `codexReloginRemedy` consts in doctor.go, used by both doctor findings
  and init decline paths. Global home dir names `globalClaudeDirName`/
  `globalCodexDirName` live in auth.go.
- **Config not consulted** (mirrors D027): the prompt fires even if the
  agent's routing is disabled in agentmod.toml — init does not load config,
  and the per-run explicit consent IS the gate; copying into an unused home
  is harmless and saves a step when routing is re-enabled.
- **Exclusion-list note for Phase 5**: consent-copied targets are exactly
  `claude/.credentials.json` and `codex/auth.json` relative to .agentmod/
  (constants claudeAuthFile/codexAuthFile in doctor.go + layout dir names).
  The T20 exclusion engine must hardcode these; authSpec's doc comment
  marks the spot.

## D029 — 2026-06-11 — doctor: Claude guard wiring finding (Phase 3 last slice)
- **Subject + placement**: label "Claude guard", inside a project only (the
  guard lives in the routed home's settings.json — no project, no line;
  D021's outside-project policy). `guardFinding(agentmodDir, env)` in
  doctor.go, read-only; init owns creation and repair (D027).
- **Reuse, not forks** (mirrors rcfile's inspectRCBlock pattern from D021):
  claudesettings.go grew `guardHookEntries(pre)` (the marker-bearing hook
  maps, shared walker) and `inspectGuardHook(path, desired)` returning
  guardHookFileAbsent/Missing/Stale/Current + the found command;
  ensureClaudeGuardHook was rewritten on guardHookEntries — write-path
  behavior unchanged, all T17 tests untouched.
- **Severities**: wired with the current binary's command → ok; settings.json
  absent → warn (re-run init); file present without a marker hook → warn;
  marker present with a different command → warn naming BOTH the found and
  expected commands, remedy re-run init (repairs in place, D027);
  unparseable / wrong-typed file → error, with the writer's own hard-error
  strings (doctor reports what init would refuse to touch). Multiple marker
  hooks: any one matching = current (ensure never creates duplicates; a
  hand-made duplicate still counts as wired).
- **Unresolvable env.Executable** (nil or erroring): ok-level "hook present
  … binary path not verified" when a marker hook exists; the file-absent /
  hook-missing warns still fire (they need no binary). Mirrors init's
  skip-not-fail stance on exotic os.Executable errors.
- **Not gated on claude.enabled** (D027: init wires unconditionally —
  routing disablement is the hook's natural off switch, so absence is a
  finding regardless of config; broken config changes nothing here).
- **Test fixture**: doctor_test.go's mkLayout now writes a guard-wired
  settings.json for fakeBinPath (`writeGuardSettings` helper), matching
  init's guarantee — same precedent as the opencode stub in D023. Layout
  tests deleting claude/ moved from os.Remove to os.RemoveAll.

## D030 — 2026-06-11 — install gstack: happy-path clone (Phase 4 slice 1)
(read with D025 — doctor's gstack findings name this command as the remedy)
- **Lives in internal/cli (install.go), not internal/installer**:
  IMPLEMENTATION_PLAN §6 sketched a separate package, but the installer
  needs gstackRelProject (doctor.go, D025: single source for the path),
  project discovery, Env injection, and the cli exit codes — a separate
  package would force exporting all of those for one consumer. Deviation
  recorded here; revisit only if a second installer component appears.
- **Source override via AGENTMOD_GSTACK_SOURCE** (read through injected
  Env.LookupEnv): tests clone from a local fixture repo (git init+commit in
  a temp dir) so no test touches the network; doubles as a mirror/fork
  escape hatch. Default stays the hardcoded
  https://github.com/garrytan/gstack (§3.4 verified fact).
- **Real exec.LookPath/exec.Command for git**, unlike doctor's
  statBinaryOnPath (which honors injected Env because it only REPORTS):
  install actually executes git, and the child inherits the real process
  environment anyway — pretending otherwise via injected PATH would be
  false isolation. Documented in install.go. GIT_TERMINAL_PROMPT=0 is set
  so a credential prompt can never hang the command.
- **Atomicity**: clone into os.MkdirTemp(skillsDir, ".gstack-clone-") —
  sibling of the target, same filesystem — then os.Rename onto
  claude/skills/gstack; deferred RemoveAll cleans the temp dir on any
  failure. The target either doesn't exist or is a complete clone. `.git`
  is kept (enables later updates; handoff exclusions strip it in Phase 5).
- **Scope landed early**: outside-project → exit 2 (the command cannot even
  locate a destination without a project) and already-installed → abort
  exit 1 (never clobber) are inherent to a safe happy path, so they landed
  now WITH tests; the Phase 4 item 2 that names them now means --force +
  any hardening. gstack's own setup script is never run (§16) — clone only.
- **No flags yet**: `--force` rejected ("takes no further arguments") until
  its slice lands; argument validation happens before any FS work (tested).
  (Superseded by D031 — --force now accepted.)

## D031 — 2026-06-11 — install gstack --force: clone first, then swap
(extends D030; Phase 4 item 2)
- **Flag parsing**: everything after `gstack` is matched against a switch;
  only `--force` is accepted (idempotent if repeated), anything else →
  `unsupported argument %q (only --force is supported)`, exit 1, before any
  FS work. No flag package — one flag doesn't justify it.
- **Replace order (IMPLEMENTATION_PLAN §10 "replaces only the project-local
  copy")**: the clone into the sibling temp dir happens FIRST, exactly as in
  the plain path; only after it succeeds is the existing install moved
  aside (rename to a `.gstack-old-*` sibling), the clone renamed in, and the
  old copy RemoveAll'd. A failed clone therefore returns before the existing
  install is touched (tested byte-intact).
- **macOS rename gotcha**: Darwin's rename(2) refuses an existing directory
  destination even when empty (cost one red run), so os.MkdirTemp only
  RESERVES the unique `.gstack-old-*` name — it is os.Remove'd immediately
  before renaming the old install onto it. The tiny window between Remove
  and Rename is acceptable for a CLI (no concurrent agentmod expected in
  one skills dir).
- **Failure honesty**: if the final rename-in fails after the old install
  was moved aside, agentmod tries to rename it back ("previous install
  restored"); if even that fails the old copy is NOT deleted and its
  `.gstack-old-*` path is printed. RemoveAll failure of the old copy after
  a successful swap is a warning, not an error — the new install is live.
- **Non-force abort message** now names the remedy: "re-run with --force to
  replace it" (was "remove that directory to reinstall").

## D032 — 2026-06-11 — install gstack: global pollution verification shape
(Phase 4 item 3; IMPLEMENTATION_PLAN §10 "record listing before/after")
- **Snapshot points**: `snapshotGlobalSkills(env)` is the FIRST thing
  installGstack does (before the exists check, before MkdirAll — maximal
  coverage); the comparison (`verifyGlobalSkillsUnchanged`) runs after the
  rename/swap, after the "Installed gstack to" line, before the success
  paragraph. Early-return failure paths (abort, no git, failed clone) do
  not verify — nothing global-adjacent ran.
- **Delta, never existence**: an absent skills dir is a valid EMPTY listing
  (names nil); only a before/after difference violates. The dev machine's
  own pre-existing global gstack (D010) therefore never trips it — doctor
  owns the existence warning.
- **Unverifiable ≠ failure**: HOME unset → "skipped (HOME not set …)" note;
  ReadDir error before or after (e.g. skills is a regular file → ENOTDIR)
  → "skipped (cannot read …)" note. Both stdout, exit 0 — defense in depth
  must not fail installs on machines without a global Claude home. All
  reads go through the injected Env's HOME only (D025 pattern).
- **Violation report**: stderr block — "VIOLATION: <dir> changed during
  install", "new entries: …" (instruct manual removal) and/or "entries
  that disappeared: …", then an honesty line: the project-local install
  itself succeeded (it is left in place — deleting it fixes nothing) but
  this is a bug to report. Exit 1; the "was not touched" success paragraph
  is suppressed.
- **Test shape** (the question STATE.md deferred): pure `diffListings`
  (sorted merge-walk) unit table; integration happy path asserting the
  "unchanged" line with PRE-EXISTING global entries (D010 negative); the
  violation path is tested END-TO-END with no production test hook by
  symlinking the fake HOME's `.claude/skills` AT the project-local skills
  dir — the legitimate local install then appears as a new "global" entry;
  ENOTDIR skip integration; direct `verifyGlobalSkillsUnchanged` call for
  the removed-entry branch. fakeEnv has no HOME, so every pre-existing
  install test also asserts/tolerates the skip line.

## D033 — 2026-06-11 — install gstack: distinct error reporting shape

IMPLEMENTATION_PLAN §10 requires distinct errors for not-a-project /
git-missing / network / target-exists. The first and last landed with
D030/D031; this records how the remaining two (plus "setup failure") are
diagnosed and tested.

- **Network/clone failure: forward git, don't classify.** git's own output
  already distinguishes the causes (`Could not resolve host` vs
  `repository … not found/does not exist` vs auth refusal) more precisely
  than any wrapper taxonomy, so agentmod forwards CombinedOutput verbatim
  after the `git clone failed: <err>` line, then appends a two-line hint:
  check network access / source reachability, and the
  `AGENTMOD_GSTACK_SOURCE=<url-or-path>` override for mirrors and offline
  use. No exit-code or message-prefix taxonomy was added. Test proves
  FORWARDING by asserting git's own words (`fatal:` + `does not exist` for
  a missing local source) — strings no agentmod message contains — plus
  the hint's env-var name.
- **git-missing: tested with a crippled REAL PATH.** installGstack resolves
  git via exec.LookPath on the process PATH (documented D030 exception to
  the injected-Env rule), so the test uses `t.Setenv("PATH", t.TempDir())`.
  PATH is not global agent state; t.Setenv auto-restores. Asserts the
  distinct "install gstack needs git, which was not found on PATH" message
  and that NOTHING was created (the git check precedes MkdirAll).
- **Setup failure = PathError passthrough, no rewording.** Local FS
  failures (Lstat/MkdirAll/MkdirTemp/Rename) surface as `agentmod: <op>
  <path>: <cause>` via %v — the PathError already names operation, path,
  and OS cause; wrapping would only duplicate it. Fault-injection test: a
  regular FILE at `claude/skills` makes every path operation under it fail
  ENOTDIR. NOTE (honesty): with that blocker the failure fires at the
  initial `Lstat(target)` (ENOTDIR is not IsNotExist), not at MkdirAll —
  the test asserts the user-visible contract (exit 1, "not a directory" +
  path on stderr, blocker byte-untouched), not which call tripped first.
  A read-only-dir MkdirAll variant was considered and skipped: it needs a
  root-skip guard and proves the same passthrough.
- Phase 4 is COMPLETE with this slice; T18 ✅.

## D034 — 2026-06-11 — .amod writer shape (Phase 5 slice 1)

`agentmod handoff create` landed as new package `internal/handoff`
(IMPLEMENTATION_PLAN §3 architecture) + thin glue `internal/cli/handoff.go`.
Read this before touching snapshot code; later Phase 5/6 slices build on
every choice here.

- **Zip member layout** (IMPLEMENTATION_PLAN §12): `manifest.json`,
  `inventory.json`, `checksums.txt` at the root, payload under
  `payload/<project-root-relative path>` with forward slashes — i.e.
  `payload/.agentmod/...`. The prefix leaves room for the §12 project-level
  members (`payload/.claude/...`, MCP config) without a format break, and
  restore maps members onto the project root directly.
  REDACTION.md / HANDOFF.md / RESTORE.md are NOT written yet — they are
  their own TASKS items; T19 stays 🟡 until they exist.
- **Scope of slice 1:** payload = everything under `.agentmod/` except
  `.agentmod/snapshots/`. That one exclusion is STRUCTURAL, not policy:
  snapshots/ is the default output dir, so packing it would nest prior
  snapshots (and the in-progress temp file) inside the new one. The walk
  also skips the output/temp file by absolute path in case `--output`
  points elsewhere inside `.agentmod/`. The POLICY exclusion engine (auth
  incl. D028's consent-copied paths, caches, .env, …) is the next task —
  until it lands, a snapshot MAY contain auth files; do not ship/describe
  handoff as secret-safe before that slice.
- **Manifest v1 fields:** schema_version=1, created_at (RFC3339 UTC),
  agentmod_version, platform ("GOOS/GOARCH"; cli maps empty injected GOOS
  to "unknown"). Git metadata + policy flags are later Phase 5 slices —
  they EXTEND Manifest; restore must tolerate their absence.
- **Inventory:** every non-directory payload member: zip member name
  (incl. `payload/` prefix — unambiguous join key), size, sha256 of the
  member content, mode as 4-digit octal permission string, and
  symlink_target for symlinks. Sorted by path. Directories appear in the
  zip (empty dirs must restore) but not in the inventory.
- **Symlinks** are stored as zip symlink entries (mode bit + target as
  content, the archive/zip convention); the sha256 is of the TARGET STRING,
  so "every content-bearing member hashes to its inventory/checksums entry"
  holds uniformly for verify. Targets are recorded verbatim — validation
  (escape-the-payload checks) is restore's job per §21, where the security
  boundary is. Irregular files (fifo/socket/device) are a hard create-time
  error naming the path.
- **checksums.txt** is sha256sum format (`<hex>  <name>`) covering
  manifest.json, inventory.json, then payload members in path order; it
  cannot list itself. Verified compatible with `shasum -a 256 -c` in the
  binary smoke.
- **Determinism + atomicity:** all zip mtimes = CreatedAt (injected);
  identical input tree + CreatedAt ⇒ byte-identical .amod (tested). Output
  is written to a `.amod-partial-*` temp sibling and renamed in; failures
  remove the temp. Existing output → refuse (no overwrite, D013 pattern).
  The file keeps CreateTemp's 0600 mode DELIBERATELY: snapshots may carry
  sessions/working context, so restrictive perms are the right default.
- **Clock injection:** cli `Env` gained `Now func() time.Time` (osEnv =
  time.Now; fakeEnv = fixed `fakeNow` 2026-06-11T12:30:45Z). Default output
  name `<base(projectRoot)>-<UTC yyyymmdd-hhmmss>.amod` under snapshots/ —
  with the fixed fake clock a second default-name create collides and must
  refuse (tested; also a real two-creates-in-one-second behavior).
  Nil Now falls back to time.Now (field optional by contract, tested).
- **CLI surface:** `handoff create [--output PATH]`; restore/inspect/
  verify/list answer "not implemented yet" (exit 1) so they read as
  planned, not mistyped; unknown subcommand/flags rejected before any FS
  work (tested). Outside a project → exit 2 naming 'agentmod init'.

## D035 — 2026-06-11 — T20: default exclusion engine shape

`internal/handoff/exclude.go`: `Rule{ID, Reason, Matches(relPath, base,
isDir)}` + `DefaultRules()`. Read this before touching exclusion/redaction
code; the REDACTION.md slice renders what this records.

- **Matcher contract**: relPath is the project-root-relative forward-slash
  path (".agentmod/claude/.credentials.json"); first matching rule wins, so
  security-relevant rules sit first in the list and an entry matched by
  several rules is reported under the most security-relevant one. A matched
  directory is pruned (SkipDir) and recorded ONCE with a trailing "/" —
  never per descendant. The rule check runs BEFORE the member-kind switch,
  so an excluded fifo is silently dropped instead of being the irregular-
  file hard error.
- **Recording**: `Result.Excluded []ExcludedEntry{Path, RuleID, Reason}` in
  walk (lexical, deterministic) order. The structural snapshots/ skip
  (D034) is recorded too, as `snapshots-output` — recorded only when the
  dir actually exists, so explicit `--output` against a snapshot-less tree
  reports nothing. Reasons are human-readable sentences; REDACTION.md
  renders them verbatim.
- **Rules opt-out**: `CreateOptions.Rules` nil → DefaultRules(); non-nil
  used as-is; an explicitly EMPTY slice disables all policy exclusions
  (structural skip remains) — the documented escape hatch, pinned by test.
  Phase 7 --for-git APPENDS rules (sessions/logs) instead of forking the
  walk.
- **Auth matched by NAME at any depth** (per D028's "exclude the NAME, not
  the provenance"): `.credentials.json`, `auth.json`, `credentials.json`,
  bare `credentials`. Exact names only — `auth.json.bak` stays (the secret
  scan slice owns fuzzy/content detection).
- **Other rules**: env-file = `*.env` + `.env.*` (NOT `.envrc` — content
  scan's job); ssh-key = id_rsa/dsa/ecdsa/ed25519(+_sk) families with
  optional .pub + `*.pem`; credential-dir = .ssh/.aws/.azure/.gcloud/.kube/
  .gnupg/.docker dirs; os-credential-store = `*.keychain`/`*.keychain-db`;
  vcs-git = `.git` as dir OR regular file (worktrees); node-modules dirs;
  tmp = `tmp`/`.tmp` dirs; cache = any `.cache` dir + the three routing
  cache targets PATH-ANCHORED (`.agentmod/node/{npm-cache,pnpm,bun}` via
  new layout consts NodeNPMCacheDir/NodePnpmDir/NodeBunDir, which
  routing.Vars now also uses) — a dir merely NAMED npm-cache elsewhere is
  user content and stays.
- **Deliberately NOT excluded**: sessions + logs (normal handoffs keep
  them; --for-git excludes them in Phase 7), fuzzy "token" name matching
  (tokenizer.json false positive — T21's content scan covers real tokens).
  No source-code rule yet: the payload is .agentmod-only, so source is
  structurally absent; the rule gets added when project-level payload
  roots (payload/.claude etc.) land.
- **Known consequences for the docs slices**: the gstack clone loses its
  .git (reinstall via `agentmod install gstack --force`); npm-prefix bin
  symlinks under node/bin dangle in the snapshot because lib/node_modules
  is excluded (restore docs must say "reinstall global npm tools").
  HANDOFF.md/RESTORE.md must mention both.
- **CLI**: `handoff create` prints each excluded path with its rule ID
  (singular/plural-correct count line; "nothing" when zero) — interim
  visibility until REDACTION.md exists.

## D036 — 2026-06-11 — T21: secret-candidate scan + REDACTION.md shape

`internal/handoff/scan.go` (patterns + `scanContent`) and
`internal/handoff/redaction.go` (`RedactionName` + `renderRedaction`),
wired into the writeSnapshot walk. Read D034+D035+this before touching
snapshot/redaction code.

- **Pipeline position** (IMPLEMENTATION_PLAN §12 collect → filter → scan →
  write): only files the exclusion engine KEPT are scanned — private-key
  material inside an excluded `.env` neither blocks nor appears as a
  finding (pinned by test). The regular-file zip path switched from
  streaming io.Copy to `os.ReadFile` so the scanned bytes are exactly the
  packed bytes; agent-env files are small enough that whole-file reads are
  fine, and the unreadable-file error still names the path via ReadFile.
- **Finding shape**: `ScanFinding{Path, Pattern, Line, Hard}` — Path is
  project-root-relative like ExcludedEntry.Path; Line is the 1-based line
  of the pattern's FIRST match; at most one finding per (file, pattern) so
  output stays readable. The matched bytes are NEVER recorded — not in the
  Result, not in REDACTION.md, not on stdout/stderr.
- **Patterns** (stdlib regexp, checked in order): `private-key` (HARD:
  `-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----`), `aws-access-key-id`
  (AKIA/ASIA + 16), `github-token` (gh[pousr]_ + 20), `sk-token` (sk- +
  20 — sk-FAKE-fixture stays under the bar by design), and three
  assignment-context patterns `api-key`/`token`/`secret` that require a
  `[:=]` after the keyword so prose ("the tokenizer", "secretary") never
  warns. Token also requires an auth/access/refresh/bearer/session/api
  prefix.
- **Hard vs warn**: only private-key is hard. Hard findings refuse
  creation AFTER the walk (all hard findings listed in one error naming
  path/line/pattern + the `--allow-findings` remedy; the temp-file defer
  keeps refusal atomic — nothing is left on disk). Warn findings never
  block. `CreateOptions.AllowFindings` packs hard findings anyway; they
  are then marked "(HARD finding; packed because --allow-findings was
  given)" in the report and on stdout.
- **REDACTION.md**: root zip member (D034 layout), listed in checksums.txt
  between inventory.json and the payload, rendered deterministically from
  Result.Excluded (path — ruleID: reason, verbatim) + Result.Findings
  (path, line, pattern only). Both empty states render explicit sentences
  ("Nothing was excluded…", "No secret candidates…").
- **CLI**: `handoff create [--output PATH] [--allow-findings]`; stdout
  gained a "secret scan:" line (clean / N candidate findings) + per-finding
  lines. Unsupported-arg message updated to name both flags.
- T19 stays 🟡: HANDOFF.md/RESTORE.md are the next slice; T21 is complete.

## D037 — 2026-06-11 — HANDOFF.md + RESTORE.md docs members

`internal/handoff/docs.go`: `HandoffDocName`/`RestoreDocName` +
`renderHandoffDoc`/`renderRestoreDoc`, wired into writeSnapshot. T19 ✅.
Read D034–D036 + this before touching snapshot/docs code.

- **Member position** (D034 layout): both are root zip members; write
  order and checksums.txt order are manifest, inventory, REDACTION.md,
  HANDOFF.md, RESTORE.md, then payload — checksums.txt last (cannot list
  itself). The member-set, checksums-coverage, and determinism tests all
  cover them.
- **Renderer shape** mirrors renderRedaction: pure functions over
  create-time data already in scope (createdAt, version, platform,
  `filepath.Base(filepath.Clean(opts.ProjectRoot))` as the project name,
  and the populated `*Result` for counts), so identical snapshots stay
  byte-identical. No new Manifest fields, no new CreateOptions.
- **HANDOFF.md** = §12's "what this is, how to restore, what's missing":
  identity paragraph; payload size + inventory/checksums pointer; restore
  pointer PLUS the honest note that the creating build does not implement
  restore yet (the docs travel with the snapshot for the machine that
  has restore); "What is missing" renders exclusion count (or the
  explicit nothing-excluded sentence), scan summary (clean / N findings,
  singular/plural correct via `countNoun`), auth-never-travels, and the
  two D035 notes (gstack clone loses .git → `install gstack --force`;
  node/bin npm symlinks dangle → reinstall global npm tools).
- **RESTORE.md** = §12's "step-by-step restore + re-login guidance":
  4 numbered steps (install agentmod+agents → `agentmod init` →
  `handoff restore` with the safety properties named: checksum/schema
  verification, backup-first, extract-only-under-.agentmod, never
  executes anything → `agentmod doctor`), a re-login section, the macOS
  Keychain note (§15.1 wording: shared Keychain, one login covers every
  project), and the two D035 reinstall notes. Deliberately renders from
  version only — restore guidance is snapshot-independent.
- **Canonical re-login remedies MOVED to internal/handoff** (exported
  `ClaudeReloginRemedy`/`CodexReloginRemedy` in docs.go); doctor.go's
  unexported consts are now aliases (`claudeReloginRemedy =
  handoff.ClaudeReloginRemedy`), so doctor findings, init's auth flow,
  and the RESTORE.md that travels with every snapshot can never drift
  apart. Direction matters: cli already imports handoff (no cycle);
  handoff importing cli is impossible. All existing cli string
  assertions pass unchanged.
- **Tests**: docs_test.go — end-to-end content anchors for both members
  (incl. verbatim remedy strings compared against the constants) +
  renderer unit test pinning empty/singular/plural states of the
  "What is missing" lines. Binary smoke: init → create → unzip both
  docs read correctly → `shasum -a 256 -c` passes incl. both new
  members.

## D038 — 2026-06-11 — loop.sh limit-detection broadened (harness, run 3)
Run 2 ended at the MONTHLY SPEND limit ("You've hit your monthly spend
limit · raise it at claude.ai/settings/usage") — a message the D015 backoff
grep (session/usage/rate limit) did not match, so iterations 26–60 burned in
seconds again. Fix: pattern now also matches `spend limit|limit reached|hit
your` (the stable prefix of every Claude limit message observed so far);
the 2000-byte size guard still prevents false positives on real work logs.
Note a spend limit does not reset on a timer — the 15-min retry loop now
doubles as "wait for the user to raise it" (bounded at 48 sleeps). Run 2
garbage logs archived to reports/run2-spendlimited/. Run 2 was killed
mid-iteration leaving a PARTIAL but green edit to internal/handoff
(git-metadata slice start; builds, all tests pass) — left in the working
tree for the next iteration to finish. Run 3: AGENTMOD_LOOP_MAX_ITERS=30
(~15 tasks remain).

## D039 — 2026-06-11 — T22: git state metadata + --allow-dirty gate

`internal/cli/gitstate.go` (collectGitState/summarizeStatus/redactRemoteURL/
gitIdentity) + manifest `git` key. Read D030+D034+this before touching
git-metadata code. Run 2 left the GitState struct in the working tree;
this iteration finished and tested it.

- **Split of responsibilities**: internal/handoff stays exec-free —
  `CreateOptions.Git *GitState` is plain data rendered into manifest.json
  (`omitempty`: nil ⇒ key absent, the D034 tolerate-absence contract;
  asserted absent-not-null in tests). The cli EXECUTES git (D030 exception,
  like install): real `exec.LookPath`, real environment, plus
  GIT_TERMINAL_PROMPT=0 and GIT_OPTIONAL_LOCKS=0 so every probe is
  non-interactive and read-only (status must not refresh the index).
- **Unavailable ⇒ note, never failure**: git binary absent → "git binary
  not found on PATH"; not a work tree → "not a git repository". Both print
  as `git: metadata omitted (<note>)` and omit the manifest key — handoff
  must work in non-git projects (§20). A repo ABOVE the project root counts
  as the project's repo (rev-parse --is-inside-work-tree semantics).
- **Fields** (§20): branch (empty = detached; symbolic-ref --short -q),
  head (empty = unborn branch; rev-parse --verify -q), dirty +
  status_summary ("clean" or "N staged, N modified, N untracked" — parsed
  from `status --porcelain` with `-c status.showUntrackedFiles=normal` so
  user display config cannot hide dirt; untracked counts as dirty;
  a conflicted UU entry counts as both staged and modified), remote_url
  (origin only; absent remote = omitted), and source_included — ALWAYS
  false in MVP but spelled out explicitly so manifest readers never guess.
- **Redaction**: in `scheme://` URLs the ENTIRE userinfo is stripped
  (https://user:token@h, https://token@h, and ssh://git@h all lose it —
  conservative: token-as-username is a real GitHub form, and the manifest
  value documents WHERE the remote is, it is not meant to be dialed
  verbatim). scp-like `git@host:path` has no credential slot → unchanged.
  Port preserved (authority scan up to first `/`, last `@` within it).
- **Dirty consent gate** lives in the CLI before handoff.Create (same shape
  as --allow-findings): dirty + no `--allow-dirty` → stderr refusal naming
  the summary and the flag, exit 1, NO file written. With the flag, stdout
  marks the line `DIRTY (…) — packed anyway (--allow-dirty)`.
- **Tests**: gitstate_test.go (redact table ×8, porcelain table ×7, missing
  git via PATH mask, non-repo, clean repo w/ credentialed remote, dirty+
  detached, unborn branch — fixtures via runGitFixture with `init -b main`)
  + 3 cli handoff funcs (omitted-note + nil manifest key, refuse-then-allow
  on unborn dirty repo, clean repo manifest w/ redacted remote) +
  TestCreateManifestGitState in internal/handoff. Binary smoke in /tmp:
  all four shapes verified incl. manifest JSON.

## D040 — 2026-06-11 — T23: inspect / verify / list / pack alias shape

Read side of .amod lives in new `internal/handoff/read.go`
(`Open(path) (*Snapshot, error)` + `(*Snapshot).Verify() *VerifyResult`);
cli grew runHandoffInspect/runHandoffVerify/runHandoffList + the `pack`
and `unpack` top-level aliases. Read D034–D037+this before touching
snapshot read/restore code — Phase 6 restore MUST build on Open/Verify.

- **Open contract**: succeeds on any structurally complete snapshot —
  zip readable, all six §21 root members present (missing ones named in
  the error), manifest/inventory parseable, REDACTION.md readable. It
  does NOT hash anything and does NOT gate on schema version: the caller
  decides (inspect prints a WARNING line for newer schemas, Verify
  records a problem, restore will hard-refuse). Exposes Manifest,
  Inventory, Redaction bytes, total member count, payload dir count.
- **Verify contract**: re-hashes every content-bearing member (everything
  except dir entries and checksums.txt itself) against checksums.txt,
  then cross-checks inventory↔payload both directions: presence, size,
  sha256, mode, and symlink-target-hashes-to-recorded-sha (content IS the
  target per D034, so no second read needed). Read failures become
  problems, never aborts — one bad member must not hide the rest.
  Problems are human sentences in detection order; empty = clean.
- **Exit codes** (cli): verify problems AND structurally-invalid snapshot
  → ExitValidation (3, the §3 verification-failure code); a path that
  cannot even be stat'ed → ExitError (1) — a typo is not a validation
  verdict. inspect always exits 0/1 (it informs, verify judges).
- **Inspect output**: manifest fields, git line (reuses gitIdentity +
  redacted remote; "no metadata recorded" when the key is absent), member/
  payload counts, then REDACTION.md verbatim under a `--- REDACTION.md ---`
  separator — the report IS the excluded/findings summary; re-parsing it
  into a second format would just drift. No project required for
  inspect/verify: recipients have a file, not a project.
- **list**: project-required (exit 2), names *.amod in
  .agentmod/snapshots/ newest-first (mtime desc, ties name asc — matches
  the old recentHandoff pick), with size + mtime. status.go's
  recentHandoff now delegates to the shared cli listSnapshotFiles helper.
  Snapshots written elsewhere via --output are outside list's view by
  design (documented in the function comment).
- **Aliases** (FABLE_PLAN §11): `agentmod pack` dispatches straight to
  runHandoffCreate (same flags, §19's `pack --for-git` lands with
  Phase 7); `agentmod unpack` is an explicit not-implemented stub naming
  'handoff restore' so it reads as planned. Usage text lists both.
- **Test helper**: read_test.go's rewriteSnapshot (mutate/drop/add members
  + optional checksums regeneration so ONLY the deliberate inconsistency
  remains) is the tamper harness for T24's malicious restore fixtures.

## D041 — 2026-06-11 — Restore validation layer: (*Snapshot).PlanRestore (Phase 6 slice 1)

Restore-side path-safety validation lives in new
`internal/handoff/validate.go`: `(*Snapshot).PlanRestore() (*RestorePlan,
[]string)` — problems (human sentences, detection order, ALL collected in
one pass) or a plan, never both. Read D034+D040+this before touching
restore code; the extraction slice EXECUTES this plan and must not invent
its own path handling.

- **Validation-only slice**: `handoff restore`/`unpack` stay
  not-implemented stubs. A half-wired restore that validates but cannot
  extract would change its user contract twice in two slices; the layer is
  library-level until extraction lands (next two TASKS items).
- **Orthogonal to Verify** (D040): Verify judges integrity (hashes,
  inventory agreement), PlanRestore judges path safety and never hashes.
  Restore must run Open → Verify → PlanRestore and refuse on ANY problem
  from either. The schema-version gate is IN PlanRestore too (restore
  hard-refuses; shared wording via new `schemaProblem()` helper so verify
  output and restore refusals never drift).
- **Member rules**: every member must be a §21 root member or under
  `payload/` (anything else — e.g. a smuggled `../evil` — is a problem);
  payload rel paths must be canonical (`path.Clean` fixpoint), relative
  (no leading `/`, no `C:` drive prefix), backslash-free, free of `..`
  (zip-slip named explicitly when the cleaned path escapes), first element
  `.agentmod` (the schema-v1 whitelist; project-level payload roots get a
  schema bump), no duplicates, and no protected elements.
- **Protected elements** = exactly §21's four (`.git`, `.ssh`, `.aws`,
  `.docker`) as path elements anywhere under `.agentmod/`. Create's
  exclusion rules cover a broader credential-dir set, but on restore every
  write is already confined to `.agentmod/`; only these four need an
  explicit deny (`.git` notably — a hostile snapshot could plant git
  hooks). Legit snapshots never contain them (create excludes them).
- **Symlinks**: target read from member content (capped at 4096 bytes —
  larger is hostile, not a link); must be non-empty, relative,
  backslash-free, and LEXICALLY resolve inside `.agentmod/`
  (`path.Join(dir(rel), target)` must stay under the whitelist root).
  Containment is inductive: every link points inside `.agentmod/`, so
  chains cannot escape. Extraction must still write Dirs → Files → Links
  (links LAST) so no file write passes through a just-restored symlink —
  RestorePlan's field doc pins this order.
- **Modes**: plan entries carry `Mode().Perm()` only — setuid/setgid/
  sticky from hostile zips are silently stripped, not refused (a refusal
  would block snapshots from systems with odd umasks for no safety gain).
  Irregular member types (fifo etc.) are problems.
- **Plan shape**: Dirs/Files/Links each sorted by RelPath (parents sort
  before children, so extraction can create in order); RelPath is
  project-root-relative forward-slash (`.agentmod/...`) — extraction joins
  it to the project root via `filepath.FromSlash`.
- **Test helpers**: validate_test.go adds `addZipMember` (append ONE
  member with an explicit mode — hostile symlinks, fifos, setuid bits —
  which rewriteSnapshot's fixed-mode extras cannot express) and
  `wantNoPlan`. Hostile symlink TARGETS are made by mutating the fixture
  link's content via rewriteSnapshot with fixChecksums=true (a symlink's
  content IS its target, D034).

## D042 — 2026-06-11 — Pre-restore backup: rename to .agentmod.backup-<utc-stamp> (Phase 6 slice 2)

`BackupAgentmod(projectRoot, now) (string, error)` in new
`internal/handoff/backup.go` — library-level like D041's validation slice;
`handoff restore`/`unpack` stay stubs. The restore pipeline is pinned as
Open → Verify → PlanRestore → BackupAgentmod → extract; the extraction
slice consumes this function and must not invent its own backup handling.

- **Rename, not copy**: one `os.Rename` — atomic, reads none of the
  contents (sessions/auth bytes never pass through agentmod), preserves
  every mode/symlink byte, and the original survives any later extraction
  failure AS the backup; the complete rollback is renaming the backup
  back. A copy would double disk for large session dirs and leave a
  half-copied backup on failure.
- **Name**: `.agentmod.backup-<utc-stamp>` (IMPLEMENTATION_PLAN §12),
  stamp format identical to default snapshot names (`20060102-150405`),
  ALWAYS rendered in UTC regardless of the clock's zone. `now` comes from
  the caller (cli passes env.Now per D034) — testable, deterministic.
  Exported `BackupPrefix` so the restore cli names the pattern in output
  and gitignore handling without re-spelling it.
- **Edge contract**: absent `.agentmod` → ("", nil) — a fresh-machine
  restore has nothing to back up and that is not an error. An existing
  entry at the backup name → refusal naming it, NOTHING moved (D034
  collision discipline; same-second restores are the only realistic
  cause). Type-agnostic: a stray regular FILE at `.agentmod` is backed up
  as-is — losing user data is never acceptable, judging the entry is
  doctor's job (init errors on it, backup must not).
- **Discovery/list safety** (verified, not just asserted): Discover
  matches the `.agentmod/agentmod.toml` path exactly, so
  `.agentmod.backup-*` can never be discovered as a project root;
  handoff list reads `.agentmod/snapshots/` only.
- **Gitignore decision (settled now — extraction slice implements, do not
  re-litigate)**: D014's entry covers `.agentmod/` only, so a backup dir
  is untracked → shows in git status AND trips the D039 dirty gate on the
  next `handoff create`. When (and only when) restore actually created a
  backup, the restore cli must extend `.gitignore` with
  `.agentmod.backup-*/` via the ensureGitignore machinery generalized to
  take an entry, and print the backup path. init stays untouched — no
  permanent pattern for an artifact that usually never exists. The
  post-restore-notices slice should also tell the user to delete the
  backup once the restore is verified.

## D043 — 2026-06-11 — Restore extraction: (*Snapshot).Restore + `handoff restore` cli (Phase 6 slice 3)

`internal/handoff/restore.go` (`(*Snapshot).Restore(projectRoot, plan,
now) (*RestoreResult, error)` + extractPlan/writeFileMember) and
`runHandoffRestore` in internal/cli/handoff.go. The library half was
written by a prior iteration that stopped before tests/commit; this
iteration verified it sound, tested it, and wired the cli (T06
precedent). Read D034+D040+D041+D042+this before touching restore code.

- **cli pipeline + exit codes**: restore REQUIRES a project (RESTORE.md's
  packed step order is install → init → restore → doctor; ErrNotFound →
  exit 2). Then: unstat-able path → exit 1 (typo ≠ validation verdict,
  D040); Open failure / Verify problems / PlanRestore problems → exit 3,
  every problem listed — all BEFORE anything on disk moves (a refused
  restore provably creates no backup; tested from the cli). Extraction
  failure after that → exit 1 with the rollback statement.
- **Extraction order** (D041): MkdirAll Dirs at 0700 first (restrictive so
  every later write succeeds), then Files, then Links LAST; recorded dir
  modes are applied deepest-first AFTER content. Files = O_CREATE|O_EXCL
  + explicit Chmod (umask cannot strip recorded exec bits; O_EXCL means
  any collision in the fresh tree = hostile archive → fail, never
  overwrite — proven via a duplicate-plan-entry test). Members are read
  through the zip handle Open established, so a snapshot living inside
  the very `.agentmod/snapshots/` being renamed away restores fine
  (POSIX rename keeps open fds; tested).
- **Rollback is automatic** (D042 settled "rename back"): on any
  extraction failure RemoveAll the partial `.agentmod`, rename the backup
  back, return one error naming cause + "rolled back"; if the rollback
  itself fails the error names BOTH the partial tree and the backup so
  nothing is silently lost. Tree-digest tests prove byte-exact rollback.
- **Post-extract**: layout.Subdirs() recreated (snapshots/ never travels
  — structurally excluded at create; doctor finds a complete tree,
  smoke-verified "all 6 directories present").
- **Gitignore (D042 implemented)**: `ensureGitignore` generalized to
  `(dir, entry)` (covers-check derives the 4 spellings from the entry;
  init passes gitignoreEntry — zero behavior change, no test churn). New
  `gitignoreBackupEntry = handoff.BackupPrefix + "*/"`, ensured ONLY when
  a backup was actually created. A gitignore failure after a successful
  restore is a stderr WARNING, exit stays 0 — the restore itself
  succeeded; the next create's dirty gate surfaces the stray dir.
- **unpack stays a stub** deliberately: TASKS assigns the alias to the
  post-restore-notices slice; the stub message ("alias of 'agentmod
  handoff restore'") remains true and points at the real command.
- **Docs honesty**: HANDOFF.md's and RESTORE.md's "does not implement
  restore yet" notes removed (renderers + docs_test anchors); create's
  closing stdout line now points at verify/restore.
- **Known wrinkle (not this slice's bug)**: init-before-`git init`
  projects have no `.agentmod/` gitignore entry (D014 skip); a later
  in-repo restore then creates `.gitignore` containing only the backup
  pattern. Re-running init fixes it (idempotent, D014's documented
  remedy). The post-restore-notices slice may mention it.
