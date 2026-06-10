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
