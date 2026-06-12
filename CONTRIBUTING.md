# Contributing to agentmod

Thanks for considering a contribution. This project values small, surgical,
well-tested changes.

## Build and test

```sh
go build ./...
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

All four must be clean before a PR. Tests must pass **without** real Claude
Code / Codex CLI / OpenCode installs — CI machines have none. Shell-hook
tests skip themselves when `zsh`/`bash` are absent; everything else runs
anywhere Go runs.

## Hard rules for tests

These are non-negotiable; the test suite is built around them:

- **Never touch real global agent state.** Tests must not read or write the
  developer's `~/.claude`, `~/.codex`, `~/.config/opencode`, rc files, or
  Keychain. Fake homes are temp dirs injected through the `Env` struct
  (`internal/cli/status.go`) — cwd, env vars, stdin, clock (`Now`), `GOOS`,
  and `Executable` are all injectable. New code that needs any of those must
  read them from `Env`, never from `os.*`/`runtime.*` directly, or it cannot
  be tested hermetically.
- **Never reassign the real `HOME`** for the test process. Pass a fake home
  via the injected env instead.
- Fixture secrets must be obviously fake (`sk-FAKE-...`); never commit
  anything resembling a real credential or private key.
- Shell-hook tests run real `zsh -f`/`bash --norc` sessions with the test
  binary impersonating agentmod (see `hook_test.go`'s `fakeAgentmodBin`
  harness) — reuse that harness rather than inventing a new one.

## Dependencies

`github.com/BurntSushi/toml` is deliberately the **only** module dependency.
Adding a dependency needs a strong reason and a maintainer's agreement first.

## Code style

- `gofmt` is the formatter; `go vet` must be clean.
- Match the surrounding code's idiom. Comments state constraints the code
  can't show, not narration.
- One logical change per commit, with a message that says why.

## Where things live

- `internal/cli` — command surface, flag parsing, output text.
- `internal/handoff` — snapshot format (exec-free: it never runs git or any
  other binary; the cli layer does).
- `internal/routing`, `internal/shellhook`, `internal/layout`,
  `internal/config`, `internal/project`, `internal/guard` — see
  `IMPLEMENTATION_PLAN.md` for the architecture and the per-package
  contracts.

## Reporting bugs / security issues

Plain bugs: open a GitHub issue with repro steps (`agentmod doctor` output
helps). Security issues: see [SECURITY.md](SECURITY.md) — do not open a
public issue with exploit details.
