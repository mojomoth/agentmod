package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wantGuardCommand is the hook command every test expects: fakeEnv resolves
// the binary to fakeBinPath, and the path is single-quoted for the shell.
const wantGuardCommand = "'" + fakeBinPath + "' guard claude-bash"

func settingsPath(root string) string {
	return filepath.Join(root, ".agentmod", "claude", "settings.json")
}

// loadSettings parses .agentmod/claude/settings.json under root.
func loadSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v\ncontents:\n%s", err, raw)
	}
	return settings
}

// guardCommands returns every PreToolUse hook command containing the guard
// marker, plus the matcher of each entry that carries one.
func guardCommands(t *testing.T, settings map[string]any) (commands, matchers []string) {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]any)
	pre, _ := hooks["PreToolUse"].([]any)
	for _, entry := range pre {
		entryMap, _ := entry.(map[string]any)
		inner, _ := entryMap["hooks"].([]any)
		for _, h := range inner {
			hookMap, _ := h.(map[string]any)
			cmd, _ := hookMap["command"].(string)
			if strings.Contains(cmd, guardHookMarker) {
				commands = append(commands, cmd)
				matcher, _ := entryMap["matcher"].(string)
				matchers = append(matchers, matcher)
			}
		}
	}
	return commands, matchers
}

func TestInitWritesGuardHook(t *testing.T) {
	root := t.TempDir()
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Claude guard:    PreToolUse Bash hook written")

	commands, matchers := guardCommands(t, loadSettings(t, root))
	if len(commands) != 1 || commands[0] != wantGuardCommand {
		t.Errorf("guard commands = %q, want exactly [%q]", commands, wantGuardCommand)
	}
	if len(matchers) != 1 || matchers[0] != "Bash" {
		t.Errorf("guard matchers = %q, want exactly [\"Bash\"]", matchers)
	}
	// The hook entry's type must be "command" for Claude Code to run it.
	settings := loadSettings(t, root)
	hooks := settings["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	hook := pre[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if hook["type"] != "command" {
		t.Errorf("hook type = %v, want \"command\"", hook["type"])
	}
}

func TestInitGuardHookPreservesUserSettings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	user := `{
  "env": {"FOO": "bar"},
  "model": "opus",
  "hooks": {
    "PostToolUse": [{"matcher": "Edit", "hooks": [{"type": "command", "command": "echo done"}]}],
    "PreToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "echo write"}]}]
  }
}
`
	if err := os.WriteFile(settingsPath(root), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Claude guard:    guard hook added to existing")

	settings := loadSettings(t, root)
	if env, _ := settings["env"].(map[string]any); env == nil || env["FOO"] != "bar" {
		t.Errorf("user \"env\" key lost: %v", settings["env"])
	}
	if settings["model"] != "opus" {
		t.Errorf("user \"model\" key lost: %v", settings["model"])
	}
	hooks := settings["hooks"].(map[string]any)
	if post, _ := hooks["PostToolUse"].([]any); len(post) != 1 {
		t.Errorf("user PostToolUse hooks lost: %v", hooks["PostToolUse"])
	}
	pre, _ := hooks["PreToolUse"].([]any)
	if len(pre) != 2 {
		t.Fatalf("PreToolUse entries = %d, want 2 (user's + guard)", len(pre))
	}
	commands, _ := guardCommands(t, settings)
	if len(commands) != 1 || commands[0] != wantGuardCommand {
		t.Errorf("guard commands = %q, want exactly [%q]", commands, wantGuardCommand)
	}
}

func TestInitGuardHookAlreadyConfiguredNoRewrite(t *testing.T) {
	// A file that already carries the current hook command must not be
	// rewritten at all — the user's own formatting (here: single line, odd
	// spacing) survives byte-identically.
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` +
		wantGuardCommand + `"}]}]}}`
	if err := os.WriteFile(settingsPath(root), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Claude guard:    guard hook already configured")

	got, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Errorf("already-configured file rewritten:\ngot:  %q\nwant: %q", got, existing)
	}
}

func TestInitGuardHookUpdatesStaleBinaryPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := `{
  "permissions": {"allow": ["Bash(ls:*)"]},
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "'/old/gone/agentmod' guard claude-bash"}]}]
  }
}
`
	if err := os.WriteFile(settingsPath(root), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Claude guard:    guard hook binary path updated")

	settings := loadSettings(t, root)
	commands, _ := guardCommands(t, settings)
	if len(commands) != 1 || commands[0] != wantGuardCommand {
		t.Errorf("guard commands = %q, want exactly [%q] (stale path repaired, no second entry)", commands, wantGuardCommand)
	}
	if perms, _ := settings["permissions"].(map[string]any); perms == nil {
		t.Errorf("user \"permissions\" key lost on update")
	}
}

func TestInitGuardHookInvalidJSON(t *testing.T) {
	for name, contents := range map[string]string{
		"syntax error":          `{"hooks": [unclosed`,
		"non-object":            `["a", "list"]`,
		"hooks wrong type":      `{"hooks": "a string"}`,
		"PreToolUse wrong type": `{"hooks": {"PreToolUse": {"not": "an array"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".agentmod", "claude")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settingsPath(root), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			code, _, stderr := runInitForTest(t, root)
			if code != ExitError {
				t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitError, stderr)
			}
			wantContains(t, "stderr", stderr, "re-run init")
			got, err := os.ReadFile(settingsPath(root))
			if err != nil || string(got) != contents {
				t.Errorf("broken settings.json modified: %q, %v", got, err)
			}
		})
	}
}

func TestInitGuardHookEmptyFileTreatedAsEmptyObject(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Claude guard:    guard hook added to existing")
	commands, _ := guardCommands(t, loadSettings(t, root))
	if len(commands) != 1 || commands[0] != wantGuardCommand {
		t.Errorf("guard commands = %q, want exactly [%q]", commands, wantGuardCommand)
	}
}

func TestInitGuardHookNeverTouchesProjectClaudeDir(t *testing.T) {
	// FABLE_PLAN §17 placement: the hook goes in the ROUTED home only. The
	// project's own .claude/settings.json is shared via git and must come
	// through init byte-identical.
	root := t.TempDir()
	projectClaude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(projectClaude, 0o755); err != nil {
		t.Fatal(err)
	}
	shared := `{"permissions": {"allow": ["Bash(go test:*)"]}}` + "\n"
	sharedPath := filepath.Join(projectClaude, "settings.json")
	if err := os.WriteFile(sharedPath, []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}

	got, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != shared {
		t.Errorf("project .claude/settings.json modified by init:\ngot:  %q\nwant: %q", got, shared)
	}
	// And the routed home got the hook.
	commands, _ := guardCommands(t, loadSettings(t, root))
	if len(commands) != 1 {
		t.Errorf("routed settings.json missing the guard hook")
	}
}

func TestInitGuardHookUnresolvableBinary(t *testing.T) {
	for name, executable := range map[string]func() (string, error){
		"nil Executable": nil,
		"erroring Executable": func() (string, error) {
			return "", errors.New("procfs unavailable")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			env := fakeEnv(root, nil)
			env.Executable = executable
			var out, errBuf strings.Builder
			code := run([]string{"init"}, &out, &errBuf, env)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, errBuf.String())
			}
			wantContains(t, "stdout", out.String(), "Claude guard:    skipped (cannot resolve the agentmod binary path")
			if _, err := os.Stat(settingsPath(root)); !os.IsNotExist(err) {
				t.Errorf("settings.json created despite unresolvable binary (stat err = %v)", err)
			}
		})
	}
}

func TestInitGuardHookSettingsIsADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(settingsPath(root), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runInitForTest(t, root)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if stderr == "" {
		t.Errorf("want an error naming the problem, got empty stderr")
	}
}
