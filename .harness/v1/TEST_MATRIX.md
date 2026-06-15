# TEST_MATRIX — per-deliverable verification and status

A deliverable is COMPLETE only when its checks pass. "Local" checks run on the
dev box now; "CI" checks are authoritative on the release pipeline (the dev box
lacks goreleaser/shellcheck).

| # | Area | Must verify | How | Status |
|---|------|------------|-----|--------|
| V01 | Module rename | 0 remaining `github.com/agentmod/agentmod` refs in `.go`/`go.mod`; build/vet/test green; gofmt clean | `go build/vet/test ./…`, `gofmt -l .`, grep | ✅ local |
| V02 | Version inject | ldflags value shown when injected | `go build -ldflags "-X …Version=v1.2.3"` → `agentmod version` = `v1.2.3` | ✅ local |
| V03 | Version fallback | untagged build resolves a non-empty, non-`(devel)` version; ldflags still wins | `TestResolveVersion`; default `go build` prints pseudo-version | ✅ local |
| V04 | goreleaser config | schema valid; 6 archives + checksums + formula + manifest | YAML parse local; `goreleaser check` + `--snapshot` | ✅ local YAML / ⏳ CI check |
| V05 | release workflow | valid workflow; tag-triggered; jobs ordered; secrets by name | YAML parse; review | ✅ local YAML / ⏳ CI run |
| V06 | npm launcher | resolves platform binary, forwards argv, mirrors exit code; clean error when absent | `node --check`; node_modules simulation (exit 7 propagated; missing-binary message) | ✅ local |
| V07 | npm build.mjs | reads artifacts.json; correct os/cpu; 0755 unix bit; version-stamped deps | `node --check`; dry-run with a fake artifacts.json | ✅ local |
| V08 | install.sh | parses; correct archive name; sha256 verify path | `sh -n`/`bash -n`; archive-name derivation; (full run ⏳ needs a published release) | ✅ local / ⏳ live |
| V09 | README | every channel documented with mojomoth URLs | review | ✅ |
| V10 | secret hygiene | no token value/name in any tracked or harness file; `.env*`/`npm/dist/` gitignored | `CHECKS.md §secret-scan`; grep harness | ✅ |
| V11 | channel smoke | `agentmod version` == tag via brew/npm/curl/go install/scoop | after publish (Handoff) | ⏳ live |
