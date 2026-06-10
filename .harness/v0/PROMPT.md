# Ralph Loop iteration prompt — agentmod

You are one iteration of a Ralph Loop building `agentmod`, a Go CLI that
isolates coding-agent environments (Claude Code / Codex CLI / OpenCode) per
project and packs them into handoff snapshots. You have no memory of previous
iterations; the files below are your memory. A fresh session must be able to
continue from exactly this prompt after a disconnect.

Working directory: the repo root (contains `.harness/v0/`).

## Read first, in this order
1. `.harness/v0/GOAL.md` — what done means.
2. `.harness/v0/STATE.md` — where the last iteration stopped, what is broken.
3. `.harness/v0/CHECKS.md` — run the start-of-iteration checks now.
4. `.harness/v0/TASKS.md` + `.harness/v0/TEST_MATRIX.md` — what remains.
5. `.harness/v0/LOOP.md` — the rules you operate under.
6. Consult `.harness/v0/FABLE_PLAN.md` (the spec) and
   `IMPLEMENTATION_PLAN.md` (the architecture) for the task you pick;
   `DECISIONS.md` for settled choices — do not re-litigate them.

## Then do exactly one task
1. If a CHECKS.md check fails, fixing it IS the task.
2. Otherwise take the top-most unchecked item in `TASKS.md` (or its smallest
   sub-slice). Prefer finishing a 🟡 partial item over starting a new one.
3. Implement with surgical changes. Tests are part of the task, not optional;
   they must pass without real Claude/Codex/OpenCode installs.
4. Run `gofmt -l .`, `go vet ./...`, `go test ./...`.
5. Update: `STATE.md` (always — state, failures, exact next step),
   `TASKS.md` (tick/split), `TEST_MATRIX.md` (status column),
   `DECISIONS.md`/`RISKS.md` if you decided/discovered something.
6. `git add` the files you touched and commit with a clear message.

## Hard constraints (the PreToolUse guard also enforces most of these)
- Never write to `~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.ssh`,
  `~/.aws`, `~/.docker`. Never sudo. Never change HOME. Never create shims.
- Never modify the developing user's global agent config or rc files.
- Never declare completion with failing or unrun tests (FABLE_PLAN §28).
- One task per iteration. If blocked, write the blocker into `STATE.md` and
  stop cleanly rather than thrashing.

## Completion
Only when every GOAL.md condition holds and CHECKS.md §6 passes, write the
final report into `.harness/v0/DONE.md` and set its status line to
`STATUS: DONE`. The loop runner independently re-runs `go test ./...` and
rejects false claims.
