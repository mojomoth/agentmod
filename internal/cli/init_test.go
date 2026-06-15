package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomoth/agentmod/internal/config"
	"github.com/mojomoth/agentmod/internal/layout"
	"github.com/mojomoth/agentmod/internal/project"
)

func runInitForTest(t *testing.T, cwd string, flags ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(append([]string{"init"}, flags...), &out, &errBuf, fakeEnv(cwd, nil))
	return code, out.String(), errBuf.String()
}

func TestInitFresh(t *testing.T) {
	root := t.TempDir()
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	wantContains(t, "stdout", stdout, "AgentMod: initialized", "agentmod status")

	agentmodDir := filepath.Join(root, ".agentmod")
	for _, rel := range layout.Subdirs() {
		info, err := os.Stat(filepath.Join(agentmodDir, rel))
		if err != nil || !info.IsDir() {
			t.Errorf(".agentmod/%s: want directory, got err=%v", rel, err)
		}
	}

	// The written config must round-trip to exactly the defaults.
	cfg, err := config.Load(filepath.Join(agentmodDir, "agentmod.toml"))
	if err != nil {
		t.Fatalf("written agentmod.toml does not load: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("written config = %+v, want defaults %+v", cfg, config.Default())
	}

	stub, err := os.ReadFile(layout.OpencodeConfigPath(agentmodDir))
	if err != nil {
		t.Fatalf("opencode.json not written: %v", err)
	}
	if !strings.Contains(string(stub), "opencode.ai/config.json") {
		t.Errorf("opencode.json stub = %q, want $schema reference", stub)
	}

	// And the result must be a discoverable project that status reports active.
	if _, err := project.Discover(root); err != nil {
		t.Errorf("Discover after init: %v", err)
	}
	code, statusOut, _ := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitOK || !strings.Contains(statusOut, "AgentMod: active") {
		t.Errorf("status after init: exit=%d output:\n%s", code, statusOut)
	}
}

func TestInitReinitNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	agentmodDir := filepath.Join(root, ".agentmod")

	// Pre-existing user-edited files must come through byte-identical.
	customToml := []byte("schema_version = 1\n\n[claude]\nenabled = false\n")
	if err := os.MkdirAll(filepath.Join(agentmodDir, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentmodDir, "agentmod.toml"), customToml, 0o644); err != nil {
		t.Fatal(err)
	}
	customStub := []byte("{\"theme\":\"user-edited\"}\n")
	if err := os.WriteFile(layout.OpencodeConfigPath(agentmodDir), customStub, 0o644); err != nil {
		t.Fatal(err)
	}
	// A file the layout knows nothing about must survive too. (Not
	// claude/settings.json — since T17 that file is managed: init merges the
	// guard hook into it, covered by claudesettings_test.go.)
	stray := filepath.Join(agentmodDir, "claude")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	strayFile := filepath.Join(stray, "user-notes.md")
	if err := os.WriteFile(strayFile, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "already initialized", "left untouched")

	for path, want := range map[string][]byte{
		filepath.Join(agentmodDir, "agentmod.toml"): customToml,
		layout.OpencodeConfigPath(agentmodDir):      customStub,
		strayFile:                                   []byte("# mine\n"),
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed by re-init:\ngot:  %q\nwant: %q", path, got, want)
		}
	}

	// Missing layout dirs are filled in.
	for _, rel := range layout.Subdirs() {
		if _, err := os.Stat(filepath.Join(agentmodDir, rel)); err != nil {
			t.Errorf(".agentmod/%s not created on re-init: %v", rel, err)
		}
	}
}

func TestInitAgentmodIsAFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".agentmod"), []byte("oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runInitForTest(t, root)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "not a directory")
	got, err := os.ReadFile(filepath.Join(root, ".agentmod"))
	if err != nil || string(got) != "oops" {
		t.Errorf("file in the way modified: %q, %v", got, err)
	}
}

func TestInitNestedWarnsAndCreates(t *testing.T) {
	outer := makeProject(t, config.Default())
	inner := filepath.Join(outer, "sub", "dir")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, inner)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "already inside the agentmod project at "+outer, "shadows")
	proj, err := project.Discover(inner)
	if err != nil {
		t.Fatal(err)
	}
	if proj.Root != inner {
		t.Errorf("after nested init, Discover root = %s, want %s", proj.Root, inner)
	}
}

func TestInitAtExistingRootDoesNotWarnNested(t *testing.T) {
	root := makeProject(t, config.Default())
	code, stdout, _ := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if strings.Contains(stdout, "already inside") {
		t.Errorf("re-init at root warned about nesting:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout, "already initialized")
}

// snapshotTree records every entry under root: directories by presence,
// files by their full contents. Two equal snapshots mean the tree is
// byte-identical with the same directory set.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			snap[rel] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snap[rel] = "file:" + string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestInitSecondRunIsNoOp(t *testing.T) {
	root := t.TempDir()
	// A .git directory makes root a git repository for ensureGitignore, so
	// the first run exercises every init effect, .gitignore creation included.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("first init: exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	before := snapshotTree(t, root)

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("second init: exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("second init stderr = %q, want empty", stderr)
	}

	after := snapshotTree(t, root)
	for rel, want := range before {
		got, ok := after[rel]
		if !ok {
			t.Errorf("second init removed %s", rel)
		} else if got != want {
			t.Errorf("second init changed %s:\ngot:  %q\nwant: %q", rel, got, want)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("second init created %s", rel)
		}
	}

	// The report must say every part was already in place.
	wantContains(t, "stdout", stdout,
		"already initialized",
		"all directories already present",
		"already covers .agentmod/",
		"guard hook already configured",
	)
	if got := strings.Count(stdout, "already present, left untouched"); got != 2 {
		t.Errorf("want 2 'left untouched' lines (agentmod.toml, opencode.json), got %d:\n%s", got, stdout)
	}
}

func TestInitRejectsArguments(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown flag":   {"--bogus"},
		"positional arg": {"somedir"},
		"known + bogus":  {"--no-shell-hook", "--bogus"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			code, _, stderr := runInitForTest(t, root, args...)
			if code != ExitError {
				t.Fatalf("exit = %d, want %d", code, ExitError)
			}
			wantContains(t, "stderr", stderr, args[len(args)-1])
			// A rejected invocation must not have started initializing.
			if _, err := os.Stat(filepath.Join(root, ".agentmod")); !os.IsNotExist(err) {
				t.Errorf(".agentmod created despite rejected arguments (stat err = %v)", err)
			}
		})
	}
}

func TestInitFlagNoShellHook(t *testing.T) {
	code, stdout, stderr := runInitForTest(t, t.TempDir(), "--no-shell-hook")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "Shell hook:      skipped (--no-shell-hook)")
}

func TestInitDefaultShellHookLine(t *testing.T) {
	// runInitForTest leaves SHELL and HOME unset, so a plain init reports
	// the can't-detect-shell skip — and must not claim the flag was given.
	code, stdout, _ := runInitForTest(t, t.TempDir())
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	wantContains(t, "stdout", stdout, "Shell hook:      skipped ($SHELL is not set")
	if strings.Contains(stdout, "skipped (--no-shell-hook)") {
		t.Errorf("plain init claims the hook was skipped by flag:\n%s", stdout)
	}
}

// TestInitFlagsBuildIdenticalTree locks in that the T06 flags only change
// reporting (and, later, rc/auth side effects that live OUTSIDE the project
// tree): the .agentmod/ tree and .gitignore they produce are byte-identical
// to a plain init's. Prompting needs no assertion beyond this: runInit has
// no stdin parameter at all, so no flag combination can read input.
func TestInitFlagsBuildIdenticalTree(t *testing.T) {
	freshRepo := func() string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		return root
	}

	plainRoot := freshRepo()
	if code, _, stderr := runInitForTest(t, plainRoot); code != ExitOK {
		t.Fatalf("plain init: exit = %d; stderr:\n%s", code, stderr)
	}
	want := snapshotTree(t, plainRoot)

	for name, flags := range map[string][]string{
		"--yes":             {"--yes"},
		"--non-interactive": {"--non-interactive"},
		"--no-shell-hook":   {"--no-shell-hook"},
		"all combined":      {"--no-shell-hook", "--yes", "--non-interactive"},
	} {
		t.Run(name, func(t *testing.T) {
			root := freshRepo()
			code, _, stderr := runInitForTest(t, root, flags...)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
			}
			got := snapshotTree(t, root)
			for rel, w := range want {
				if g, ok := got[rel]; !ok {
					t.Errorf("flagged init missing %s", rel)
				} else if g != w {
					t.Errorf("flagged init differs at %s:\ngot:  %q\nwant: %q", rel, g, w)
				}
			}
			for rel := range got {
				if _, ok := want[rel]; !ok {
					t.Errorf("flagged init created extra %s", rel)
				}
			}
		})
	}
}
