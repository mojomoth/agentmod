# Ralph Loop iteration prompt — agentmod distribution (v1)

You are one iteration of a Ralph Loop making `agentmod` installable via
Homebrew, npm, curl|sh, `go install`, and Scoop, driven by a GoReleaser +
GitHub Actions pipeline. You have no memory of previous iterations; the files
below are your memory. A fresh session must be able to continue from exactly
this prompt after a disconnect.

Working directory: the repo root (contains `.harness/v1/`).

## Read first, in this order
1. `.harness/v1/GOAL.md` — what done means (and what is out of scope).
2. `.harness/v1/STATE.md` — where the last iteration stopped, what is broken.
3. `.harness/v1/CHECKS.md` — run the start-of-iteration checks now.
4. `.harness/v1/TASKS.md` + `.harness/v1/TEST_MATRIX.md` — what remains.
5. `.harness/v1/LOOP.md` — the rules you operate under.
6. `.harness/v1/DIST_PLAN.md` (the spec) and `.harness/v1/DECISIONS.md`
   (settled choices) for the task you pick.

## Then do exactly one task
1. If a CHECKS.md check fails, fixing it IS the task.
2. Otherwise take the top-most unchecked item in `TASKS.md` (or its smallest
   sub-slice). Prefer finishing a 🟡 partial over starting a new one. Skip the
   "Out of scope (human/credential)" items — they are not yours.
3. Implement with surgical changes. Verification is part of the task.
4. Run the relevant `CHECKS.md` sections.
5. Update `STATE.md` (always), `TASKS.md`, `TEST_MATRIX.md`, and
   `DECISIONS.md`/`RISKS.md` if you decided/discovered something.
6. `git add` the files you touched and commit with a clear message.

## Hard constraints (the PreToolUse guard enforces most of these)
- **Never write a GitHub/npm token value — or its variable name — into any
  tracked or harness file.** Never commit `.env*` or `npm/dist/`.
- Never change HOME, global agent homes, or global npm/brew config. Never sudo.
- Don't run the DIST_PLAN §Handoff steps (repo creation, CI secrets, tag push,
  publish). They are credential-bearing and external.
- One task per iteration. If blocked, write the blocker into `STATE.md` and stop
  cleanly rather than thrashing.

## Completion
Only when every GOAL.md completion condition holds and CHECKS.md §7 passes,
write the final report into `.harness/v1/DONE.md` and set its status line to
`STATUS: DONE`. The loop runner independently re-runs the completion gate and
rejects false claims.
