# STATE — current status and exact next step

## Status: in-repo distribution work COMPLETE; verified locally.

All of DIST_PLAN "Deliverables" are implemented and committed on `main`. The
completion gate (CHECKS.md §7) passes locally. Remaining work is the human /
credential handoff (DIST_PLAN §Handoff), which is out of the loop's scope.

## Baseline (toolchain on the dev box)
- go 1.26.2, node v20.12.2, npm 10.5.0, jq present.
- goreleaser: NOT installed → `.goreleaser.yaml` validated as YAML only; CI is
  the authoritative `goreleaser check` gate (RISKS R1).
- shellcheck: NOT installed → `install.sh` validated via `sh -n`/`bash -n` only.

## What landed (commits on main, newest first)
- README per-channel install section.
- install.sh (curl|sh): platform detect, release download, sha256 verify.
- release workflow + npm packaging (launcher + build.mjs).
- .goreleaser.yaml (6 targets, archives, checksums, brews, scoops).
- Version build-info fallback + TestResolveVersion.
- chore: untrack stray agentmod.textClipping; ignore clippings + npm/dist + v1 reports.
- Module path rename agentmod/agentmod → mojomoth/agentmod (32 .go + go.mod).

## Verified locally
- go build/vet/test green; gofmt clean; 0 old-module-path refs.
- ldflags injection → `agentmod version` = the injected value; untagged build →
  Go pseudo-version (D6).
- build.mjs dry-run stages correct os/cpu packages, 0755 unix bit, version-
  stamped optionalDependencies; launcher forwards argv + exit code and prints a
  clean error when no platform binary is present.
- install.sh archive-name derivation matches the GoReleaser name_template.
- guard.sh blocks `.env*` edits/`git add`, global-home writes, sudo, HOME change;
  allows normal builds and `git add` of source.

## Exact next step (human, not the loop — DIST_PLAN §Handoff)
1. Create `mojomoth/homebrew-tap` and `mojomoth/scoop-bucket`.
2. Register CI secrets (cross-repo PAT + npm token) and secure the `@agentmod`
   npm scope (or change the SCOPE/MAIN_PKG constants).
3. `git remote add origin … && git push`, then push a `vX.Y.Z` tag.
4. Smoke-test brew/npm/curl/go install/scoop against the published release.

## Global-pollution audit
No global state touched this session (no writes to ~/.claude, ~/.codex,
~/.config/opencode, HOME, or global npm/brew config). The stray
`agentmod.textClipping` was untracked (left on disk) and is now gitignored.
