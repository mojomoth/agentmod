# RISKS — known risks and mitigations

- **R1. GoReleaser not installed on the dev box.** `goreleaser check` and
  `--snapshot` can't run locally, so the `.goreleaser.yaml` schema is validated
  only as YAML here. *Mitigation:* CI runs `goreleaser check` via
  goreleaser-action (`version: ~> v2`); a schema slip surfaces on first tag and
  is a one-line fix. If installing locally, run `goreleaser release --snapshot
  --clean` to confirm the six archives + checksums + formula/manifest.

- **R2. npm names may be unavailable.** The `agentmod` launcher name and the
  `@agentmod` org scope must be owned before publishing. *Mitigation:* names are
  isolated to two files (`npm/build.mjs` SCOPE/MAIN_PKG and
  `npm/agentmod/package.json`). Fallbacks: unscoped platform packages
  (`agentmod-<os>-<arch>`) or scope under `@mojomoth`.

- **R3. Cross-repo push needs a PAT.** The default Actions `GITHUB_TOKEN` cannot
  push to `homebrew-tap`/`scoop-bucket`. *Mitigation:* a PAT secret is exported
  as `GITHUB_TOKEN` for the goreleaser job (D7); documented in `release.yml` and
  DIST_PLAN §Handoff. Risk: PAT scope/expiry — maintainer-owned.

- **R4. CI network/tooling.** goreleaser-action pins `~> v2`; setup-go pins
  1.26; setup-node pins 20. A major bump in any could change behavior. Pins are
  explicit so upgrades are deliberate.

- **R5. windows/arm64 binary** is built and published but is the least-exercised
  target. Low blast radius (Scoop default is amd64); npm only installs it on a
  matching host.

- **R6. `go install` shows a pseudo-version for untagged installs.** Expected
  per D6; tagged installs (`@vX.Y.Z`) show the tag. Not a bug.

- **R7. Secret leakage.** The highest-severity risk. *Mitigation:* layered —
  gitignore (`.env*`), the PreToolUse guard (`hooks/guard.sh` blocks `.env*`
  edits and global-path writes), and `CHECKS.md §secret-scan` over every staged
  diff. Never paste a token into a command that writes to a tracked file.
