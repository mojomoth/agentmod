package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestRedactRemoteURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"https plain", "https://example.com/org/repo.git", "https://example.com/org/repo.git"},
		{"https user:token", "https://user:sk-FAKE-fixture@example.com/org/repo.git", "https://example.com/org/repo.git"},
		{"https token only", "https://ghp-FAKE-fixture@example.com/org/repo.git", "https://example.com/org/repo.git"},
		{"https with port", "https://user:pw@example.com:8443/org/repo.git", "https://example.com:8443/org/repo.git"},
		{"ssh url", "ssh://git@example.com/org/repo.git", "ssh://example.com/org/repo.git"},
		{"scp-like kept", "git@example.com:org/repo.git", "git@example.com:org/repo.git"},
		{"local path kept", "/srv/git/repo.git", "/srv/git/repo.git"},
		// An @ in the path (not the authority) is not userinfo.
		{"at sign in path", "https://example.com/org/repo@v2.git", "https://example.com/org/repo@v2.git"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactRemoteURL(tc.in); got != tc.want {
				t.Errorf("redactRemoteURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSummarizeStatus(t *testing.T) {
	cases := []struct {
		name      string
		porcelain string
		wantDirty bool
		want      string
	}{
		{"clean", "", false, "clean"},
		{"staged", "A  new.go\n", true, "1 staged"},
		{"modified", " M cmd/main.go\n", true, "1 modified"},
		{"untracked", "?? scratch.txt\n", true, "1 untracked"},
		{"all kinds", "A  new.go\nM  also.go\n M edited.go\n?? a.txt\n?? b.txt\n", true, "2 staged, 1 modified, 2 untracked"},
		{"staged and modified same file", "MM both.go\n", true, "1 staged, 1 modified"},
		{"conflict counts both", "UU clash.go\n", true, "1 staged, 1 modified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dirty, summary := summarizeStatus(tc.porcelain)
			if dirty != tc.wantDirty || summary != tc.want {
				t.Errorf("summarizeStatus(%q) = (%v, %q), want (%v, %q)",
					tc.porcelain, dirty, summary, tc.wantDirty, tc.want)
			}
		})
	}
}

func TestCollectGitStateGitMissing(t *testing.T) {
	// collectGitState resolves git on the REAL process PATH (D030, like
	// install); masking PATH simulates a git-less machine.
	t.Setenv("PATH", t.TempDir())
	st, note := collectGitState(t.TempDir())
	if st != nil {
		t.Errorf("state = %+v, want nil", st)
	}
	if note != "git binary not found on PATH" {
		t.Errorf("note = %q", note)
	}
}

func TestCollectGitStateNotARepo(t *testing.T) {
	requireGit(t)
	st, note := collectGitState(t.TempDir())
	if st != nil {
		t.Errorf("state = %+v, want nil", st)
	}
	if note != "not a git repository" {
		t.Errorf("note = %q", note)
	}
}

func TestCollectGitStateCleanRepoWithRemote(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGitFixture(t, root, "init", "--quiet", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, root, "add", "-A")
	runGitFixture(t, root, "commit", "--quiet", "-m", "fixture")
	runGitFixture(t, root, "remote", "add", "origin", "https://user:sk-FAKE-fixture@example.com/org/repo.git")

	st, note := collectGitState(root)
	if st == nil {
		t.Fatalf("state nil, note %q", note)
	}
	if note != "" {
		t.Errorf("note = %q, want empty", note)
	}
	if st.Branch != "main" {
		t.Errorf("branch = %q, want main", st.Branch)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(st.Head) {
		t.Errorf("head = %q, want a 40-hex commit hash", st.Head)
	}
	if st.Dirty || st.StatusSummary != "clean" {
		t.Errorf("dirty = %v, summary = %q, want clean", st.Dirty, st.StatusSummary)
	}
	if st.RemoteURL != "https://example.com/org/repo.git" {
		t.Errorf("remote = %q, want credentials stripped", st.RemoteURL)
	}
	if st.SourceIncluded {
		t.Error("SourceIncluded = true; no code path can include source yet")
	}
}

func TestCollectGitStateDirtyAndDetached(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGitFixture(t, root, "init", "--quiet", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, root, "add", "-A")
	runGitFixture(t, root, "commit", "--quiet", "-m", "fixture")
	runGitFixture(t, root, "checkout", "--quiet", "--detach")

	// One of each: an unstaged edit, a staged new file, an untracked file.
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, root, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(root, "loose.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, note := collectGitState(root)
	if st == nil {
		t.Fatalf("state nil, note %q", note)
	}
	if st.Branch != "" {
		t.Errorf("branch = %q, want empty on a detached HEAD", st.Branch)
	}
	if st.Head == "" {
		t.Error("head empty; a detached HEAD still has a commit")
	}
	if !st.Dirty {
		t.Error("dirty = false")
	}
	if st.StatusSummary != "1 staged, 1 modified, 1 untracked" {
		t.Errorf("summary = %q", st.StatusSummary)
	}
}

func TestCollectGitStateUnbornBranch(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGitFixture(t, root, "init", "--quiet", "-b", "main")

	st, note := collectGitState(root)
	if st == nil {
		t.Fatalf("state nil, note %q", note)
	}
	if st.Branch != "main" {
		t.Errorf("branch = %q, want main (symbolic-ref works before the first commit)", st.Branch)
	}
	if st.Head != "" {
		t.Errorf("head = %q, want empty on an unborn branch", st.Head)
	}
	if st.Dirty || st.StatusSummary != "clean" {
		t.Errorf("dirty = %v, summary = %q, want clean (empty worktree)", st.Dirty, st.StatusSummary)
	}
}
