# TEST_MATRIX — per-feature test scope and completion criteria

All tests run via `go test ./...` with **no real agent installs** (temp dirs,
fixture trees, mock binaries on PATH, injected env). A feature is COMPLETE
only when its rows are all ✅ in the Status column. Update Status as tests land.

| # | Area | Must cover | Status |
|---|------|-----------|--------|
| T01 | Project discovery | found in cwd; found in ancestor; nearest-wins with nested projects; not found; stops at filesystem root | ✅ |
| T02 | Config | defaults match FABLE_PLAN §13 (change_home=false, guards on, exclusions on, XDG opt-in off); unknown schema version rejected; round-trip | ✅ |
| T03 | status | active output (roots+homes); inactive output | ⬜ |
| T04 | init layout | creates .agentmod tree (claude/codex/opencode/node/snapshots); never deletes/overwrites existing dirs | ⬜ |
| T05 | init idempotency | second run = no-op; no dup rc block; no dup .gitignore line; existing config untouched | ⬜ |
| T06 | init flags | --no-shell-hook skips rc edits; non-interactive never prompts, never copies auth | ⬜ |
| T07 | .gitignore | created if missing; entry deduped; non-git dir handled gracefully | ⬜ |
| T08 | rc fencing | block inserted once; updated in place; user content byte-preserved around it | ⬜ |
| T09 | zsh hook | scripted zsh: cd in → vars set; cd out → vars unset; new shell inside project activates (precmd); nested project nearest-wins | ⬜ |
| T10 | bash hook | same via PROMPT_COMMAND | ⬜ |
| T11 | env hygiene | no duplicate PATH entries after repeated transitions; prior user env values restored; HOME never changed; no shims created anywhere | ⬜ |
| T12 | Claude routing | CLAUDE_CONFIG_DIR → .agentmod/claude inside; unset/restored outside | ⬜ |
| T13 | Codex routing | CODEX_HOME → .agentmod/codex inside; unset/restored outside | ⬜ |
| T14 | OpenCode routing | partial: OPENCODE_CONFIG set, XDG untouched; opt-in: XDG_CONFIG_HOME/XDG_DATA_HOME set; off by default | ⬜ |
| T15 | Auth bootstrap | consent→copy (codex auth.json; claude .credentials.json linux path); decline→instructions; non-interactive→never copies; copied files in exclusion list | ⬜ |
| T16 | guard claude-bash | blocks rm/cp/mv/mkdir/clone/redirect into global homes; allows reads; allows project writes; unparseable input fail-safe (denies only global-path writes); exit-2 + deny-JSON modes | ⬜ |
| T17 | guard wiring | init writes PreToolUse hook into .agentmod/claude/settings.json, NOT project .claude/ | ⬜ |
| T18 | gstack | clones only into .agentmod/claude/skills/gstack; fails outside project; already-installed abort + --force; global home unchanged before/after (verified); no-git/network/setup errors distinct | ⬜ |
| T19 | handoff create | .amod contains manifest/inventory/checksums/redaction-report/HANDOFF/RESTORE; payload matches inventory | ⬜ |
| T20 | exclusions | source code, .git, node_modules, caches, tmp, auth (incl. consent-copied), .env, ssh/cloud creds all absent by default | ⬜ |
| T21 | secret scan | fixture secrets detected → listed in redaction report; clean fixture → empty report | ⬜ |
| T22 | git metadata | repo/branch/HEAD/dirty/summary recorded; token in remote URL redacted; dirty tree warns, needs explicit consent | ⬜ |
| T23 | inspect/verify/list | inspect shows manifest+inventory; verify detects checksum tamper + schema mismatch; list finds snapshots | ⬜ |
| T24 | restore safety | zip-slip entry rejected; absolute path rejected; symlink escape rejected; writes confined to .agentmod/; no script auto-exec | ⬜ |
| T25 | restore backup | existing .agentmod backed up before write; backup restorable | ⬜ |
| T26 | round-trip | create→restore on fresh tree reproduces config/skills/plugins; excluded items absent + re-login notice present | ⬜ |
| T27 | portability | windows-style paths normalized; exec bits restored; MCP absolute paths warned/rewritten | ⬜ |
| T28 | git handoff | --for-git output under .agentmod-handoff/; no sessions/logs/secrets/source; --include-sessions fails citing encryption; HANDOFF doc present | ⬜ |
| T29 | doctor | each §23 warning has a positive + negative test (warn fires / stays silent) | ⬜ |
| T30 | scenarios §27 | proj00 global untouched; proj01 isolated (global plugin invisible); proj01 gstack local-only; proj02 unaffected; A→B handoff; git handoff | ⬜ |

Legend: ⬜ not started · 🟡 partial · ✅ complete (tests passing)
