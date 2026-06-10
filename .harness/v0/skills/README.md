# Harness skills — install record (FABLE_PLAN §9)

All skills are installed **project-locally only** at `.claude/skills/`.
Nothing was installed to the global `~/.claude` (the PreToolUse guard blocks
that anyway).

## mattpocock/skills — VERIFIED PRESENT, not reinstalled
- Already installed at `.claude/skills/` before the harness was built;
  pinned and hash-tracked by the repo-root `skills-lock.json`
  (source `mattpocock/skills`, per-skill `computedHash`).
- Skills relevant to this build and why:
  - `tdd`, `diagnose`, `review`, `triage` — test-based completion verdicts
    and disciplined debugging during loop iterations.
  - `git-guardrails-claude-code`, `setup-pre-commit` — safety posture
    consistent with the harness guard.
  - `design-an-interface`, `request-refactor-plan` — interface design for the
    internal packages.
  - The rest (writing/personal skills) are inert for this project; left in
    place rather than pruned, since removal would drift from skills-lock.json.

## multica-ai/andrej-karpathy-skills — INSTALLED 2026-06-10
- Method: `git clone --depth 1` to a temp dir, copied
  `skills/karpathy-guidelines/` → `.claude/skills/karpathy-guidelines/`,
  temp clone removed. Pinned commit: `2c606141936f1eeef17fa3043a72095b4765b9c2`.
- The repo ships exactly one skill, `karpathy-guidelines`. Installed because
  it is the literal source of the FABLE_PLAN §9 mandatory thinking
  principles: Think Before Coding, Simplicity First, Surgical Changes,
  Goal-Driven Execution (verifiable success criteria, no unnecessary
  abstraction, small change units, failure conditions defined up front).

## Policy for future skills
Need → record the reason in `DECISIONS.md` first → install project-locally
only → record here (source, commit, method, rationale).
