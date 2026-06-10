package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentmod/agentmod/internal/config"
)

// makeProject writes .agentmod/agentmod.toml with the given config under a
// fresh temp dir and returns the project root.
func makeProject(t *testing.T, cfg config.Config) string {
	t.Helper()
	root := t.TempDir()
	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".agentmod")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agentmod.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fakeEnv returns an Env reporting cwd as the working directory and vars as
// the entire environment.
func fakeEnv(cwd string, vars map[string]string) Env {
	return Env{
		Getwd: func() (string, error) { return cwd, nil },
		LookupEnv: func(key string) (string, bool) {
			v, ok := vars[key]
			return v, ok
		},
	}
}

func runStatusForTest(t *testing.T, env Env) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run([]string{"status"}, &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

func wantContains(t *testing.T, name, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q\ngot:\n%s", name, w, got)
		}
	}
}

func TestStatusInactive(t *testing.T) {
	code, stdout, stderr := runStatusForTest(t, fakeEnv(t.TempDir(), nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	wantContains(t, "stdout", stdout,
		"AgentMod: inactive",
		".agentmod/agentmod.toml not found",
		"Default global agent settings will be used",
	)
	if strings.Contains(stdout, "AgentMod: active") {
		t.Errorf("inactive output claims active:\n%s", stdout)
	}
}

func TestStatusActiveDefaults(t *testing.T) {
	root := makeProject(t, config.Default())
	// Run from a nested directory: discovery must still find the root.
	sub := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runStatusForTest(t, fakeEnv(sub, nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr)
	}
	agentmodDir := filepath.Join(root, ".agentmod")
	wantContains(t, "stdout", stdout,
		"AgentMod: active",
		"Project root:    "+root,
		"AgentMod root:   "+agentmodDir,
		"Claude home:     "+filepath.Join(agentmodDir, "claude"),
		"Codex home:      "+filepath.Join(agentmodDir, "codex"),
		"OpenCode config: "+filepath.Join(agentmodDir, "opencode", "opencode.json"),
		"Node dir:        "+filepath.Join(agentmodDir, "node"),
		"Recent handoff:  none",
		"not applied in this shell",
	)
	if strings.Contains(stdout, "XDG full isolation") {
		t.Errorf("default config must not advertise XDG full isolation:\n%s", stdout)
	}
}

func TestStatusShellRoutingApplied(t *testing.T) {
	root := makeProject(t, config.Default())
	env := fakeEnv(root, map[string]string{
		"AGENTMOD_ACTIVE":       "1",
		"AGENTMOD_PROJECT_ROOT": root,
	})
	code, stdout, _ := runStatusForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout, "applied (AGENTMOD_ACTIVE=1)")
}

func TestStatusShellRoutingStale(t *testing.T) {
	root := makeProject(t, config.Default())
	env := fakeEnv(root, map[string]string{
		"AGENTMOD_ACTIVE":       "1",
		"AGENTMOD_PROJECT_ROOT": "/somewhere/else",
	})
	code, stdout, _ := runStatusForTest(t, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout,
		"applied for a different project (/somewhere/else)",
		"stale environment?",
	)
}

func TestStatusDisabledAgents(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Enabled = false
	cfg.Node.Enabled = false
	root := makeProject(t, cfg)
	code, stdout, _ := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout,
		"Claude home:     disabled (claude.enabled = false)",
		"Node dir:        disabled (node.enabled = false)",
		// The others stay routed.
		"Codex home:      "+filepath.Join(root, ".agentmod", "codex"),
	)
}

func TestStatusXDGOptIn(t *testing.T) {
	cfg := config.Default()
	cfg.OpenCode.XDGFullIsolation = true
	root := makeProject(t, cfg)
	code, stdout, _ := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout, "(+ XDG full isolation)")
}

func TestStatusRecentHandoff(t *testing.T) {
	root := makeProject(t, config.Default())
	snapDir := filepath.Join(root, ".agentmod", "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(snapDir, "old.amod")
	newer := filepath.Join(snapDir, "newer.amod")
	notSnap := filepath.Join(snapDir, "notes.txt")
	for _, p := range []string{old, newer, notSnap} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(notSnap, base.Add(90*time.Minute), base.Add(90*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Hour), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout, "Recent handoff:  newer.amod (created ")
	if strings.Contains(stdout, "old.amod") || strings.Contains(stdout, "notes.txt") {
		t.Errorf("stale or non-.amod entry reported:\n%s", stdout)
	}
}

func TestStatusBrokenConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "agentmod.toml")
	if err := os.WriteFile(cfgPath, []byte("schema_version = 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	// config.Load names the offending file; status must not eat that.
	wantContains(t, "stderr", stderr, cfgPath)
}

func TestStatusGetwdError(t *testing.T) {
	env := Env{
		Getwd:     func() (string, error) { return "", errors.New("cwd unavailable") },
		LookupEnv: func(string) (string, bool) { return "", false },
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"status"}, &out, &errBuf, env)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "cwd unavailable")
}

func TestStatusRejectsArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"status", "extra"}, &out, &errBuf, fakeEnv(t.TempDir(), nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "status takes no arguments")
}
