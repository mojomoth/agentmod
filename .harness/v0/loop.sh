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
#
# Stop conditions:
#   1. .harness/v0/DONE.md contains a line `STATUS: DONE`
#      AND `go test ./...` passes (a DONE claim with failing tests is
#      rewritten to STATUS: REJECTED and the loop continues).
#   2. Max iterations reached.
# Never loops unboundedly.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
H="$ROOT/.harness/v0"
REPORTS="$H/reports"
MAX_ITERS="${AGENTMOD_LOOP_MAX_ITERS:-25}"
CLAUDE_BIN="${AGENTMOD_LOOP_CLAUDE_BIN:-claude}"
PERM_ARGS="${AGENTMOD_LOOP_PERM_ARGS:---dangerously-skip-permissions}"
EXTRA_ARGS="${AGENTMOD_LOOP_EXTRA_ARGS:-}"

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

for i in $(seq 1 "$MAX_ITERS"); do
  n="$(printf 'iter-%03d' "$i")"
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

  if check_done; then exit 0; fi
done

echo "[loop] Max iterations ($MAX_ITERS) reached without a verified DONE. Inspect $H/STATE.md and reports/."
exit 1
