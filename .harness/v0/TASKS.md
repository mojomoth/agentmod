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
- [x] doctor: project/root/shell/hook/env checks (+ tests)
- [x] doctor: HOME-change, shim, lingering-vars, dup-PATH warnings (+ tests)
- [x] doctor: per-agent home state incl. auth present / re-login needed (+ tests)
- [x] doctor: OpenCode partial-isolation + merge-chain leak warnings (+ tests)
- [x] doctor: macOS Keychain note; gstack global-risk check (+ tests)
- [x] guard claude-bash: stdin contract, deny rules, fail-safe (+ table tests)
- [x] init wires guard into .agentmod/claude/settings.json (+ tests)
- [x] auth copy-on-consent: detect, prompt, copy/decline/non-interactive paths (+ tests)
- [x] doctor: guard hook wired / stale-binary-path finding in claude/settings.json (+ tests)

## Phase 4 — gstack
- [x] install gstack: clone to .agentmod/claude/skills/gstack only (+ fixture-repo tests)
- [x] outside-project failure; already-installed abort; --force (+ tests)
      (outside-project exit-2 + already-installed abort landed with the clone
       task; --force clone-first-then-swap landed in its own slice, D031)
- [x] global before/after pollution verification + abort path (+ tests)
- [x] error reporting: no git, network failure, setup failure (+ tests)
      (D033: git output forwarded verbatim + source-override hint;
       git-missing via crippled real PATH; ENOTDIR fault injection)

## Phase 5 — handoff create
- [x] .amod writer: zip + manifest + inventory + sha256 checksums (+ tests)
      (D034: internal/handoff package + `handoff create [--output]`;
       snapshots/ structurally excluded; REDACTION/HANDOFF/RESTORE members
       and all policy exclusions are the items below)
- [x] default exclusion engine (source, .git, node_modules, caches, auth, .env…) (+ tests)
      (D035: Rule list in internal/handoff/exclude.go with per-rule
       human-readable reasons; Result.Excluded feeds the REDACTION.md slice;
       source code structurally absent until project-level payload roots)
- [x] redaction report + secret-candidate scan (+ tests)
      (D036: scan.go content patterns over KEPT files only; private-key =
       hard refusal unless --allow-findings; REDACTION.md root member
       renders Excluded reasons + findings, never the matched bytes)
- [x] HANDOFF + RESTORE human docs generation (+ tests)
      (D037: docs.go renderers; root members between REDACTION.md and
       checksums.txt; canonical re-login remedies moved to internal/handoff,
       doctor/auth alias them; T19 flips ✅)
- [x] git state metadata w/ sanitized remote URL, dirty warning (+ tests)
      (D039: cli collects via exec git — internal/handoff stays exec-free;
       manifest `git` key omitted when no repo/binary; userinfo stripped
       from scheme:// remotes; dirty → refusal unless --allow-dirty;
       source_included always-false but explicit; T22 flips ✅)
- [x] inspect / verify / list / pack alias (+ tests)
      (D040: internal/handoff/read.go Open/Snapshot/Verify — restore builds
       on them; inspect prints manifest + counts + REDACTION.md verbatim,
       no extraction; verify re-hashes vs checksums.txt + inventory
       cross-check, exit 3 on problems; list = snapshots/ newest-first,
       status.recentHandoff refactored onto the shared lister; pack ≡
       handoff create, unpack = stub until Phase 6; T23 ✅)

## Phase 6 — restore
- [x] validation: schema version, checksums, zip-slip, absolute paths, symlinks (+ malicious fixtures)
      (D041: (*Snapshot).PlanRestore in internal/handoff/validate.go —
       problems-or-plan; schema gate shared with Verify via schemaProblem();
       checksums stay Verify's job, restore runs Open→Verify→PlanRestore;
       .agentmod whitelist + §21 protected elements + lexical symlink
       containment; restore/unpack stay stubs until extraction lands)
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
