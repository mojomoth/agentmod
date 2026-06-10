package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// gitignoreEntry is the line init ensures in the project's .gitignore
// (FABLE_PLAN §12, §25: .agentmod/ is gitignored by default).
const gitignoreEntry = ".agentmod/"

// ensureGitignore makes sure <dir>/.gitignore ignores .agentmod/ and returns
// a one-line status for the init report. User content is preserved
// byte-for-byte; the entry is appended, never inserted. When .gitignore is
// missing it is created only inside a git repository — in a plain directory
// init skips instead of surprising the user with a new file (D014); an
// already-present .gitignore is extended either way, since it protects a
// future `git init`.
func ensureGitignore(dir string) (string, error) {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	existed := err == nil
	switch {
	case existed:
		if gitignoreCovers(data) {
			return "already covers " + gitignoreEntry, nil
		}
	case os.IsNotExist(err):
		if !insideGitRepo(dir) {
			return "skipped (not a git repository; re-run init after 'git init')", nil
		}
	default:
		return "", err
	}

	entry := gitignoreEntry + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		entry = "\n" + entry
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(entry); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if existed {
		return "added " + gitignoreEntry, nil
	}
	return "created with " + gitignoreEntry, nil
}

// gitignoreCovers reports whether the .gitignore content already ignores the
// .agentmod directory: a line whose trimmed form is the entry itself or one
// of its equivalent spellings (with/without trailing slash, root-anchored).
// Git strips unescaped trailing whitespace itself, so trimming is faithful;
// commented or negated lines do not match and so do not count (D014).
func gitignoreCovers(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case ".agentmod", ".agentmod/", "/.agentmod", "/.agentmod/":
			return true
		}
	}
	return false
}

// insideGitRepo reports whether dir is inside a git repository: a lexical
// upward walk looking for a .git entry. Any file type counts — a directory
// in normal repos, a regular file in worktrees and submodules. No git
// executable is invoked (D014).
func insideGitRepo(dir string) bool {
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
