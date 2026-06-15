package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mojomoth/agentmod/internal/config"
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
	// fakeEnv has no HOME: the pollution check reports the skip honestly.
	wantContains(t, "stdout", stdout, "Global skills check: skipped (HOME not set")

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
	// git's own diagnosis must be forwarded verbatim (D033): for a missing
	// local path git says `fatal: repository '…' does not exist` — words no
	// agentmod message uses, so their presence proves the forwarding.
	wantContains(t, "stderr", stderr, "fatal:", "does not exist")
	// And the hint names the source override for mirror/offline situations.
	wantContains(t, "stderr", stderr, "check network access", gstackSourceEnvVar)
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

func TestInstallGstackGitMissing(t *testing.T) {
	root := makeProject(t, config.Default())

	// install resolves git with exec.LookPath on the REAL process PATH
	// (unlike doctor's injected-Env walk — see installGstack's doc comment),
	// so crippling the real PATH is the honest way to simulate a machine
	// without git. t.Setenv restores it afterwards; PATH is not global agent
	// state, so this stays within the harness rules.
	t.Setenv("PATH", t.TempDir())

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "install gstack needs git, which was not found on PATH")
	// The git check runs before any directory creation.
	if _, err := os.Lstat(filepath.Join(root, ".agentmod", "claude", "skills")); !os.IsNotExist(err) {
		t.Errorf("skills dir created despite missing git (err %v)", err)
	}
}

func TestInstallGstackSetupFailureSkillsBlocked(t *testing.T) {
	root := makeProject(t, config.Default())
	claudeDir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A regular FILE where the skills directory belongs: every path-based
	// operation on claude/skills/... fails with ENOTDIR. The bogus local
	// source guarantees that even a (buggy) clone attempt fails fast offline.
	blocker := filepath.Join(claudeDir, "skills")
	if err := os.WriteFile(blocker, []byte("user file in the way\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: filepath.Join(t.TempDir(), "no-such-repo"),
	}))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	// The PathError passthrough is the distinct diagnosis: it names the
	// operation, the blocked path, and the OS cause (D033).
	wantContains(t, "stderr", stderr, "not a directory", filepath.Join("claude", "skills"))
	if got, err := os.ReadFile(blocker); err != nil || string(got) != "user file in the way\n" {
		t.Errorf("blocking file was disturbed: %q, %v", got, err)
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

func TestDiffListings(t *testing.T) {
	cases := []struct {
		name           string
		before, after  []string
		added, removed []string
	}{
		{"both empty", nil, nil, nil, nil},
		{"identical", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"added one", []string{"a"}, []string{"a", "b"}, []string{"b"}, nil},
		{"added into empty", nil, []string{"x"}, []string{"x"}, nil},
		{"removed one", []string{"a", "b"}, []string{"b"}, nil, []string{"a"}},
		{"removed all", []string{"a", "b"}, nil, nil, []string{"a", "b"}},
		{"added and removed", []string{"a", "c"}, []string{"b", "c", "d"}, []string{"b", "d"}, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			added, removed := diffListings(tc.before, tc.after)
			if !slicesEqual(added, tc.added) || !slicesEqual(removed, tc.removed) {
				t.Errorf("diffListings(%v, %v) = added %v removed %v, want added %v removed %v",
					tc.before, tc.after, added, removed, tc.added, tc.removed)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestInstallGstackGlobalSkillsUnchanged(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	home := t.TempDir()
	skills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(skills, "gstack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "other-skill"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A PRE-EXISTING global gstack (the dev-machine D010 situation) must not
	// trip the check — only a before/after delta is a violation.
	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
		"HOME":             home,
	}))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "Global skills check: "+skills+" unchanged")
	if strings.Contains(stderr, "VIOLATION") {
		t.Errorf("pre-existing global entries reported as a violation:\n%s", stderr)
	}
	entries, err := os.ReadDir(skills)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("global skills dir changed: %d entries, want 2", len(entries))
	}
}

func TestInstallGstackGlobalDeltaViolation(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Point the fake global skills dir AT the project-local skills dir via a
	// symlink: the (legitimate, project-local) install then shows up as a new
	// entry in the "global" listing, exercising the real violation path
	// end-to-end without any production test hook.
	projectSkills := filepath.Join(root, ".agentmod", "claude", "skills")
	if err := os.Symlink(projectSkills, filepath.Join(home, ".claude", "skills")); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
		"HOME":             home,
	}))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitError, stdout, stderr)
	}
	wantContains(t, "stderr", stderr, "VIOLATION")
	wantContains(t, "stderr", stderr, "new entries: gstack")
	wantContains(t, "stderr", stderr, "report this as a bug")
	// The local install itself succeeded and is reported before the check.
	wantContains(t, "stdout", stdout, "Installed gstack to ")
	// The not-touched success paragraph must NOT print after a violation.
	if strings.Contains(stdout, "was not touched") {
		t.Errorf("success paragraph printed despite violation:\n%s", stdout)
	}
}

func TestInstallGstackGlobalCheckUnreadableSkips(t *testing.T) {
	requireGit(t)
	fixture := makeGstackFixtureRepo(t)
	root := makeProject(t, config.Default())
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	// skills is a regular file: ReadDir fails with ENOTDIR, the check skips
	// with a note instead of failing the install.
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInstallForTest(t, fakeEnv(root, map[string]string{
		gstackSourceEnvVar: fixture,
		"HOME":             home,
	}))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout, stderr)
	}
	wantContains(t, "stdout", stdout, "Global skills check: skipped (cannot read ")
}

func TestVerifyGlobalSkillsRemovedEntry(t *testing.T) {
	home := t.TempDir()
	skills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(skills, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := fakeEnv(t.TempDir(), map[string]string{"HOME": home})
	before := snapshotGlobalSkills(env)
	if err := os.Remove(filepath.Join(skills, "b")); err != nil {
		t.Fatal(err)
	}

	var out, errBuf strings.Builder
	if verifyGlobalSkillsUnchanged(before, "/fake/target", &out, &errBuf, env) {
		t.Fatalf("removed entry not reported as a violation\nstdout:\n%s\nstderr:\n%s", out.String(), errBuf.String())
	}
	wantContains(t, "stderr", errBuf.String(), "VIOLATION", "entries that disappeared: b")
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
