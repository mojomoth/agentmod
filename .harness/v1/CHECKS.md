# CHECKS — run every iteration

Run at the START of each iteration (cheap, fail fast) and again before
finishing. Record failures in `STATE.md`.

## 1. Go build & tests

```sh
go build ./...
go vet ./...
go test ./...
gofmt -l .          # must print nothing
```

## 2. Module-path consistency

```sh
# Want 0: the old module path must be fully gone from code.
grep -rn 'github.com/agentmod/agentmod' --include='*.go' . go.mod | wc -l
head -1 go.mod      # module github.com/mojomoth/agentmod
```

## 3. Packaging artifacts syntax

```sh
node --check npm/agentmod/bin/agentmod.js
node --check npm/build.mjs
sh -n install.sh && bash -n install.sh
# YAML well-formedness (goreleaser/workflow):
python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml')); yaml.safe_load(open('.github/workflows/release.yml'))"
# Authoritative schema check when available:
command -v goreleaser >/dev/null && goreleaser check || echo "goreleaser absent — CI is the gate (RISKS R1)"
command -v shellcheck >/dev/null && shellcheck install.sh || echo "shellcheck absent — sh -n/bash -n only"
```

## 4. Secret hygiene (BLOCKING — before any commit)

```sh
# No credential-looking content staged:
git diff --cached | grep -inE 'gh[pousr]_[A-Za-z0-9]{20,}|github_pat_|npm_[A-Za-z0-9]{20,}|api[_-]?key|secret|BEGIN .*PRIVATE KEY' || true
# .env files must never be staged:
git diff --cached --name-only | grep -E '(^|/)\.env(\.|$)' && echo "VIOLATION: .env staged" || true
# No hardcoded credential assignment (TOKEN/KEY/SECRET/PASSWORD = literal value;
# `=${...}` env references are fine):
git diff --cached | grep -inE '(TOKEN|KEY|SECRET|PASSWORD)[A-Z_]*=[^$"'"'"' )]' || true
```

Any hit is a hard stop. Fixtures must use obviously-fake values.

## 5. Repo hygiene

```sh
git status --short
grep -q '^npm/dist/$' .gitignore || echo "VIOLATION: npm/dist not gitignored"
grep -q '^\.env\.local$' .gitignore || echo "VIOLATION: .env.local not gitignored"
```

## 6. Harness integrity

```sh
test -x .harness/v1/hooks/guard.sh && test -x .harness/v1/loop.sh || echo "VIOLATION: harness scripts not executable"
```

## 7. Completion gate (only when about to declare DONE)

Walk GOAL.md "Completion conditions" line by line — every one true. Walk
GOAL.md "Hard prohibitions" — every one false. Then run §1 + §3 + §4 once more
and paste the summary into `DONE.md`. `loop.sh` independently re-verifies.
