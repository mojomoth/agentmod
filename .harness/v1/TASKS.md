# TASKS — small-unit checklist

Tick items as they land (with verification). One task ≈ one loop iteration or
less. Prefer finishing a 🟡 partial over starting a new item.

## Phase 0 — Harness
- [x] v1 harness docs (GOAL/DIST_PLAN/TASKS/TEST_MATRIX/CHECKS/LOOP/PROMPT/STATE/DONE/DECISIONS/RISKS)
- [x] loop.sh (max-iter cap, DONE sentinel + verification gate, reports/)
- [x] PreToolUse guard hook (`.env*` + global-path protection)
- [x] .gitignore covers `npm/dist/` and `.env*`

## Phase 1 — go install foundation
- [x] B1: rename module path `agentmod/agentmod` → `mojomoth/agentmod` (go.mod + 32 .go); build/vet/test green, gofmt clean
- [x] B6: `resolveVersion()` build-info fallback + `TestResolveVersion`; ldflags injection smoke-tested

## Phase 2 — release engine
- [x] B2: `.goreleaser.yaml` (6 targets, archives, checksums, brews→tap, scoops→bucket); YAML valid
- [x] B3: `.github/workflows/release.yml` (goreleaser job + npm-publish job; secret names only)

## Phase 3 — npm
- [x] B4: launcher `npm/agentmod` (package.json + bin/agentmod.js + README)
- [x] B4: `npm/build.mjs` generator/publisher reading `dist/artifacts.json`
- [x] B4: verify — `node --check`; build.mjs dry-run stages correct os/cpu + 0755; launcher forwards argv+exit code and degrades cleanly

## Phase 4 — curl|sh
- [x] B5: `install.sh` (platform detect, release download, sha256 verify, PATH note); `sh -n`/`bash -n` pass; archive name matches name_template

## Phase 5 — docs
- [x] B7: README per-channel install section with `mojomoth` URLs

## Phase 6 — completion
- [x] CHECKS §completion walked; local verification green; DONE written

## Out of scope (human / credential — DIST_PLAN §Handoff)
- [ ] Create `mojomoth/homebrew-tap` + `mojomoth/scoop-bucket`
- [ ] Register CI secrets (cross-repo PAT, npm token) + secure `@agentmod` scope
- [ ] Push `main` + a `vX.Y.Z` tag; smoke-test every published channel
