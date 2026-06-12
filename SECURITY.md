# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately via **GitHub Security
Advisories** on this repository ("Report a vulnerability" under the Security
tab). If that is not available to you, open an issue that says only that you
have a security report and how to reach you — do **not** put exploit details,
secrets, or snapshot contents in a public issue.

You should hear back within 7 days. Please give us a reasonable window to
ship a fix before public disclosure.

## Threat model — what agentmod does and does not defend against

agentmod is an **environment router and packaging tool, not a sandbox**.
Honest boundaries:

### Routing / isolation

- Isolation is *configuration-level*: agent processes are pointed at
  project-local homes via environment variables. Nothing stops a process
  from reading or writing any path your user account can reach.
- It is **not** a Docker sandbox, not full security isolation, and never
  changes `HOME`.
- macOS Claude credentials live in the shared system Keychain; agentmod
  cannot isolate them per project (documented in README limitations).
- OpenCode isolation is partial by default; global session data still
  accumulates unless `opencode.xdg_full_isolation` is enabled.

### The Claude Bash guard (`agentmod guard claude-bash`)

- A **heuristic** deny-list over proposed shell commands (sudo, `HOME=`
  reassignment, writes targeting global agent homes). It reduces accidents;
  it is trivially bypassable by a determined or obfuscated command and must
  not be treated as a security boundary.

### Snapshots and restore

- **Treat any `.amod` file you did not create as untrusted input.** Restore
  validates structure and checksums, refuses zip-slip / absolute paths /
  escaping symlinks / paths outside `.agentmod/`, strips setuid/setgid bits,
  backs up the existing `.agentmod/` first, rolls back on failure, and never
  executes snapshot content. Validation bugs are exactly the class of issue
  we want reported.
- Restored *configuration* can still be dangerous when later consumed by an
  agent (e.g. MCP server definitions that run commands). Restore and doctor
  warn about machine-specific paths but do not vet config semantics — review
  `HANDOFF.md`/`REDACTION.md` and the restored configs before launching
  agents.

### Secret handling in snapshots

- The default exclusion rules (auth files, `.env`, key files, credential
  dirs) and the content scan (private keys, common token shapes) are
  **heuristics**. They will not catch every secret. Review the packed
  `REDACTION.md` before sharing a snapshot; `--allow-findings` is an explicit
  opt-out, not a safe default.
- Snapshots are written `0600` and may contain session transcripts; share
  them like you would share your shell history.

## Supported versions

Pre-1.0: only the latest release/`main` receives fixes.
