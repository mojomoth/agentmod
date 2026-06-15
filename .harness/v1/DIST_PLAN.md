# DIST_PLAN — authoritative spec for agentmod distribution (v1)

This is the spec the loop builds against. It supersedes ad-hoc notes. Keep it in
sync when a decision changes (also record the change in `DECISIONS.md`).

## 0. Facts (verified, do not re-research)

- `agentmod` is one `package main` at the repo root building a single binary
  `agentmod`. Pure Go, `CGO_ENABLED=0`, only module dependency
  `github.com/BurntSushi/toml`. Cross-compiles to all GOOS/GOARCH.
- Go 1.26 (`go.mod`).
- Version injection point: `var Version` in `internal/cli/cli.go`, settable via
  `-ldflags "-X github.com/mojomoth/agentmod/internal/cli.Version=…"`.
- Repository: `github.com/mojomoth/agentmod` (exists). The Go module path was
  `github.com/agentmod/agentmod` and is renamed to match the repo URL.
- Target audience are Claude Code / Codex / OpenCode users, who almost always
  have Node and/or Homebrew — so npm and brew are first-class, not afterthoughts.

## 1. Engine

GoReleaser + GitHub Actions is the whole engine. One `v*` tag produces, in one
run: cross-built binaries, archives, checksums, a GitHub Release, a Homebrew
formula, and a Scoop manifest. A second CI job turns the same binaries into npm
packages. `go install` needs no pipeline (module path == repo URL). `install.sh`
consumes the published Release assets.

## 2. Module path rename (prerequisite)

`go install github.com/mojomoth/agentmod@latest` only works when the module path
equals the repo URL. Rename `module …` in `go.mod` and every `.go` import from
`github.com/agentmod/agentmod` to `github.com/mojomoth/agentmod`. Scope the
replacement to the full path so directory/binary/CLI strings named `agentmod`
are untouched. Gate: `go build ./… && go vet ./… && go test ./…`, `gofmt` clean.

## 3. Version fallback

`go install …@vX.Y.Z` does not pass ldflags, so the binary would otherwise print
the dev sentinel. `resolveVersion()` prefers an injected `Version`, else reads
`debug.ReadBuildInfo().Main.Version` (skipping empty/`(devel)`), else the
sentinel. Release builds still get the exact version via ldflags.

## 4. `.goreleaser.yaml`

- One build: `main: .`, `binary: agentmod`, `CGO_ENABLED=0`, `-trimpath`,
  `ldflags: -s -w -X …/internal/cli.Version={{ .Version }}`,
  `goos: [linux, darwin, windows]`, `goarch: [amd64, arm64]` (6 targets).
- `archives`: `tar.gz`, `format_overrides` windows → `zip`,
  `name_template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`.
- `checksum`: `checksums.txt`, sha256.
- `brews`: → `mojomoth/homebrew-tap` (formula installs the binary, `test` runs
  `agentmod version`).
- `scoops`: → `mojomoth/scoop-bucket`.
- No token in the file: CI exports the cross-repo PAT as `GITHUB_TOKEN`.
- Validate: `goreleaser check`; dry-run: `goreleaser release --snapshot --clean`.

## 5. `.github/workflows/release.yml`

- Trigger `push: tags: ['v*']`; `permissions: contents: write`.
- Job `goreleaser`: checkout (fetch-depth 0) → setup-go 1.26 →
  goreleaser-action `release --clean` with `GITHUB_TOKEN` = the cross-repo PAT
  secret → upload `dist/` as a CI artifact.
- Job `npm-publish` (`needs: goreleaser`): checkout → setup-node 20 (npm
  registry) → download `dist` artifact → `node npm/build.mjs --publish` with
  `AGENTMOD_VERSION = github.ref_name` and the npm token as `NODE_AUTH_TOKEN`.
- Secrets referenced **by name only**.

## 6. npm packaging (esbuild model)

- Launcher package `agentmod` (`npm/agentmod/`): `bin/agentmod.js` resolves
  `@agentmod/cli-<process.platform>-<process.arch>/bin/agentmod[.exe]`, execs it
  with inherited stdio, mirrors the child exit code, and prints install
  alternatives when no platform package is present. `optionalDependencies` lists
  all six platform packages.
- Per-platform packages `@agentmod/cli-<os>-<arch>`: `os`/`cpu` constraints +
  the single binary. Generated, never hand-written.
- `npm/build.mjs`: reads `dist/artifacts.json` (robust to GoReleaser's
  version-suffixed dir names), maps GOOS/GOARCH → node platform/arch, stages one
  package per binary (0755 on the unix binary) + a version-stamped launcher into
  `npm/dist/` (gitignored), and with `--publish` runs `npm publish --access
  public` platform-packages-first. `AGENTMOD_VERSION` (leading `v` stripped)
  required for `--publish`.
- Scope/name constants live at the top of `build.mjs` and in the launcher
  `package.json`; if the `@agentmod` org or the `agentmod` name is unavailable,
  change them in those two places only (see RISKS).

## 7. `install.sh` (curl | sh)

POSIX sh. Detect OS (`Linux`→linux, `Darwin`→darwin) and arch
(`x86_64`→amd64, `arm64|aarch64`→arm64); refuse others with a `go install` hint.
Resolve the tag from `AGENTMOD_VERSION`/arg or the GitHub "latest release" API.
Download `agentmod_<num>_<os>_<arch>.tar.gz` + `checksums.txt`, verify sha256
(`sha256sum` or `shasum -a 256`), extract the binary to
`/usr/local/bin` (if writable) or `~/.local/bin` (override `AGENTMOD_INSTALL_DIR`),
and warn if the dir is off `PATH`.

## 8. README

Document all channels: brew, npm, curl|sh, `go install`, scoop, and
build-from-source — using the `mojomoth` URLs.

## Deliverables (the completion checklist)

`go.mod` + `.go` renamed · `resolveVersion()` + test · `.goreleaser.yaml` ·
`.github/workflows/release.yml` · `npm/agentmod/{package.json,bin/agentmod.js,README.md}` ·
`npm/build.mjs` · `install.sh` · `README.md` install section · `.gitignore`
covers `npm/dist/` and `.env*`.

## Handoff (human / credential steps — NOT done by the loop)

1. Create `mojomoth/homebrew-tap` and `mojomoth/scoop-bucket` (empty repos).
2. Register CI secrets: a cross-repo PAT and an npm publish token (names in
   `release.yml`). Never echo or commit their values.
3. Secure the npm `@agentmod` org scope (or change the scope/name constants).
4. `git remote add origin …`, push `main`, then push a `vX.Y.Z` tag to trigger
   the pipeline.
5. Smoke-test each channel against the published release (brew/npm/curl/go
   install/scoop), confirming `agentmod version` matches the tag.
