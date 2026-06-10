package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// All rc-file tests run against throwaway homes injected through Env
// (T08's hard constraint): the real user's rc files are never read or
// written — the guard hook would block it, and LOOP.md forbids it.

// runInitWithEnv runs init with explicit env vars (HOME, SHELL, ZDOTDIR…).
func runInitWithEnv(t *testing.T, cwd string, vars map[string]string, flags ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(append([]string{"init"}, flags...), &out, &errBuf, fakeEnv(cwd, vars))
	return code, out.String(), errBuf.String()
}

func zshEnv(home string) map[string]string {
	return map[string]string{"HOME": home, "SHELL": "/bin/zsh"}
}

func TestInitInstallsZshHookBlock(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInitWithEnv(t, t.TempDir(), zshEnv(home))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	got, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf(".zshrc not created: %v", err)
	}
	if string(got) != rcBlockFor("zsh") {
		t.Errorf(".zshrc = %q, want exactly the fenced block %q", got, rcBlockFor("zsh"))
	}
	wantContains(t, "stdout", stdout,
		"Shell hook:      installed in ~"+string(filepath.Separator)+".zshrc",
		"takes effect in new shells",
	)
}

func TestInitAppendsToExistingRcPreservingBytes(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	// No trailing newline: the editor must glue one on, not corrupt the line.
	userContent := "export FOO=1\nalias ll='ls -l'"
	if err := os.WriteFile(rc, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runInitWithEnv(t, t.TempDir(), zshEnv(home))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	got, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	want := userContent + "\n" + rcBlockFor("zsh")
	if string(got) != want {
		t.Errorf(".zshrc after init:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestInitRcBlockIdempotent(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	if code, _, stderr := runInitWithEnv(t, proj, zshEnv(home)); code != ExitOK {
		t.Fatalf("first init: exit = %d; stderr:\n%s", code, stderr)
	}
	before, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitWithEnv(t, proj, zshEnv(home))
	if code != ExitOK {
		t.Fatalf("second init: exit = %d; stderr:\n%s", code, stderr)
	}
	after, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("second init changed .zshrc:\nbefore: %q\nafter:  %q", before, after)
	}
	if got := strings.Count(string(after), rcBeginMarker); got != 1 {
		t.Errorf("want exactly 1 begin marker after re-init, got %d", got)
	}
	wantContains(t, "stdout", stdout, "Shell hook:      already installed in")
}

func TestInitUpdatesStaleBlockInPlace(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	stale := rcBeginMarker + "\n" + "eval \"$(agentmod hook zsh)\" # ancient v0 line\n" + rcEndMarker + "\n"
	prefix := "# user header\nexport PATH=$PATH:/custom\n"
	suffix := "# user footer — must survive\nalias g=git\n"
	if err := os.WriteFile(rc, []byte(prefix+stale+suffix), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitWithEnv(t, t.TempDir(), zshEnv(home))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	got, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	want := prefix + rcBlockFor("zsh") + suffix
	if string(got) != want {
		t.Errorf(".zshrc after update:\ngot:  %q\nwant: %q", got, want)
	}
	wantContains(t, "stdout", stdout, "Shell hook:      updated in")
}

func TestInitNoShellHookNeverTouchesRc(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	userContent := "export UNTOUCHED=1\n"
	if err := os.WriteFile(rc, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitWithEnv(t, t.TempDir(), zshEnv(home), "--no-shell-hook")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	got, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != userContent {
		t.Errorf("--no-shell-hook modified .zshrc:\ngot:  %q\nwant: %q", got, userContent)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("--no-shell-hook created extra files in home: %v", entries)
	}
	wantContains(t, "stdout", stdout, "Shell hook:      skipped (--no-shell-hook)")
}

func TestInitShellHookSkips(t *testing.T) {
	for name, tc := range map[string]struct {
		vars map[string]string
		want string
	}{
		"unsupported shell": {
			vars: map[string]string{"HOME": t.TempDir(), "SHELL": "/usr/bin/fish"},
			want: `skipped (unsupported shell "fish"`,
		},
		"no SHELL": {
			vars: map[string]string{"HOME": t.TempDir()},
			want: "skipped ($SHELL is not set",
		},
		"no HOME": {
			vars: map[string]string{"SHELL": "/bin/zsh"},
			want: "skipped ($HOME is not set",
		},
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runInitWithEnv(t, t.TempDir(), tc.vars)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
			}
			wantContains(t, "stdout", stdout, "Shell hook:      "+tc.want)
			if home := tc.vars["HOME"]; home != "" {
				entries, err := os.ReadDir(home)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Errorf("skip path created files in home: %v", entries)
				}
			}
		})
	}
}

func TestInitBashRcFileSelection(t *testing.T) {
	bashEnv := func(home string) map[string]string {
		return map[string]string{"HOME": home, "SHELL": "/bin/bash"}
	}
	t.Run("prefers existing .bashrc", func(t *testing.T) {
		home := t.TempDir()
		for _, f := range []string{".bashrc", ".bash_profile"} {
			if err := os.WriteFile(filepath.Join(home, f), []byte("# user\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if code, _, stderr := runInitWithEnv(t, t.TempDir(), bashEnv(home)); code != ExitOK {
			t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
		}
		rc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
		profile, _ := os.ReadFile(filepath.Join(home, ".bash_profile"))
		if !strings.Contains(string(rc), rcBeginMarker) {
			t.Errorf("block not in .bashrc:\n%s", rc)
		}
		if strings.Contains(string(profile), rcBeginMarker) {
			t.Errorf("block leaked into .bash_profile:\n%s", profile)
		}
	})
	t.Run("falls back to existing .bash_profile", func(t *testing.T) {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("# user\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runInitWithEnv(t, t.TempDir(), bashEnv(home))
		if code != ExitOK {
			t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
		}
		profile, err := os.ReadFile(filepath.Join(home, ".bash_profile"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(profile), `eval "$(agentmod hook bash)"`) {
			t.Errorf("bash block not in .bash_profile:\n%s", profile)
		}
		if _, err := os.Stat(filepath.Join(home, ".bashrc")); !os.IsNotExist(err) {
			t.Errorf(".bashrc created although .bash_profile existed (stat err = %v)", err)
		}
		wantContains(t, "stdout", stdout, ".bash_profile")
	})
	t.Run("creates .bashrc when neither exists", func(t *testing.T) {
		home := t.TempDir()
		if code, _, stderr := runInitWithEnv(t, t.TempDir(), bashEnv(home)); code != ExitOK {
			t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
		}
		rc, err := os.ReadFile(filepath.Join(home, ".bashrc"))
		if err != nil {
			t.Fatalf(".bashrc not created: %v", err)
		}
		if string(rc) != rcBlockFor("bash") {
			t.Errorf(".bashrc = %q, want %q", rc, rcBlockFor("bash"))
		}
	})
}

func TestInitZshHonorsZdotdir(t *testing.T) {
	home := t.TempDir()
	zdot := filepath.Join(home, "config", "zsh")
	if err := os.MkdirAll(zdot, 0o755); err != nil {
		t.Fatal(err)
	}
	vars := zshEnv(home)
	vars["ZDOTDIR"] = zdot
	if code, _, stderr := runInitWithEnv(t, t.TempDir(), vars); code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(zdot, ".zshrc")); err != nil {
		t.Errorf("ZDOTDIR .zshrc not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Errorf("~/.zshrc written despite ZDOTDIR (stat err = %v)", err)
	}
}

func TestInitCorruptFenceIsAnError(t *testing.T) {
	for name, content := range map[string]string{
		"start without end": "# user\n" + rcBeginMarker + "\neval something\n# not the end\n",
		"duplicate blocks":  rcBlockFor("zsh") + "# user\n" + rcBlockFor("zsh"),
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			rc := filepath.Join(home, ".zshrc")
			if err := os.WriteFile(rc, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			code, _, stderr := runInitWithEnv(t, t.TempDir(), zshEnv(home))
			if code != ExitError {
				t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitError, stderr)
			}
			wantContains(t, "stderr", stderr, ".zshrc", "re-run init")
			got, err := os.ReadFile(rc)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Errorf("corrupt rc file was modified:\ngot:  %q\nwant: %q", got, content)
			}
		})
	}
}

// TestRcBlockShellSyntax sanity-checks the emitted block parses in the shell
// it targets (zsh -n / bash -n), so a typo in the fence content can't brick
// a user's rc file.
func TestRcBlockShellSyntax(t *testing.T) {
	t.Run("zsh", func(t *testing.T) {
		cmd := exec.Command(requireZsh(t), "-f", "-n")
		cmd.Stdin = strings.NewReader(rcBlockFor("zsh"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("zsh -n rejected the rc block: %v\n%s", err, out)
		}
	})
	t.Run("bash", func(t *testing.T) {
		cmd := exec.Command(requireBash(t), "--norc", "--noprofile", "-n")
		cmd.Stdin = strings.NewReader(rcBlockFor("bash"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bash -n rejected the rc block: %v\n%s", err, out)
		}
	})
}
