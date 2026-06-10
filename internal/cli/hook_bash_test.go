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

func TestHookBashScriptContents(t *testing.T) {
	script := shellhook.Bash()
	wantContains(t, "bash hook script", script,
		".agentmod/agentmod.toml",
		"PROMPT_COMMAND",
		"--shell bash --activate",
		"--shell bash --deactivate",
	)
	if strings.Contains(script, "HOME=") {
		t.Error("hook script must never assign HOME")
	}
	if !strings.HasSuffix(script, "\n") {
		t.Error("script must end with a newline")
	}
}

// requireBash returns the bash path or skips the test. /bin/bash is preferred
// so macOS runs exercise the oldest supported bash (3.2: no associative
// arrays, no ${var,,}) rather than a newer Homebrew bash earlier on PATH.
func requireBash(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash"
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	return bash
}

func TestHookBashSyntaxValid(t *testing.T) {
	bash := requireBash(t)
	cmd := exec.Command(bash, "--norc", "--noprofile", "-n")
	cmd.Stdin = strings.NewReader(shellhook.Bash())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n rejected the hook script: %v\n%s", err, out)
	}
}

// runBash feeds script to `bash --norc --noprofile` (user rc files are never
// read) started in dir with the given env. PROMPT_COMMAND only fires before
// prompts, i.e. in interactive mode; non-interactive tests must call
// _agentmod_hook explicitly after cd.
func runBash(t *testing.T, dir string, env []string, interactive bool, script string) (string, string) {
	t.Helper()
	bash := requireBash(t)
	args := []string{"--norc", "--noprofile"}
	if interactive {
		args = append(args, "-i")
	}
	cmd := exec.Command(bash, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(script)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("bash failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errBuf.String())
	}
	return out.String(), errBuf.String()
}

func TestHookBashActivatesOnCdInAndOut(t *testing.T) {
	root := makeProject(t, config.Default())
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runBash(t, t.TempDir(), env, false, `
eval "$(agentmod hook bash)"
printf '%s\n' "HOME0:$HOME"
printf '%s\n' "PATH0:$PATH"
cd '`+root+`'
_agentmod_hook
printf '%s\n' "ROOT1:${AGENTMOD_PROJECT_ROOT-unset}"
printf '%s\n' "ACTIVE1:${AGENTMOD_ACTIVE-unset}"
printf '%s\n' "CLAUDE1:${CLAUDE_CONFIG_DIR-unset}"
printf '%s\n' "CODEX1:${CODEX_HOME-unset}"
printf '%s\n' "OPENCODE1:${OPENCODE_CONFIG-unset}"
printf '%s\n' "XDG1:${XDG_CONFIG_HOME-unset}"
printf '%s\n' "PATH1:$PATH"
cd /
_agentmod_hook
printf '%s\n' "ROOT2:${AGENTMOD_PROJECT_ROOT-unset}"
printf '%s\n' "CLAUDE2:${CLAUDE_CONFIG_DIR-unset}"
printf '%s\n' "PATH2:$PATH"
printf '%s\n' "HOME2:$HOME"
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

func TestHookBashNestedNearestWins(t *testing.T) {
	outer := t.TempDir()
	writeProjectMarker(t, outer, config.Default())
	inner := filepath.Join(outer, "inner")
	writeProjectMarker(t, inner, config.Default())
	sub := filepath.Join(inner, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runBash(t, t.TempDir(), env, false, `
eval "$(agentmod hook bash)"
cd '`+outer+`'
_agentmod_hook
printf '%s\n' "A:${AGENTMOD_PROJECT_ROOT-unset}"
cd '`+inner+`'
_agentmod_hook
printf '%s\n' "B:${AGENTMOD_PROJECT_ROOT-unset}"
printf '%s\n' "BCLAUDE:${CLAUDE_CONFIG_DIR-unset}"
cd '`+sub+`'
_agentmod_hook
printf '%s\n' "C:${AGENTMOD_PROJECT_ROOT-unset}"
cd /
_agentmod_hook
printf '%s\n' "D:${AGENTMOD_PROJECT_ROOT-unset}"
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

func TestHookBashPromptCommandActivatesNewShellInsideProject(t *testing.T) {
	root := makeProject(t, config.Default())
	// bash resolves its starting directory physically when no PWD is
	// inherited (macOS /var → /private/var), like zsh.
	physRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	// Interactive shell started INSIDE the project, no cd and no manual hook
	// call: PROMPT_COMMAND fires between the eval and the next command.
	// stderr is ignored — forced-interactive bash without a tty prints
	// prompts and a job-control notice there.
	stdout, _ := runBash(t, root, env, true, `eval "$(agentmod hook bash)"
printf '%s\n' "ROOT:${AGENTMOD_PROJECT_ROOT-unset}"
exit
`)
	wantContains(t, "PROMPT_COMMAND activation", stdout, "ROOT:"+physRoot+"\n")
}

func TestHookBashMissingBinaryWarnsOnce(t *testing.T) {
	root := makeProject(t, config.Default())
	hookFile := filepath.Join(t.TempDir(), "hook.bash")
	if err := os.WriteFile(hookFile, []byte(shellhook.Bash()), 0o644); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t) // no agentmod on PATH
	stdout, stderr := runBash(t, t.TempDir(), env, false, `
. '`+hookFile+`'
cd '`+root+`'
_agentmod_hook
cd /
_agentmod_hook
cd '`+root+`'
_agentmod_hook
printf '%s\n' "ACTIVE:${AGENTMOD_ACTIVE-unset}"
`)
	if got := strings.Count(stderr, "binary not found on PATH"); got != 1 {
		t.Errorf("warning printed %d times, want exactly 1\nstderr:\n%s", got, stderr)
	}
	wantContains(t, "no activation without binary", stdout, "ACTIVE:unset\n")
}

func TestHookBashBrokenConfigErrorsOnceAndDeactivatesOld(t *testing.T) {
	good := makeProject(t, config.Default())
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".agentmod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".agentmod", "agentmod.toml"), []byte("not [valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runBash(t, t.TempDir(), env, false, `
eval "$(agentmod hook bash)"
cd '`+good+`'
_agentmod_hook
printf '%s\n' "A:${AGENTMOD_PROJECT_ROOT-unset}"
cd '`+broken+`'
_agentmod_hook
printf '%s\n' "B:${AGENTMOD_PROJECT_ROOT-unset}"
printf '%s\n' "BCLAUDE:${CLAUDE_CONFIG_DIR-unset}"
_agentmod_hook
_agentmod_hook
printf '%s\n' "done"
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

func TestHookBashEvalTwiceRegistersOnceAndKeepsExisting(t *testing.T) {
	env := childEnv(t, fakeAgentmodBin(t))
	stdout, stderr := runBash(t, t.TempDir(), env, false, `
PROMPT_COMMAND='true'
eval "$(agentmod hook bash)"
eval "$(agentmod hook bash)"
printf '%s\n' "PC:$PROMPT_COMMAND"
`)
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}
	if got := lineAfter(t, stdout, "PC:"); got != "true;_agentmod_hook" {
		t.Errorf("PROMPT_COMMAND = %q, want %q (append once, keep user entry)", got, "true;_agentmod_hook")
	}
}
