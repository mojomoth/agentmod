package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
)

func requireGit(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found on PATH; skipping installer test")
	}
	return gitPath
}

// runGitFixture runs git in dir with the user's global/system config masked
// out, so commit identity, signing, and hooks come only from the flags here.
func runGitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitPath := requireGit(t)
	base := []string{"-C", dir,
		"-c", "user.name=fixture",
		"-c", "user.email=fixture@example.invalid",
		"-c", "commit.gpgsign=false",
	}
	cmd := exec.Command(gitPath, append(base, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeGstackFixtureRepo builds a tiny local git repo standing in for
// github.com/garrytan/gstack, so installer tests never touch the network.
func makeGstackFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitFixture(t, dir, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# gstack fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, dir, "add", "-A")
	runGitFixture(t, dir, "commit", "--quiet", "-m", "fixture")
	return dir
}

func runInstallForTest(t *testing.T, env Env, extra ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf strings.Builder
	code = run(append([]string{"install", "gstack"}, extra...), &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

func TestInstallGstackClonesIntoProject(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
	}))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil {
		t.Errorf("cloned SKILL.md missing: %v", err)
	} else if string(got) != "# gstack fixture\n" {
		t.Errorf("cloned SKILL.md = %q", got)
	}
	if fi, err := os.Stat(filepath.Join(target, ".git")); err != nil || !fi.IsDir() {
		t.Errorf(".git missing in clone (err %v): later updates need it", err)
	}
	wantContains(t, "stdout", stdout, "Cloning gstack from "+fixture)
	wantContains(t, "stdout", stdout, "Installed gstack to "+target)
	wantContains(t, "stdout", stdout, "project-local")

	// Atomic install leaves no temp dirs behind in the skills dir.
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "gstack" {
			t.Errorf("unexpected entry %q left in skills dir", e.Name())
		}
	}
}

func TestInstallGstackOutsideProject(t *testing.T) {
	dir := t.TempDir()
	code, stdout, stderr := runInstallForTest(t, fakeEnv(dir, nil))
	if code != ExitNotInProject {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitNotInProject, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "requires an agentmod project")
	wantContains(t, "stderr", stderr, "agentmod init")
}

func TestInstallGstackAlreadyInstalled(t *testing.T) {
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No source override: an accidental clone attempt would hit the network
	// and fail loudly rather than silently pass.
	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "already installed at "+target)
	wantContains(t, "stderr", stderr, "--force")
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "mine\n" {
		t.Errorf("existing install was disturbed: %q, %v", got, err)
	}
}

func TestInstallGstackCloneFailure(t *testing.T) {
	requireGit(t)
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: filepath.Join(t.TempDir(), "no-such-repo"),
	}))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "git clone failed")
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("target %s exists after failed clone (err %v)", target, err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("entry %q left in skills dir after failed clone", e.Name())
	}
}

func TestInstallArgValidation(t *testing.T) {
	root := makeProject(t, config.Default())
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no component", []string{"install"}, "install requires a component"},
		{"unknown component", []string{"install", "superpowers"}, `unknown install component "superpowers"`},
		{"unknown flag", []string{"install", "gstack", "--frobnicate"}, `unsupported argument "--frobnicate"`},
		{"extra arg after --force", []string{"install", "gstack", "--force", "extra"}, `unsupported argument "extra"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf strings.Builder
			code := run(tc.args, &out, &errBuf, fakeEnv(root, nil))
			if code != ExitError {
				t.Fatalf("exit = %d, want %d\nstderr:\n%s", code, ExitError, errBuf.String())
			}
			wantContains(t, "stderr", errBuf.String(), tc.want)
			// Validation failures must not create anything.
			if _, err := os.Lstat(filepath.Join(root, ".agentmod", "claude", "skills")); !os.IsNotExist(err) {
				t.Errorf("skills dir created despite argument error (err %v)", err)
			}
		})
	}
}

func TestInstallGstackForceReplacesExisting(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("old install\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
	}), "--force")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "Replacing existing install at "+target)
	wantContains(t, "stdout", stdout, "Installed gstack to "+target)
	if _, err := os.Lstat(sentinel); !os.IsNotExist(err) {
		t.Errorf("old install's keep.txt survived --force (err %v)", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil {
		t.Errorf("cloned SKILL.md missing: %v", err)
	} else if string(got) != "# gstack fixture\n" {
		t.Errorf("cloned SKILL.md = %q", got)
	}
	// The old copy and the clone temp dir are both gone.
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "gstack" {
			t.Errorf("unexpected entry %q left in skills dir after --force", e.Name())
		}
	}
}

func TestInstallGstackForceWithoutExisting(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
	}), "--force")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout, stderr)
	}
	if strings.Contains(stdout, "Replacing existing install") {
		t.Errorf("--force with nothing installed claimed to replace something:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout, "Installed gstack to "+target)
	if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
		t.Errorf("cloned SKILL.md missing: %v", err)
	}
}

func TestInstallGstackForceCloneFailureKeepsOld(t *testing.T) {
	requireGit(t)
	root := makeProject(t, config.Default())
	target := filepath.Join(root, ".agentmod", gstackRelProject)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("old install\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: filepath.Join(t.TempDir(), "no-such-repo"),
	}), "--force")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "git clone failed")
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "old install\n" {
		t.Errorf("existing install was disturbed by failed --force: %q, %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "gstack" {
			t.Errorf("unexpected entry %q left in skills dir after failed --force", e.Name())
		}
	}
}
