#!/usr/bin/env bash
# Ralph Loop runner for the agentmod build (FABLE_PLAN §7).
#
# Usage:        .harness/v0/loop.sh
# Env vars:
#   AGENTMOD_LOOP_MAX_ITERS    max iterations (default 25)
#   AGENTMOD_LOOP_CLAUDE_BIN   claude binary (default: claude)
#   AGENTMOD_LOOP_PERM_ARGS    permission args (default: --dangerously-skip-permissions;
#                              guardrails live in .claude/settings.json PreToolUse hooks)
#   AGENTMOD_LOOP_EXTRA_ARGS   extra claude args (e.g. --model)
#   AGENTMOD_LOOP_MAX_RATELIMIT_SLEEPS  max 15-min sleeps while rate-limited
#                              before giving up (default 48 = 12h)
#
# Stop conditions:
#   1. .harness/v0/DONE.md contains a line `STATUS: DONE`
#      AND `go test ./...` passes (a DONE claim with failing tests is
#      rewritten to STATUS: REJECTED and the loop continues).
#   2. Max iterations reached.
#   3. Rate-limited beyond the sleep budget.
# Never loops unboundedly. A rate-limited attempt (claude exits immediately
# with a session/usage-limit message) does NOT consume an iteration; the loop
# sleeps 15 minutes and retries the same iteration.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
H="$ROOT/.harness/v0"
REPORTS="$H/reports"
MAX_ITERS="${AGENTMOD_LOOP_MAX_ITERS:-25}"
CLAUDE_BIN="${AGENTMOD_LOOP_CLAUDE_BIN:-claude}"
PERM_ARGS="${AGENTMOD_LOOP_PERM_ARGS:---dangerously-skip-permissions}"
EXTRA_ARGS="${AGENTMOD_LOOP_EXTRA_ARGS:-}"
MAX_RATELIMIT_SLEEPS="${AGENTMOD_LOOP_MAX_RATELIMIT_SLEEPS:-48}"

mkdir -p "$REPORTS"

done_declared() {
  grep -qE '^STATUS:[[:space:]]*DONE[[:space:]]*$' "$H/DONE.md" 2>/dev/null
}

tests_pass() {
  if [ ! -f "$ROOT/go.mod" ]; then
    echo "[loop] go.mod missing — implementation has not started; DONE cannot be valid." >&2
    return 1
  fi
  (cd "$ROOT" && go test ./... >"$REPORTS/final-go-test.log" 2>&1)
}

check_done() {
  if done_declared; then
    echo "[loop] DONE sentinel found; verifying with go test ./..."
    if tests_pass; then
      echo "[loop] Tests pass. Loop complete."
      return 0
    fi
    echo "[loop] DONE rejected: tests fail (see reports/final-go-test.log)."
    ts="$(date '+%Y-%m-%d %H:%M:%S')"
    sed -i '' -e "s/^STATUS:[[:space:]]*DONE[[:space:]]*$/STATUS: REJECTED (tests failing, $ts — see reports\/final-go-test.log)/" "$H/DONE.md"
  fi
  return 1
}

# Allow a pre-declared DONE to stop before burning an iteration.
if check_done; then exit 0; fi

i=1
ratelimit_sleeps=0
while [ "$i" -le "$MAX_ITERS" ]; do
  # Find the next unused report slot so re-runs don't clobber old logs.
  slot=1
  while [ -e "$REPORTS/$(printf 'iter-%03d' "$slot").log" ]; do slot=$((slot + 1)); done
  n="$(printf 'iter-%03d' "$slot")"
  log="$REPORTS/$n.log"

  echo "[loop] === iteration $i/$MAX_ITERS ($n) === $(date '+%Y-%m-%d %H:%M:%S')"
  # shellcheck disable=SC2086
  "$CLAUDE_BIN" -p "$(cat "$H/PROMPT.md")" $PERM_ARGS $EXTRA_ARGS 2>&1 | tee "$log"
  rc=${PIPESTATUS[0]}
  echo "[loop] iteration exit code: $rc" | tee -a "$log"

  # Rate-limit detection: claude bailed immediately with a limit message.
  # The size guard avoids misreading a real iteration that merely mentions
  # the words "rate limit" in its work output.
  if [ "$rc" -ne 0 ] \
     && [ "$(wc -c < "$log")" -lt 2000 ] \
     && grep -qiE 'session limit|usage limit|rate limit|spend limit|limit reached|hit your' "$log"; then
    ratelimit_sleeps=$((ratelimit_sleeps + 1))
    if [ "$ratelimit_sleeps" -gt "$MAX_RATELIMIT_SLEEPS" ]; then
      echo "[loop] Rate-limited for more than $MAX_RATELIMIT_SLEEPS sleep periods; giving up."
      exit 1
    fi
    echo "[loop] Rate-limited (sleep $ratelimit_sleeps/$MAX_RATELIMIT_SLEEPS). Sleeping 15m, then retrying iteration $i."
    rm -f "$log"   # a no-op attempt; don't leave a garbage log or consume the slot
    sleep 900
    continue       # iteration counter NOT incremented
  fi
  ratelimit_sleeps=0

  if check_done; then exit 0; fi
  i=$((i + 1))
done

echo "[loop] Max iterations ($MAX_ITERS) reached without a verified DONE. Inspect $H/STATE.md and reports/."
exit 1
