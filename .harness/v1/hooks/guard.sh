#!/usr/bin/env bash
# agentmod dev-harness PreToolUse guard (v1 — distribution work).
#
# Protects the DEVELOPMENT PROCESS: no global-state changes and, critically for
# this phase, no secret leakage. Block by exiting 2 (stderr is fed back to
# Claude). Fail-safe: if input cannot be parsed, deny only when the raw text
# references a global agent/credential path or a .env file; never block all.

set -u

INPUT="$(cat)"

deny() {
  echo "agentmod-harness-guard BLOCKED: $1" >&2
  echo "Rule source: .harness/v1/GOAL.md (Hard prohibitions) / LOOP.md. Never touch global agent homes, HOME, sudo, or secrets; never commit .env*." >&2
  exit 2
}

# Global agent homes + credential dirs, in ~ / $HOME / absolute-home spellings.
GLOBAL_RE='(~|\$HOME|\$\{HOME\}|/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+)/(\.claude|\.codex|\.config/opencode|\.local/share/opencode|\.ssh|\.aws|\.docker)([/"'"'"'[:space:]]|$)'
# .env / .env.local / .env.* as a path segment or bare command argument.
ENV_RE='(^|[/[:space:]])\.env(\.[A-Za-z0-9_.-]+)?([[:space:]"'"'"']|$)'

if ! command -v jq >/dev/null 2>&1; then
  if printf '%s' "$INPUT" | grep -qE "$GLOBAL_RE"; then
    deny "jq unavailable and input references a global agent/credential path"
  fi
  exit 0
fi

TOOL_NAME="$(printf '%s' "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null || true)"

if [ -z "$TOOL_NAME" ]; then
  if printf '%s' "$INPUT" | grep -qE "$GLOBAL_RE"; then
    deny "unparseable hook input referencing a global agent/credential path"
  fi
  exit 0
fi

case "$TOOL_NAME" in
  Bash)
    CMD="$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null || true)"
    [ -z "$CMD" ] && exit 0

    if printf '%s' "$CMD" | grep -qE '(^|[;&|]|\$\()[[:space:]]*sudo([[:space:]]|$)'; then
      deny "sudo is forbidden in this project"
    fi
    if printf '%s' "$CMD" | grep -qE '(^|[;&|(]|[[:space:]])(export[[:space:]]+)?HOME=[^[:space:]]'; then
      deny "changing HOME is forbidden"
    fi
    if printf '%s' "$CMD" | grep -qE 'npm[[:space:]]+(config[[:space:]]+set|install[[:space:]]+.*(-g|--global))'; then
      deny "global npm install/config changes are forbidden"
    fi
    # Never stage/commit a .env file.
    if printf '%s' "$CMD" | grep -qE 'git[[:space:]]+add' && printf '%s' "$CMD" | grep -qE "$ENV_RE"; then
      deny "refusing to git add a .env* file (secret hygiene)"
    fi
    # Never read a .env file into a tracked/redirected target.
    if printf '%s' "$CMD" | grep -qE "$ENV_RE" \
       && printf '%s' "$CMD" | grep -qE '>>?[[:space:]]'; then
      deny "redirecting .env* content into a file risks leaking a secret"
    fi
    # Writes targeting global agent homes / credential dirs.
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
    if printf '%s' "$FILE_PATH" | grep -qE "$ENV_RE"; then
      deny "editing a .env* file is forbidden (secret hygiene): $FILE_PATH"
    fi
    ;;
esac

exit 0
