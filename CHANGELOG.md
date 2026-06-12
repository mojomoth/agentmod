# Changelog

All notable changes to agentmod are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased] — 0.1.0-dev

Initial MVP feature set.

### Added

- **Project environments**: `agentmod init` creates `.agentmod/` (claude /
  codex / opencode / node / snapshots / logs) with a versioned
  `agentmod.toml`, safe `.gitignore` editing, and idempotent re-runs.
  Flags: `--no-shell-hook`, `--yes`/`--non-interactive`.
- **Shell routing**: `agentmod hook zsh` / `agentmod hook bash` emit
  direnv-style prompt hooks that route `CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
  `OPENCODE_CONFIG` (plus optional XDG full isolation for OpenCode) and
  Node-family caches into `.agentmod/` while inside a project, and restore
  the previous environment exactly on exit. No shims, no `HOME` change, no
  wrapper command. `agentmod env` prints the transitions for manual eval.
- **Status & diagnostics**: `agentmod status` (active/inactive, routed
  paths, hook state, recent handoff) and `agentmod doctor` (read-only
  checks: layout, config, rc block, routing env hygiene, lingering vars,
  PATH duplicates, HOME safety, shims, per-agent auth state, OpenCode
  partial-isolation leaks, macOS Keychain note, gstack global-pollution
  risk, guard wiring, portability of restored configs).
- **Claude Bash guard**: `agentmod guard claude-bash` — a PreToolUse hook
  wired into `.agentmod/claude/settings.json` by init; heuristically blocks
  sudo, `HOME=` reassignment, and writes targeting global agent homes.
- **Auth bootstrap**: init offers copy-on-consent of existing global Claude /
  Codex credentials into the project homes (never silently; never on macOS
  Claude, which uses the Keychain).
- **gstack**: `agentmod install gstack [--force]` clones gstack
  project-locally with a before/after global-skills pollution check.
- **Handoff snapshots**: `agentmod handoff create` / `pack` writes a
  deterministic `.amod` zip (manifest, inventory, sha256 checksums,
  REDACTION.md, HANDOFF.md, RESTORE.md, payload) with default exclusion of
  auth files, `.env`, key material, credential dirs, caches, and
  `node_modules`; a content scan refuses to pack private-key findings unless
  `--allow-findings`; git state metadata with sanitized remote URLs and a
  dirty-tree gate (`--allow-dirty`).
- **Restore**: `agentmod handoff restore` / `unpack` — validate (checksums,
  schema, zip-slip, absolute paths, escaping symlinks) → backup existing
  `.agentmod/` → extract under `.agentmod/` only → rollback on failure;
  never executes snapshot content; re-wires the guard hook to the local
  binary; warns on machine-specific absolute paths in agent configs; runs
  doctor inline and prints re-login notices. `inspect` / `verify` / `list`
  read snapshots without extracting.
- **Git handoff**: `agentmod handoff create --for-git` / `pack --for-git`
  writes the same members as a plain-file tree under `.agentmod-handoff/`
  for committing; sessions and logs are additionally excluded;
  `--include-sessions` is refused pending encryption support.
- Docs: README (with honest limitations), LICENSE (MIT), SECURITY.md,
  CONTRIBUTING.md, CODE_OF_CONDUCT.md, IMPLEMENTATION_PLAN.md.

### Known limitations

See the README "Known limitations" section — notably macOS Keychain sharing,
OpenCode partial isolation by default, native project `.claude/` behavior,
first-session hook activation, non-interactive bash, and manual restore of
`--for-git` tree packages.
