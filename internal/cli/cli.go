// Package cli implements the agentmod command dispatcher: flag parsing,
// help text, and exit codes. Subcommands register here as they land.
package cli

import (
	"fmt"
	"io"
)

// Version is overridable at build time via
// -ldflags "-X github.com/agentmod/agentmod/internal/cli.Version=v1.2.3".
var Version = "0.1.0-dev"

// Exit codes (IMPLEMENTATION_PLAN §3).
const (
	ExitOK           = 0 // success
	ExitError        = 1 // generic error
	ExitNotInProject = 2 // not inside an agentmod project where one is required
	ExitValidation   = 3 // validation/verification failure (doctor, snapshot verify)
)

const usage = `agentmod — per-project isolation and handoff for coding agents

Usage:
  agentmod <command> [arguments]

Commands:
  init       create .agentmod/ with agent homes and default config here
             (--no-shell-hook skips rc edits; --yes/--non-interactive
              never prompts, never copies auth)
  status     show whether AgentMod is active here and where agent homes route
  doctor     diagnose project, config, layout, shell-hook, and routing state
             (read-only; exits 3 when problems are found)
  env        print shell code applying/undoing routing (used by the shell
             hook; --shell <zsh|bash> with --activate <root> or --deactivate)
  hook       print the shell hook script for your rc file to eval
             (hook zsh | hook bash)
  guard      PreToolUse guard for Claude Code (guard claude-bash): reads the
             hook JSON on stdin and exits 2 with the reason on stderr to
             block global-agent-home writes (--json emits a
             permissionDecision instead); allowed commands exit 0 silently
  install    install a managed component into this project only
             (install gstack clones into .agentmod/claude/skills/gstack;
              the global ~/.claude/skills is never touched)
  version    print version and exit
  help       show this help

Flags:
  --version  print version and exit
`

// Run dispatches command-line args (without the program name) and returns
// the process exit code. All output goes to the supplied writers so tests
// can capture it.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, osEnv())
}

// run is Run with the process environment injected, so tests control cwd and
// env lookups without touching real process state.
func run(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr, env)
	case "status":
		return runStatus(args[1:], stdout, stderr, env)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr, env)
	case "env":
		return runEnv(args[1:], stdout, stderr, env)
	case "hook":
		return runHook(args[1:], stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdout, stderr, env)
	case "install":
		return runInstall(args[1:], stdout, stderr, env)
	case "version", "--version":
		fmt.Fprintf(stdout, "agentmod %s\n", Version)
		return ExitOK
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	fmt.Fprintf(stderr, "agentmod: unknown command %q\n", args[0])
	fmt.Fprintf(stderr, "Run 'agentmod help' for usage.\n")
	return ExitError
}
