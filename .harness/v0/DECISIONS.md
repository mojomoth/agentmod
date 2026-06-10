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
