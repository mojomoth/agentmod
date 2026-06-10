# CHECKS — run every iteration

Run these at the START of each iteration (cheap, fail fast) and again before
finishing. Record failures in `STATE.md`.

## 1. Build & tests (once go.mod exists)

```sh
go build ./...
go vet ./...
go test ./...
```

All must pass before an iteration may end "green". `gofmt -l .` must print
nothing.

## 2. Global-pollution audit (every iteration, cheap)

The dev process must never touch global agent state:

```sh
# Mtimes of global homes must not change due to our work this iteration.
ls -ldT ~/.claude ~/.codex ~/.config/opencode 2>/dev/null
# No agentmod-created files in global homes:
ls ~/.claude/skills 2>/dev/null | grep -i gstack
```

Compare BOTH outputs against the baseline in `STATE.md`. The user's own
pre-existing global gstack install (gstack, gstack-upgrade,
open-gstack-browser — see D010) is part of the baseline, not a violation;
only NEW gstack entries or changed mtimes caused by our work are violations.

Compare against the baseline recorded in `STATE.md` (update the baseline only
when a deliberate, user-approved global change happened — there should be none).

## 3. Harness integrity

```sh
test -x .harness/v0/hooks/guard.sh && test -x .harness/v0/loop.sh || echo "VIOLATION: harness scripts not executable"
grep -q guard.sh .claude/settings.json || echo "VIOLATION: guard not wired"
```

## 4. Repo hygiene

```sh
git status --short            # know what you're committing
grep -q '^\.agentmod/$' .gitignore || echo "VIOLATION: .agentmod/ not gitignored"
```

`git status` must never show `.agentmod/` contents as committable.

## 5. Secret hygiene (before any commit touching handoff/fixtures)

```sh
git diff --cached | grep -inE 'api[_-]?key|auth[_-]?token|secret|credential|BEGIN .*PRIVATE KEY' || true
```

Fixtures must use obviously-fake values (e.g. `sk-FAKE-fixture`).

## 6. Completion gate (only when about to declare DONE)

Walk FABLE_PLAN §28 line by line; every prohibition must be false.
Walk §29 line by line; every condition must be true.
Then run `go test ./...` one final time and paste the summary into `DONE.md`.
