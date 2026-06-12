# DONE — final completion verdict

STATUS: DONE

Declared 2026-06-12 after the CHECKS.md §6 gate: FABLE_PLAN §28 walked line
by line (every prohibition false), §29 walked line by line (every condition
true), and a fresh `go test ./... -count=1` run (summary below). `loop.sh`
re-verifies `go test ./...` independently.

## Final report

### What was built

`agentmod` is a Go CLI (module `github.com/agentmod/agentmod`, go 1.26,
single external dependency BurntSushi/toml) with two roles:

1. **Agent Home Router.** `agentmod init` creates `.agentmod/` (claude/
   codex/opencode/node/snapshots/logs + `agentmod.toml`), safely extends
   `.gitignore`, wires the Claude Bash guard into
   `.agentmod/claude/settings.json`, offers auth copy-on-consent, and
   installs a fenced rc block. The zsh (precmd+chpwd) and bash
   (PROMPT_COMMAND) hooks eval `agentmod env` on project transitions,
   routing `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `OPENCODE_CONFIG` (+ XDG
   opt-in) and the Node-family caches into `.agentmod/`, prepending exactly
   one PATH entry (`node/bin`) and restoring the pre-existing environment
   as a perfect inverse on exit. No shims, no HOME change, no wrapper
   command. `agentmod status` reports routing; `agentmod doctor` performs
   the full read-only §23 audit (exit 3 on findings); `agentmod guard
   claude-bash` blocks global-agent-home writes from Claude's Bash tool;
   `agentmod install gstack` clones project-local only, with a
   before/after global-skills pollution check.

2. **Handoff Tool.** `handoff create` (alias `pack`) writes a `.amod` zip —
   manifest.json, inventory.json (per-member sha256/size/mode),
   REDACTION.md, HANDOFF.md, RESTORE.md, checksums.txt, payload — with
   default policy exclusions (auth files incl. consent-copied ones, .env,
   ssh/cloud credentials, .git, node_modules, caches) and a
   secret-candidate scan over kept files; private-key material refuses
   creation unless `--allow-findings`; a dirty git worktree refuses unless
   `--allow-dirty`; git state (branch/HEAD/dirty, userinfo-redacted remote)
   is recorded in the manifest. `inspect`/`verify`/`list` read without
   extracting; `handoff restore` (alias `unpack`) runs
   Open→Verify→PlanRestore (zip-slip/absolute-path/symlink-escape/
   protected-element refusals) before any disk move, backs up the existing
   `.agentmod` to `.agentmod.backup-<stamp>`, extracts with automatic
   rollback, never executes snapshot content, re-wires the guard to this
   machine's binary, warns on machine-specific absolute paths in agent
   configs, and runs doctor inline plus the re-login notices.
   `handoff create --for-git` / `pack --for-git` writes the same six
   members as a committable plain-file tree under `.agentmod-handoff/`,
   additionally excluding sessions, history, and logs;
   `--include-sessions` always refuses because committed sessions would
   need encryption, which this version does not implement.

### Test summary

331 test functions, all green, no real agent installs required
(temp dirs, injected Env, fixture repos, mock binaries, real `zsh -f`/
`bash --noprofile --norc` sessions). Final fresh run this iteration:

```
$ go test ./... -count=1
?   github.com/agentmod/agentmod                    [no test files]
ok  github.com/agentmod/agentmod/internal/cli       8.915s
ok  github.com/agentmod/agentmod/internal/config    0.634s
ok  github.com/agentmod/agentmod/internal/guard     1.028s
ok  github.com/agentmod/agentmod/internal/handoff   2.458s
?   github.com/agentmod/agentmod/internal/layout    [no test files]
ok  github.com/agentmod/agentmod/internal/project   2.559s
ok  github.com/agentmod/agentmod/internal/routing   1.814s
?   github.com/agentmod/agentmod/internal/shellhook [no test files]
```

`gofmt -l .` clean, `go vet ./...` clean. TEST_MATRIX.md rows T01–T30 all ✅.

### Scenario results (FABLE_PLAN §27)

- §27.1–§27.4 (`TestScenarioIsolationMatrix`): one real zsh and one real
  bash session each drive proj00 (plain) → proj01 (init + install gstack)
  → proj02 (plain) with mock claude/codex/opencode binaries mirroring the
  real env-resolution rules. The global `superpowers` skill is visible in
  proj00/proj02 only; `gstack` in proj01 only; proj00/proj02 and the fake
  global Claude home are byte-identical before/after; XDG untouched in
  partial mode; mutation-verified.
- §27.5 (`TestScenarioHandoffRoundTrip`): a machine-A environment
  (CLAUDE.md, gstack skill, session transcript, MCP server config,
  opencode.json, both fake auth files) packs with exactly the two
  auth-file exclusions, restores on machine B with every continuation
  file byte-equal, auth absent, the canonical re-login block printed, and
  B's root gaining exactly `.agentmod` + one backup dir.
- §27.6 (`TestScenarioGitHandoff`): a fixture containing all five excluded
  categories (source, .env, auth ×2, session, logs) produces a `--for-git`
  package whose payload file set is pinned exactly by DeepEqual — a leak
  from any category fails the test.

### Known limitations (stated in README, doctor, and the generated docs)

- macOS Keychain: Claude credentials are shared across all config dirs;
  per-project account isolation is impossible on macOS.
- OpenCode is partially isolated by default (global config merge chain +
  global XDG session storage); `opencode.xdg_full_isolation = true` opts
  into full XDG routing at the cost of affecting every XDG-aware tool.
- Project `.claude/` is native Claude behavior, independent of routing.
- First-session hook activation requires a new terminal / `exec $SHELL` /
  one-shot eval; the bash hook is inert in non-interactive scripts.
- pnpm/bun global bins are not on PATH (only npm's `node/bin` is managed).
- `.agentmod-handoff/` tree packages restore manually (reader out of
  scope, D049); the guard is a heuristic, not a sandbox.

### Pointers

- User documentation: `README.md` (with quick start, FAQ, limitations),
  `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`,
  `CODE_OF_CONDUCT.md`.
- Architecture: `IMPLEMENTATION_PLAN.md`; spec: `.harness/v0/FABLE_PLAN.md`.
- Settled choices: `.harness/v0/DECISIONS.md` (D001–D053); risks:
  `.harness/v0/RISKS.md`; coverage: `.harness/v0/TEST_MATRIX.md`.
