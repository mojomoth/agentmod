# agentmod

Per-project isolation and handoff for coding agent environments
(Claude Code, Codex CLI, OpenCode).

```sh
npm install -g agentmod
agentmod --help
```

This package is a thin launcher. The platform-specific binary is delivered as an
optional dependency (`@agentmod/cli-<os>-<arch>`), so installing pulls only the
binary matching your machine — no postinstall download.

Other install methods (Homebrew, install script, `go install`) and full
documentation: https://github.com/mojomoth/agentmod
