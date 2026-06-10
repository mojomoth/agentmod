package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/routing"
)

// mkLayout creates the init-managed directories under root/.agentmod so the
// layout check passes.
func mkLayout(t *testing.T, root string) {
	t.Helper()
	for _, sub := range layout.Subdirs() {
		if err := os.MkdirAll(filepath.Join(root, ".agentmod", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
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
	vars := map[string]string{
		"SHELL":                 "/bin/zsh",
		"HOME":                  home,
		"AGENTMOD_ACTIVE":       "1",
		"AGENTMOD_PROJECT_ROOT": root,
	}
	for _, v := range routing.Vars(filepath.Join(root, ".agentmod"), config.Default()) {
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
	wantContains(t, "Project line", findingLine(t, stdout, "Project"), root, filepath.Join(root, ".agentmod"))
	wantContains(t, "Routing line", findingLine(t, stdout, "Routing env"), "applied for this project")
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
	if strings.Contains(stdout, "Routing env") {
		t.Errorf("routing-env check must not report outside a project (lingering-vars is a separate check):\n%s", stdout)
	}
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
		if err := os.Remove(filepath.Join(root, ".agentmod", sub)); err != nil {
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
	if err := os.Remove(claude); err != nil {
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

func TestDoctorRejectsArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"doctor", "--json"}, &out, &errBuf, fakeEnv(t.TempDir(), nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "doctor takes no arguments")
}
