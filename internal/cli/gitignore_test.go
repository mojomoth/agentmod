package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gitDir marks root as a git repository the lexical way insideGitRepo
// detects it.
func gitDir(t *testing.T, root string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readGitignore(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore: %v", err)
	}
	return string(data)
}

func TestInitGitignoreCreatedInGitRepo(t *testing.T) {
	root := t.TempDir()
	gitDir(t, root)
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      created with .agentmod/")
	if got := readGitignore(t, root); got != ".agentmod/\n" {
		t.Errorf(".gitignore = %q, want %q", got, ".agentmod/\n")
	}
}

func TestInitGitignoreAppendsPreservingContent(t *testing.T) {
	root := t.TempDir()
	gitDir(t, root)
	prior := "node_modules/\n# .agentmod/ commented out does not count\n!.agentmod/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      added .agentmod/")
	if got := readGitignore(t, root); got != prior+".agentmod/\n" {
		t.Errorf(".gitignore = %q, want prior content + entry", got)
	}
}

func TestInitGitignoreAppendsNewlineWhenMissing(t *testing.T) {
	root := t.TempDir()
	gitDir(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("dist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runInitForTest(t, root); code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	if got := readGitignore(t, root); got != "dist\n.agentmod/\n" {
		t.Errorf(".gitignore = %q, want %q", got, "dist\n.agentmod/\n")
	}
}

func TestInitGitignoreDedup(t *testing.T) {
	for _, existing := range []string{".agentmod/", ".agentmod", "/.agentmod", "/.agentmod/", "  .agentmod/  "} {
		t.Run(existing, func(t *testing.T) {
			root := t.TempDir()
			gitDir(t, root)
			prior := "vendor/\n" + existing + "\n"
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(prior), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr := runInitForTest(t, root)
			if code != ExitOK {
				t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
			}
			wantContains(t, "stdout", stdout, ".gitignore:      already covers .agentmod/")
			if got := readGitignore(t, root); got != prior {
				t.Errorf(".gitignore changed:\ngot:  %q\nwant: %q", got, prior)
			}
		})
	}
}

func TestInitGitignoreSkippedOutsideGitRepo(t *testing.T) {
	root := t.TempDir()
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      skipped (not a git repository")
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore created in non-git directory (stat err = %v)", err)
	}
}

func TestInitGitignoreExistingFileExtendedEvenOutsideGitRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("dist/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      added .agentmod/")
	if got := readGitignore(t, root); got != "dist/\n.agentmod/\n" {
		t.Errorf(".gitignore = %q, want %q", got, "dist/\n.agentmod/\n")
	}
}

func TestInitGitignoreDetectsRepoInAncestor(t *testing.T) {
	repo := t.TempDir()
	gitDir(t, repo)
	sub := filepath.Join(repo, "services", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, sub)
	if code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      created with .agentmod/")
	if got := readGitignore(t, sub); got != ".agentmod/\n" {
		t.Errorf(".gitignore = %q, want %q", got, ".agentmod/\n")
	}
}

func TestInitGitignoreWorktreeGitFileCounts(t *testing.T) {
	root := t.TempDir()
	// Worktrees and submodules have .git as a regular file, not a directory.
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d; stderr:\n%s", code, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      created with .agentmod/")
}

func TestInitGitignoreSecondRunIsNoOp(t *testing.T) {
	root := t.TempDir()
	gitDir(t, root)
	if code, _, stderr := runInitForTest(t, root); code != ExitOK {
		t.Fatalf("first init: exit = %d; stderr:\n%s", code, stderr)
	}
	first := readGitignore(t, root)
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("second init: exit = %d; stderr:\n%s", code, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore:      already covers .agentmod/")
	if got := readGitignore(t, root); got != first {
		t.Errorf("second init changed .gitignore:\nfirst:  %q\nsecond: %q", first, got)
	}
	if strings.Count(readGitignore(t, root), ".agentmod/") != 1 {
		t.Errorf("duplicate entries:\n%s", readGitignore(t, root))
	}
}

func TestInitGitignoreIsADirectory(t *testing.T) {
	root := t.TempDir()
	gitDir(t, root)
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"init"}, &out, &errBuf, fakeEnv(root, nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d; stdout:\n%s", code, ExitError, out.String())
	}
	wantContains(t, "stderr", errBuf.String(), ".gitignore")
}
