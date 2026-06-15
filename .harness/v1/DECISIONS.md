# DECISIONS — settled choices (do not re-litigate)

- **D1. Engine = GoReleaser + GitHub Actions.** One tag-triggered pipeline
  yields binaries, archives, checksums, the GitHub Release, the Homebrew formula,
  and the Scoop manifest. Rationale: it is the de-facto standard for Go CLIs and
  removes hand-rolled cross-compilation.

- **D2. Release host = GitHub Releases**, repo `github.com/mojomoth/agentmod`.

- **D3. Channels = brew + npm + curl|sh + go install + scoop.** curl|sh + brew +
  go install are the standard Go-CLI core; npm is included because the audience
  (Claude Code / Codex / OpenCode users) lives in Node. Docker is intentionally
  excluded — agentmod manipulates the local shell/home, which a container cannot.

- **D4. npm = esbuild model (per-platform `optionalDependencies`).** No
  postinstall download → offline/firewall/CI friendly, install pulls only the
  one matching binary. Rejected: a postinstall script fetching from Releases
  (fails in sandboxed installs).

- **D5. Module path renamed to `github.com/mojomoth/agentmod`.** Required for
  `go install`. Rejected: keeping `agentmod/agentmod` via a vanity import
  (needs a domain + redirect) or creating an `agentmod` GitHub org (extra
  infra). 32 `.go` files + `go.mod` were rewritten.

- **D6. Version fallback via `debug.ReadBuildInfo()`.** ldflags wins; otherwise
  the embedded module version; otherwise the dev sentinel. Consequence: a plain
  `go build` from a git checkout now prints a `v0.0.0-<pseudo>+dirty` version
  instead of `0.1.0-dev` — accepted as more informative and standard for Go.

- **D7. Cross-repo publishing token passed as `GITHUB_TOKEN` in the workflow**,
  so `.goreleaser.yaml` contains no token reference. The default Actions
  `GITHUB_TOKEN` only reaches the current repo; the tap and bucket are separate
  repos, hence a PAT.

- **D8. npm scope `@agentmod`, launcher name `agentmod` (unscoped).** Defined as
  constants in `npm/build.mjs` (`SCOPE`), `npm/agentmod/bin/agentmod.js`
  (`SCOPE`), and the optionalDependencies in `npm/agentmod/package.json`. The
  user `mojomoth` owns the npm org `@agentmod` (verified via `npm org ls
  agentmod` → owner), so platform packages publish as `@agentmod/cli-<os>-<arch>`
  and the main package stays the unscoped `agentmod`. (A brief detour to
  `@mojomoth` was reverted once org ownership was confirmed.) To change scope,
  edit those three places.
  - **Publish requires a 2FA-exempt token.** The account enforces 2FA on writes;
    the npm token must report `bypass_2fa: true` (check via GET
    `registry.npmjs.org/-/npm/v1/tokens`). Otherwise pass a one-time code through
    `AGENTMOD_NPM_OTP` to `build.mjs`.

- **D9. Goreleaser schema targets current v2** (`formats:` list,
  `brews:`/`scoops:`). The local dev box has no `goreleaser`; `goreleaser check`
  in CI is the authoritative schema gate (see RISKS R1).

- **D10. Secrets never enter the harness or tracked files.** The GitHub token
  lives only in the gitignored `.env.local` (and CI secrets). No harness file
  names or echoes it. `.env`/`.env.local`/`npm/dist/` are gitignored; the guard
  blocks edits to `.env*` and `CHECKS.md` scans every commit.
