# LOOP — Ralph Loop operating rules

Runner: `.harness/v0/loop.sh` (max iterations default 25 via
`AGENTMOD_LOOP_MAX_ITERS`; logs to `reports/iter-NNN.log`; stops only on a
`STATUS: DONE` line in `DONE.md` **verified by `go test ./...`** — a DONE with
failing tests is rewritten to `STATUS: REJECTED` and the loop continues).

## Iteration contract

Each iteration is an independent session. Assume zero memory of prior
conversations; the files ARE the memory.

Per-iteration order (FABLE_PLAN §7):

1. Read `GOAL.md`, `STATE.md`, `CHECKS.md`, `TEST_MATRIX.md`, `TASKS.md`.
2. Run the checks in `CHECKS.md`; note current failures.
3. Pick the **single smallest, most important** next task (usually the top
   unchecked item in `TASKS.md`, unless a check failure preempts it).
4. Modify only the files that task needs (Surgical Changes).
5. Run `go test ./...` (and `go vet ./...`).
6. Record results; on failure, write the cause into `STATE.md` for the next
   iteration — do not paper over it.
7. Update `STATE.md` (always), `TASKS.md` (tick/add items), `DECISIONS.md` /
   `RISKS.md` (when a decision/risk was made/found), `TEST_MATRIX.md` (when
   coverage changed).
8. Update `PROMPT.md` only if the standing instructions must change.
9. Commit with a descriptive message. Never commit with failing tests unless
   the commit itself is the failing-test record (say so in the message).
10. Judge completion against `GOAL.md`;§28 prohibitions are hard gates.

## Hard rules

- One task per iteration. Small > big.
- Never declare DONE without all completion conditions; `loop.sh` will reject
  it anyway and the rejection wastes an iteration.
- Never modify global agent homes (`~/.claude`, `~/.codex`,
  `~/.config/opencode`), HOME, or rc files of the developing user — the
  PreToolUse guard (`hooks/guard.sh`, wired in `.claude/settings.json`)
  enforces this; do not fight it, design around it.
- Tests must run without real Claude/Codex/OpenCode installs (fixtures/mock
  binaries; `t.Setenv`, temp dirs).
- Verify before building on unverified behavior (FABLE_PLAN §31); §3 facts
  are settled — sanity-check cheaply, do not re-research them.
- If an iteration ends mid-task, `STATE.md` must say exactly where it stopped
  and what is broken.
