package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/shellhook"
)

// TestMain lets the test binary impersonate the real agentmod CLI: the zsh
// integration tests put a wrapper script named `agentmod` on the child
// shell's PATH that re-execs this binary with AGENTMOD_TEST_RUN_MAIN=1. The
// real agentmod binary is never built or installed, per the test strategy.
func TestMain(m *testing.M) {
	if os.Getenv("AGENTMOD_TEST_RUN_MAIN") == "1" {
		os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestHookCommand(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"zsh prints script", []string{"hook", "zsh"}, ExitOK, "_agentmod_hook", ""},
		{"no shell", []string{"hook"}, ExitError, "", "requires exactly one shell"},
		{"too many args", []string{"hook", "zsh", "extra"}, ExitError, "", "requires exactly one shell"},
		{"bash not yet", []string{"hook", "bash"}, ExitError, "", "not implemented yet"},
		{"unsupported shell", []string{"hook", "fish"}, ExitError, "", `unsupported shell "fish"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			code := run(tc.args, &out, &errBuf, fakeEnv("/anywhere", nil))
			if code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
			checkStream(t, "stdout", out.String(), tc.wantStdout)
			checkStream(t, "stderr", errBuf.String(), tc.wantStderr)
		})
	}
}

func TestHookZshScriptContents(t *testing.T) {
	script := shellhook.Zsh()
	wantContains(t, "zsh hook script", script,
		".agentmod/agentmod.toml",
		"precmd_functions",
		"chpwd_functions",
		"--shell zsh --activate",
		"--shell zsh --deactivate",
	)
	if strings.Contains(script, "HOME=") {
		t.Error("hook script must never assign HOME")
	}
	if !strings.HasSuffix(script, "\n") {
		t.Error("script must end with a newline")
	}
}

// requireZsh returns the zsh path or skips the test.
func requireZsh(t *testing.T) string {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not installed")
	}
	return zsh
}

func TestHookZshSyntaxValid(t *testing.T) {
	zsh := requireZsh(t)
	cmd := exec.Command(zsh, "-f", "-n")
	cmd.Stdin = strings.NewReader(shellhook.Zsh())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh -n rejected the hook script: %v\n%s", err, out)
	}
}

// fakeAgentmodBin writes an `agentmod` wrapper that re-execs the test binary
// and returns its directory, ready to prepend to a child shell's PATH.
func fakeAgentmodBin(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nAGENTMOD_TEST_RUN_MAIN=1 exec '" + exe + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "agentmod"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// childEnv builds the controlled environment for a child zsh. HOME points at
// a throwaway dir in the CHILD's env only — the parent process never
// reassigns the real HOME. No AGENTMOD_* state is inherited.
func childEnv(t *testing.T, pathDirs ...string) []string {
	t.Helper()
	path := strings.Join(append(pathDirs, "/usr/bin", "/bin"), string(os.PathListSeparator))
	return []string{
		"PATH=" + path,
		"HOME=" + t.TempDir(),
		"TERM=dumb",
	}
}

// runZsh feeds script to `zsh -f` (user rc files are never read) started in
// dir with the given env. Interactive mode fires precmd before each prompt.
func runZsh(t *testing.T, dir string, env []string, interactive bool, script string) (string, string) {
	t.Helper()
	zsh := requireZsh(t)
	args := []string{"-f"}
	if interactive {
		args = append(args, "-i")
	}
	cmd := exec.Command(zsh, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(script)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("zsh failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errBuf.String())
	}
	return out.String(), errBuf.String()
}

// writeProjectMarker turns root into an agentmod project root.
func writeProjectMarker(t *testing.T, root string, cfg config.Config) {
	t.Helper()
	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agentmod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agentmod", "agentmod.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHookZshActivatesOnCdInAndOut(t *testing.T) {
	root := makeProject(t, config.Default())
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runZsh(t, t.TempDir(), env, false, `
eval "$(agentmod hook zsh)"
print -r -- "HOME0:$HOME"
print -r -- "PATH0:$PATH"
cd '`+root+`'
print -r -- "ROOT1:${AGENTMOD_PROJECT_ROOT-unset}"
print -r -- "ACTIVE1:${AGENTMOD_ACTIVE-unset}"
print -r -- "CLAUDE1:${CLAUDE_CONFIG_DIR-unset}"
print -r -- "CODEX1:${CODEX_HOME-unset}"
print -r -- "OPENCODE1:${OPENCODE_CONFIG-unset}"
print -r -- "XDG1:${XDG_CONFIG_HOME-unset}"
print -r -- "PATH1:$PATH"
cd /
print -r -- "ROOT2:${AGENTMOD_PROJECT_ROOT-unset}"
print -r -- "CLAUDE2:${CLAUDE_CONFIG_DIR-unset}"
print -r -- "PATH2:$PATH"
print -r -- "HOME2:$HOME"
`)
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}
	am := filepath.Join(root, ".agentmod")
	wantContains(t, "cd in", stdout,
		"ROOT1:"+root+"\n",
		"ACTIVE1:1\n",
		"CLAUDE1:"+filepath.Join(am, "claude")+"\n",
		"CODEX1:"+filepath.Join(am, "codex")+"\n",
		"OPENCODE1:"+filepath.Join(am, "opencode", "opencode.json")+"\n",
		"XDG1:unset\n", // partial isolation by default: XDG untouched
		"PATH1:"+filepath.Join(am, "node", "bin")+sep,
	)
	wantContains(t, "cd out", stdout,
		"ROOT2:unset\n",
		"CLAUDE2:unset\n",
	)
	// Leaving must restore PATH and HOME exactly.
	path0 := lineAfter(t, stdout, "PATH0:")
	if got := lineAfter(t, stdout, "PATH2:"); got != path0 {
		t.Errorf("PATH not restored on leave: %q, want %q", got, path0)
	}
	home0 := lineAfter(t, stdout, "HOME0:")
	if got := lineAfter(t, stdout, "HOME2:"); got != home0 {
		t.Errorf("HOME changed across activation: %q, want %q", got, home0)
	}
}

// lineAfter returns the rest of the (unique) line starting with prefix.
func lineAfter(t *testing.T, text, prefix string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			found = append(found, strings.TrimPrefix(line, prefix))
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %q line, got %d in:\n%s", prefix, len(found), text)
	}
	return found[0]
}

func TestHookZshNestedNearestWins(t *testing.T) {
	outer := t.TempDir()
	writeProjectMarker(t, outer, config.Default())
	inner := filepath.Join(outer, "inner")
	writeProjectMarker(t, inner, config.Default())
	sub := filepath.Join(inner, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runZsh(t, t.TempDir(), env, false, `
eval "$(agentmod hook zsh)"
cd '`+outer+`'
print -r -- "A:${AGENTMOD_PROJECT_ROOT-unset}"
cd '`+inner+`'
print -r -- "B:${AGENTMOD_PROJECT_ROOT-unset}"
print -r -- "BCLAUDE:${CLAUDE_CONFIG_DIR-unset}"
cd '`+sub+`'
print -r -- "C:${AGENTMOD_PROJECT_ROOT-unset}"
cd /
print -r -- "D:${AGENTMOD_PROJECT_ROOT-unset}"
`)
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}
	wantContains(t, "nearest-wins transitions", stdout,
		"A:"+outer+"\n",
		"B:"+inner+"\n",
		"BCLAUDE:"+filepath.Join(inner, ".agentmod", "claude")+"\n",
		"C:"+inner+"\n", // a plain subdir stays with the nearest project
		"D:unset\n",
	)
}

func TestHookZshPrecmdActivatesNewShellInsideProject(t *testing.T) {
	root := makeProject(t, config.Default())
	// zsh resolves its starting directory physically, so the root the hook
	// finds is the symlink-free path (macOS /var → /private/var).
	physRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	// Interactive shell started INSIDE the project, no cd at all: the eval
	// runs at the first prompt, precmd activates before the second.
	stdout, _ := runZsh(t, root, env, true, `eval "$(agentmod hook zsh)"
print -r -- "ROOT:${AGENTMOD_PROJECT_ROOT-unset}"
exit
`)
	wantContains(t, "precmd activation", stdout, "ROOT:"+physRoot+"\n")
}

func TestHookZshMissingBinaryWarnsOnce(t *testing.T) {
	root := makeProject(t, config.Default())
	hookFile := filepath.Join(t.TempDir(), "hook.zsh")
	if err := os.WriteFile(hookFile, []byte(shellhook.Zsh()), 0o644); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t) // no agentmod on PATH
	stdout, stderr := runZsh(t, t.TempDir(), env, false, `
source '`+hookFile+`'
cd '`+root+`'
cd /
cd '`+root+`'
print -r -- "ACTIVE:${AGENTMOD_ACTIVE-unset}"
`)
	if got := strings.Count(stderr, "binary not found on PATH"); got != 1 {
		t.Errorf("warning printed %d times, want exactly 1\nstderr:\n%s", got, stderr)
	}
	wantContains(t, "no activation without binary", stdout, "ACTIVE:unset\n")
}

func TestHookZshBrokenConfigErrorsOnceAndDeactivatesOld(t *testing.T) {
	good := makeProject(t, config.Default())
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".agentmod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".agentmod", "agentmod.toml"), []byte("not [valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runZsh(t, t.TempDir(), env, false, `
eval "$(agentmod hook zsh)"
cd '`+good+`'
print -r -- "A:${AGENTMOD_PROJECT_ROOT-unset}"
cd '`+broken+`'
print -r -- "B:${AGENTMOD_PROJECT_ROOT-unset}"
print -r -- "BCLAUDE:${CLAUDE_CONFIG_DIR-unset}"
_agentmod_hook
_agentmod_hook
print -r -- "done"
`)
	if got := strings.Count(stderr, "agentmod:"); got != 1 {
		t.Errorf("config error printed %d times, want exactly 1 (failed root must be cached)\nstderr:\n%s", got, stderr)
	}
	// The old project's routing must not linger while standing in the broken
	// project: failed activation falls back to a plain deactivation.
	wantContains(t, "broken-config state", stdout,
		"A:"+good+"\n",
		"B:unset\n",
		"BCLAUDE:unset\n",
		"done\n",
	)
}

func TestHookZshEvalTwiceRegistersOnce(t *testing.T) {
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runZsh(t, t.TempDir(), env, false, `
eval "$(agentmod hook zsh)"
eval "$(agentmod hook zsh)"
print -r -- "PRECMD:${precmd_functions}"
print -r -- "CHPWD:${chpwd_functions}"
`)
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}
	for _, prefix := range []string{"PRECMD:", "CHPWD:"} {
		if got := strings.Count(lineAfter(t, stdout, prefix), "_agentmod_hook"); got != 1 {
			t.Errorf("%s registered %d times, want exactly 1\ngot:\n%s", prefix, got, stdout)
		}
	}
}
