# DONE — final completion verdict (v1)

STATUS: DONE

Declared 2026-06-15 after the CHECKS.md §7 gate: every GOAL.md completion
condition holds, every hard prohibition is false, and the completion gate
(below) passed. `loop.sh` re-verifies the gate independently.

## Final report

### What was built

`agentmod` now has a complete, single-source release & install pipeline:

1. **go install foundation.** The Go module path was renamed
   `github.com/agentmod/agentmod` → `github.com/mojomoth/agentmod` (go.mod + 32
   `.go` files) so `go install github.com/mojomoth/agentmod@latest` resolves.
   `resolveVersion()` prefers an ldflags-injected version and otherwise reads
   the Go build-info module version, so tagged installs show the tag.

2. **Release engine (`.goreleaser.yaml`).** One build cross-compiles
   linux/darwin/windows × amd64/arm64 (CGO off, `-trimpath`, version via
   ldflags), producing tar.gz/zip archives + sha256 `checksums.txt`, a Homebrew
   formula pushed to `mojomoth/homebrew-tap`, and a Scoop manifest pushed to
   `mojomoth/scoop-bucket`.

3. **CI (`.github/workflows/release.yml`).** On a `v*` tag, a GoReleaser job
   publishes the GitHub Release + Homebrew + Scoop and uploads `dist/`; a
   dependent job runs `npm/build.mjs --publish`. Tokens are referenced by secret
   name only (`GORELEASER_TOKEN`, `NPM_TOKEN`).

4. **npm (esbuild model).** The `agentmod` launcher package resolves and execs
   the platform-specific optional dependency `@agentmod/cli-<os>-<arch>`,
   forwarding argv and exit code, with a clear message when no binary matches.
   `npm/build.mjs` reads GoReleaser's `dist/artifacts.json`, stages one package
   per binary (0755 unix bit) + a version-stamped launcher, and publishes
   platform-packages-first.

5. **curl | sh (`install.sh`).** POSIX installer: platform detection, latest-tag
   resolution via the GitHub API, archive + checksum download, sha256
   verification, install to `/usr/local/bin` or `~/.local/bin`, PATH guidance.

6. **README** documents every channel (brew, npm, curl|sh, go install, scoop,
   source) with `mojomoth` URLs.

### Completion gate (local)

```
go build ./...        OK
go vet ./...          OK
go test ./...         OK (internal/cli, config, guard, handoff, project, routing)
gofmt -l .            (empty)
node --check npm/agentmod/bin/agentmod.js   OK
node --check npm/build.mjs                  OK
sh -n install.sh                            OK
old module-path refs in .go/go.mod          0
goreleaser check                            (absent locally — CI gate, RISKS R1)
GATE: PASS
```

### Deferred (human / credential — DIST_PLAN §Handoff, NOT part of this scope)

Creating the tap/bucket repos, registering CI secrets, securing the npm
`@agentmod` scope, pushing `main` + a `vX.Y.Z` tag, and the live per-channel
smoke tests. None of these are doable without credentials/network and are
intentionally left to the maintainer. No token value or name was written into
any tracked or harness file.
