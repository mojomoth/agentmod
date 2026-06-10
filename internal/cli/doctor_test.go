package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/routing"
)

// mkLayout creates the init-managed directories under root/.agentmod plus
// the opencode.json stub and the guard-wired claude/settings.json (built for
// fakeBinPath, fakeEnv's Executable answer), so the layout, OpenCode-config,
// and Claude-guard checks pass — matching what init guarantees.
func mkLayout(t *testing.T, root string) {
	t.Helper()
	for _, sub := range layout.Subdirs() {
		if err := os.MkdirAll(filepath.Join(root, ".agentmod", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub := layout.OpencodeConfigPath(filepath.Join(root, ".agentmod"))
	if err := os.WriteFile(stub, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGuardSettings(t, root, guardHookCommand(fakeBinPath))
}

// writeGuardSettings (over)writes root/.agentmod/claude/settings.json with
// exactly one guard hook running command.
func writeGuardSettings(t *testing.T, root, command string) {
	t.Helper()
	data, err := marshalSettings(freshGuardSettings(command))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guardSettingsPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func guardSettingsPath(root string) string {
	return filepath.Join(root, ".agentmod", layout.ClaudeDir, claudeSettingsFile)
}

// healthyVars builds the env-var map of a shell where the zsh hook is
// installed (current block in $HOME/.zshrc) and routing is correctly applied
// for root with the default config. Tests break individual pieces from here.
func healthyVars(t *testing.T, root string) map[string]string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(rcBlockFor("zsh")), 0o644); err != nil {
		t.Fatal(err)
	}
	agentmodDir := filepath.Join(root, ".agentmod")
	vars := map[string]string{
		"SHELL":                 "/bin/zsh",
		"HOME":                  home,
		"PATH":                  routing.NodeBinDir(agentmodDir) + sep + "/usr/bin",
		"AGENTMOD_ACTIVE":       "1",
		"AGENTMOD_PROJECT_ROOT": root,
	}
	for _, v := range routing.Vars(agentmodDir, config.Default()) {
		vars[v.Name] = v.Value
	}
	return vars
}

func runDoctorForTest(t *testing.T, env Env) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run([]string{"doctor"}, &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

// findingLine returns the doctor output line for a label, including its
// level column, so tests can assert the severity and not just the text.
func findingLine(t *testing.T, stdout, label string) string {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "  "+label+": ") {
			return line
		}
	}
	t.Fatalf("no %q finding in doctor output:\n%s", label, stdout)
	return ""
}

func wantLevel(t *testing.T, line, level string) {
	t.Helper()
	if !strings.HasPrefix(line, strings.Repeat(" ", 5-len(level))+level+"  ") {
		t.Errorf("finding level = %q line, want level %q", line, level)
	}
}

func TestDoctorAllHealthy(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	code, stdout, stderr := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:%s", code, ExitOK, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "doctor: all checks passed")
	wantLevel(t, findingLine(t, stdout, "Project"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Config"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Layout"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Shell"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Shell hook"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Routing env"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Shims"), diagOK)
	wantLevel(t, findingLine(t, stdout, "PATH"), diagOK)
	wantLevel(t, findingLine(t, stdout, "HOME"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Claude home"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Claude guard"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Codex home"), diagOK)
	wantLevel(t, findingLine(t, stdout, "OpenCode config"), diagOK)
	wantLevel(t, findingLine(t, stdout, "OpenCode sessions"), diagOK)
	wantLevel(t, findingLine(t, stdout, "OpenCode merge chain"), diagOK)
	wantLevel(t, findingLine(t, stdout, "Agent binaries"), diagOK)
	wantLevel(t, findingLine(t, stdout, "gstack (global)"), diagOK)
	wantLevel(t, findingLine(t, stdout, "gstack (project)"), diagOK)
	wantContains(t, "Project line", findingLine(t, stdout, "Project"), root, filepath.Join(root, ".agentmod"))
	wantContains(t, "Routing line", findingLine(t, stdout, "Routing env"), "applied for this project")
	wantContains(t, "PATH line", findingLine(t, stdout, "PATH"), "on PATH once")
}

func TestDoctorOutsideProjectFreshMachineIsClean(t *testing.T) {
	// No project, no rc file, no routing vars: nothing is wrong yet, so a
	// fresh machine must exit 0 (not-installed is informational out here).
	home := t.TempDir()
	env := fakeEnv(t.TempDir(), map[string]string{"SHELL": "/bin/zsh", "HOME": home})
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantContains(t, "stdout", stdout, "doctor: all checks passed", "not inside an agentmod project")
	hook := findingLine(t, stdout, "Shell hook")
	wantLevel(t, hook, diagOK)
	wantContains(t, "Shell hook line", hook, "not installed", "run 'agentmod init' inside a project")
	// Outside a project the routing check is the lingering-vars audit.
	lingering := findingLine(t, stdout, "Routing env")
	wantLevel(t, lingering, diagOK)
	wantContains(t, "Routing line", lingering, "no agentmod variables lingering")
	wantLevel(t, findingLine(t, stdout, "HOME"), diagOK)
	// The guard lives in a project's routed Claude home — no project, no line.
	wantNoFinding(t, stdout, "Claude guard")
	// Binary presence is reported out here too (§23), informationally.
	binaries := findingLine(t, stdout, "Agent binaries")
	wantLevel(t, binaries, diagOK)
	wantContains(t, "Agent binaries line", binaries, "claude not found on PATH")
}

func TestDoctorHookInstalledButInactive(t *testing.T) {
	// §23: "Shell hook installed but inactive in the current shell".
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	for k := range vars {
		if strings.HasPrefix(k, "AGENTMOD") {
			delete(vars, k)
		}
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Routing env")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Routing line", line,
		"shell hook installed but not active in this shell",
		"exec $SHELL",
		`eval "$(agentmod hook zsh)"`,
	)
	// The hook itself is fine — only the env warns.
	wantLevel(t, findingLine(t, stdout, "Shell hook"), diagOK)
	wantContains(t, "stdout", stdout, "doctor: 1 problem(s) found")
}

func TestDoctorInsideProjectNoHookNoEnv(t *testing.T) {
	// §23: "Inside an agentmod project but required env vars unset".
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	env := fakeEnv(root, map[string]string{"SHELL": "/bin/zsh", "HOME": t.TempDir()})
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	hook := findingLine(t, stdout, "Shell hook")
	wantLevel(t, hook, diagWarn)
	wantContains(t, "Shell hook line", hook, "not installed", "run 'agentmod init'")
	routingLine := findingLine(t, stdout, "Routing env")
	wantLevel(t, routingLine, diagWarn)
	wantContains(t, "Routing line", routingLine, "no shell hook is installed")
	wantContains(t, "stdout", stdout, "doctor: 2 problem(s) found")
}

func TestDoctorRoutingAppliedForOtherProject(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	vars["AGENTMOD_PROJECT_ROOT"] = "/somewhere/else"
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Routing env")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Routing line", line, "applied for a different project (/somewhere/else)")
}

func TestDoctorMisroutedVars(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	vars["CLAUDE_CONFIG_DIR"] = "/wrong/place"
	delete(vars, "CODEX_HOME")
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Routing env")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Routing line", line,
		"do not match the expected paths",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME",
		"cd out of the project and back in",
	)
	if strings.Contains(line, "OPENCODE_CONFIG") {
		t.Errorf("correctly-routed var reported as misrouted:\n%s", line)
	}
}

func TestDoctorLayoutMissingDirs(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	for _, sub := range []string{"claude", "snapshots"} {
		if err := os.RemoveAll(filepath.Join(root, ".agentmod", sub)); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Layout")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Layout line", line, "missing under .agentmod/", "claude", "snapshots", "re-run 'agentmod init'")
	if strings.Contains(line, "codex") {
		t.Errorf("present directory reported missing:\n%s", line)
	}
}

func TestDoctorLayoutEntryNotADirectory(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	claude := filepath.Join(root, ".agentmod", "claude")
	if err := os.RemoveAll(claude); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Layout")
	wantLevel(t, line, diagError)
	wantContains(t, "Layout line", line, "not a directory under .agentmod/: claude")
}

func TestDoctorBrokenConfigStillRunsOtherChecks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "agentmod.toml")
	if err := os.WriteFile(cfgPath, []byte("schema_version = 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkLayout(t, root)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Config")
	wantLevel(t, line, diagError)
	wantContains(t, "Config line", line, cfgPath)
	// Doctor reports and continues — the remaining checks still appear.
	wantLevel(t, findingLine(t, stdout, "Shell hook"), diagOK)
	// Without a loadable config the per-var match is skipped, but the basic
	// classification still works.
	wantContains(t, "Routing line", findingLine(t, stdout, "Routing env"), "applied for this project")
}

func TestDoctorStaleRCBlock(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	stale := strings.Replace(rcBlockFor("zsh"), "agentmod hook zsh", "agentmod hook-old zsh", 1)
	if err := os.WriteFile(filepath.Join(vars["HOME"], ".zshrc"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Shell hook")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Shell hook line", line, "outdated", "re-run 'agentmod init'")
}

func TestDoctorCorruptRCFence(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	rc := filepath.Join(vars["HOME"], ".zshrc")
	if err := os.WriteFile(rc, []byte(rcBeginMarker+"\nno end marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Shell hook")
	wantLevel(t, line, diagError)
	wantContains(t, "Shell hook line", line, rc, "start marker but no")
}

func TestDoctorShellUndetectable(t *testing.T) {
	// Inside a project the hook can never activate → warn; outside it is
	// merely informational.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, nil))
	if code != ExitValidation {
		t.Fatalf("inside: exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Shell hook")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Shell hook line", line, "$SHELL is not set")

	code, stdout, _ = runDoctorForTest(t, fakeEnv(t.TempDir(), nil))
	if code != ExitOK {
		t.Fatalf("outside: exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantLevel(t, findingLine(t, stdout, "Shell hook"), diagOK)
}

func TestDoctorLingeringVarsOutsideProject(t *testing.T) {
	// §23: "agentmod env vars lingering in a folder without .agentmod" —
	// bookkeeping vars, saved values, routed values pointing into an
	// .agentmod, and the managed PATH entry must all be flagged.
	env := fakeEnv(t.TempDir(), map[string]string{
		"SHELL":                            "/bin/zsh",
		"HOME":                             t.TempDir(),
		"AGENTMOD_ACTIVE":                  "1",
		"AGENTMOD_VARS":                    "CLAUDE_CONFIG_DIR",
		"CLAUDE_CONFIG_DIR":                "/gone/.agentmod/claude",
		"AGENTMOD_SAVED_CLAUDE_CONFIG_DIR": "/users/me/custom",
		"PATH":                             "/gone/.agentmod/node/bin" + sep + "/usr/bin",
	})
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Routing env")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Routing line", line,
		"lingering outside any project",
		"AGENTMOD_ACTIVE", "AGENTMOD_VARS",
		"CLAUDE_CONFIG_DIR", "AGENTMOD_SAVED_CLAUDE_CONFIG_DIR",
		"PATH entry /gone/.agentmod/node/bin",
		`eval "$(agentmod env --shell zsh --deactivate)"`,
	)
}

func TestDoctorOutsideProjectUsersOwnVarsAreNotLingering(t *testing.T) {
	// A routed-name variable the USER set (no .agentmod in its value) is
	// their own business — silence, exit 0.
	env := fakeEnv(t.TempDir(), map[string]string{
		"SHELL":             "/bin/zsh",
		"HOME":              t.TempDir(),
		"CLAUDE_CONFIG_DIR": "/users/me/my-own-claude-home",
		"PATH":              "/usr/bin" + sep + "/bin",
	})
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantContains(t, "Routing line", findingLine(t, stdout, "Routing env"), "no agentmod variables lingering")
}

func TestDoctorDuplicatePathEntries(t *testing.T) {
	// §23: "Duplicate agentmod PATH entries".
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	binDir := routing.NodeBinDir(filepath.Join(root, ".agentmod"))
	vars["PATH"] = binDir + sep + "/usr/bin" + sep + binDir
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "PATH")
	wantLevel(t, line, diagWarn)
	wantContains(t, "PATH line", line, "appears 2 times", "open a new terminal")
}

func TestDoctorNodeBinMissingFromPathWhileActive(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	vars["PATH"] = "/usr/bin" + sep + "/bin"
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "PATH")
	wantLevel(t, line, diagWarn)
	wantContains(t, "PATH line", line, "missing from PATH while routing is active")

	// Negative: same PATH while routing is NOT applied is expected — that
	// state is routingFinding's warning, not PATH's.
	for k := range vars {
		if strings.HasPrefix(k, "AGENTMOD") {
			delete(vars, k)
		}
	}
	_, stdout, _ = runDoctorForTest(t, fakeEnv(root, vars))
	line = findingLine(t, stdout, "PATH")
	wantLevel(t, line, diagOK)
	wantContains(t, "PATH line", line, "no agentmod entries")
}

func TestDoctorForeignAgentmodPathEntry(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	vars["PATH"] = vars["PATH"] + sep + "/elsewhere/.agentmod/node/bin"
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "PATH")
	wantLevel(t, line, diagWarn)
	wantContains(t, "PATH line", line, "another .agentmod", "/elsewhere/.agentmod/node/bin")
}

func TestDoctorHomeChanged(t *testing.T) {
	// §23: "HOME changed". agentmod never saves or routes HOME, so either
	// signal means some other tool tampered with the environment.
	root := makeProject(t, config.Default())
	mkLayout(t, root)

	vars := healthyVars(t, root)
	vars["AGENTMOD_SAVED_HOME"] = "/users/me"
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("saved-home: exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "HOME")
	wantLevel(t, line, diagWarn)
	wantContains(t, "HOME line", line, "AGENTMOD_SAVED_HOME", "never saves or changes HOME")

	vars = healthyVars(t, root)
	vars["HOME"] = filepath.Join(root, ".agentmod", "home")
	code, stdout, _ = runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("home-inside: exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line = findingLine(t, stdout, "HOME")
	wantLevel(t, line, diagWarn)
	wantContains(t, "HOME line", line, "points inside an .agentmod directory")
}

func TestDoctorShimDetected(t *testing.T) {
	// §23: "Shim detected" — agent-named entries in the managed node/bin
	// that are not npm-style symlinks into .agentmod shadow the real
	// binaries.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	binDir := filepath.Join(root, ".agentmod", "node", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A script shim and a symlink escaping .agentmod: both warn.
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexec /usr/bin/true\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(outside, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(binDir, "codex")); err != nil {
		t.Fatal(err)
	}
	// An npm-style project-local install: symlink into .agentmod — fine.
	target := filepath.Join(root, ".agentmod", "node", "lib", "node_modules", "opencode", "cli.js")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(binDir, "opencode")); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Shims")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Shims line", line, "claude", "codex", "never creates shims")
	if strings.Contains(line, "opencode") {
		t.Errorf("project-local npm install reported as a shim:\n%s", line)
	}
}

func TestDoctorProjectLocalInstallIsNotAShim(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	binDir := filepath.Join(root, ".agentmod", "node", "bin")
	target := filepath.Join(root, ".agentmod", "node", "lib", "node_modules", "claude", "cli.js")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(binDir, "claude")); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	line := findingLine(t, stdout, "Shims")
	wantLevel(t, line, diagOK)
	wantContains(t, "Shims line", line, "claude", "project-local installs")
}

func TestDoctorAuthMissingIsInformational(t *testing.T) {
	// §23 "auth present / re-login needed" with §12's re-login instructions.
	// A fresh project has no auth — that is a state, not a problem (D023):
	// doctor reports the exact next step at ok level and still exits 0.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	claude := findingLine(t, stdout, "Claude home")
	wantLevel(t, claude, diagOK)
	wantContains(t, "Claude home line", claude,
		filepath.Join(root, ".agentmod", "claude"),
		"no auth file (.credentials.json)",
		"running 'claude' inside this project",
	)
	codex := findingLine(t, stdout, "Codex home")
	wantLevel(t, codex, diagOK)
	wantContains(t, "Codex home line", codex,
		filepath.Join(root, ".agentmod", "codex"),
		"no auth file (auth.json)",
		"re-login needed: run 'codex login' inside this project",
	)
}

func TestDoctorAuthPresent(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	for _, f := range []struct{ dir, name string }{
		{"claude", ".credentials.json"},
		{"codex", "auth.json"},
	} {
		path := filepath.Join(root, ".agentmod", f.dir, f.name)
		if err := os.WriteFile(path, []byte(`{"token":"sk-FAKE-fixture"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	claude := findingLine(t, stdout, "Claude home")
	wantLevel(t, claude, diagOK)
	wantContains(t, "Claude home line", claude, "auth present (.credentials.json)")
	codex := findingLine(t, stdout, "Codex home")
	wantLevel(t, codex, diagOK)
	wantContains(t, "Codex home line", codex, "auth present (auth.json)")
}

func TestDoctorAuthPathNotARegularFile(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".agentmod", "codex", "auth.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Codex home")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Codex home line", line, "not a regular file", "move it aside")
}

func TestDoctorDisabledAgentsReportDisabledRouting(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Enabled = false
	cfg.Codex.Enabled = false
	cfg.OpenCode.Enabled = false
	root := makeProject(t, cfg)
	mkLayout(t, root)
	vars := healthyVars(t, root)
	// healthyVars routes for the default config; rebuild for this one.
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), config.Default()) {
		delete(vars, v.Name)
	}
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), cfg) {
		vars[v.Name] = v.Value
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantContains(t, "Claude home line", findingLine(t, stdout, "Claude home"), "routing disabled (claude.enabled = false)")
	wantContains(t, "Codex home line", findingLine(t, stdout, "Codex home"), "routing disabled (codex.enabled = false)")
	wantContains(t, "OpenCode line", findingLine(t, stdout, "OpenCode config"), "routing disabled (opencode.enabled = false)")
}

func TestDoctorOpencodeConfigState(t *testing.T) {
	// Missing stub: OpenCode would fall back to the global merge chain alone;
	// re-init recreates it → warn.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	stub := layout.OpencodeConfigPath(filepath.Join(root, ".agentmod"))
	if err := os.Remove(stub); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("missing: exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "OpenCode config")
	wantLevel(t, line, diagWarn)
	wantContains(t, "OpenCode line", line, stub, "missing", "re-run 'agentmod init'")

	// A directory where the config file belongs blocks routing → error.
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("not-regular: exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line = findingLine(t, stdout, "OpenCode config")
	wantLevel(t, line, diagError)
	wantContains(t, "OpenCode line", line, "not a regular file")
}

func TestDoctorAgentBinariesOnPath(t *testing.T) {
	// Stat-based PATH walk: an executable regular file counts; a
	// non-executable file does not. Always ok-level — absence is information.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	vars := healthyVars(t, root)
	vars["PATH"] = vars["PATH"] + sep + binDir
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	line := findingLine(t, stdout, "Agent binaries")
	wantLevel(t, line, diagOK)
	wantContains(t, "Agent binaries line", line,
		"claude at "+filepath.Join(binDir, "claude"),
		"codex not found on PATH",
		"opencode not found on PATH",
	)
}

// wantNoFinding asserts doctor printed no line for a label (used for
// findings that must be skipped entirely, not reported at ok level).
func wantNoFinding(t *testing.T, stdout, label string) {
	t.Helper()
	if strings.Contains(stdout, "  "+label+": ") {
		t.Errorf("unexpected %q finding in doctor output:\n%s", label, stdout)
	}
}

// writeGlobalOpencodeConfig plants a global opencode.json under home's XDG
// default config dir and returns its path. Content must be obviously fake.
func writeGlobalOpencodeConfig(t *testing.T, home, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDoctorOpenCodeSessionsWarnWhenGlobalDataDirExists(t *testing.T) {
	// Partial mode + an existing global data dir = sessions ARE accumulating
	// outside the project (§15.3) → warn with the opt-in remedy (D024).
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	dataDir := filepath.Join(vars["HOME"], ".local", "share", "opencode")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "OpenCode sessions")
	wantLevel(t, line, diagWarn)
	wantContains(t, "OpenCode sessions line", line,
		"not project-isolated", dataDir, "opencode.xdg_full_isolation = true")
}

func TestDoctorOpenCodeSessionsRespectsXDGDataHome(t *testing.T) {
	// A user-set XDG_DATA_HOME relocates the global data dir; doctor must
	// look there, not under HOME's default.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	xdgData := t.TempDir()
	vars["XDG_DATA_HOME"] = xdgData
	dataDir := filepath.Join(xdgData, "opencode")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "OpenCode sessions")
	wantLevel(t, line, diagWarn)
	wantContains(t, "OpenCode sessions line", line, dataDir)
}

func TestDoctorOpenCodeSessionsNoGlobalDataStatesLimitationAtOK(t *testing.T) {
	// Nothing stored globally yet: the limitation is still stated, but at ok
	// level so a fresh default-config project exits 0 (D024).
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	line := findingLine(t, stdout, "OpenCode sessions")
	wantLevel(t, line, diagOK)
	wantContains(t, "OpenCode sessions line", line,
		"sessions, storage, and auth in the global data dir",
		"nothing stored there yet", "opencode.xdg_full_isolation = true")
}

func TestDoctorOpenCodeXDGOptInSuppressesBothWarnings(t *testing.T) {
	// With full XDG isolation on, sessions and the merge chain are routed
	// into the project, so neither global fixture may trigger a warning.
	cfg := config.Default()
	cfg.OpenCode.XDGFullIsolation = true
	root := makeProject(t, cfg)
	mkLayout(t, root)
	vars := healthyVars(t, root)
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), cfg) {
		vars[v.Name] = v.Value
	}
	if err := os.MkdirAll(filepath.Join(vars["HOME"], ".local", "share", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeGlobalOpencodeConfig(t, vars["HOME"], `{"model": "fake-model", "plugin": ["fake-plugin"]}`)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	sessions := findingLine(t, stdout, "OpenCode sessions")
	wantLevel(t, sessions, diagOK)
	wantContains(t, "OpenCode sessions line", sessions, "opencode.xdg_full_isolation = true")
	merge := findingLine(t, stdout, "OpenCode merge chain")
	wantLevel(t, merge, diagOK)
	wantContains(t, "OpenCode merge chain line", merge, "not merged", "opencode.xdg_full_isolation = true")
}

func TestDoctorOpenCodeMergeChainWarnsOnGlobalConfig(t *testing.T) {
	// A global opencode.json carrying real settings leaks into the project
	// view via the merge chain (§23) → warn naming the leaking keys.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	path := writeGlobalOpencodeConfig(t, vars["HOME"],
		`{"$schema": "https://opencode.ai/config.json", "model": "fake-model", "plugin": ["fake-plugin"]}`)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "OpenCode merge chain")
	wantLevel(t, line, diagWarn)
	wantContains(t, "OpenCode merge chain line", line,
		path, "will leak: model, plugin", "opencode.xdg_full_isolation = true")
	if strings.Contains(line, "$schema") {
		t.Errorf("$schema must not be reported as leaking:\n%s", line)
	}
}

func TestDoctorOpenCodeMergeChainTrivialGlobalConfigIsOK(t *testing.T) {
	// $schema-only, empty-object, and empty-file global configs change no
	// behavior — nothing leaks, stay ok.
	for name, content := range map[string]string{
		"schema-only": `{"$schema": "https://opencode.ai/config.json"}`,
		"empty-obj":   "{}\n",
		"empty-file":  "",
	} {
		root := makeProject(t, config.Default())
		mkLayout(t, root)
		vars := healthyVars(t, root)
		writeGlobalOpencodeConfig(t, vars["HOME"], content)
		code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
		if code != ExitOK {
			t.Fatalf("%s: exit = %d, want %d\n%s", name, code, ExitOK, stdout)
		}
		line := findingLine(t, stdout, "OpenCode merge chain")
		wantLevel(t, line, diagOK)
		wantContains(t, name+" merge line", line, "no settings")
	}
}

func TestDoctorOpenCodeMergeChainUnparseableGlobalConfigWarns(t *testing.T) {
	// Doctor cannot prove an unparseable (e.g. JSONC) global config leaks
	// nothing, and OpenCode may still merge it → conservative warn.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	vars := healthyVars(t, root)
	path := writeGlobalOpencodeConfig(t, vars["HOME"], "// jsonc comment\n{\"model\": \"fake-model\"}\n")
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "OpenCode merge chain")
	wantLevel(t, line, diagWarn)
	wantContains(t, "OpenCode merge chain line", line, path, "could not be parsed")
}

func TestDoctorOpenCodeDisabledSkipsIsolationFindings(t *testing.T) {
	// opencode.enabled = false: routing never points at OpenCode, so both
	// §15.3 findings are moot and must not appear at all.
	cfg := config.Default()
	cfg.OpenCode.Enabled = false
	root := makeProject(t, cfg)
	mkLayout(t, root)
	vars := healthyVars(t, root)
	if err := os.MkdirAll(filepath.Join(vars["HOME"], ".local", "share", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeGlobalOpencodeConfig(t, vars["HOME"], `{"model": "fake-model"}`)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, vars))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantNoFinding(t, stdout, "OpenCode sessions")
	wantNoFinding(t, stdout, "OpenCode merge chain")
	wantContains(t, "OpenCode config line", findingLine(t, stdout, "OpenCode config"), "routing disabled")
}

func TestDoctorKeychainNoteOnDarwin(t *testing.T) {
	// The §15.1 macOS limitation is a platform fact: stated at ok level on
	// darwin (exit stays 0), absent everywhere else. fakeEnv leaves GOOS ""
	// (not-darwin), so each case sets it explicitly — host-independent.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	for _, tc := range []struct {
		goos string
		want bool
	}{{"darwin", true}, {"linux", false}, {"", false}} {
		env := fakeEnv(root, healthyVars(t, root))
		env.GOOS = tc.goos
		code, stdout, _ := runDoctorForTest(t, env)
		if code != ExitOK {
			t.Fatalf("GOOS=%q: exit = %d, want %d\n%s", tc.goos, code, ExitOK, stdout)
		}
		if !tc.want {
			wantNoFinding(t, stdout, "Claude auth (macOS)")
			continue
		}
		line := findingLine(t, stdout, "Claude auth (macOS)")
		wantLevel(t, line, diagOK)
		wantContains(t, "Keychain line", line, "shared macOS Keychain",
			"per-project account isolation is not possible")
	}
}

func TestDoctorKeychainNoteSkippedWhenClaudeDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Enabled = false
	root := makeProject(t, cfg)
	mkLayout(t, root)
	vars := healthyVars(t, root)
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), config.Default()) {
		delete(vars, v.Name)
	}
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), cfg) {
		vars[v.Name] = v.Value
	}
	env := fakeEnv(root, vars)
	env.GOOS = "darwin"
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantNoFinding(t, stdout, "Claude auth (macOS)")
}

func TestDoctorKeychainNoteOutsideProjectAbsent(t *testing.T) {
	// The limitation concerns project-local homes; outside a project there
	// is nothing it qualifies, so no line even on darwin.
	env := fakeEnv(t.TempDir(), map[string]string{"SHELL": "/bin/zsh", "HOME": t.TempDir()})
	env.GOOS = "darwin"
	code, stdout, _ := runDoctorForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	wantNoFinding(t, stdout, "Claude auth (macOS)")
}

func TestDoctorGstackGlobalExistsWarns(t *testing.T) {
	// §23 must-warn: a global ~/.claude/skills/gstack affects every project,
	// so it warns in AND out of a project. Lstat-based: a dir, a stray file,
	// or a symlink (even dangling) all count as a global install.
	for _, kind := range []string{"dir", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			skills := filepath.Join(home, ".claude", "skills")
			if err := os.MkdirAll(skills, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(skills, "gstack")
			var err error
			switch kind {
			case "dir":
				err = os.Mkdir(path, 0o755)
			case "file":
				err = os.WriteFile(path, []byte("fake\n"), 0o644)
			case "symlink":
				err = os.Symlink(filepath.Join(home, "nowhere"), path)
			}
			if err != nil {
				t.Fatal(err)
			}
			env := fakeEnv(t.TempDir(), map[string]string{"SHELL": "/bin/zsh", "HOME": home})
			code, stdout, _ := runDoctorForTest(t, env)
			if code != ExitValidation {
				t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
			}
			line := findingLine(t, stdout, "gstack (global)")
			wantLevel(t, line, diagWarn)
			wantContains(t, "gstack global line", line, path, "affects every project",
				"agentmod install gstack")
		})
	}
}

func TestDoctorGstackGlobalAbsentInsideProjectIsOK(t *testing.T) {
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	line := findingLine(t, stdout, "gstack (global)")
	wantLevel(t, line, diagOK)
	wantContains(t, "gstack global line", line, "no global install at")
}

func TestDoctorGstackGlobalHomeUnset(t *testing.T) {
	code, stdout, _ := runDoctorForTest(t, fakeEnv(t.TempDir(), nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
	}
	line := findingLine(t, stdout, "gstack (global)")
	wantLevel(t, line, diagOK)
	wantContains(t, "gstack global line", line, "HOME unset")
}

func TestDoctorGstackProjectState(t *testing.T) {
	// Project-local install state is informational both ways (installing
	// gstack is optional); only a non-directory entry at the install path
	// warns. The finding only exists inside a project.
	t.Run("not installed", func(t *testing.T) {
		root := makeProject(t, config.Default())
		mkLayout(t, root)
		code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
		if code != ExitOK {
			t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
		}
		line := findingLine(t, stdout, "gstack (project)")
		wantLevel(t, line, diagOK)
		wantContains(t, "gstack project line", line, "not installed", "agentmod install gstack")
	})
	t.Run("installed", func(t *testing.T) {
		root := makeProject(t, config.Default())
		mkLayout(t, root)
		path := filepath.Join(root, ".agentmod", "claude", "skills", "gstack")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
		if code != ExitOK {
			t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
		}
		line := findingLine(t, stdout, "gstack (project)")
		wantLevel(t, line, diagOK)
		wantContains(t, "gstack project line", line, "installed at "+path)
	})
	t.Run("not a directory", func(t *testing.T) {
		root := makeProject(t, config.Default())
		mkLayout(t, root)
		skills := filepath.Join(root, ".agentmod", "claude", "skills")
		if err := os.MkdirAll(skills, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skills, "gstack"), []byte("fake\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
		if code != ExitValidation {
			t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
		}
		line := findingLine(t, stdout, "gstack (project)")
		wantLevel(t, line, diagWarn)
		wantContains(t, "gstack project line", line, "not a directory", "move it aside")
	})
	t.Run("outside a project", func(t *testing.T) {
		env := fakeEnv(t.TempDir(), map[string]string{"SHELL": "/bin/zsh", "HOME": t.TempDir()})
		_, stdout, _ := runDoctorForTest(t, env)
		wantNoFinding(t, stdout, "gstack (project)")
	})
}

func TestDoctorGuardSettingsMissing(t *testing.T) {
	// settings.json gone = the Bash guard is not wired at all; init recreates
	// it, so this is degraded-recoverable → warn.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	if err := os.Remove(guardSettingsPath(root)); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Claude guard")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Claude guard line", line, "missing", "not wired", "re-run 'agentmod init'")
}

func TestDoctorGuardHookMissingFromSettings(t *testing.T) {
	// The file exists (user-managed settings) but carries no guard hook —
	// e.g. created before T17 or hand-edited. Re-init appends it → warn.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	if err := os.WriteFile(guardSettingsPath(root),
		[]byte(`{"env": {"FAKE": "1"}, "hooks": {"PostToolUse": []}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Claude guard")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Claude guard line", line, "no guard hook in", "re-run 'agentmod init'")
}

func TestDoctorGuardStaleBinaryPath(t *testing.T) {
	// IMPLEMENTATION_PLAN §11: the hook's absolute binary path goes stale
	// when the binary moves; doctor names both paths, init repairs.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	writeGuardSettings(t, root, guardHookCommand("/old/place/agentmod"))
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Claude guard")
	wantLevel(t, line, diagWarn)
	wantContains(t, "Claude guard line", line,
		"'/old/place/agentmod' guard claude-bash",
		"'"+fakeBinPath+"' guard claude-bash",
		"re-run 'agentmod init' to repair it",
	)
}

func TestDoctorGuardInvalidSettingsIsError(t *testing.T) {
	// A settings file doctor cannot parse blocks any guard statement — and
	// init refuses to touch it too, so this is error-level, not warn.
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	if err := os.WriteFile(guardSettingsPath(root), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runDoctorForTest(t, fakeEnv(root, healthyVars(t, root)))
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitValidation, stdout)
	}
	line := findingLine(t, stdout, "Claude guard")
	wantLevel(t, line, diagError)
	wantContains(t, "Claude guard line", line, "not a valid JSON object")
}

func TestDoctorGuardUnresolvableBinary(t *testing.T) {
	// Without a resolvable current binary the path comparison is impossible;
	// a wired-looking hook is reported at ok level with the caveat (the
	// equivalent init case skips wiring, also without failing).
	root := makeProject(t, config.Default())
	mkLayout(t, root)
	for name, fn := range map[string]func() (string, error){
		"nil resolver":      nil,
		"erroring resolver": func() (string, error) { return "", errors.New("fake exotic failure") },
	} {
		t.Run(name, func(t *testing.T) {
			env := fakeEnv(root, healthyVars(t, root))
			env.Executable = fn
			code, stdout, _ := runDoctorForTest(t, env)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout)
			}
			line := findingLine(t, stdout, "Claude guard")
			wantLevel(t, line, diagOK)
			wantContains(t, "Claude guard line", line,
				"hook present", "cannot be resolved", "binary path not verified")
		})
	}
}

func TestDoctorRejectsArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"doctor", "--json"}, &out, &errBuf, fakeEnv(t.TempDir(), nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "doctor takes no arguments")
}
