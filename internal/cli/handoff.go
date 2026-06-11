package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"time"

	"github.com/agentmod/agentmod/internal/handoff"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

// runHandoff dispatches `agentmod handoff <subcommand>` (FABLE_PLAN §18).
// Only create is implemented so far; the others are named explicitly so
// users learn they are planned rather than mistyped.
func runHandoff(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "agentmod: handoff requires a subcommand (try 'agentmod handoff create')\n")
		return ExitError
	}
	switch args[0] {
	case "create":
		return runHandoffCreate(args[1:], stdout, stderr, env)
	case "restore", "inspect", "verify", "list":
		fmt.Fprintf(stderr, "agentmod: handoff %s is not implemented yet\n", args[0])
		return ExitError
	}
	fmt.Fprintf(stderr, "agentmod: unknown handoff subcommand %q (try 'agentmod handoff create')\n", args[0])
	return ExitError
}

// runHandoffCreate implements `agentmod handoff create [--output PATH]
// [--allow-findings] [--allow-dirty]`: pack this project's .agentmod/ into
// a .amod snapshot (handoff.Create). The default output is
// .agentmod/snapshots/<project>-<timestamp>.amod.
func runHandoffCreate(args []string, stdout, stderr io.Writer, env Env) int {
	output := ""
	allowFindings := false
	allowDirty := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "agentmod: handoff create: --output requires a path\n")
				return ExitError
			}
			i++
			output = args[i]
		case args[i] == "--allow-findings":
			allowFindings = true
		case args[i] == "--allow-dirty":
			allowDirty = true
		default:
			fmt.Fprintf(stderr, "agentmod: handoff create: unsupported argument %q (supported: --output PATH, --allow-findings, --allow-dirty)\n", args[i])
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
		fmt.Fprintf(stderr, "agentmod: handoff create requires an agentmod project; run 'agentmod init' first (%v)\n", err)
		return ExitNotInProject
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	// §20 dirty-worktree gate: a snapshot carries the agent environment,
	// not source changes, so a handoff cut from a dirty tree silently loses
	// work unless the user explicitly accepts that.
	gitState, gitNote := collectGitState(proj.Root)
	if gitState != nil && gitState.Dirty && !allowDirty {
		fmt.Fprintf(stderr, "agentmod: handoff create: refusing to pack: the git worktree is dirty (%s); uncommitted source changes do not travel in a snapshot — commit or stash them so the handoff matches a commit, or re-run with --allow-dirty to pack anyway\n", gitState.StatusSummary)
		return ExitError
	}

	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}
	if output == "" {
		name := fmt.Sprintf("%s-%s.amod", filepath.Base(proj.Root), now.UTC().Format("20060102-150405"))
		output = filepath.Join(proj.AgentmodDir, layout.SnapshotsDir, name)
	}

	goos := env.GOOS
	if goos == "" {
		goos = "unknown"
	}
	res, err := handoff.Create(handoff.CreateOptions{
		ProjectRoot:   proj.Root,
		OutputPath:    output,
		CreatedAt:     now,
		Version:       Version,
		Platform:      goos + "/" + runtime.GOARCH,
		AllowFindings: allowFindings,
		Git:           gitState,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "Created handoff snapshot: %s\n", res.OutputPath)
	fmt.Fprintf(stdout, "  payload: %d files, %d bytes (manifest, inventory, and checksums included)\n", res.PayloadFiles, res.PayloadBytes)
	switch {
	case gitState == nil:
		fmt.Fprintf(stdout, "  git: metadata omitted (%s)\n", gitNote)
	case gitState.Dirty:
		fmt.Fprintf(stdout, "  git: %s, DIRTY (%s) — packed anyway (--allow-dirty); uncommitted source changes do not travel in a snapshot\n", gitIdentity(gitState), gitState.StatusSummary)
	default:
		fmt.Fprintf(stdout, "  git: %s, clean\n", gitIdentity(gitState))
	}
	switch n := len(res.Excluded); n {
	case 0:
		fmt.Fprintf(stdout, "  excluded by default policy: nothing\n")
	case 1:
		fmt.Fprintf(stdout, "  excluded by default policy: 1 entry\n")
	default:
		fmt.Fprintf(stdout, "  excluded by default policy: %d entries\n", n)
	}
	for _, e := range res.Excluded {
		fmt.Fprintf(stdout, "    %s (%s)\n", e.Path, e.RuleID)
	}
	switch n := len(res.Findings); n {
	case 0:
		fmt.Fprintf(stdout, "  secret scan: clean (no candidate patterns in packed files)\n")
	case 1:
		fmt.Fprintf(stdout, "  secret scan: 1 candidate finding (details in REDACTION.md inside the snapshot)\n")
	default:
		fmt.Fprintf(stdout, "  secret scan: %d candidate findings (details in REDACTION.md inside the snapshot)\n", n)
	}
	for _, f := range res.Findings {
		mark := ""
		if f.Hard {
			mark = ", HARD — packed because --allow-findings was given"
		}
		fmt.Fprintf(stdout, "    %s line %d (%s%s)\n", f.Path, f.Line, f.Pattern, mark)
	}
	fmt.Fprintf(stdout, "Verify or restore it on the target machine with 'agentmod handoff' (restore lands in a later release).\n")
	return ExitOK
}
