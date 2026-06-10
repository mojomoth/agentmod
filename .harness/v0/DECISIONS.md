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
