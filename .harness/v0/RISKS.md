# RISKS

Live register. Add/retire entries as the implementation evolves; each risk
has a mitigation that must be reflected in tests or doctor warnings.

## R1 — Global pollution during development or use (HIGH)
Writes to `~/.claude`, `~/.codex`, `~/.config/opencode` from tests, gstack
install, or hook bugs. **Mitigation:** harness PreToolUse guard; product
Bash guard; tests use temp dirs + `t.Setenv("HOME", …)` is FORBIDDEN for the
real process (use injected paths instead); gstack installer verifies global
mtime/contents before+after; CHECKS.md §2 audit every iteration.

## R2 — rc-file corruption (HIGH)
`init` edits `~/.zshrc`/`~/.bashrc`. **Mitigation:** only the fenced
`# >>> agentmod >>> / # <<< agentmod <<<` block is ever added/replaced;
idempotency tests; never delete user content; `--no-shell-hook` for CI.

## R3 — Env leakage between projects / lingering vars (HIGH)
Vars persist after leaving a project, or leak from project A into B; PATH
accumulates duplicates. **Mitigation:** hook saves/restores prior values,
unconditionally unsets on exit; PATH de-dup logic; shell-level tests run
scripted zsh/bash sessions cd-ing across fixture projects.

## R4 — Secrets in snapshots or Git Handoffs (CRITICAL)
`auth.json`, `.credentials.json` (incl. consent-copied ones), `.env`, tokens
in git remote URLs. **Mitigation:** default exclusion list is hardcoded and
not overridable for `--for-git`; redaction report enumerates candidate hits;
secret-pattern scan tests; remote URLs sanitized in git metadata.

## R5 — Restore as attack vector (CRITICAL)
zip-slip, absolute paths, symlink escape, writes outside `.agentmod/`,
auto-executing scripts from snapshots. **Mitigation:** path normalization +
containment check on every entry; symlink targets validated; restore writes
only under `.agentmod/` (backup first); no script execution; malicious-archive
test fixtures.

## R6 — OpenCode partial isolation misunderstood (MEDIUM)
Users assume sessions are isolated; global config merge chain leaks plugins.
**Mitigation:** doctor warns explicitly in partial mode; README states it;
XDG full routing is opt-in with a prominent "affects all XDG tools" warning.

## R7 — macOS Claude auth expectations (LOW)
Keychain is shared; per-project account isolation impossible on macOS.
**Mitigation:** doctor + README state it honestly; no fake isolation claims.

## R8 — First-session hook limitation confuses users (LOW)
`init` cannot mutate the parent shell. **Mitigation:** init prints exact
"restart shell or `eval $(agentmod hook zsh)`" guidance; doctor detects
installed-but-inactive.

## R9 — Windows portability of snapshots (MEDIUM)
Path separators, exec bits, symlinks. **Mitigation:** forward-slash internal
paths; exec-bit table in manifest; symlink policy (store target, validate on
restore); portability tests with synthetic Windows-style paths. PowerShell
hook may be deferred; restore format must not break Windows.

## R10 — Tool behavior drift (MEDIUM)
Future Claude/Codex/OpenCode versions change env-var handling. **Mitigation:**
doctor performs cheap runtime sanity checks (e.g. routed dirs actually used);
§3 facts noted with version observed (claude 2.1.170, codex 0.137.0,
opencode 1.4.3 on this machine).

## R11 — Loop burns tokens without progress (MEDIUM)
**Mitigation:** loop.sh max-iteration cap; one-small-task rule; STATE.md must
record blockers so the next iteration doesn't repeat the same failure.
