package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agentmod/agentmod/internal/handoff"
)

// collectGitState gathers the FABLE_PLAN §20 manifest metadata for the git
// repository containing projectRoot by EXECUTING git (the D030 exception:
// like install, this needs the real tool; internal/handoff stays exec-free
// so snapshot writing is deterministic under test).
//
// A nil state means metadata is unavailable and note says why in one clause
// ("git binary not found on PATH", "not a git repository"). That is never a
// hard failure — handoff must work in non-git projects.
func collectGitState(projectRoot string) (st *handoff.GitState, note string) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, "git binary not found on PATH"
	}
	if out, err := gitOutput(gitPath, projectRoot, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(out) != "true" {
		return nil, "not a git repository"
	}
	st = &handoff.GitState{}
	// symbolic-ref fails (quietly, -q) on a detached HEAD: branch stays "".
	if out, err := gitOutput(gitPath, projectRoot, "symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		st.Branch = strings.TrimSpace(out)
	}
	// rev-parse --verify fails on an unborn branch (no commits yet): head
	// stays "".
	if out, err := gitOutput(gitPath, projectRoot, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		st.Head = strings.TrimSpace(out)
	}
	// -c forces untracked files into the listing even when the user's config
	// hides them from display: the dirty flag is about what the snapshot
	// does not carry, not about the status UI.
	porcelain, err := gitOutput(gitPath, projectRoot, "-c", "status.showUntrackedFiles=normal", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Sprintf("git status failed: %v", err)
	}
	st.Dirty, st.StatusSummary = summarizeStatus(porcelain)
	// No origin remote → RemoteURL stays "" and the manifest omits it.
	if out, err := gitOutput(gitPath, projectRoot, "remote", "get-url", "origin"); err == nil {
		st.RemoteURL = redactRemoteURL(strings.TrimSpace(out))
	}
	return st, ""
}

// gitOutput runs git -C root <args> and returns raw stdout (not trimmed:
// `status --porcelain` lines start with a significant space).
// GIT_OPTIONAL_LOCKS=0 keeps status from refreshing the index and
// GIT_TERMINAL_PROMPT=0 keeps a misconfigured remote from prompting, so
// every call here is read-only and non-interactive.
func gitOutput(gitPath, root string, args ...string) (string, error) {
	cmd := exec.Command(gitPath, append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	return string(out), err
}

// summarizeStatus folds `git status --porcelain` output into the manifest's
// dirty flag and human summary: "clean", or nonzero counts like
// "1 staged, 2 modified, 3 untracked". Untracked files count as dirty —
// they are source state the snapshot does not carry. A conflicted entry
// (both columns set) counts as both staged and modified.
func summarizeStatus(porcelain string) (dirty bool, summary string) {
	var staged, modified, untracked int
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' {
			untracked++
			continue
		}
		if x != ' ' {
			staged++
		}
		if y != ' ' {
			modified++
		}
	}
	var parts []string
	for _, c := range []struct {
		n    int
		word string
	}{{staged, "staged"}, {modified, "modified"}, {untracked, "untracked"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.word))
		}
	}
	if len(parts) == 0 {
		return false, "clean"
	}
	return true, strings.Join(parts, ", ")
}

// redactRemoteURL strips the userinfo from scheme://-style remote URLs:
// credential tokens travel there (https://user:token@host/...,
// https://token@host/...), and even a bare username is not the snapshot
// reader's business. ssh:// loses its conventional git@ too — the manifest
// value documents WHERE the remote is, it is not meant to be dialed
// verbatim. scp-like syntax (git@host:path) has no credential slot and is
// kept unchanged.
func redactRemoteURL(raw string) string {
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return raw
	}
	rest := raw[schemeEnd+3:]
	authority := rest
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		authority = rest[:slash]
	}
	at := strings.LastIndexByte(authority, '@')
	if at < 0 {
		return raw
	}
	return raw[:schemeEnd+3] + rest[at+1:]
}

// gitIdentity renders the collected branch@commit for stdout, naming the
// detached/unborn cases instead of printing empty fields.
func gitIdentity(st *handoff.GitState) string {
	head := st.Head
	if head == "" {
		head = "(no commits yet)"
	}
	if st.Branch == "" {
		return "detached HEAD @ " + head
	}
	return fmt.Sprintf("branch %s @ %s", st.Branch, head)
}
