#!/usr/bin/env bash
# agentmod dev-harness PreToolUse guard (FABLE_PLAN §8).
#
# Contract (verified, FABLE_PLAN §3.1): stdin is JSON with tool_name and
# tool_input. Block by exiting 2 (stderr is fed back to Claude).
# Fail-safe rule: if input cannot be parsed, deny only when the raw input
# references a global agent/credential path; never block everything.
#
# This guard protects the DEVELOPMENT PROCESS of agentmod. The product's own
# guard (`agentmod guard claude-bash`) is a separate Go implementation.

set -u

INPUT="$(cat)"

deny() {
  echo "agentmod-harness-guard BLOCKED: $1" >&2
  echo "Rule source: .harness/v0/FABLE_PLAN.md §8 / §25. Work inside the project; never touch global agent homes, HOME, sudo, or credential dirs." >&2
  exit 2
}

# Global agent homes + credential dirs, in ~ / $HOME / absolute-home spellings.
GLOBAL_RE='(~|\$HOME|\$\{HOME\}|/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+)/(\.claude|\.codex|\.config/opencode|\.local/share/opencode|\.ssh|\.aws|\.docker)([/"'"'"'[:space:]]|$)'

if ! command -v jq >/dev/null 2>&1; then
  # Fail safe without jq: deny only global-path writes we can see in raw text.
  if printf '%s' "$INPUT" | grep -qE "$GLOBAL_RE"; then
    deny "jq unavailable and input references a global agent/credential path"
  fi
  exit 0
fi

TOOL_NAME="$(printf '%s' "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null || true)"

if [ -z "$TOOL_NAME" ]; then
  # Unparseable input: fail safe (deny only on global-path reference).
  if printf '%s' "$INPUT" | grep -qE "$GLOBAL_RE"; then
    deny "unparseable hook input referencing a global agent/credential path"
  fi
  exit 0
fi

case "$TOOL_NAME" in
  Bash)
    CMD="$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"
    [ -z "$CMD" ] && exit 0

    # sudo
    if printf '%s' "$CMD" | grep -qE '(^|[;&|]|\$\()[[:space:]]*sudo([[:space:]]|$)'; then
      deny "sudo is forbidden in this project"
    fi

    # HOME reassignment (export HOME=..., HOME=... cmd, env HOME=...)
    if printf '%s' "$CMD" | grep -qE '(^|[;&|(]|[[:space:]])(export[[:space:]]+)?HOME=[^[:space:]]'; then
      deny "changing HOME is forbidden (FABLE_PLAN §2.3)"
    fi

    # Global npm / brew configuration changes
    if printf '%s' "$CMD" | grep -qE 'npm[[:space:]]+(config[[:space:]]+set|install[[:space:]]+.*(-g|--global))'; then
      deny "global npm install/config changes are forbidden"
    fi

    # Writes targeting global agent homes / credential dirs.
    # Block only high-write-likelihood commands or redirections; reads stay allowed.
    if printf '%s' "$CMD" | grep -qE "$GLOBAL_RE"; then
      WRITE_RE='(^|[;&|]|\$\()[[:space:]]*(cp|mv|rm|mkdir|rmdir|touch|tee|ln|rsync|install|unzip|chmod|chown|truncate|dd)([[:space:]]|$)'
      if printf '%s' "$CMD" | grep -qE "$WRITE_RE"; then
        deny "write-like command targets a global agent/credential path"
      fi
      if printf '%s' "$CMD" | grep -qE 'git[[:space:]]+clone'; then
        deny "git clone into/near a global agent path; clone into the project instead"
      fi
      if printf '%s' "$CMD" | grep -qE '>>?[[:space:]]*"?(~|\$HOME|\$\{HOME\}|/Users/|/home/)'; then
        deny "output redirection targets a path under the user home"
      fi
    fi
    ;;

  Write|Edit|NotebookEdit)
    FILE_PATH="$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null || true)"
    [ -z "$FILE_PATH" ] && exit 0
    if printf '%s' "$FILE_PATH" | grep -qE '^(/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+|~)/(\.claude|\.codex|\.config/opencode|\.local/share/opencode|\.ssh|\.aws|\.docker)(/|$)'; then
      deny "file edit targets a global agent/credential path: $FILE_PATH"
    fi
    ;;
esac

exit 0
