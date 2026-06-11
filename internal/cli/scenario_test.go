package cli

// T30 — required user scenarios (FABLE_PLAN §27.1–§27.4): the isolation
// matrix across three sibling folders, driven through ONE real shell session
// per supported shell. Mock claude/codex/opencode executables on the child
// PATH report which home each agent would resolve, following the real
// binaries' resolution rules (CLAUDE_CONFIG_DIR / CODEX_HOME falling back to
// $HOME, OPENCODE_CONFIG with no fallback) — real agent installs are never
// required or touched (GOAL quality bar).
//
// §27.5 (TestScenarioHandoffRoundTrip) and §27.6 (TestScenarioGitHandoff)
// are cli-level: the handoff clauses are about package content and
// continuation, not shell routing, and their backup/format mechanics are
// already pinned by handoff_test.go / restore_test.go / gitpack_test.go —
// these two tests pin the §27 framing (continuation-vs-auth split; all
// five git-exclusion categories in one run) end to end.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/handoff"
)

// mockAgentBins writes fake claude/codex/opencode executables into a fresh
// dir, ready to append to a child shell's PATH. The claude mock also lists
// the skills visible in its resolved home, which is what makes plugin
// (in)visibility observable per scenario.
func mockAgentBins(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("claude", `dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
printf 'claude-home:%s\n' "$dir"
for s in "$dir/skills"/*; do
  [ -e "$s" ] && printf 'claude-skill:%s\n' "$(basename "$s")"
done
`)
	write("codex", `printf 'codex-home:%s\n' "${CODEX_HOME:-$HOME/.codex}"
`)
	write("opencode", `printf 'opencode-config:%s\n' "${OPENCODE_CONFIG-unset}"
`)
	return dir
}

// scenarioSection returns the stdout text between ===<tag>=== and
// ===END<tag>=== markers.
func scenarioSection(t *testing.T, stdout, tag string) string {
	t.Helper()
	begin, end := "==="+tag+"===\n", "===END"+tag+"==="
	_, rest, ok := strings.Cut(stdout, begin)
	if !ok {
		t.Fatalf("marker %s not found in:\n%s", begin, stdout)
	}
	sec, _, ok := strings.Cut(rest, end)
	if !ok {
		t.Fatalf("marker %s not found in:\n%s", end, stdout)
	}
	return sec
}

// prefixedLines collects the remainders of every line starting with prefix,
// in output order.
func prefixedLines(text, prefix string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			out = append(out, strings.TrimPrefix(line, prefix))
		}
	}
	return out
}

func TestScenarioIsolationMatrix(t *testing.T) {
	for _, sh := range shellCases() {
		t.Run(sh.name, func(t *testing.T) {
			// The child session's HOME: a fake global Claude home whose
			// skills dir holds the §27.1 "globally installed superpowers
			// plugin". The parent process HOME is never reassigned.
			fakeHome := t.TempDir()
			globalClaude := filepath.Join(fakeHome, ".claude")
			globalSkills := filepath.Join(globalClaude, "skills")
			if err := os.MkdirAll(filepath.Join(globalSkills, "superpowers"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(globalSkills, "superpowers", "SKILL.md"), []byte("# superpowers fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			proj00 := t.TempDir() // plain folder, never init'd (§27.1)
			proj01 := t.TempDir() // init'd + gstack IN the session (§27.2–.3)
			proj02 := t.TempDir() // plain folder next to proj01 (§27.4)
			fixtureRepo := makeGstackFixtureRepo(t)
			binDir := fakeAgentmodBin(t)
			mockBin := mockAgentBins(t)
			startDir := t.TempDir()

			env := []string{
				"PATH=" + strings.Join([]string{binDir, mockBin, "/usr/bin", "/bin"}, sep),
				"HOME=" + fakeHome,
				"TERM=dumb",
				gstackSourceEnvVar + "=" + fixtureRepo,
				// Mask the developer's git config for the clone the session
				// runs (same discipline as runGitFixture).
				"GIT_CONFIG_GLOBAL=" + os.DevNull,
				"GIT_CONFIG_SYSTEM=" + os.DevNull,
			}

			// Plain folders and the global Claude home must come through the
			// whole session untouched (§27.1/§27.3/§27.4, §28 no-shims).
			watched := []string{proj00, proj02, globalClaude}
			before := make([]map[string]string, len(watched))
			for i, dir := range watched {
				before[i] = snapshotTree(t, dir)
			}

			script := strings.Join([]string{
				`eval "$(agentmod hook ` + sh.name + `)"`,
				// §27.1 proj00: default global Claude, superpowers active.
				sh.cd(proj00),
				`printf '%s\n' "===P00==="`,
				`claude`,
				`codex`,
				`opencode`,
				`printf '%s\n' "===ENDP00==="`,
				// §27.2 proj01: init non-interactively (must not stall at a
				// login prompt), then re-enter to activate routing.
				sh.cd(proj01),
				`agentmod init --no-shell-hook --yes`,
				`printf 'init-rc:%s\n' "$?"`,
				sh.cd("/"),
				sh.cd(proj01),
				// §27.3: gstack installs project-local only.
				`agentmod install gstack`,
				`printf 'install-rc:%s\n' "$?"`,
				`printf '%s\n' "===P01==="`,
				`claude`,
				`codex`,
				`opencode`,
				`printf 'xdg-config:%s\n' "${XDG_CONFIG_HOME-unset}"`,
				`printf '%s\n' "===ENDP01==="`,
				// §27.4 proj02: back to global defaults; proj01 leaks nothing.
				sh.cd(proj02),
				`printf '%s\n' "===P02==="`,
				`claude`,
				`codex`,
				`opencode`,
				`printf '%s\n' "===ENDP02==="`,
			}, "\n") + "\n"

			stdout, stderr := sh.run(t, startDir, env, false, script)
			if stderr != "" {
				t.Errorf("unexpected stderr:\n%s", stderr)
			}

			// Both commands the session ran must have succeeded, and init
			// must have reported auth guidance instead of stalling (§12).
			for _, rc := range []string{"init-rc:", "install-rc:"} {
				if got := lineAfter(t, stdout, rc); got != "0" {
					t.Errorf("%s%s, want 0", rc, got)
				}
			}
			wantContains(t, "init auth guidance", stdout, "Claude auth:", "Codex auth:")
			wantContains(t, "install pollution check", stdout,
				"Global skills check: "+globalSkills+" unchanged")

			am := filepath.Join(proj01, ".agentmod")
			cases := []struct {
				tag      string
				claude   string
				codex    string
				opencode string
				skills   []string
			}{
				{"P00", globalClaude, filepath.Join(fakeHome, ".codex"), "unset", []string{"superpowers"}},
				{"P01", filepath.Join(am, "claude"), filepath.Join(am, "codex"), filepath.Join(am, "opencode", "opencode.json"), []string{"gstack"}},
				{"P02", globalClaude, filepath.Join(fakeHome, ".codex"), "unset", []string{"superpowers"}},
			}
			for _, tc := range cases {
				sec := scenarioSection(t, stdout, tc.tag)
				if got := lineAfter(t, sec, "claude-home:"); got != tc.claude {
					t.Errorf("%s claude-home = %q, want %q", tc.tag, got, tc.claude)
				}
				if got := lineAfter(t, sec, "codex-home:"); got != tc.codex {
					t.Errorf("%s codex-home = %q, want %q", tc.tag, got, tc.codex)
				}
				if got := lineAfter(t, sec, "opencode-config:"); got != tc.opencode {
					t.Errorf("%s opencode-config = %q, want %q", tc.tag, got, tc.opencode)
				}
				// The skill matrix: superpowers visible ONLY via the global
				// home, gstack ONLY inside proj01 — never both anywhere.
				if got := prefixedLines(sec, "claude-skill:"); !reflect.DeepEqual(got, tc.skills) {
					t.Errorf("%s claude skills = %v, want %v", tc.tag, got, tc.skills)
				}
			}

			// Partial isolation (§15.3): XDG stays untouched inside proj01.
			if got := lineAfter(t, scenarioSection(t, stdout, "P01"), "xdg-config:"); got != "unset" {
				t.Errorf("XDG_CONFIG_HOME inside proj01 = %q, want unset", got)
			}

			// §27.3 on disk: gstack exists project-local with the fixture
			// content, and the global skills dir still lists only superpowers.
			skillData, err := os.ReadFile(filepath.Join(am, "claude", "skills", "gstack", "SKILL.md"))
			if err != nil {
				t.Fatalf("project-local gstack missing: %v", err)
			}
			if string(skillData) != "# gstack fixture\n" {
				t.Errorf("gstack SKILL.md = %q, want fixture content", skillData)
			}
			entries, err := os.ReadDir(globalSkills)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			if !reflect.DeepEqual(names, []string{"superpowers"}) {
				t.Errorf("global skills after session = %v, want [superpowers] only", names)
			}

			// proj00, proj02 and the global Claude home are byte-identical
			// to before the session.
			for i, dir := range watched {
				diffTrees(t, dir, before[i], snapshotTree(t, dir))
			}
		})
	}
}

// writeAgentmodFixture writes content files (rel → bytes) under
// <root>/.agentmod, creating parent dirs as needed.
func writeAgentmodFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, ".agentmod", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScenarioHandoffRoundTrip(t *testing.T) {
	// §27.5 — create a handoff package on "machine A", restore it on
	// "machine B" over the same git checkout (git moves the source; the
	// .amod moves the agent env). Config, the gstack skill, MCP config,
	// and working context continue; auth never travels; re-login guidance
	// prints.
	srcRoot := makeProject(t, config.Default())

	// What §27.5 says must continue "to the extent possible"…
	travels := map[string]string{
		filepath.Join("claude", "CLAUDE.md"):                         "machine A claude instructions\n",
		filepath.Join("claude", "skills", "gstack", "SKILL.md"):      "# gstack fixture\n",
		filepath.Join("claude", "projects", "demo", "session.jsonl"): `{"role":"user","text":"context from machine A"}` + "\n",
		// MCP server with a relative command — ports cleanly (D044).
		filepath.Join("codex", "config.toml"):      "[mcp_servers.demo]\ncommand = \"npx\"\nargs = [\"-y\", \"demo-mcp\"]\n",
		filepath.Join("opencode", "opencode.json"): "{}\n",
	}
	// …and what must never travel (§18).
	authRel := []string{
		filepath.Join("claude", ".credentials.json"),
		filepath.Join("codex", "auth.json"),
	}
	writeAgentmodFixture(t, srcRoot, travels)
	for _, rel := range authRel {
		writeAgentmodFixture(t, srcRoot, map[string]string{rel: `{"token":"sk-FAKE-fixture"}`})
	}

	output := filepath.Join(t.TempDir(), "machine-a.amod")
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(srcRoot, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("create exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "create stdout", stdout,
		"excluded by default policy: 2 entries",
		".agentmod/claude/.credentials.json (auth-file)",
		".agentmod/codex/auth.json (auth-file)",
	)

	// "Machine B": the same checkout, so the project (with its own init'd
	// .agentmod) already exists; only the agent env arrives via agentmod.
	dstRoot := makeProject(t, config.Default())
	code, stdout, stderr = runHandoffForTest(t, fakeEnv(dstRoot, nil), "restore", output)
	if code != ExitOK {
		t.Fatalf("restore exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}

	for rel, content := range travels {
		data, err := os.ReadFile(filepath.Join(dstRoot, ".agentmod", rel))
		if err != nil || string(data) != content {
			t.Errorf("%s did not continue on machine B: %q, %v", rel, data, err)
		}
	}
	for _, rel := range authRel {
		if _, err := os.Lstat(filepath.Join(dstRoot, ".agentmod", rel)); !os.IsNotExist(err) {
			t.Errorf("auth file %s present on machine B (err = %v), want absent", rel, err)
		}
	}
	wantContains(t, "restore stdout", stdout,
		"Re-login (auth and credentials never travel in a snapshot",
		"Claude Code: "+handoff.ClaudeReloginRemedy+".",
		"Codex CLI: "+handoff.CodexReloginRemedy+".",
		"portability: no foreign absolute paths in restored agent configs",
	)

	// Restore touched nothing at the project root beyond .agentmod and its
	// backup (machine B's checkout here is not a git repo, so no .gitignore
	// appears either).
	entries, err := os.ReadDir(dstRoot)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	wantNames := []string{".agentmod", ".agentmod.backup-" + fakeNow.Format("20060102-150405")}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("machine B project root after restore = %v, want %v", names, wantNames)
	}
}

func TestScenarioGitHandoff(t *testing.T) {
	// §27.6 — `handoff create --for-git` produces a git-storable tree under
	// .agentmod-handoff/ excluding source code, secrets, auth, sessions,
	// and logs by default: all five categories pinned in one run. (Format
	// mechanics live in gitpack_test.go; the `pack --for-git` spelling in
	// TestPackForGitAlias.)
	root := makeProject(t, config.Default())
	// Source code lives at the project root — git's job, structurally
	// outside the payload.
	for name, content := range map[string]string{
		"main.go":   "package main\n",
		"README.md": "# demo\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := map[string]string{
		filepath.Join("claude", "CLAUDE.md"):  "instructions travel\n",
		filepath.Join("codex", "config.toml"): "[mcp_servers.demo]\ncommand = \"npx\"\n",
	}
	drop := map[string]string{
		filepath.Join("claude", ".env"):                              "API_KEY=sk-FAKE-fixture\n",   // secrets
		filepath.Join("claude", ".credentials.json"):                 `{"token":"sk-FAKE-fixture"}`, // auth
		filepath.Join("codex", "auth.json"):                          `{"token":"sk-FAKE-fixture"}`, // auth
		filepath.Join("claude", "projects", "demo", "session.jsonl"): `{"text":"history"}` + "\n",   // sessions
		filepath.Join("logs", "agentmod.log"):                        "log line\n",                  // logs
	}
	writeAgentmodFixture(t, root, keep)
	writeAgentmodFixture(t, root, drop)

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--for-git")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		".agentmod/claude/.env (env-file)",
		".agentmod/claude/.credentials.json (auth-file)",
		".agentmod/codex/auth.json (auth-file)",
		".agentmod/claude/projects/ (session-data)",
		".agentmod/logs/ (log-data)",
	)

	target := filepath.Join(root, handoff.GitDirName)
	var m handoff.Manifest
	data, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if !m.ForGit {
		t.Error("manifest for_git = false, want true")
	}

	// The packed payload carries the agent env and NOTHING else: exactly
	// the two keepers plus the project's agentmod.toml, no source file, no
	// member of any excluded category.
	payload := filepath.Join(target, "payload")
	var rels []string
	err = fs.WalkDir(os.DirFS(payload), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rels = append(rels, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRels := []string{
		".agentmod/agentmod.toml",
		".agentmod/claude/CLAUDE.md",
		".agentmod/codex/config.toml",
	}
	if !reflect.DeepEqual(rels, wantRels) {
		t.Errorf("payload files = %v, want %v", rels, wantRels)
	}
}
