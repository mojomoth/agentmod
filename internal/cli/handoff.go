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

// runHandoffCreate implements `agentmod handoff create [--output PATH]`:
// pack this project's .agentmod/ into a .amod snapshot (handoff.Create).
// The default output is .agentmod/snapshots/<project>-<timestamp>.amod.
func runHandoffCreate(args []string, stdout, stderr io.Writer, env Env) int {
	output := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "agentmod: handoff create: --output requires a path\n")
				return ExitError
			}
			i++
			output = args[i]
		default:
			fmt.Fprintf(stderr, "agentmod: handoff create: unsupported argument %q (only --output PATH is supported)\n", args[i])
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
		ProjectRoot: proj.Root,
		OutputPath:  output,
		CreatedAt:   now,
		Version:     Version,
		Platform:    goos + "/" + runtime.GOARCH,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "Created handoff snapshot: %s\n", res.OutputPath)
	fmt.Fprintf(stdout, "  payload: %d files, %d bytes (manifest, inventory, and checksums included)\n", res.PayloadFiles, res.PayloadBytes)
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
	fmt.Fprintf(stdout, "Verify or restore it on the target machine with 'agentmod handoff' (restore lands in a later release).\n")
	return ExitOK
}
