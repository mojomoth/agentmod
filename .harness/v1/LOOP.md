# LOOP — Ralph Loop operating rules (v1)

Runner: `.harness/v1/loop.sh` (max iterations default 25 via
`AGENTMOD_LOOP_MAX_ITERS`; logs to `reports/iter-NNN.log`; stops only on a
`STATUS: DONE` line in `DONE.md` **verified by the completion gate** — a DONE
whose gate fails is rewritten to `STATUS: REJECTED` and the loop continues).

## Iteration contract

Each iteration is an independent session. Assume zero memory of prior
conversations; the files ARE the memory.

1. Read `GOAL.md`, `STATE.md`, `CHECKS.md`, `TEST_MATRIX.md`, `TASKS.md`.
2. Run `CHECKS.md`; note current failures.
3. Pick the single smallest, most important next task (top unchecked item in
   `TASKS.md`, unless a check failure preempts it). Consult `DIST_PLAN.md` for
   the spec and `DECISIONS.md` for settled choices — do not re-litigate them.
4. Make surgical changes only to what the task needs.
5. Re-run the relevant `CHECKS.md` sections.
6. On failure, write the cause into `STATE.md` for the next iteration — do not
   paper over it.
7. Update `STATE.md` (always), `TASKS.md` (tick/split), `TEST_MATRIX.md`
   (status), `DECISIONS.md`/`RISKS.md` (when something was decided/found).
8. Commit with a descriptive message. Never commit with a failing check unless
   the commit IS the failing-record (say so).
9. Judge completion against `GOAL.md`; the secret-hygiene prohibition is a hard
   gate.

## Hard rules

- One task per iteration. Small > big.
- **Never write a token value or its variable name into any file.** Never
  commit `.env*` or `npm/dist/`. The PreToolUse guard (`hooks/guard.sh`) blocks
  `.env*` edits and global-path writes — design around it, don't fight it.
- Never change global state: HOME, global agent homes, global npm/brew config.
- The dev box has no `goreleaser`/`shellcheck`; treat their checks as CI-gated
  (RISKS R1) — never fake a local pass.
- Don't perform the DIST_PLAN §Handoff steps (repo creation, secrets, tag push,
  publish): they need credentials and are explicitly out of the loop's scope.
- If an iteration ends mid-task, `STATE.md` must say exactly where it stopped.
