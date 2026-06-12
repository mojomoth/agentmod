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
- [x] backup existing .agentmod before restore (+ tests)
      (D042: handoff.BackupAgentmod — atomic rename to
       .agentmod.backup-<utc-stamp>, rollback = rename back; absent source
       no-op, occupied name refusal, stray-file backed up too; pipeline
       pinned Open→Verify→PlanRestore→Backup→extract; extraction slice must
       gitignore `.agentmod.backup-*/` when a backup was made)
- [x] restore writes only under .agentmod/; no script execution (+ tests)
      (D043: (*Snapshot).Restore in internal/handoff/restore.go +
       `handoff restore` cli; pipeline Stat→Open→Verify→PlanRestore all
       before any disk move (refusals create no backup), exit 2/1/3/3/3;
       Dirs(0700-then-chmod)→Files(O_EXCL+Chmod)→Links order; automatic
       rollback = RemoveAll partial + rename backup back; layout.Subdirs
       recreated; ensureGitignore generalized to (dir, entry),
       .agentmod.backup-*/ added only when a backup was made; nothing
       from a snapshot is ever executed; unpack stays a stub for the
       notices slice; T24+T25 flip ✅, T26 🟡)
- [x] portability: separators, exec bits, MCP absolute-path warn/rewrite (+ tests)
      (D044: separators/exec bits were already structural (D041 refusals,
       D043 umask-proof chmod) — the slice added the post-restore
       portability pass: ensureClaudeGuardHook re-wires the guard command
       to THIS machine's binary (the one rewrite, agentmod owns the file);
       scanRestoredConfigs walks every string value in
       claude/settings.json + claude/.claude.json + codex/config.toml +
       opencode/opencode.json and WARNS on machine-specific absolute paths
       (never rewrites user-owned files, never changes exit code); T27 ✅)
- [x] post-restore doctor + re-login notices; unpack alias (+ tests)
      (D045: doctor runs INLINE after a successful restore, its exit code
       never propagates (restore stays 0 — D044's vars-unset warning fires
       routinely in fresh shells); unconditional re-login block with the
       canonical D037 remedy strings + darwin-only Keychain line via
       env.GOOS; D043's gitignore-coverage wrinkle noted in output when it
       happens; unpack = true alias of handoff restore; T26 flips ✅)
- [x] doctor: portability/MCP absolute-path finding (§23 "MCP warnings" +
      "Portability risks"; reuse scanRestoredConfigs from D044)
      (D046: agentConfigPathFindings — inside-project, always-a-line
       (ok when clean), one warn finding per scanRestoredConfigs warning,
       not gated on enabled flags, doctor stays read-only; Phase 6 done)

## Phase 7 — git handoff
- [x] --for-git → .agentmod-handoff/, git-safe contents (+ tests)
      (D047: tree of PLAIN FILES under .agentmod-handoff/ — same six root
       members + payload/ as a .amod, fed by the same writeSnapshot walk
       via a new memberSink interface (zipSink/treeSink); replace-or-refuse
       gate keyed on manifest.json; --for-git incompatible with --output
       and --allow-findings; manifest gains for_git; HANDOFF/RESTORE
       renderers grew honest git-mode wording. NOTE: sessions/logs still
       travel until the next slice — --for-git is NOT yet git-safe for
       sessions; T28 stays 🟡)
- [x] sessions/logs excluded; --include-sessions fails w/ encryption explanation (+ tests)
      (D048: ForGitRules() = DefaultRules + session-data + log-data,
       path-anchored to the routed homes (targets verified against real
       agent installs); CreateForGit applies it when Rules is nil;
       --include-sessions always refuses — without --for-git "regular
       snapshots already include sessions", with it the §19 encryption
       explanation — before any FS work; git-mode HANDOFF.md states the
       exclusion; .amod format provably still packs sessions; T28 ✅)
- [x] pack --for-git alias (+ tests)
      (D049: TestPackForGitAlias + TestPackForGitAliasIncludeSessionsRefused
       pin the §19-required command through the top-level alias; zero
       product code change needed)
- [x] handoff inspect/verify/restore accept a tree package directory
      (D049: decided OUT OF SCOPE for the MVP — GOAL §29 requires git
       handoff CREATION only; honesty notes in the git-mode docs stay;
       README limitations (Phase 8) must list manual tree restore)

## Phase 8 — docs + scenarios
- [x] Scenario tests §27: proj00/proj01/proj02 isolation matrix (mock binaries)
      (D050: TestScenarioIsolationMatrix in scenario_test.go — one real
       {zsh,bash} session each runs init + install gstack through the
       fakeAgentmodBin wrapper across three folders; mock
       claude/codex/opencode on the child PATH mirror the real resolution
       rules and list visible skills; superpowers/gstack visibility matrix,
       non-stalling init auth guidance, in-session global-pollution check,
       before/after tree snapshots, XDG untouched; mutation-verified;
       T30 stays 🟡 until the §27.5/.6 slice below)
- [x] Scenario test: A→B handoff round-trip; git handoff (§27.5/§27.6 —
      check overlap with restore_test.go/gitpack_test.go first, D050)
      (D051: TestScenarioHandoffRoundTrip — continuation files byte-equal
       on B, auth absent, re-login block, root gains only
       .agentmod+backup; TestScenarioGitHandoff — all five §27.6
       exclusion categories in one --for-git run, payload file set
       pinned by DeepEqual; cli-level, no shell session; mechanics
       overlap mapped to existing tests instead of duplicated; T30 ✅)
- [x] README.md (what it is/is not, quick start, limitations, FAQ)
      (covers every §30 bullet: two roles + "git moves source; agentmod
       moves the agent env", the five is-NOT bullets, routing table from
       routing.Vars, init/auth/gstack/handoff/git-handoff/restore/doctor/
       guard sections, secrets exclusion policy, restore cautions, the four
       §28-mandatory limitations PLUS D016/D018/D049/D034-D040/D035
       honest notes, FAQ; command syntax + stamp format verified against
       the real binary — no product code change)
- [x] LICENSE, SECURITY.md, CONTRIBUTING.md, CHANGELOG.md, CODE_OF_CONDUCT.md
      (loop iterations wrote the first four but were repeatedly output-blocked
       by the content filter before committing — see D052; CODE_OF_CONDUCT.md
       was written as an original short policy, NOT the verbatim Contributor
       Covenant, and the slice was committed by the supervising session)
- [x] doctor: last three §23 must-warn rows — snapshot secret candidates,
      git-handoff session/log leak, restore-target HEAD drift (+ tests)
      (D053: found 🟡 by the final audit — earlier slices mapped these to
       "Phases 7–8" and none picked them up; snapshotFindings parses each
       snapshot's REDACTION.md via handoff.RedactionFindingCounts,
       gitHandoffFinding audits .agentmod-handoff/payload with the new
       handoff.GitPublishRules, gitStateFinding compares the repo HEAD to
       the newest snapshot's recorded HEAD (the one doctor check that
       executes git, read-only); T29 flips ✅)
- [x] Final §28/§29 audit + final report + DONE.md
      (2026-06-12: CHECKS.md §1–§5 all clean; fresh `go test ./... -count=1`
       green across all packages; §28 walked line by line — every
       prohibition false; §29 walked line by line — every condition true,
       verified against the real binary's help surface, the all-✅
       TEST_MATRIX, and the README's four mandatory limitation bullets;
       final report written into DONE.md, STATUS: DONE set)
