package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mojomoth/agentmod/internal/config"
)

func runEnvForTest(t *testing.T, env Env, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(append([]string{"env"}, args...), &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

const sep = string(os.PathListSeparator)

func TestEnvActivateFresh(t *testing.T) {
	root := makeProject(t, config.Default())
	am := filepath.Join(root, ".agentmod")
	for _, shell := range []string{"zsh", "bash"} {
		env := fakeEnv(root, map[string]string{"PATH": "/usr/bin" + sep + "/bin"})
		code, stdout, stderr := runEnvForTest(t, env, "--shell", shell, "--activate", root)
		if code != ExitOK || stderr != "" {
			t.Fatalf("[%s] code=%d stderr=%q", shell, code, stderr)
		}
		want := fmt.Sprintf(`export CLAUDE_CONFIG_DIR='%[1]s/claude'
export CODEX_HOME='%[1]s/codex'
export OPENCODE_CONFIG='%[1]s/opencode/opencode.json'
export NPM_CONFIG_CACHE='%[1]s/node/npm-cache'
export NPM_CONFIG_PREFIX='%[1]s/node'
export PNPM_HOME='%[1]s/node/pnpm'
export BUN_INSTALL='%[1]s/node/bun'
export PATH='%[1]s/node/bin%[3]s/usr/bin%[3]s/bin'
export AGENTMOD_ACTIVE='1'
export AGENTMOD_PROJECT_ROOT='%[2]s'
export AGENTMOD_ROOT='%[1]s'
export AGENTMOD_VARS='CLAUDE_CONFIG_DIR CODEX_HOME OPENCODE_CONFIG NPM_CONFIG_CACHE NPM_CONFIG_PREFIX PNPM_HOME BUN_INSTALL'
`, am, root, sep)
		if stdout != want {
			t.Errorf("[%s] activation script mismatch\ngot:\n%s\nwant:\n%s", shell, stdout, want)
		}
	}
}

func TestEnvActivateSavesExisting(t *testing.T) {
	root := makeProject(t, config.Default())
	env := fakeEnv(root, map[string]string{
		"PATH":              "/usr/bin",
		"CLAUDE_CONFIG_DIR": "/home/me/claude-global",
	})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--activate", root)
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	saveLine := "export AGENTMOD_SAVED_CLAUDE_CONFIG_DIR='/home/me/claude-global'\n"
	setLine := "export CLAUDE_CONFIG_DIR='" + filepath.Join(root, ".agentmod", "claude") + "'\n"
	si, vi := strings.Index(stdout, saveLine), strings.Index(stdout, setLine)
	if si < 0 || vi < 0 || si > vi {
		t.Errorf("save must precede set\ngot:\n%s", stdout)
	}
	if strings.Contains(stdout, "AGENTMOD_SAVED_CODEX_HOME") {
		t.Errorf("CODEX_HOME was unset; nothing to save\ngot:\n%s", stdout)
	}
}

func TestEnvActivateQuotesSavedValue(t *testing.T) {
	root := makeProject(t, config.Default())
	env := fakeEnv(root, map[string]string{
		"CLAUDE_CONFIG_DIR": "/tmp/it's here",
	})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--activate", root)
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	want := `export AGENTMOD_SAVED_CLAUDE_CONFIG_DIR='/tmp/it'\''s here'` + "\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("missing escaped save line %q\ngot:\n%s", want, stdout)
	}
}

func TestEnvActivateDisabledAgents(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Enabled = false
	cfg.Node.Enabled = false
	root := makeProject(t, cfg)
	env := fakeEnv(root, map[string]string{"PATH": "/usr/bin"})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--activate", root)
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	for _, absent := range []string{"CLAUDE_CONFIG_DIR", "NPM_CONFIG", "PNPM_HOME", "BUN_INSTALL", "export PATH"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("disabled agent leaked %q\ngot:\n%s", absent, stdout)
		}
	}
	if !strings.Contains(stdout, "export AGENTMOD_VARS='CODEX_HOME OPENCODE_CONFIG'\n") {
		t.Errorf("AGENTMOD_VARS wrong\ngot:\n%s", stdout)
	}
}

func TestEnvActivateXDGOptIn(t *testing.T) {
	cfg := config.Default()
	cfg.OpenCode.XDGFullIsolation = true
	root := makeProject(t, cfg)
	xdg := filepath.Join(root, ".agentmod", "opencode", "xdg")
	env := fakeEnv(root, map[string]string{"PATH": "/usr/bin"})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--activate", root)
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	wantContains(t, "xdg activation", stdout,
		"export XDG_CONFIG_HOME='"+filepath.Join(xdg, "config")+"'\n",
		"export XDG_DATA_HOME='"+filepath.Join(xdg, "data")+"'\n",
		"export XDG_CACHE_HOME='"+filepath.Join(xdg, "cache")+"'\n",
		"export XDG_STATE_HOME='"+filepath.Join(xdg, "state")+"'\n",
	)
}

func TestEnvActivatePathDedup(t *testing.T) {
	root := makeProject(t, config.Default())
	entry := filepath.Join(root, ".agentmod", "node", "bin")
	env := fakeEnv(root, map[string]string{
		"PATH": "/usr/bin" + sep + entry + sep + "/bin",
	})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--activate", root)
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	want := "export PATH='" + entry + sep + "/usr/bin" + sep + "/bin'\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("PATH not deduped to front\nwant line %q\ngot:\n%s", want, stdout)
	}
}

func TestEnvActivateRelativeRoot(t *testing.T) {
	root := makeProject(t, config.Default())
	env := fakeEnv(filepath.Dir(root), map[string]string{"PATH": "/usr/bin"})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "zsh", "--activate", filepath.Base(root))
	if code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "export AGENTMOD_PROJECT_ROOT='"+root+"'\n") {
		t.Errorf("relative root not resolved against cwd\ngot:\n%s", stdout)
	}
}

func TestEnvDeactivateWhenInactive(t *testing.T) {
	env := fakeEnv("/anywhere", map[string]string{"PATH": "/usr/bin"})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "zsh", "--deactivate")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Errorf("inactive deactivate must be a silent no-op: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestEnvDeactivateRestores(t *testing.T) {
	env := fakeEnv("/anywhere", map[string]string{
		"AGENTMOD_ACTIVE":                  "1",
		"AGENTMOD_PROJECT_ROOT":            "/p",
		"AGENTMOD_ROOT":                    "/p/.agentmod",
		"AGENTMOD_VARS":                    "CLAUDE_CONFIG_DIR CODEX_HOME",
		"AGENTMOD_SAVED_CLAUDE_CONFIG_DIR": "/home/me/claude-global",
		"CLAUDE_CONFIG_DIR":                "/p/.agentmod/claude",
		"CODEX_HOME":                       "/p/.agentmod/codex",
		"PATH":                             "/p/.agentmod/node/bin" + sep + "/usr/bin",
	})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "bash", "--deactivate")
	if code != ExitOK || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	want := `export CLAUDE_CONFIG_DIR='/home/me/claude-global'
unset AGENTMOD_SAVED_CLAUDE_CONFIG_DIR
unset CODEX_HOME
export PATH='/usr/bin'
unset AGENTMOD_VARS
unset AGENTMOD_ROOT
unset AGENTMOD_PROJECT_ROOT
unset AGENTMOD_ACTIVE
`
	if stdout != want {
		t.Errorf("deactivation script mismatch\ngot:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestEnvDeactivateSkipsUnsafeNames(t *testing.T) {
	env := fakeEnv("/anywhere", map[string]string{
		"AGENTMOD_ACTIVE": "1",
		"AGENTMOD_ROOT":   "/p/.agentmod",
		"AGENTMOD_VARS":   "CODEX_HOME bad;name $(evil)",
		"CODEX_HOME":      "/p/.agentmod/codex",
	})
	code, stdout, _ := runEnvForTest(t, env, "--shell", "zsh", "--deactivate")
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "unset CODEX_HOME\n") {
		t.Errorf("valid name not processed\ngot:\n%s", stdout)
	}
	for _, bad := range []string{";", "$("} {
		if strings.Contains(stdout, bad) {
			t.Errorf("unsafe token %q reached shell output\ngot:\n%s", bad, stdout)
		}
	}
}

func TestEnvSwitchProjects(t *testing.T) {
	newRoot := makeProject(t, config.Default())
	newAm := filepath.Join(newRoot, ".agentmod")
	oldEntry := "/old/proj/.agentmod/node/bin"
	env := fakeEnv(newRoot, map[string]string{
		"AGENTMOD_ACTIVE":       "1",
		"AGENTMOD_PROJECT_ROOT": "/old/proj",
		"AGENTMOD_ROOT":         "/old/proj/.agentmod",
		"AGENTMOD_VARS":         "CLAUDE_CONFIG_DIR",
		"CLAUDE_CONFIG_DIR":     "/old/proj/.agentmod/claude",
		"PATH":                  oldEntry + sep + "/usr/bin",
	})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "zsh", "--activate", newRoot)
	if code != ExitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// Old routing is undone before the new one is applied, so the new
	// activation must not "save" the old project's routed value.
	di := strings.Index(stdout, "unset CLAUDE_CONFIG_DIR\n")
	ai := strings.Index(stdout, "export CLAUDE_CONFIG_DIR='"+filepath.Join(newAm, "claude")+"'\n")
	if di < 0 || ai < 0 || di > ai {
		t.Errorf("old routing must be undone before new is applied\ngot:\n%s", stdout)
	}
	if strings.Contains(stdout, "AGENTMOD_SAVED_") {
		t.Errorf("switch must not save the previous project's routing\ngot:\n%s", stdout)
	}
	wantPath := "export PATH='" + filepath.Join(newAm, "node", "bin") + sep + "/usr/bin'\n"
	if !strings.Contains(stdout, wantPath) {
		t.Errorf("final PATH must drop the old entry and lead with the new\nwant line %q\ngot:\n%s", wantPath, stdout)
	}
	if !strings.Contains(stdout, "export AGENTMOD_PROJECT_ROOT='"+newRoot+"'\n") {
		t.Errorf("bookkeeping must point at the new project\ngot:\n%s", stdout)
	}
}

func TestEnvActivateNotAProject(t *testing.T) {
	dir := t.TempDir()
	env := fakeEnv(dir, map[string]string{"PATH": "/usr/bin"})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "zsh", "--activate", dir)
	if code != ExitNotInProject {
		t.Errorf("code = %d, want %d", code, ExitNotInProject)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty on failure (it gets eval'd): %q", stdout)
	}
	wantContains(t, "stderr", stderr, "not an agentmod project root")
}

func TestEnvActivateBrokenConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".agentmod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agentmod", "agentmod.toml"), []byte("not [valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := fakeEnv(dir, map[string]string{"PATH": "/usr/bin"})
	code, stdout, stderr := runEnvForTest(t, env, "--shell", "zsh", "--activate", dir)
	if code != ExitError {
		t.Errorf("code = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty on failure: %q", stdout)
	}
	if stderr == "" {
		t.Error("expected an error on stderr")
	}
}

func TestEnvFlagErrors(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "requires --shell"},
		{[]string{"--shell"}, "--shell requires a value"},
		{[]string{"--shell", "fish", "--deactivate"}, "unsupported shell"},
		{[]string{"--shell", "zsh"}, "exactly one of"},
		{[]string{"--shell", "zsh", "--activate", "/x", "--deactivate"}, "exactly one of"},
		{[]string{"--shell", "zsh", "--activate"}, "--activate requires"},
		{[]string{"--shell", "zsh", "--bogus"}, "unknown argument"},
	}
	for _, tc := range cases {
		env := fakeEnv("/anywhere", nil)
		code, stdout, stderr := runEnvForTest(t, env, tc.args...)
		if code != ExitError {
			t.Errorf("args %v: code = %d, want %d", tc.args, code, ExitError)
		}
		if stdout != "" {
			t.Errorf("args %v: stdout must stay empty: %q", tc.args, stdout)
		}
		if !strings.Contains(stderr, tc.want) {
			t.Errorf("args %v: stderr %q missing %q", tc.args, stderr, tc.want)
		}
	}
}

// applyOps replays emitted ops onto a plain map, simulating what eval does to
// the shell's environment.
func applyOps(env map[string]string, ops []shellOp) {
	for _, op := range ops {
		if op.unset {
			delete(env, op.name)
		} else {
			env[op.name] = op.value
		}
	}
}

func modelOver(env map[string]string) *opList {
	return &opList{model: &envModel{
		lookup: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		overrides: map[string]*string{},
	}}
}

// TestEnvActivateDeactivateRoundTrip proves deactivation is a perfect inverse
// of activation: after activate + deactivate the environment is byte-equal to
// the starting one, including previously-set routed vars and PATH.
func TestEnvActivateDeactivateRoundTrip(t *testing.T) {
	cfg := config.Default()
	cfg.OpenCode.XDGFullIsolation = true
	start := map[string]string{
		"PATH":              "/usr/bin" + sep + "/bin",
		"CLAUDE_CONFIG_DIR": "/home/me/claude-global",
		"XDG_CONFIG_HOME":   "/home/me/.config",
		"UNRELATED":         "kept as-is",
	}
	env := map[string]string{}
	for k, v := range start {
		env[k] = v
	}

	act := modelOver(env)
	appendActivate(act, "/proj", "/proj/.agentmod", cfg)
	applyOps(env, act.ops)
	if env["CLAUDE_CONFIG_DIR"] != "/proj/.agentmod/claude" {
		t.Fatalf("activation did not route: %q", env["CLAUDE_CONFIG_DIR"])
	}

	deact := modelOver(env)
	appendDeactivate(deact)
	applyOps(env, deact.ops)
	if !reflect.DeepEqual(env, start) {
		t.Errorf("environment after round trip differs\ngot:  %#v\nwant: %#v", env, start)
	}
}
