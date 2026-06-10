package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentmod/agentmod/internal/project"
)

// install gstack (FABLE_PLAN §16, IMPLEMENTATION_PLAN §10): clone gstack
// directly into the project-local skills dir, never the global Claude home,
// and never run gstack's own setup script (it hardcodes ~/.claude/skills).

const (
	gstackDefaultSource = "https://github.com/garrytan/gstack"

	// gstackSourceEnvVar overrides the clone source (read through the
	// injected Env). It exists so tests clone from a local fixture repo
	// instead of the network; it also serves mirrors/forks.
	gstackSourceEnvVar = "AGENTMOD_GSTACK_SOURCE"
)

func runInstall(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "agentmod: install requires a component (try 'agentmod install gstack')\n")
		return ExitError
	}
	if args[0] != "gstack" {
		fmt.Fprintf(stderr, "agentmod: unknown install component %q (only \"gstack\" is supported)\n", args[0])
		return ExitError
	}
	force := false
	for _, a := range args[1:] {
		switch a {
		case "--force":
			force = true
		default:
			fmt.Fprintf(stderr, "agentmod: install gstack: unsupported argument %q (only --force is supported)\n", a)
			return ExitError
		}
	}

	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	proj, err := project.Discover(cwd)
	if errors.Is(err, project.ErrNotFound) {
		fmt.Fprintf(stderr, "agentmod: install gstack requires an agentmod project; run 'agentmod init' first (%v)\n", err)
		return ExitNotInProject
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	return installGstack(proj.AgentmodDir, force, stdout, stderr, env)
}

// installGstack clones gstack into agentmodDir/claude/skills/gstack via a
// sibling temp dir + atomic rename, so the target only ever appears complete.
// With force, the clone still happens FIRST; the existing install is only
// moved aside after the clone succeeded, so a failed clone never destroys it.
// Unlike doctor's statBinaryOnPath (a read-only report honoring injected
// Env), install actually executes git, so it resolves it with exec.LookPath
// on the real process PATH — the same PATH the child inherits.
func installGstack(agentmodDir string, force bool, stdout, stderr io.Writer, env Env) int {
	// IMPLEMENTATION_PLAN §10 defense in depth: snapshot the global Claude
	// skills listing before doing anything, re-read it after the install,
	// and treat any delta as a violation. The clone targets a project-local
	// temp dir and cannot write there, but verify anyway.
	globalBefore := snapshotGlobalSkills(env)

	target := filepath.Join(agentmodDir, gstackRelProject)
	exists := false
	if _, err := os.Lstat(target); err == nil {
		if !force {
			fmt.Fprintf(stderr, "agentmod: gstack is already installed at %s; re-run with --force to replace it\n", target)
			return ExitError
		}
		exists = true
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: install gstack needs git, which was not found on PATH\n")
		return ExitError
	}

	source := gstackDefaultSource
	if v, ok := env.LookupEnv(gstackSourceEnvVar); ok && v != "" {
		source = v
	}

	skillsDir := filepath.Dir(target)
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	// Temp dir next to the target so the final rename is atomic (same fs).
	tmp, err := os.MkdirTemp(skillsDir, ".gstack-clone-")
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	defer os.RemoveAll(tmp)

	fmt.Fprintf(stdout, "Cloning gstack from %s\n", source)
	cmd := exec.Command(gitPath, "clone", "--", source, tmp)
	// Never let git sit on an interactive credential prompt.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(stderr, "agentmod: git clone failed: %v\n%s", err, out)
		return ExitError
	}
	if exists {
		fmt.Fprintf(stdout, "Replacing existing install at %s (--force)\n", target)
		oldTmp, err := os.MkdirTemp(skillsDir, ".gstack-old-")
		if err != nil {
			fmt.Fprintf(stderr, "agentmod: %v\n", err)
			return ExitError
		}
		// macOS rename(2) refuses an existing directory destination even
		// when empty, so MkdirTemp only reserves a unique name; remove it
		// before renaming the old install onto it.
		if err := os.Remove(oldTmp); err != nil {
			fmt.Fprintf(stderr, "agentmod: %v\n", err)
			return ExitError
		}
		if err := os.Rename(target, oldTmp); err != nil {
			fmt.Fprintf(stderr, "agentmod: %v\n", err)
			return ExitError
		}
		if err := os.Rename(tmp, target); err != nil {
			if rerr := os.Rename(oldTmp, target); rerr == nil {
				fmt.Fprintf(stderr, "agentmod: %v (previous install restored)\n", err)
			} else {
				fmt.Fprintf(stderr, "agentmod: %v (previous install preserved at %s: %v)\n", err, oldTmp, rerr)
			}
			return ExitError
		}
		if err := os.RemoveAll(oldTmp); err != nil {
			fmt.Fprintf(stderr, "agentmod: warning: previous install left at %s (%v)\n", oldTmp, err)
		}
	} else if err := os.Rename(tmp, target); err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	fmt.Fprintf(stdout, "Installed gstack to %s\n", target)
	if !verifyGlobalSkillsUnchanged(globalBefore, target, stdout, stderr, env) {
		return ExitError
	}
	fmt.Fprintf(stdout, "\ngstack is project-local: only this project's Claude (routed via the\nshell hook) sees it. The global ~/.claude/skills was not touched.\nRun 'agentmod doctor' to confirm.\n")
	return ExitOK
}

// globalSkillsSnapshot captures the entry listing of the global Claude
// skills directory ($HOME + dir(gstackRelGlobal)) for the §10 before/after
// pollution verification.
type globalSkillsSnapshot struct {
	dir     string   // "" when HOME is unset (cannot locate the directory)
	names   []string // sorted entry names; nil when the directory is absent
	readErr error    // directory present (or stat-ambiguous) but unreadable
}

// snapshotGlobalSkills reads the global skills listing through the injected
// Env only (D025 pattern). An absent directory is a valid empty listing —
// the violation test is a before/after DELTA, never mere existence (doctor
// already warns on existence, D010).
func snapshotGlobalSkills(env Env) globalSkillsSnapshot {
	home, ok := env.LookupEnv("HOME")
	if !ok || home == "" {
		return globalSkillsSnapshot{}
	}
	dir := filepath.Join(home, filepath.Dir(gstackRelGlobal))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return globalSkillsSnapshot{dir: dir}
		}
		return globalSkillsSnapshot{dir: dir, readErr: err}
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return globalSkillsSnapshot{dir: dir, names: names}
}

// diffListings compares two sorted name listings and returns the names only
// in after (added) and only in before (removed).
func diffListings(before, after []string) (added, removed []string) {
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		switch {
		case before[i] == after[j]:
			i++
			j++
		case before[i] < after[j]:
			removed = append(removed, before[i])
			i++
		default:
			added = append(added, after[j])
			j++
		}
	}
	removed = append(removed, before[i:]...)
	added = append(added, after[j:]...)
	return added, removed
}

// verifyGlobalSkillsUnchanged re-snapshots the global skills directory and
// compares it against the pre-install snapshot. A delta prints a VIOLATION
// report on stderr and returns false (the caller exits nonzero); an
// unverifiable state (HOME unset, unreadable directory) prints a skip note
// and returns true — the check is defense in depth, not a gate that may
// fail installs on machines without a global Claude home.
func verifyGlobalSkillsUnchanged(before globalSkillsSnapshot, target string, stdout, stderr io.Writer, env Env) bool {
	if before.dir == "" {
		fmt.Fprintf(stdout, "Global skills check: skipped (HOME not set; cannot locate the global Claude skills directory)\n")
		return true
	}
	if before.readErr != nil {
		fmt.Fprintf(stdout, "Global skills check: skipped (cannot read %s: %v)\n", before.dir, before.readErr)
		return true
	}
	after := snapshotGlobalSkills(env)
	if after.readErr != nil {
		fmt.Fprintf(stdout, "Global skills check: skipped (cannot read %s: %v)\n", after.dir, after.readErr)
		return true
	}
	added, removed := diffListings(before.names, after.names)
	if len(added) == 0 && len(removed) == 0 {
		fmt.Fprintf(stdout, "Global skills check: %s unchanged\n", before.dir)
		return true
	}
	fmt.Fprintf(stderr, "agentmod: VIOLATION: %s changed during install\n", before.dir)
	if len(added) > 0 {
		fmt.Fprintf(stderr, "agentmod:   new entries: %s — inspect and remove them by hand\n", strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		fmt.Fprintf(stderr, "agentmod:   entries that disappeared: %s\n", strings.Join(removed, ", "))
	}
	fmt.Fprintf(stderr, "agentmod: the project-local install at %s itself succeeded, but agentmod must\nagentmod: never affect the global directory — please report this as a bug\n", target)
	return false
}
