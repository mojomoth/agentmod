# TASKS — small-unit checklist

Tick items as they land (with tests). One task ≈ one loop iteration or less.
Add/split items freely; keep units small.

## Phase 0 — Harness
- [x] git init
- [x] Harness docs (GOAL/PLAN/TASKS/DECISIONS/RISKS/CHECKS/TEST_MATRIX/LOOP/DONE/PROMPT/STATE)
- [x] loop.sh (max-iter cap, DONE sentinel + test verification, reports/)
- [x] PreToolUse guard hook + .claude/settings.json wiring + smoke tests
- [x] .gitignore
- [x] Skills: verify mattpocock (present), install karpathy skills project-locally, skills/README.md
- [x] IMPLEMENTATION_PLAN.md
- [x] Initial commit

## Phase 1 — Skeleton
- [x] go.mod + main.go + subcommand dispatcher + `--version`
- [x] internal/project: upward discovery of .agentmod/agentmod.toml (+ tests)
- [x] internal/config: TOML schema, defaults per FABLE_PLAN §13, validation (+ tests)
- [x] `agentmod status` active/inactive output (+ tests)

## Phase 2 — init + hooks
- [x] init: .agentmod/ layout + agentmod.toml writer (+ tests)
- [x] init: .gitignore add w/ dedup + no-git-repo grace (+ tests)
- [x] init: idempotency guarantee tests (run twice, byte-identical results)
- [x] init: flags --no-shell-hook, --yes/non-interactive (+ tests)
- [x] `agentmod env` activate/deactivate/switch transition logic (+ tests)
- [x] `agentmod hook zsh` emitter: precmd/chpwd, pure-shell upward search, evals env on transitions (+ tests)
- [x] `agentmod hook bash` (+ tests)
- [x] rc fenced-block insert/update, never duplicate, never touch user content (+ tests)
- [x] scripted-shell integration tests: activate/deactivate/cross-project, PATH dedup, HOME untouched
- [x] init: first-session limitation message + hook-active diagnosis

## Phase 3 — doctor + guard + auth
- [ ] doctor: project/root/shell/hook/env checks (+ tests)
- [ ] doctor: HOME-change, shim, lingering-vars, dup-PATH warnings (+ tests)
- [ ] doctor: per-agent home state incl. auth present / re-login needed (+ tests)
- [ ] doctor: OpenCode partial-isolation + merge-chain leak warnings (+ tests)
- [ ] doctor: macOS Keychain note; gstack global-risk check (+ tests)
- [ ] guard claude-bash: stdin contract, deny rules, fail-safe (+ table tests)
- [ ] init wires guard into .agentmod/claude/settings.json (+ tests)
- [ ] auth copy-on-consent: detect, prompt, copy/decline/non-interactive paths (+ tests)

## Phase 4 — gstack
- [ ] install gstack: clone to .agentmod/claude/skills/gstack only (+ fixture-repo tests)
- [ ] outside-project failure; already-installed abort; --force (+ tests)
- [ ] global before/after pollution verification + abort path (+ tests)
- [ ] error reporting: no git, network failure, setup failure (+ tests)

## Phase 5 — handoff create
- [ ] .amod writer: zip + manifest + inventory + sha256 checksums (+ tests)
- [ ] default exclusion engine (source, .git, node_modules, caches, auth, .env…) (+ tests)
- [ ] redaction report + secret-candidate scan (+ tests)
- [ ] HANDOFF + RESTORE human docs generation (+ tests)
- [ ] git state metadata w/ sanitized remote URL, dirty warning (+ tests)
- [ ] inspect / verify / list / pack alias (+ tests)

## Phase 6 — restore
- [ ] validation: schema version, checksums, zip-slip, absolute paths, symlinks (+ malicious fixtures)
- [ ] backup existing .agentmod before restore (+ tests)
- [ ] restore writes only under .agentmod/; no script execution (+ tests)
- [ ] portability: separators, exec bits, MCP absolute-path warn/rewrite (+ tests)
- [ ] post-restore doctor + re-login notices; unpack alias (+ tests)

## Phase 7 — git handoff
- [ ] --for-git → .agentmod-handoff/, git-safe contents (+ tests)
- [ ] sessions/logs excluded; --include-sessions fails w/ encryption explanation (+ tests)
- [ ] pack --for-git alias (+ tests)

## Phase 8 — docs + scenarios
- [ ] Scenario tests §27: proj00/proj01/proj02 isolation matrix (mock binaries)
- [ ] Scenario test: A→B handoff round-trip; git handoff
- [ ] README.md (what it is/is not, quick start, limitations, FAQ)
- [ ] LICENSE, SECURITY.md, CONTRIBUTING.md, CHANGELOG.md, CODE_OF_CONDUCT.md
- [ ] Final §28/§29 audit + final report + DONE.md
