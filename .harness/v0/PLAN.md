# PLAN — development phases

Each phase ends green: build + vet + tests pass, STATE.md updated, committed.
Details per phase live in `TASKS.md`; architecture in `IMPLEMENTATION_PLAN.md`.

## Phase 0 — Harness (this scaffold) ✅
git init, harness docs, guard hook wired, loop.sh, skills project-local,
IMPLEMENTATION_PLAN.md.

## Phase 1 — Go skeleton + discovery + config
go.mod (`github.com/agentmod/agentmod`, dep: BurntSushi/toml only), CLI
dispatcher (stdlib flag), `internal/project` upward discovery of
`.agentmod/agentmod.toml`, `internal/config` TOML schema + mandatory-default
validation, minimal `status`. Exit codes settled. Tests: discovery, config
defaults, status active/inactive.

## Phase 2 — init + shell hooks + routing
`agentmod init` (dir layout, agentmod.toml, .gitignore dedup incl. no-repo
case, fenced rc block, `--no-shell-hook`, `--yes`/non-interactive, idempotent,
honest first-session message), `agentmod hook zsh|bash` (chpwd/precmd +
PROMPT_COMMAND, save/restore env, unset on exit, PATH dedup, never HOME),
`agentmod env` plumbing the hooks eval. Tests: idempotency, rc fencing,
scripted zsh/bash sessions over fixture projects, env unset outside, no dup
PATH, HOME untouched.

## Phase 3 — doctor + Claude guard + auth bootstrap
`agentmod doctor` (full §23 checklist incl. macOS-Keychain note, OpenCode
partial-isolation warning, hook installed-vs-active, shim/HOME detection,
gstack global-risk), `agentmod guard claude-bash` (PreToolUse contract §3.1,
deny global writes, fail-safe on unparseable input) wired into
`.agentmod/claude/settings.json` by init, auth copy-on-consent (Codex
auth.json; Claude .credentials.json on Linux/Windows; decline → re-login
instructions; copied files always on exclusion list). Tests: guard cases
(allow reads, block writes, garbage input), consent/decline/non-interactive,
doctor warning matrix.

## Phase 4 — gstack installer
`agentmod install gstack`: clone into `.agentmod/claude/skills/gstack` only;
fail outside a project; force flag for reinstall; before/after global
pollution check; clear errors for no-git/network/setup failure; doctor
visibility. Tests with a local fixture git repo standing in for gstack.

## Phase 5 — Handoff create/inspect/verify/list
`.amod` (zip) with manifest/inventory/checksums(sha256)/redaction report/
HANDOFF + RESTORE docs; default exclusions (source, .git, node_modules,
caches, tmp, auth incl. consent-copied, .env, ssh/cloud creds); git state
metadata (sanitized remote, branch, HEAD, dirty summary); `pack` alias.
Tests: creation, exclusion lists, secret-candidate scan, inspect/verify/list,
checksum tamper detection.

## Phase 6 — Restore + portability
Validation pipeline (schema version, checksums, zip-slip, no absolute paths,
symlink safety), backup existing `.agentmod` first, restore only under
`.agentmod/`, MCP absolute-path warn/rewrite, exec-bit restore, re-login
notices, doctor after restore; `unpack` alias. Tests: malicious archives
(traversal, absolute, symlink escape), backup, round-trip, cross-OS paths.

## Phase 7 — Git Handoff
`handoff create --for-git` / `pack --for-git` → `.agentmod-handoff/`;
sessions/logs/secrets/source excluded; `--include-sessions` fails with
encryption explanation (MVP: no encryption). Tests: content policy, failure
message, package is git-safe.

## Phase 8 — Docs + scenarios + release polish
README (incl. honest limitations + FAQ), LICENSE (MIT), SECURITY.md,
CONTRIBUTING.md, CHANGELOG.md, CODE_OF_CONDUCT.md; §27 scenario tests
(proj00/proj01/proj02, A→B handoff, git handoff) as integration tests with
mock agent binaries; final report; DONE.md verdict.
