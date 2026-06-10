# AgentMod Ralph Loop Harness — Development Meta-Prompt (FABLE_PLAN)

You are a senior systems CLI engineer and agentic-development-harness designer working in Claude Code.

You will build `agentmod`, an open-source CLI utility.

This is not a one-shot build. First design and generate a **Ralph Loop Harness Scaffold** for developing `agentmod`, then iterate on top of that harness until the goal is complete.

---

# 0. Changes vs GPT_PLAN.md (validation summary)

This plan supersedes `.harness/v0/GPT_PLAN.md`. The original plan was validated against official docs and source code (June 2026) and found sound. The following corrections are integrated:

1. **Verified Facts section added (§3)** — `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, OpenCode config resolution, per-OS credential locations, the Claude Code PreToolUse hook contract, and gstack's install path are now verified facts with sources, not open questions. Do not re-derive them; do sanity-check them at runtime where cheap.
2. **OpenCode has no single home env var** — strategy decided: partial isolation by default (`OPENCODE_CONFIG` + project `.opencode/` dirs), full XDG routing as an opt-in toml flag (§15.3).
3. **Auth bootstrapping is a first-class flow** — fresh local homes have empty credentials; init/doctor implement copy-on-consent (§12).
4. **Claude Code always reads project `.claude/` regardless of `CLAUDE_CONFIG_DIR`** — the Bash guard hook must live in the routed local home's settings, not in project `.claude/settings.json` (§17).
5. **The Ralph loop has explicit stop conditions** — `loop.sh` with max iterations and a DONE sentinel; no unbounded `while :` (§7).
6. **`agentmod init` gains `--no-shell-hook` and non-interactive flags** for CI and tests (§12).
7. **Harness lives under `.harness/v0/`**, matching the existing repo convention.
8. **gstack install clones directly into the project-local skills dir** and never runs its setup against the global home (§16).
9. **The repo is not yet a git repository** — `git init` early; init must handle missing `.gitignore`/repo gracefully.

---

# 1. Product Definition

`agentmod` is a CLI utility that isolates the configuration, skills, plugins, sessions, caches, MCP settings, and working context of coding agents (Claude Code, Codex CLI, OpenCode) on a per-project basis, and can pack that environment into a snapshot for handoff to another machine.

`agentmod` plays two roles:

## 1.1 Agent Home Router

Routes the config homes and plugin/skill/session/cache paths of Claude Code, Codex CLI, and OpenCode into the project directory.

- If `.agentmod/agentmod.toml` exists in the current directory or any ancestor, the project-local agent environment is activated.
- If it does not exist, nothing is injected; the user's existing global Claude / Codex / OpenCode setup is used untouched.

## 1.2 Agent Environment Handoff Tool

Packs the per-project agent environment into a snapshot and restores it on another machine.

Source code moves via Git. `agentmod` does not pack source code; it hands off the agent environment and working context.

---

# 2. Inviolable Principles

These must never be broken:

1. `agentmod` is not a Docker-based sandbox.
2. `agentmod` does not intercept the `claude`, `codex`, or `opencode` commands via shims.
3. `agentmod` does not change `HOME` in its default mode.
4. Inside an AgentMod project, users keep using the plain `claude`, `codex`, `opencode` commands.
5. There is no `agentmod run claude` style wrapper command.
6. There is no separate `agentmod setup-shell` command required of the user.
7. `agentmod init` alone handles project initialization and shell auto-env hook installation.
8. agentmod activates only in projects containing `.agentmod/agentmod.toml`.
9. Leaving the project must unset all agentmod environment variables.
10. The user's global Claude / Codex / OpenCode settings are never modified.
11. Config, skills, plugins, sessions, and caches must not leak between projects.
12. Tools like `gstack` that install into `~/.claude/skills` must be confined to the project.
13. Handoff does not include full source code by default.
14. Handoff does not include secrets, auth, tokens, or credentials by default.
15. Git Handoff excludes sessions and logs by default.
16. Including sessions in a Git Handoff requires encryption.
17. Restore must not trust external snapshots; it must validate them safely.
18. Never declare completion before tests pass.

---

# 3. Verified Facts (do not re-litigate; sanity-check cheaply at runtime)

These were verified against official docs and source in June 2026. They are the load-bearing facts of the routing design.

## 3.1 Claude Code

- **`CLAUDE_CONFIG_DIR` redirects the entire user-level config home** (default `~/.claude`): settings.json, skills/, agents/, plugins/, hooks/, sessions/transcripts, history, caches, logs. Source: https://code.claude.com/docs/en/claude-directory
- **Credentials**: macOS stores OAuth creds in the **Keychain** (service shared across all config dirs — per-project auth isolation is *impossible* on macOS, and no re-login is needed per project). Linux/Windows store `.credentials.json` inside the config dir — a fresh project-local home means **re-login or copy** on Linux/Windows. Source: https://code.claude.com/docs/en/authentication, anthropics/claude-code#20553
- **Project-level `.claude/` is always read regardless of `CLAUDE_CONFIG_DIR`** (`.claude/settings.json`, `.claude/skills`, etc.). Consequence: agentmod's added value for Claude is isolating *user-level* state (global plugins/skills, sessions, history); project `.claude/` already works natively. State this honestly in the README.
- **PreToolUse hook contract**: stdin JSON with `session_id`, `transcript_path`, `cwd`, `permission_mode`, `hook_event_name: "PreToolUse"`, `tool_name` (e.g. `"Bash"`), `tool_input` (e.g. `{"command": "..."}`). Block by **exit code 2** (stderr fed back to Claude) or exit 0 with `{"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": "..."}}`. Source: https://code.claude.com/docs/en/hooks

## 3.2 Codex CLI

- **`CODEX_HOME` redirects the entire home** (default `~/.codex`): `config.toml`, `auth.json`, `sessions/`, `history.jsonl`, `log/`, `skills/`. Verified in openai/codex source (`scripts/install/install.sh`, `codex-rs/login/src/auth/storage.rs`).
- **Auth moves with it**: `auth.json` is resolved strictly relative to `CODEX_HOME`. A fresh project-local home requires `codex login` or copying `auth.json`.

## 3.3 OpenCode

- **No single env var routes all state.** Config resolution is a merge chain: global `~/.config/opencode/opencode.json` → file at `OPENCODE_CONFIG` → project `opencode.json` → project `.opencode/` dirs (agents/commands/plugins). Sessions, storage, and `auth.json` live in XDG data dirs (`~/.local/share/opencode`). Sources: https://opencode.ai/docs/config/, opencode `packages/core/src/global.ts`
- Full isolation is only possible via the generic `XDG_CONFIG_HOME`/`XDG_DATA_HOME` vars, which also affect *other* XDG-aware tools run inside the project.

## 3.4 gstack

- `github.com/garrytan/gstack` hardcodes installation to `~/.claude/skills/gstack` (git clone + symlinks). It is not `CLAUDE_CONFIG_DIR`-aware. Its `setup` script can also target other hosts (e.g. `~/.codex/skills/`).

Anything *not* listed here (e.g. exact session file layouts, MCP config nuances, gstack setup internals) remains "verify before building on it" per §30.

---

# 4. Language and Architecture

`agentmod` is written in **Go**, because it gives single-binary distribution, easy macOS/Linux/Windows support, strong stdlib coverage for filesystem/paths/zip/checksums/shell-hook generation, and no Node/Python runtime dependency.

If Go is unsuitable for a specific feature, record the reason in `DECISIONS.md` and propose an alternative. Do not switch languages unilaterally.

Recommended layer responsibilities (separable and testable; exact packages/function names are the agent's design):

- CLI layer
- Project discovery layer
- Config / manifest layer
- Shell hook layer
- Environment routing layer
- Doctor / status diagnostics layer
- Installer layer
- Guardrail layer
- Handoff / snapshot layer
- Git state layer
- Redaction / security layer
- Portability layer
- Test harness layer

---

# 5. First Steps (before implementing AgentMod proper)

Do not start the main implementation immediately. First:

1. `git init` the repository (it is not yet a git repo) and inspect current state.
2. Review the requirements in this document.
3. Hunt for contradictions, risks, and uncertainties.
4. Write `IMPLEMENTATION_PLAN.md`.
5. Generate the Ralph Loop Harness scaffold (§6).
6. Write the harness documents.
7. Document the Claude Code hook guardrail design (using the verified contract in §3.1).
8. Plan project-local-only activation of required skills.
9. Build the test matrix.
10. Write a `PROMPT.md` that the Ralph Loop can re-run.
11. Then begin the AgentMod implementation.

`IMPLEMENTATION_PLAN.md` must include:

- Restated requirements
- Feasibility verification
- Architecture decisions
- Risks
- CLI command design
- Project directory layout
- Shell hook strategy
- Claude / Codex / OpenCode routing strategy (per §3 and §15)
- Auth bootstrapping strategy (per §12)
- gstack isolation strategy
- Claude Code guard hook strategy
- Handoff / snapshot strategy
- Git Handoff strategy
- Windows / macOS / Linux portability strategy
- Test strategy
- Implementation order
- Completion criteria

---

# 6. Ralph Loop Harness Scaffold

Before the main implementation, create this harness structure under `.harness/v0/` (matching the existing repo convention — note: *not* at `.harness/` root):

```txt
.harness/v0/
  GPT_PLAN.md      (existing — superseded original)
  FABLE_PLAN.md    (this file)
  GOAL.md
  PROMPT.md
  STATE.md
  PLAN.md
  TASKS.md
  DECISIONS.md
  RISKS.md
  CHECKS.md
  TEST_MATRIX.md
  LOOP.md
  DONE.md
  loop.sh
  hooks/
  reports/
  skills/
```

File responsibilities:

- **GOAL.md** — AgentMod's final goal and completion conditions.
- **PROMPT.md** — the current iteration prompt fed into the Ralph Loop. Must let a fresh session continue even after a disconnect.
- **STATE.md** — current implementation state, failing tests, remaining work, cautions.
- **PLAN.md** — overall development plan and per-phase goals.
- **TASKS.md** — implementation work broken into a small-unit checklist.
- **DECISIONS.md** — significant design decisions and reasons for changes.
- **RISKS.md** — security, path portability, global pollution, handoff, secrets, and restore risks.
- **CHECKS.md** — verifications that must run every iteration.
- **TEST_MATRIX.md** — per-feature test scope and completion criteria.
- **LOOP.md** — Ralph Loop operating rules.
- **DONE.md** — final completion verdict and result summary. Also the loop's stop sentinel.
- **loop.sh** — the loop runner (§7).
- **reports/** — per-iteration logs.

---

# 7. Ralph Loop Method

Conceptually:

```txt
while not done; do
  feed .harness/v0/PROMPT.md to claude
done
```

The actual runner is `.harness/v0/loop.sh`, which must:

- Invoke `claude -p "$(cat .harness/v0/PROMPT.md)"` (print mode), with permission settings appropriate to the environment (verify and adjust the exact invocation against the installed Claude Code version; record in `DECISIONS.md`).
- Enforce a **max-iteration cap** (default e.g. 25, overridable via env var).
- **Stop when `.harness/v0/DONE.md` contains a completion declaration** (a well-defined sentinel line, e.g. `STATUS: DONE`).
- Write each iteration's output to `.harness/v0/reports/iter-NNN.log`.
- Never loop unboundedly; an unbounded loop burns tokens with no exit.

What matters is not the loop itself but these principles:

- Each iteration can behave as an independent session.
- The agent does not rely on memory of previous conversations.
- All state is recorded in files.
- Each iteration picks the single smallest, most important next task.
- Each iteration runs tests or verification.
- On failure, record the cause and hand it to the next iteration.
- Never declare completion unless the completion conditions are met.
- When they are met, write `DONE.md` and the final report.

Default per-iteration order:

1. Read `GOAL.md`
2. Read `STATE.md`
3. Read `CHECKS.md`
4. Read `TEST_MATRIX.md`
5. Check current failures
6. Pick the most important next task
7. Modify only the files needed
8. Run tests
9. Record results
10. Update `STATE.md`
11. Update `PROMPT.md`
12. Judge completion

---

# 8. Claude Code Hook Guardrail (for the harness itself)

The harness must use Claude Code hooks as guardrails, managed as project-local settings. Do not pollute global Claude settings.

The hook contract is **known** (§3.1): PreToolUse stdin JSON; block via exit code 2 or `permissionDecision: "deny"`. Build directly on it; keep a cheap runtime sanity check, and if input ever fails to parse, fail safe — deny only writes targeting global agent paths, never block everything.

Guardrail purposes:

- Block dangerous commands
- Prevent global config pollution
- Prevent `HOME` changes
- Prevent shim creation
- Prevent secrets leakage
- Prevent snapshot/restore security violations
- Prevent skipped test runs
- Prevent completion declarations while completion conditions are unmet

Guardrails must detect at minimum:

- Attempts to change `HOME`
- Attempts to create shims
- Attempts to write directly to global `~/.claude`, `~/.codex`, `~/.config/opencode`
- Attempts to use `sudo`
- Attempts to modify `.ssh`, `.aws`, `.docker`, `.git` directly
- Attempts to include secrets in a snapshot or Git Handoff
- Attempts to include full source code in a handoff package
- zip-slip-vulnerable restore logic
- Attempts to mark work complete without running tests

---

# 9. Skills

Activate these skill repositories **project-locally only**:

- https://github.com/mattpocock/skills — *already installed at `.claude/skills/` in this repo; verify rather than reinstall*
- https://github.com/multica-ai/andrej-karpathy-skills

Principles:

- No global installation.
- Activate only in the project-local Claude Code area.
- Activate only the skills actually needed.
- Skill installation/activation is managed by the harness.
- Record install results and rationale in `.harness/v0/skills/README.md` and `DECISIONS.md`.
- If another skill is needed, record the reason first, then install project-locally only.

Mandatory thinking principles:

- Think Before Coding
- Simplicity First
- Surgical Changes
- Goal-Driven Execution
- Test-based completion verdicts
- No unnecessary abstraction
- Define failure conditions before implementing
- Keep change units small

---

# 10. Product Structure and Base Files

Product name: `agentmod`. CLI command: `agentmod`.

- Project-local directory: `.agentmod/`
- Config file: `.agentmod/agentmod.toml`
- Git-storable handoff directory: `.agentmod-handoff/`

After `agentmod init`, `.agentmod/` must cover at least these responsibility areas (exact layout is the agent's design):

- agentmod configuration
- Claude Code local home
- Codex CLI local home
- OpenCode local config area
- npm / pnpm / bun local cache / prefix
- snapshot storage area
- hooks / sessions / logs / skills / plugins / commands / agents areas

Requirements on the layout:

- Claude config/skills/plugins/sessions live inside the project.
- Codex config/skills/sessions live inside the project.
- OpenCode config/plugins/commands/modes live inside the project (sessions: see §15.3).
- `.agentmod/` is not committed to Git by default.
- `.agentmod-handoff/` is usable as the Git-safe package area.

---

# 11. Required CLI Commands

## Core
- `agentmod init`
- `agentmod doctor`
- `agentmod status`

## Shell / Guard
- `agentmod hook zsh` (prints the hook script; rc files `eval` it)
- `agentmod hook bash`
- `agentmod guard claude-bash` (the PreToolUse guard executable entrypoint)

## Installer
- `agentmod install gstack`

## Handoff
- `agentmod handoff create`
- `agentmod handoff restore`
- `agentmod handoff inspect`
- `agentmod handoff verify`
- `agentmod handoff list`

## Alias
- `agentmod pack`
- `agentmod unpack`

Optional commands may be proposed but must not blur MVP scope.

---

# 12. `agentmod init` Requirements

`agentmod init` must:

- Create `.agentmod/`
- Create `.agentmod/agentmod.toml`
- Create Claude / Codex / OpenCode project-local homes
- Create Node-family local prefix/cache directories
- Create the snapshot directory
- Add `.agentmod/` to `.gitignore` (create `.gitignore` if missing; behave gracefully when the directory is not a git repo)
- Check whether the shell auto-env hook is installed
- If missing, add the agentmod block to the shell rc file
- Diagnose whether the hook is active in the current shell
- If it cannot take effect in the current terminal session, say so precisely
- Print a doctor-level diagnostic summary

**Flags (required for CI and the test harness):**

- `--no-shell-hook` — skip all rc-file modification.
- A non-interactive mode (e.g. `--yes` / `--non-interactive`) that never prompts and never copies auth.

**rc-file edits** use a fenced, idempotent block:

```
# >>> agentmod >>>
eval "$(agentmod hook zsh)"
# <<< agentmod <<<
```

Only this block is ever added or updated; user-authored shell config is never touched.

**Auth bootstrapping (copy-on-consent — decided policy):**

Fresh local homes have empty credentials. Concretely (per §3): Codex requires login per project; Claude requires login per project on Linux/Windows only; on macOS, Claude auth lives in the shared Keychain (no action needed, and no isolation possible — document this limitation).

- When init (or doctor) detects an empty local home for a tool whose global auth exists, it **explicitly asks** before copying the global auth file (`auth.json` for Codex; `.credentials.json` for Claude on Linux/Windows) into the project-local home.
- On decline (or in non-interactive mode), print exact re-login instructions instead.
- Copied auth files are **always** on the default handoff/snapshot exclusion list (§18).
- Never copy any other global config without explicit consent.

`agentmod init` must be idempotent. These failures are unacceptable:

- Duplicate rc hook insertion
- Duplicate `.gitignore` entries
- Careless overwriting of existing config
- Deleting existing directories
- Modifying the user's global settings

A normal CLI process cannot change its parent shell's environment. The first `agentmod init` may therefore not take effect in the current terminal session. Do not hide this limitation; state it clearly.

---

# 13. `agentmod.toml` Requirements

`.agentmod/agentmod.toml` must be able to express:

- schema version
- mode
- isolation policy
- Claude Code routing settings
- Codex CLI routing settings
- OpenCode routing settings — including the **XDG full-routing opt-in flag** (§15.3)
- Node / npm / pnpm / bun local cache/prefix settings
- gstack installer settings
- snapshot default policy
- handoff default policy
- Git Handoff default policy
- secrets-exclusion policy
- HOME-change prohibition policy
- global-write blocking policy

Mandatory defaults:

- `change_home` is false.
- Global agent-write prevention is enabled.
- The Claude Bash guard is enabled.
- Snapshots exclude source code.
- Snapshots exclude secrets.
- Git Handoff excludes sessions/logs.
- Including sessions in Git Handoff requires encryption.
- OpenCode full-XDG routing is **off** (partial isolation is the default).

Exact TOML schema and field names are the agent's design, expressing the policies above.

---

# 14. Shell Auto-Env Requirements

agentmod uses shell hooks, not shims (the same activation model as direnv, but self-contained — direnv/mise are deliberately *not* dependencies, since `init` alone must be sufficient).

Support priority:

1. zsh
2. bash
3. fish — optional
4. PowerShell — future work, or restore-compatibility-focused

Shell hook responsibilities:

- Search from the current directory upward for `.agentmod/agentmod.toml`
- Activate the nearest agentmod project
- Set required env vars inside an agentmod project
- Unset them outside any agentmod project
- Prevent environment leakage when moving between projects
- Prevent duplicate PATH entries
- Never change HOME
- Also trigger on shell startup (e.g. zsh `precmd`/`chpwd`, bash `PROMPT_COMMAND`), so a new terminal opened inside a project activates correctly — not only after a `cd`

Inside an agentmod project, at least this routing must work:

- Claude Code project-local home (`CLAUDE_CONFIG_DIR`)
- Codex CLI project-local home (`CODEX_HOME`)
- OpenCode project-local config (per §15.3)
- Node / npm / pnpm / bun project-local cache/prefix (and its bin dir on PATH, removed on deactivation)

agentmod's own env vars use uppercase naming, e.g.:

- `AGENTMOD_ACTIVE`
- `AGENTMOD_PROJECT_ROOT`
- `AGENTMOD_ROOT`

Verify and document exact names and behavior during implementation.

---

# 15. Claude / Codex / OpenCode Isolation Requirements

## 15.1 Claude Code

- Inside a project, use `.agentmod/claude` as `CLAUDE_CONFIG_DIR` instead of global `~/.claude`.
- Outside, the existing global Claude settings apply.
- Inside a project, commands writing directly to the global Claude home are blocked by the guardrail.
- gstack installs only to `.agentmod/claude/skills/gstack`.
- Known fact (§3.1): project `.claude/` is read regardless of routing. The README must honestly state that agentmod's Claude isolation covers *user-level* state (global plugins/skills, sessions, history); project `.claude/` is native Claude behavior.
- macOS credential limitation (§3.1): Keychain auth is shared across config dirs — no per-project account isolation on macOS; doctor should state this, not pretend otherwise.

## 15.2 Codex CLI

- Inside a project, use `.agentmod/codex` as `CODEX_HOME`.
- Outside, the existing global Codex settings apply.
- Auth: handled by the copy-on-consent flow (§12); never copy global config without consent.
- Sanity-check the installed Codex version's `CODEX_HOME` behavior at runtime (cheap check; the mechanism itself is verified, §3.2).

## 15.3 OpenCode — partial isolation by default (decided policy)

There is no single OpenCode home variable (§3.3). Therefore:

**Default (partial isolation):**
- Route config via `OPENCODE_CONFIG` pointing into `.agentmod/opencode/`, plus project `.opencode/` dirs for plugins/commands/agents/modes.
- Sessions, storage, and auth remain in the global XDG data dir. `agentmod doctor` must **warn** that OpenCode sessions are not project-isolated in this mode.
- Beware the merge chain: the global `~/.config/opencode/opencode.json` is still merged in. doctor warns when global plugins/config will leak into the project view.

**Opt-in (full isolation):** an `agentmod.toml` flag enables setting `XDG_CONFIG_HOME` / `XDG_DATA_HOME` (and cache/state) to project-local paths while inside the project. Document prominently that this affects **all** XDG-aware tools run inside the project, not just OpenCode. Off by default.

- Outside a project, existing global OpenCode settings apply.
- Verify the installed OpenCode version's config resolution at runtime before relying on details beyond §3.3.

---

# 16. gstack Isolation Requirements

gstack hardcodes `~/.claude/skills/gstack` (§3.4), so agentmod manages it specially:

- Provide `agentmod install gstack`.
- Install gstack **only** inside the current agentmod project, by cloning directly into `.agentmod/claude/skills/gstack`. Never run gstack's own `setup` against the global home; if any setup step must run, run it with the environment routed to the project-local home and verify before/after that no global path changed.
- Never install to global `~/.claude/skills/gstack`.
- Check for global-pollution before and after installation.
- Abort if the setup process attempts to write to the global Claude home.
- The Claude Bash guard must block direct global-Claude-home writes as defense-in-depth.
- gstack installation status must be visible in `doctor`.
- `agentmod install gstack` outside an agentmod project must fail.
- If already installed, abort safely or require an explicit force option.
- Report network failure, missing git, and setup failure clearly.

Exact clone/verification/failure-handling design is the agent's, but global pollution is never acceptable.

---

# 17. Claude Code Guard Hook Requirements (product feature)

Inside an agentmod project, Claude Code must be prevented from running Bash commands that pollute global agent homes.

**Placement (decided):** the guard's PreToolUse hook configuration lives in the **routed project-local home's settings** (i.e. `.agentmod/claude/settings.json`), so it is active exactly when routing is active. Do **not** put it in the project's `.claude/settings.json` — that file is shared with collaborators via git and would impose the guard on non-agentmod users (and behave wrongly outside routing).

Guard targets:

- Global `~/.claude/skills`
- Global `~/.claude/plugins`
- Writes under `$HOME/.claude`
- OS-specific global Claude paths under the user home
- Other global agent-home pollution paths (`~/.codex`, `~/.config/opencode`)

Behaviors to block:

- clone / copy / move / write into the global Claude home
- Creating directories under the global Claude home
- Deleting files under the global Claude home
- Direct global plugin / skill installation

Cautions:

- Do not blanket-block read commands.
- Block only commands with high write likelihood.
- The hook input contract is verified (§3.1) — implement against it; if input is unparseable, fail safe (deny global-path writes, never block everything).
- A failing guard must degrade safely.

---

# 18. Handoff Requirements

agentmod must pack the per-project agent environment into a snapshot and restore it.

## Handoff create

Included:

- agentmod config
- Claude config, skills, plugins, agents, commands, hooks
- Codex config, skills, sessions
- OpenCode config, plugins, agents, commands, modes
- MCP config
- Working context / memory / handoff documents
- Git state metadata
- Sessions may be included in a normal (non-git) handoff

Excluded by default:

- Full source code
- `.git`
- `node_modules`
- Build artifacts
- caches
- tmp
- auth (including any auth files copied in via §12 consent flow)
- credentials
- tokens
- `.env`
- SSH / cloud credentials
- OS credential store

Handoff creation must produce:

- a `.amod` package
- manifest
- inventory
- checksums
- redaction report
- a human-readable HANDOFF document

## Handoff restore

Restore must be safe:

- Snapshot validation
- Checksum verification
- Schema version check
- zip-slip prevention
- No absolute-path restoration
- Backup of the existing `.agentmod` first
- Git remote / branch / HEAD comparison
- Notice of secrets-excluded items (and re-login guidance)
- OS path portability handling
- MCP absolute-path warning or rewriting
- Run doctor after restore
- Never auto-execute arbitrary scripts

## Handoff inspect / verify / list

Users must be able to open, verify, and list handoff packages.

---

# 19. Git Handoff Requirements

agentmod must provide a Git-storable safe handoff mode.

Principles:

- `.agentmod/` is never committed.
- Git-storable output lives in `.agentmod-handoff/`.
- Git Handoff excludes secrets by default.
- Git Handoff excludes sessions/logs by default.
- Git Handoff never includes full source code.
- Git Handoff includes a human-readable HANDOFF document.
- Git Handoff includes manifest and inventory.

Required commands:

- `agentmod handoff create --for-git`
- `agentmod pack --for-git`

Including sessions in a Git Handoff requires encryption. If encryption is not implemented in the MVP:

- `--for-git --include-sessions` must fail.
- The failure message must explain why encryption is required.

---

# 20. Git State Requirements

When creating a handoff, check Git state.

Default policy:

- Warn if the worktree is dirty.
- Source changes are not included by default.
- Proceed on dirty state only with explicit user permission.
- Patch inclusion only via an explicit option.
- Redact tokens in remote URLs.

Git metadata must record at least:

- whether it is a repository
- sanitized remote URL
- branch
- HEAD commit
- dirty flag
- staged / modified / untracked summary
- whether source code was included

---

# 21. Snapshot Format Requirements

Snapshot extension: `.amod`. Internal format: zip-family preferred.

A snapshot must contain:

- manifest
- inventory
- checksums
- redaction report
- HANDOFF document
- RESTORE document
- payload

Critical:

- Never trust external snapshots.
- No path traversal.
- No absolute-path restoration.
- Never write to `.ssh`, `.aws`, `.docker`, `.git`.
- Restore writes only under `.agentmod/` by default.

Exact JSON schemas and file structure are the agent's design; manifest / inventory / checksums / HANDOFF document must exist.

---

# 22. Path Portability Requirements

Handoffs must consider Windows / macOS / Linux moves:

- Normalize snapshot-internal paths portably.
- Account for OS path-separator differences.
- Rewrite or warn on absolute paths at restore.
- Handle symlinks safely.
- Restore executable bits where possible.
- PowerShell support may be deferred past MVP, but the restore format must not break Windows.

---

# 23. `agentmod doctor` Requirements

`agentmod doctor` diagnoses current state:

- Project discovery
- agentmod root
- Shell type
- Shell hook installed?
- Shell hook active?
- Current env var state
- HOME changed?
- Shims present?
- Claude / Codex / OpenCode binaries present
- Claude project-local home state (including **auth present / re-login needed**, per §12)
- Codex project-local home state (including auth state)
- OpenCode project-local config state (including the **partial-isolation session warning**, §15.3)
- Global Claude write guard state
- gstack project-local install state
- gstack global-install risk
- Snapshot directory state
- Recent handoff state
- Git state
- Portability risks
- Secrets risks
- MCP warnings

doctor must warn on at least:

- Inside an agentmod project but required env vars unset
- agentmod env vars lingering in a folder without `.agentmod`
- HOME changed
- Shim detected
- Global `~/.claude/skills/gstack` exists
- Shell hook installed but inactive in the current shell
- Duplicate agentmod PATH entries
- Git Handoff containing unencrypted sessions
- Snapshot containing secret candidates
- Restore-target Git HEAD differing from the snapshot
- OpenCode global config/plugins leaking into the project view (merge chain, §15.3)

---

# 24. `agentmod status` Requirements

`agentmod status` shows whether AgentMod is active, briefly.

Active:

- project root
- agentmod root
- Claude local home
- Codex local home
- OpenCode local config
- recent handoff info

Inactive:

- AgentMod inactive
- `.agentmod/agentmod.toml` not found in current or ancestor directories
- default global agent settings will be used

---

# 25. Security Requirements

Always:

- Never overwrite existing files in the user's home.
- Shell rc edits add/update only the fenced agentmod block.
- Never delete user-authored shell config.
- `.agentmod/` is gitignored by default.
- No HOME changes.
- No shim creation.
- No sudo.
- No global npm/brew config changes.
- No global Claude/Codex/OpenCode config modification.
- No global Claude home pollution during gstack install.
- Snapshots exclude secrets/auth by default (including consent-copied auth files, §12).
- Git Handoff excludes sessions/logs by default.
- zip-slip prevention on restore.
- No arbitrary script auto-execution on restore.
- Back up the existing `.agentmod` before restore.
- Never trust external snapshots.

---

# 26. Test Strategy

Tests are the basis of the completion verdict. The harness defines these categories; the implementation must pass them:

- Project discovery tests
- Env var set/unset tests
- Shell hook tests
- init idempotency tests
- init `--no-shell-hook` / non-interactive tests
- `.gitignore` dedup tests (including no-git-repo case)
- HOME-change prevention tests
- Shim-prevention tests
- Claude local home routing tests
- Codex local home routing tests
- OpenCode local config routing tests (partial mode and XDG opt-in mode)
- Auth bootstrap copy-on-consent tests (consent / decline / non-interactive)
- gstack install path tests
- Global Claude write guard tests (including unparseable-input fail-safe)
- Handoff create tests
- Handoff inspect tests
- Handoff verify tests
- Handoff restore tests
- Git Handoff tests
- Secrets exclusion tests
- Source exclusion tests
- zip-slip prevention tests
- Restore backup tests
- Path portability tests
- doctor diagnostics tests

Tests must be runnable without real Claude / Codex / OpenCode installs, using mock binaries or fixtures.

---

# 27. Required User Scenarios

These scenarios must pass:

## 27.1 proj00: default global Claude

In a folder without `.agentmod`, running `claude` uses the default global Claude settings. A globally installed superpowers plugin stays active in plain folders.

## 27.2 proj01: creating an agentmod project

After `agentmod init`, running `claude` uses the project-local Claude home. The superpowers plugin globally installed in proj00 must not be visible in proj01. (Auth: per §12, init offers consent-copy or prints login guidance — the scenario must not silently stall at a login screen.)

## 27.3 proj01: gstack project-isolated install

gstack installed via `agentmod install gstack` exists only at proj01's `.agentmod/claude/skills/gstack`. It is never installed to global `~/.claude/skills/gstack`.

## 27.4 proj02: a normal project

In proj02 (no `.agentmod`), `claude` uses default global settings. proj01's gstack is invisible; proj00's global superpowers remains visible.

## 27.5 Machine A → Machine B handoff

Create a handoff package on machine A; restore it on machine B over the same Git checkout. After restore, Claude / Codex / OpenCode config, gstack, MCP config, and context continue to the extent possible. Secrets and auth are excluded by default, with re-login guidance where needed.

## 27.6 Git Handoff

`agentmod handoff create --for-git` or `agentmod pack --for-git` produces a Git-storable package under `.agentmod-handoff/`, excluding source code, secrets, auth, sessions, and logs by default.

---

# 28. Completion-Declaration Prohibitions

Do not declare completion if any of these hold:

- No tests exist.
- Tests were not run.
- Failing tests remain.
- `agentmod init` is not idempotent.
- A shim is created.
- HOME is changed.
- agentmod env vars linger in folders without `.agentmod`.
- Required env vars are unset in folders with `.agentmod`.
- Environments leak between projects.
- gstack can be installed to global `~/.claude`.
- Handoff includes source code by default.
- Git Handoff includes sessions without encryption.
- Restore is zip-slip-vulnerable.
- Restore does not back up the existing `.agentmod`.
- README does not state limitations clearly (including: macOS Keychain non-isolation; OpenCode partial isolation; project `.claude/` native behavior; shell-hook first-session limitation).
- Open-source distribution docs are missing.
- `.harness/v0/STATE.md`, `.harness/v0/DONE.md`, `.harness/v0/TEST_MATRIX.md` were not updated.

---

# 29. Final Completion Conditions

All of the following must hold:

## Core
- AgentMod is implemented in Go.
- `agentmod init` works.
- `agentmod doctor` works.
- `agentmod status` works.
- The `.agentmod/` structure is created.
- `.agentmod/agentmod.toml` is created.
- `.gitignore` is updated safely.
- init is idempotent.

## Shell Auto-Env
- zsh hook support
- bash hook support
- Active only inside `.agentmod` projects
- Env vars unset outside projects
- No duplicate PATH entries
- No HOME change
- No shims

## Agent Routing
- Claude Code project-local home routing (`CLAUDE_CONFIG_DIR`)
- Codex CLI project-local home routing (`CODEX_HOME`)
- OpenCode project-local config routing (partial default + XDG opt-in)
- Auth bootstrap copy-on-consent flow
- No global config pollution
- gstack project-local install
- Claude Bash guard

## Handoff
- Handoff create / restore / inspect / verify / list
- pack / unpack aliases
- `.amod` package creation
- manifest / inventory / checksums / HANDOFF document generation
- secrets excluded by default
- source code excluded by default
- backup before restore
- doctor after restore
- zip-slip prevention

## Git Handoff
- `.agentmod-handoff/` creation
- `agentmod handoff create --for-git`
- `agentmod pack --for-git`
- sessions/logs excluded by default
- secrets/auth excluded by default
- source code excluded by default
- encryption-required policy for session inclusion

## Quality
- Tests exist
- Core-scenario tests exist
- README exists
- LICENSE exists
- SECURITY.md exists
- CONTRIBUTING.md exists
- CHANGELOG.md exists
- IMPLEMENTATION_PLAN.md exists
- `.harness/v0/` exists and is current
- Final report exists

---

# 30. Open-Source Distribution Docs

Prepare:

- README.md
- LICENSE
- CONTRIBUTING.md
- CHANGELOG.md
- SECURITY.md
- CODE_OF_CONDUCT.md

README must explain:

- What agentmod is
- What agentmod is **not**:
  - not a Docker sandbox
  - not full security isolation
  - not a shim
  - not a HOME-changing tool
  - not a source-code backup tool
- Quick start
- `agentmod init`
- Using plain `claude`, `codex`, `opencode` commands
- gstack installation
- Handoff usage
- Git Handoff usage
- Security cautions
- Secrets exclusion policy
- Restore cautions
- doctor usage
- Known limitations (macOS Keychain sharing; OpenCode partial isolation; project `.claude/` native behavior; first-session shell-hook activation)
- FAQ

---

# 31. Implementation Attitude

Do not implement by guessing. §3 facts are verified — build on them with cheap runtime sanity checks. Everything else, especially:

- exact session/history file layouts of each tool
- MCP config storage locations
- gstack setup internals
- the installed versions' behavior of each tool on this machine

must be verified by observing actual behavior before depending on it.

When uncertain:

1. Verify first.
2. Record results in `IMPLEMENTATION_PLAN.md` or `DECISIONS.md`.
3. If failure is possible, warn in `agentmod doctor`.
4. If global pollution is possible, do not execute.
5. If secrets leakage is possible, exclude by default.
6. Prioritize safety when restoring external snapshots.

---

# 32. Top-Priority Judgment Criteria

- Safety over convenience.
- Verifiability over speed.
- Small iterations over big implementations.
- File state over session memory.
- Passing tests over declaring completion.
- Project-scoped isolation with zero global pollution.
- Git moves the source; AgentMod hands off the agent environment.

If any criterion is violated, stop implementing and fix the design.
