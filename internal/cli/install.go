package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

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
	fmt.Fprintf(stdout, "\ngstack is project-local: only this project's Claude (routed via the\nshell hook) sees it. The global ~/.claude/skills was not touched.\nRun 'agentmod doctor' to confirm.\n")
	return ExitOK
}
