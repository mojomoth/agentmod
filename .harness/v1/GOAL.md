# GOAL — agentmod distribution & install (v1)

Make `agentmod` trivially installable through the package managers its users
already have, driven by a single release pipeline. `agentmod` is a pure-Go,
single-binary CLI (Go 1.26, CGO off, only dependency `BurntSushi/toml`), so it
cross-compiles cleanly and ships as one binary per platform.

Authoritative spec: `.harness/v1/DIST_PLAN.md`.

## Channels (all in scope)

1. **GitHub Releases** — the source of truth for every prebuilt binary,
   produced by GoReleaser on a `v*` tag.
2. **Homebrew** — `brew install mojomoth/tap/agentmod` (formula pushed to
   `mojomoth/homebrew-tap`).
3. **npm** — `npm install -g agentmod` using the esbuild model: a launcher
   package plus per-platform `optionalDependencies` (`@agentmod/cli-<os>-<arch>`);
   no postinstall network download.
4. **curl | sh** — `install.sh` downloads + sha256-verifies the right release
   archive.
5. **go install** — `go install github.com/mojomoth/agentmod@latest` (works once
   the module path matches the repo URL).
6. **Scoop (Windows)** — manifest pushed to `mojomoth/scoop-bucket`.

## Completion conditions (in-repo scope)

Completion = all of DIST_PLAN §"Deliverables" present and all of `CHECKS.md`
green. Concretely:

- Module path is `github.com/mojomoth/agentmod` across `go.mod` + every `.go`
  import; `go build/vet/test` pass, `gofmt` clean.
- `version` reports an ldflags value when injected and otherwise falls back to
  the Go build-info module version (so `go install …@vX` shows the tag).
- `.goreleaser.yaml` exists and is schema-valid (validated by `goreleaser check`
  where the binary is available; YAML-validated otherwise).
- `.github/workflows/release.yml` builds on `v*` and publishes Releases +
  Homebrew + Scoop, then publishes npm. Secrets are referenced **by name only**.
- npm packaging: `npm/agentmod` launcher + `npm/build.mjs` generator/publisher;
  `node --check` clean; launcher resolves the platform binary, forwards argv and
  exit code, and degrades with a clear message when no binary matches.
- `install.sh` passes `sh -n`/`bash -n`; archive name matches the GoReleaser
  `name_template`.
- `README.md` documents every channel.

## Out of scope (human / credential steps — see DIST_PLAN §"Handoff")

Creating the `homebrew-tap`/`scoop-bucket` repos, registering CI secrets,
securing the npm org scope, pushing tags, and the actual publish are external,
credential-bearing actions performed by the maintainer, not by this harness.

## Hard prohibitions

- Never write a GitHub token, npm token, or any credential value — or a path
  that reveals one — into any tracked file or any `.harness/v1/` file.
- Never commit `.env`, `.env.local`, or generated `npm/dist/` artifacts.
- Never change global state (HOME, global agent homes, global npm/brew config).
