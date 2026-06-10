package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Obviously-fake fixture credentials (CHECKS.md §5).
const (
	fakeClaudeCred = "{\"token\":\"sk-FAKE-claude-fixture\"}\n"
	fakeCodexAuth  = "{\"token\":\"sk-FAKE-codex-fixture\"}\n"
)

// mkGlobalHome builds a fake $HOME containing global Claude and/or Codex
// auth files. Our code only ever sees it through the injected Env — the real
// global homes are never touched.
func mkGlobalHome(t *testing.T, withClaude, withCodex bool) string {
	t.Helper()
	home := t.TempDir()
	if withClaude {
		dir := filepath.Join(home, globalClaudeDirName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, claudeAuthFile), []byte(fakeClaudeCred), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if withCodex {
		dir := filepath.Join(home, globalCodexDirName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, codexAuthFile), []byte(fakeCodexAuth), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// runInitWithAuth runs init with a HOME-bearing Env and the given stdin.
func runInitWithAuth(t *testing.T, cwd, home, stdin string, flags ...string) (code int, stdout, stderr string) {
	t.Helper()
	env := fakeEnv(cwd, map[string]string{"HOME": home})
	if stdin != "" {
		env.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	code = run(append([]string{"init"}, flags...), &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

func localAuthPath(root, agentDir, authFile string) string {
	return filepath.Join(root, ".agentmod", agentDir, authFile)
}

func wantNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("%s exists, want absent", path)
	}
}

func wantAuthCopy(t *testing.T, path, content string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("%s mode = %o, want 600", path, perm)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("%s content = %q, want %q", path, got, content)
	}
}

func TestInitAuthConsentCopiesBoth(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	code, stdout, stderr := runInitWithAuth(t, root, home, "y\nyes\n")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"Copy global Claude auth ("+filepath.Join(home, ".claude", claudeAuthFile)+") into this project's Claude home? [y/N]",
		"Copy global Codex auth ("+filepath.Join(home, ".codex", codexAuthFile)+") into this project's Codex home? [y/N]",
		"Claude auth:     copied from "+filepath.Join(home, ".claude", claudeAuthFile)+" (mode 0600)",
		"Codex auth:      copied from "+filepath.Join(home, ".codex", codexAuthFile)+" (mode 0600)")
	wantAuthCopy(t, localAuthPath(root, "claude", claudeAuthFile), fakeClaudeCred)
	wantAuthCopy(t, localAuthPath(root, "codex", codexAuthFile), fakeCodexAuth)
}

func TestInitAuthMixedConsentSharedReader(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	// "y" for Claude, then decline Codex via EOF on a final partial line —
	// proves the shared reader sequences answers and handles missing \n.
	code, stdout, _ := runInitWithAuth(t, root, home, "y\nn")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     copied from",
		"Codex auth:      not copied (declined); "+codexReloginRemedy)
	wantAuthCopy(t, localAuthPath(root, "claude", claudeAuthFile), fakeClaudeCred)
	wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
}

func TestInitAuthDeclineCopiesNothing(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	code, stdout, _ := runInitWithAuth(t, root, home, "n\n\n")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     not copied (declined); "+claudeReloginRemedy,
		"Codex auth:      not copied (declined); "+codexReloginRemedy)
	wantNoFile(t, localAuthPath(root, "claude", claudeAuthFile))
	wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
}

func TestInitAuthNilStdinDeclines(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	// fakeEnv leaves Stdin nil — the prompt must decline, never block or copy.
	code, stdout, _ := runInitWithAuth(t, root, home, "")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     not copied (declined)",
		"Codex auth:      not copied (declined)")
	wantNoFile(t, localAuthPath(root, "claude", claudeAuthFile))
	wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
}

func TestInitAuthNonInteractiveNeverPromptsNeverCopies(t *testing.T) {
	for _, flag := range []string{"--yes", "--non-interactive"} {
		t.Run(flag, func(t *testing.T) {
			root := t.TempDir()
			home := mkGlobalHome(t, true, true)
			// Stdin offers consent, but non-interactive mode must not read it.
			code, stdout, _ := runInitWithAuth(t, root, home, "y\ny\n", flag)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d", code, ExitOK)
			}
			if strings.Contains(stdout, "[y/N]") {
				t.Errorf("non-interactive init prompted:\n%s", stdout)
			}
			wantContains(t, "stdout", stdout,
				"Claude auth:     not copied (non-interactive mode never copies auth); "+claudeReloginRemedy,
				"Codex auth:      not copied (non-interactive mode never copies auth); "+codexReloginRemedy)
			wantNoFile(t, localAuthPath(root, "claude", claudeAuthFile))
			wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
		})
	}
}

func TestInitAuthNoGlobalAuthNothingToOffer(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, false, false)
	code, stdout, _ := runInitWithAuth(t, root, home, "y\ny\n")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("init prompted with no global auth present:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     no global auth to copy ("+filepath.Join(home, ".claude", claudeAuthFile)+" not found); "+claudeReloginRemedy,
		"Codex auth:      no global auth to copy ("+filepath.Join(home, ".codex", codexAuthFile)+" not found); "+codexReloginRemedy)
	wantNoFile(t, localAuthPath(root, "claude", claudeAuthFile))
	wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
}

func TestInitAuthHomeUnset(t *testing.T) {
	root := t.TempDir()
	code, stdout, stderr := runInitForTest(t, root) // fakeEnv(root, nil): no HOME
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("init prompted with HOME unset:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     cannot locate the global Claude home (HOME unset); "+claudeReloginRemedy,
		"Codex auth:      cannot locate the global Codex home (HOME unset); "+codexReloginRemedy)
}

func TestInitAuthDarwinClaudeIsKeychain(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	env := fakeEnv(root, map[string]string{"HOME": home})
	env.GOOS = "darwin"
	env.Stdin = strings.NewReader("y\n") // single answer: only Codex may prompt
	var out, errBuf bytes.Buffer
	code := run([]string{"init"}, &out, &errBuf, env)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, errBuf.String())
	}
	stdout := out.String()
	if strings.Contains(stdout, "Copy global Claude auth") {
		t.Errorf("darwin init offered a Claude auth copy:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     stored in the shared macOS Keychain",
		"Codex auth:      copied from "+filepath.Join(home, ".codex", codexAuthFile))
	wantNoFile(t, localAuthPath(root, "claude", claudeAuthFile))
	wantAuthCopy(t, localAuthPath(root, "codex", codexAuthFile), fakeCodexAuth)
}

func TestInitAuthAlreadyPresentReinitDoesNotPromptOrTouch(t *testing.T) {
	root := t.TempDir()
	home := mkGlobalHome(t, true, true)
	if code, _, stderr := runInitWithAuth(t, root, home, "y\ny\n"); code != ExitOK {
		t.Fatalf("first init exit = %d; stderr:\n%s", code, stderr)
	}
	before := snapshotTree(t, filepath.Join(root, ".agentmod"))

	code, stdout, _ := runInitWithAuth(t, root, home, "y\ny\n")
	if code != ExitOK {
		t.Fatalf("re-init exit = %d, want %d", code, ExitOK)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("re-init prompted although auth is already present:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout,
		"Claude auth:     already present ("+claudeAuthFile+"), left untouched",
		"Codex auth:      already present ("+codexAuthFile+"), left untouched")
	after := snapshotTree(t, filepath.Join(root, ".agentmod"))
	diffTrees(t, ".agentmod", before, after)
}

func TestInitAuthGlobalNotRegularFile(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	// The "auth file" is a directory — must not be copied, must not prompt.
	if err := os.MkdirAll(filepath.Join(home, globalCodexDirName, codexAuthFile), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runInitWithAuth(t, root, home, "y\ny\n")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Errorf("init prompted for a non-regular global auth file:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout,
		"Codex auth:      global "+filepath.Join(home, ".codex", codexAuthFile)+" is not a regular file — not copying; "+codexReloginRemedy)
	wantNoFile(t, localAuthPath(root, "codex", codexAuthFile))
}
