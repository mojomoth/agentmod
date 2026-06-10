package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// guardEnv is a fakeEnv with stdin attached; HOME is set to a fixed fake
// home so absolute-spelling rules are exercised without the real one.
func guardEnv(stdin string) Env {
	env := fakeEnv("/tmp/proj", map[string]string{"HOME": "/Users/tester"})
	env.Stdin = strings.NewReader(stdin)
	return env
}

func bashHookJSON(t *testing.T, command string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGuardForTest(t *testing.T, env Env, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(append([]string{"guard"}, args...), &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

func TestGuardDenyExit2(t *testing.T) {
	env := guardEnv(bashHookJSON(t, `rm -rf ~/.claude/skills/foo`))
	code, stdout, stderr := runGuardForTest(t, env, "claude-bash")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (hook deny)", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty in exit-2 mode", stdout)
	}
	wantContains(t, "stderr", stderr, "BLOCKED", ".agentmod/")
}

func TestGuardAllowSilentExit0(t *testing.T) {
	env := guardEnv(bashHookJSON(t, `ls -la ~/.claude/skills`))
	code, stdout, stderr := runGuardForTest(t, env, "claude-bash")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("allow must be silent, got stdout %q stderr %q", stdout, stderr)
	}
}

func TestGuardDenyJSONMode(t *testing.T) {
	env := guardEnv(bashHookJSON(t, `echo x >> $HOME/.codex/auth.json`))
	code, stdout, stderr := runGuardForTest(t, env, "claude-bash", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (JSON deny exits 0)", code, ExitOK)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty in --json mode", stderr)
	}
	var out struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if out.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", out.HookSpecificOutput.HookEventName)
	}
	if out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want deny", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Error("permissionDecisionReason is empty")
	}
}

func TestGuardAllowJSONModeSilent(t *testing.T) {
	env := guardEnv(bashHookJSON(t, `cp a.txt b.txt`))
	code, stdout, stderr := runGuardForTest(t, env, "claude-bash", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("allow must be silent, got stdout %q stderr %q", stdout, stderr)
	}
}

func TestGuardHomeFromInjectedEnv(t *testing.T) {
	// A custom HOME (outside /Users and /home) is only catchable when the
	// CLI hands the engine the injected HOME value.
	env := fakeEnv("/tmp/proj", map[string]string{"HOME": "/srv/home1"})
	env.Stdin = strings.NewReader(bashHookJSON(t, `rm -rf /srv/home1/.claude/skills`))
	code, _, stderr := runGuardForTest(t, env, "claude-bash")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr %q", code, stderr)
	}
}

func TestGuardUnparseableFailSafe(t *testing.T) {
	// Garbage referencing a global home → deny.
	code, _, stderr := runGuardForTest(t, guardEnv(`%% rm -rf ~/.claude %%`), "claude-bash")
	if code != 2 {
		t.Fatalf("unparseable+global exit = %d, want 2; stderr %q", code, stderr)
	}
	// Garbage without a global reference → allow, never block everything.
	code, stdout, stderr := runGuardForTest(t, guardEnv(`%% gibberish %%`), "claude-bash")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("unparseable-clean = (%d, %q, %q), want silent 0", code, stdout, stderr)
	}
}

func TestGuardNilAndEmptyStdin(t *testing.T) {
	env := fakeEnv("/tmp/proj", map[string]string{"HOME": "/Users/tester"})
	// fakeEnv leaves Stdin nil — must behave as empty input, not panic.
	code, stdout, stderr := runGuardForTest(t, env, "claude-bash")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("nil stdin = (%d, %q, %q), want silent 0", code, stdout, stderr)
	}
	code, stdout, stderr = runGuardForTest(t, guardEnv(""), "claude-bash")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("empty stdin = (%d, %q, %q), want silent 0", code, stdout, stderr)
	}
}

func TestGuardStdinReadErrorFailsSafe(t *testing.T) {
	// §17: a failing guard must degrade safely — a broken stdin pipe with
	// no global reference in the readable prefix allows.
	env := fakeEnv("/tmp/proj", map[string]string{"HOME": "/Users/tester"})
	env.Stdin = &erringReader{data: []byte(`{"tool_name":`)}
	code, _, _ := runGuardForTest(t, env, "claude-bash")
	if code != ExitOK {
		t.Fatalf("read-error exit = %d, want %d", code, ExitOK)
	}
	// …but a readable prefix referencing a global home still denies.
	env.Stdin = &erringReader{data: []byte(`rm -rf ~/.codex/`)}
	code, _, _ = runGuardForTest(t, env, "claude-bash")
	if code != 2 {
		t.Fatalf("read-error+global exit = %d, want 2", code)
	}
}

type erringReader struct {
	data []byte
	done bool
}

func (r *erringReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, errors.New("boom")
	}
	r.done = true
	return copy(p, r.data), nil
}

func TestGuardUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"zsh"}, {"claude-bash", "extra"}} {
		t.Run(fmt.Sprintf("%v", args), func(t *testing.T) {
			code, _, stderr := runGuardForTest(t, guardEnv(""), args...)
			if code != ExitError {
				t.Fatalf("exit = %d, want %d", code, ExitError)
			}
			wantContains(t, "stderr", stderr, "guard claude-bash")
		})
	}
}
