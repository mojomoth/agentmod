# agentmod — Implementation Plan

Spec: `.harness/v0/FABLE_PLAN.md` (authoritative). Process: `.harness/v0/`
Ralph Loop harness. This document is the architecture; `DECISIONS.md` logs
changes to it.

## 1. Restated requirements (condensed)

A single Go binary, `agentmod`, with two roles:

1. **Router** — when (and only when) `.agentmod/agentmod.toml` exists in cwd
   or an ancestor, shell hooks (zsh/bash, direnv-style, self-contained) route
   `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `OPENCODE_CONFIG` (+ opt-in XDG), and
   Node-family cache/prefix into `.agentmod/`. Leaving the project restores
   the previous environment completely. No shims, no HOME change, no wrapper
   commands, no global config edits — ever.
2. **Handoff** — `.amod` snapshots (zip: manifest, inventory, sha256
   checksums, redaction report, HANDOFF/RESTORE docs, payload) of the agent
   environment; secrets/auth/source excluded by default; safe validated
   restore (backup first, zip-slip-proof, writes only under `.agentmod/`);
   Git-safe variant under `.agentmod-handoff/` additionally excluding
   sessions/logs (session inclusion requires encryption — fails in MVP with
   explanation).

Plus: idempotent `init` (fenced rc block, `--no-shell-hook`, non-interactive),
`doctor` (full diagnostic + honest-limitation warnings), `status`,
`guard claude-bash` (PreToolUse Bash guard for global-home writes),
`install gstack` (project-local only), `pack`/`unpack` aliases, open-source
docs, and tests for all of it that run without real agent installs.

## 2. Feasibility verification

Verified facts (FABLE_PLAN §3; versions: claude 2.1.170, codex 0.137.0,
opencode 1.4.3, go 1.26.2 on this machine):

- `CLAUDE_CONFIG_DIR` and `CODEX_HOME` relocate the full user-level homes →
  Claude/Codex routing is **feasible by env var alone**. Auth caveats:
  macOS Claude auth = shared Keychain (isolation impossible, none needed);
  Linux/Windows Claude + all-OS Codex need per-project login or
  copy-on-consent.
- OpenCode has no single home var → **partial isolation** (`OPENCODE_CONFIG`
  + project `.opencode/`) by default; **full XDG routing opt-in**. Sessions
  not isolated in default mode — doctor warns, README states.
- Claude PreToolUse hook contract is known (stdin JSON; exit 2 or deny JSON)
  → guard is feasible and testable with piped fixtures.
- gstack hardcodes `~/.claude/skills` → must be cloned by us directly into
  the routed home; never run its setup against the global home.
- A child process cannot mutate the parent shell env → activation must be a
  sourced shell hook; the first `init` cannot take effect in the current
  terminal. Stated honestly by init/doctor.

Conclusion: every required feature is implementable with the chosen
mechanisms; no open feasibility questions remain. Unverified details (exact
session file layouts, MCP nuances, gstack setup internals) are deferred to
runtime observation per FABLE_PLAN §31.

## 3. Architecture

Language: Go (single static binary, three OSes, stdlib zip/sha/paths).
Dependencies: `github.com/BurntSushi/toml` only. CLI: stdlib `flag` + small
dispatcher (no cobra). Layout:

```
main.go                     thin entry → cmd dispatch
internal/cli/               dispatcher, flag parsing, help, exit codes
internal/project/           upward discovery of .agentmod/agentmod.toml
internal/config/            agentmod.toml schema, defaults, validation
internal/shellhook/         zsh/bash hook script generation; `env` transitions
internal/routing/           env-var computation (claude/codex/opencode/node)
internal/doctor/            diagnostics engine: checks → findings (warn/err)
internal/initcmd/           init orchestration; rc fenced-block editor;
                            .gitignore editor; auth copy-on-consent
internal/guard/             PreToolUse stdin parsing + decision engine
internal/installer/         gstack project-local installer
internal/handoff/           .amod create/inspect/verify/list/restore;
                            manifest/inventory/checksums/redaction
internal/gitstate/          git metadata collection, URL sanitization
internal/redact/            secret-candidate scanning, exclusion lists
internal/portability/       path normalization, exec bits, symlink policy
internal/testutil/          fixture trees, mock agent binaries, scripted shells
```

Each package is independently testable; doctor/init/handoff consume the
others through narrow interfaces (e.g. routing exposes `EnvFor(project) map`,
doctor consumes it).

Exit codes: 0 ok · 1 generic error · 2 not-in-a-project (where that is an
error) · 3 validation/verification failure (doctor failures, snapshot verify).

## 4. Project directory layout (created by init)

```
.agentmod/
  agentmod.toml          config (schema below)
  claude/                CLAUDE_CONFIG_DIR  (settings.json carries the guard hook)
  codex/                 CODEX_HOME
  opencode/
    opencode.json        OPENCODE_CONFIG target
    xdg/                 XDG_CONFIG_HOME/XDG_DATA_HOME roots (opt-in mode only)
  node/                  npm/pnpm/bun cache+prefix; node/bin goes on PATH
  snapshots/             .amod files (default output dir)
  logs/
.agentmod-handoff/       git-safe handoff output (committed deliberately)
```

`.agentmod/` is gitignored by init; existing directories are never deleted or
overwritten.

## 5. CLI command design

```
agentmod init [--no-shell-hook] [--yes] [--force-rc-update]
agentmod status
agentmod doctor [--json]
agentmod hook zsh|bash            # prints hook; rc evals it
agentmod env --shell zsh|bash     # prints export/unset lines for transitions (internal-ish)
agentmod guard claude-bash        # PreToolUse entrypoint (stdin JSON → exit 0/2 or deny JSON)
agentmod install gstack [--force]
agentmod handoff create  [--output PATH] [--for-git] [--include-sessions] [--include-patch] [--allow-dirty]
agentmod handoff restore  FILE [--dry-run]
agentmod handoff inspect  FILE
agentmod handoff verify   FILE
agentmod handoff list
agentmod pack   [= handoff create]   agentmod unpack [= handoff restore]
agentmod --version
```

## 6. agentmod.toml schema (v1)

```toml
schema_version = 1
mode = "standard"

[isolation]
change_home = false            # hard-false; any other value is a config error
block_global_writes = true

[claude]
enabled = true                 # route CLAUDE_CONFIG_DIR
bash_guard = true

[codex]
enabled = true                 # route CODEX_HOME

[opencode]
enabled = true                 # route OPENCODE_CONFIG + project .opencode/
xdg_full_isolation = false     # opt-in: route XDG_CONFIG_HOME/XDG_DATA_HOME/XDG_CACHE_HOME/XDG_STATE_HOME

[node]
enabled = true                 # npm/pnpm/bun cache+prefix routing + PATH

[gstack]
auto_doctor_check = true

[snapshot]
exclude_source = true          # hard defaults; create-time flags may add,
exclude_secrets = true         # never remove, protected entries
[handoff.git]
include_sessions = false       # true requires encryption → MVP: error
include_logs = false
```

Validation enforces the mandatory defaults of FABLE_PLAN §13: `change_home`
must be false; guard/exclusion defaults on; XDG opt-in off unless set.

## 7. Shell hook strategy

`agentmod hook zsh` prints a self-contained function set; the rc block is:

```
# >>> agentmod >>>
eval "$(agentmod hook zsh)"
# <<< agentmod <<<
```

- Per-prompt: a pure-shell upward search for `.agentmod/agentmod.toml`
  (no binary exec on the hot path). zsh: `chpwd` + `precmd` (first prompt of
  a new terminal inside a project activates); bash: `PROMPT_COMMAND`.
- On transition (enter project / leave / switch nearest project): eval
  `agentmod env --shell <sh> [--deactivate|--activate ROOT]` output.
- Activation: save pre-existing values of every var we set into
  `AGENTMOD_SAVED_<VAR>`; set `AGENTMOD_ACTIVE=1`, `AGENTMOD_PROJECT_ROOT`,
  `AGENTMOD_ROOT`, routed vars; prepend `.agentmod/node/bin` to PATH once
  (dedup guard).
- Deactivation: restore saved values (or unset when none), strip our PATH
  entry, unset all `AGENTMOD_*`. Leaving must be a perfect inverse —
  tested by scripted shell sessions comparing `env` before/after.
- Never touches HOME. If the binary is missing the hook prints one warning
  and no-ops.

## 8. Routing strategy (per agent)

- **Claude**: `CLAUDE_CONFIG_DIR=$AGENTMOD_ROOT/claude`. Project `.claude/`
  remains natively read by Claude — documented, not fought. Guard hook lives
  in `$AGENTMOD_ROOT/claude/settings.json` (active exactly when routing is).
- **Codex**: `CODEX_HOME=$AGENTMOD_ROOT/codex`.
- **OpenCode (default/partial)**: `OPENCODE_CONFIG=$AGENTMOD_ROOT/opencode/opencode.json`;
  plugins/commands/agents via project `.opencode/`. Sessions/auth stay
  global → doctor warns. Global config merge chain → doctor warns when the
  global file defines plugins/MCP that will leak into the project view.
- **OpenCode (opt-in/XDG)**: also set `XDG_CONFIG_HOME=$AGENTMOD_ROOT/opencode/xdg/config`,
  `XDG_DATA_HOME=…/data`, `XDG_CACHE_HOME`, `XDG_STATE_HOME`. README +
  doctor: affects ALL XDG-aware tools inside the project.
- **Node family**: `NPM_CONFIG_CACHE`, `NPM_CONFIG_PREFIX`, `PNPM_HOME`,
  `BUN_INSTALL` under `.agentmod/node/`; `.agentmod/node/bin` on PATH.
  (Exact var set verified against installed tools during Phase 2.)

## 9. Auth bootstrapping (copy-on-consent)

Detection in init and doctor: local home lacks auth file AND global one
exists → interactive: explicit per-tool consent prompt; copy `auth.json`
(Codex) / `.credentials.json` (Claude, Linux/Windows only) with 0600 perms.
Decline or `--yes`/non-interactive: print exact re-login commands instead
(`codex login`, `claude` login flow). macOS Claude: print "Keychain is
shared; nothing to do; isolation impossible" note. Copied auth files are on
the hardcoded snapshot/handoff exclusion list. No other global file is ever
copied.

## 10. gstack isolation strategy

`agentmod install gstack`: require active project (else exit 2); require git;
`git clone https://github.com/garrytan/gstack` →
`$AGENTMOD_ROOT/claude/skills/gstack` (temp dir + atomic rename). Never run
its setup script. Record global `~/.claude/skills` listing before/after; any
delta → report VIOLATION and instruct cleanup (clone into a temp dir can't
write there, but verify anyway — defense in depth). Already installed →
abort unless `--force` (which replaces only the project-local copy). Distinct
errors: not-a-project / git-missing / network / target-exists. Doctor shows
install state and warns if a global `~/.claude/skills/gstack` exists.

## 11. Claude Bash guard strategy (product)

`agentmod guard claude-bash`: reads PreToolUse JSON from stdin (contract
§3.1). Deny (exit 2 with reason on stderr; `--json` mode emits the
`permissionDecision: "deny"` form) when a Bash command has high write
likelihood against global agent homes: write commands (`cp mv rm mkdir touch
tee ln rsync install chmod …`), `git clone`, or output redirection targeting
`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`
(in `~`, `$HOME`, or absolute-home spellings) — plus `sudo` and `HOME=`
reassignment. Reads (`ls cat grep find …`) pass. Unparseable input →
fail-safe: deny only if raw input references a global path; never block
everything. Wired by init into `.agentmod/claude/settings.json` PreToolUse,
referencing the absolute agentmod binary path (re-resolved by doctor if the
binary moved). Engine is a pure function `Decide(input []byte) Decision` —
table-driven tests.

## 12. Handoff / snapshot strategy

`.amod` = zip. Internal layout:

```
manifest.json      schema_version, created_at, tool versions, platform,
                   git metadata, policy flags used
inventory.json     every payload file: path, size, sha256, mode, symlink target
checksums.txt      sha256 of manifest/inventory/payload members
REDACTION.md       what was excluded and why; secret-candidate scan results
HANDOFF.md         human-readable: what this is, how to restore, what's missing
RESTORE.md         step-by-step restore + re-login guidance
payload/           .agentmod subset + project .claude/.opencode dirs + MCP cfg
```

Create pipeline: collect → filter (exclusion engine) → secret-scan remaining
files (warn + list in REDACTION.md; refuse on hard hits like private keys
unless `--allow-findings`) → write zip + checksums. Default exclusions:
source code (everything outside the agent-env dirs), `.git`, `node_modules`,
caches, tmp, logs(only for --for-git), auth files (incl. consent-copied),
`.env*`, ssh/cloud credential patterns. Sessions included in normal handoffs,
excluded in `--for-git`.

Restore pipeline: open → verify schema version + checksums → validate every
entry (relative, cleaned, no `..` escape, symlink targets inside payload,
no absolute paths) → backup existing `.agentmod` to
`.agentmod.backup-<timestamp>` → extract only under `.agentmod/` (plus
explicitly whitelisted project-relative agent dirs) → never execute anything →
rewrite/warn MCP absolute paths → print re-login notices → run doctor.

## 13. Git Handoff strategy

`--for-git` writes an uncompressed-tree variant (or .amod file — decided in
Phase 7 for reviewability; leaning: tree of files for diff-ability) under
`.agentmod-handoff/`, additionally excluding sessions/logs and anything
secret-scanned. `--for-git --include-sessions` → hard error explaining the
encryption requirement (MVP ships no encryption). Git metadata recorded with
token-redacted remote URLs; dirty worktree → warn, require `--allow-dirty`.

## 14. Portability strategy

Zip paths always forward-slash relative; `filepath.ToSlash`/`FromSlash` at
the boundaries. Exec bits recorded in inventory and restored on unix; noted
in RESTORE.md for Windows. Symlinks stored as link entries with relative
targets, validated on restore, skipped-with-warning when the platform can't
create them. Absolute paths in MCP configs detected → rewritten when they
point inside the project, warned otherwise. PowerShell hook deferred past
MVP; restore format is OS-neutral so Windows restore works from day one.

## 15. Test strategy

`go test ./...`, zero real agent installs:

- **Unit**: table-driven per package (guard decisions, exclusion engine,
  rc-block editor as pure text transforms, path validation incl. malicious
  fixtures, TOML defaults).
- **Scripted-shell integration**: run real zsh/bash with a temp HOME-like
  fixture rc, cd around fixture project trees, assert env transitions, PATH
  dedup, perfect deactivation. Skipped gracefully when the shell is absent.
- **Mock binaries**: fake `claude`/`codex`/`opencode`/`git` scripts on PATH
  recording their env/args to a log file — scenario tests (§27) assert the
  fakes saw routed homes inside projects and globals outside.
- **Snapshot security**: hand-built malicious zips (traversal `../`, absolute
  paths, symlink escape) must all be rejected.
- Completion criteria per feature: `TEST_MATRIX.md` (T01–T30).

## 16. Implementation order

Phases 1–8 as in `.harness/v0/PLAN.md`: skeleton+discovery+config → init+
hooks+routing → doctor+guard+auth → gstack → handoff create → restore+
portability → git handoff → docs+scenarios. Rationale: each phase's tests
become fixtures for the next; routing must exist before doctor can diagnose
it; create before restore; everything before docs claim it.

## 17. Risks

Tracked live in `.harness/v0/RISKS.md` (R1 global pollution, R2 rc
corruption, R3 env leakage, R4 secrets, R5 restore attacks, R6 OpenCode
expectations, R7 macOS Keychain, R8 first-session UX, R9 Windows, R10 tool
drift, R11 loop burn).

## 18. Completion criteria

FABLE_PLAN §29 in full, §28 prohibitions all clear, `TEST_MATRIX.md` all ✅,
docs complete with honest limitations. Verdict recorded in
`.harness/v0/DONE.md`; `loop.sh` re-verifies tests before accepting it.
