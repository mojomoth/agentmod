# GOAL — agentmod

Build `agentmod`: a Go CLI that isolates the configuration, skills, plugins,
sessions, caches, and working context of coding agents (Claude Code, Codex CLI,
OpenCode) per project, and packs that environment into snapshots for handoff.

Authoritative spec: `.harness/v0/FABLE_PLAN.md` (supersedes `GPT_PLAN.md`).

## Two roles

1. **Agent Home Router** — activates only in directories under a
   `.agentmod/agentmod.toml`; routes `CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
   `OPENCODE_CONFIG` (+ optional XDG), and Node-family caches into
   `.agentmod/` via shell hooks (direnv-style, self-contained, no shims,
   no HOME change, no wrapper command).
2. **Handoff Tool** — `.amod` snapshots (manifest/inventory/checksums/
   redaction report/HANDOFF doc) and Git-safe handoffs under
   `.agentmod-handoff/`. Git moves source; agentmod moves the agent env.

## Completion conditions

Completion = **all** of FABLE_PLAN §29, with §28 prohibitions all clear:

- Core: `init` (idempotent, `--no-shell-hook`, non-interactive), `doctor`,
  `status`; `.agentmod/` layout + `agentmod.toml`; safe `.gitignore` edits.
- Shell: zsh + bash hooks; activate only inside projects; full unset outside;
  no dup PATH entries; never HOME; never shims.
- Routing: Claude/Codex/OpenCode (partial default + XDG opt-in); auth
  copy-on-consent; zero global pollution; gstack project-local install;
  Claude Bash guard (`agentmod guard claude-bash`) wired into
  `.agentmod/claude/settings.json`.
- Handoff: create/restore/inspect/verify/list + pack/unpack; secrets & source
  excluded by default; backup before restore; zip-slip-proof; doctor after
  restore.
- Git Handoff: `--for-git` to `.agentmod-handoff/`; sessions/logs/secrets
  excluded; `--include-sessions` without encryption fails with explanation.
- Quality: tests for everything in `TEST_MATRIX.md` pass via `go test ./...`
  without real agent installs; README (with honest limitations: macOS
  Keychain sharing, OpenCode partial isolation, native project `.claude/`,
  first-session hook caveat), LICENSE, SECURITY.md, CONTRIBUTING.md,
  CHANGELOG.md, CODE_OF_CONDUCT.md, IMPLEMENTATION_PLAN.md, final report.

When and only when all hold: set `STATUS: DONE` in `DONE.md` with the final
summary. `loop.sh` independently re-verifies `go test ./...` before stopping.
