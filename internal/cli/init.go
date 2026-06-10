package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

// opencodeStub is the OPENCODE_CONFIG target init creates. OpenCode merges
// this file over the global config (FABLE_PLAN §3.3); an empty document is a
// valid starting point the user can extend.
const opencodeStub = "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"

// initOptions carries the parsed `agentmod init` flags (FABLE_PLAN §12).
// Later steps consume these: the rc-file editor (T08) honors NoShellHook,
// and the auth bootstrap (Phase 3) honors NonInteractive by never prompting
// and never copying auth.
type initOptions struct {
	// NoShellHook skips all rc-file modification.
	NoShellHook bool
	// NonInteractive (--yes / --non-interactive) means: never prompt,
	// never copy auth. Init reads nothing from stdin in this mode — and,
	// today, in any mode.
	NonInteractive bool
}

// parseInitFlags accepts only the documented init flags. Anything else —
// unknown flag or positional argument — is an error naming the offender.
func parseInitFlags(args []string) (initOptions, error) {
	var opts initOptions
	for _, arg := range args {
		switch arg {
		case "--no-shell-hook":
			opts.NoShellHook = true
		case "--yes", "--non-interactive":
			opts.NonInteractive = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag %q for init (valid: --no-shell-hook, --yes, --non-interactive)", arg)
			}
			return opts, fmt.Errorf("init takes no positional arguments (got %q)", arg)
		}
	}
	return opts, nil
}

// runInit implements `agentmod init` (FABLE_PLAN §12, IMPLEMENTATION_PLAN
// §4): the .agentmod/ tree, agentmod.toml with defaults, the opencode.json
// stub, the .gitignore entry, and the fenced rc-file hook block, always at
// the current directory. It never deletes or overwrites anything that
// exists, so re-running is safe and only fills gaps.
func runInit(args []string, stdout, stderr io.Writer, env Env) int {
	opts, err := parseInitFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	// Init always targets cwd. An enclosing project does not redirect it:
	// discovery is nearest-wins, so a new project here shadows the outer one
	// (D013) — but that is worth a heads-up, since running init in a subdir
	// of an existing project is a likely accident.
	if proj, err := project.Discover(cwd); err == nil && proj.Root != cwd {
		fmt.Fprintf(stdout, "Note: %s is already inside the agentmod project at %s.\n", cwd, proj.Root)
		fmt.Fprintf(stdout, "Initializing here creates a nested project that shadows it for this directory and below.\n")
	}

	agentmodDir := filepath.Join(cwd, project.DirName)
	reinit := false
	if info, err := os.Stat(agentmodDir); err == nil {
		if !info.IsDir() {
			fmt.Fprintf(stderr, "agentmod: %s exists and is not a directory; move it aside and re-run init\n", agentmodDir)
			return ExitError
		}
		reinit = true
	}

	var created []string
	for _, rel := range layout.Subdirs() {
		dir := filepath.Join(agentmodDir, rel)
		if _, err := os.Stat(dir); err == nil {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "agentmod: %v\n", err)
			return ExitError
		}
		created = append(created, rel)
	}

	configPath := filepath.Join(agentmodDir, project.ConfigFileName)
	defaults, err := config.Marshal(config.Default())
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	wroteConfig, err := writeIfAbsent(configPath, defaults)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	wroteStub, err := writeIfAbsent(layout.OpencodeConfigPath(agentmodDir), []byte(opencodeStub))
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	gitignoreStatus, err := ensureGitignore(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	hookRes, err := ensureShellHook(opts, env)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	if reinit {
		fmt.Fprintf(stdout, "AgentMod: already initialized at %s\n", cwd)
	} else {
		fmt.Fprintf(stdout, "AgentMod: initialized %s\n", agentmodDir)
	}
	fmt.Fprintf(stdout, "  Layout:          %s\n", describeCreated(created))
	fmt.Fprintf(stdout, "  agentmod.toml:   %s\n", describeWrite(wroteConfig, "defaults"))
	fmt.Fprintf(stdout, "  opencode.json:   %s\n", describeWrite(wroteStub, "stub"))
	fmt.Fprintf(stdout, "  .gitignore:      %s\n", gitignoreStatus)
	fmt.Fprintf(stdout, "  Shell hook:      %s\n", hookRes.Line)
	if notice := hookActivationNotice(hookRes, cwd, env); notice != "" {
		fmt.Fprint(stdout, notice)
	}
	fmt.Fprintf(stdout, "Run 'agentmod status' to see where agent homes will route.\n")
	return ExitOK
}

// writeIfAbsent creates path with data only when no file exists there.
// O_EXCL makes the existence check and the create one atomic step, so a
// pre-existing file is never truncated. Returns whether it wrote.
func writeIfAbsent(path string, data []byte) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}

func describeCreated(created []string) string {
	if len(created) == 0 {
		return "all directories already present"
	}
	return "created " + strings.Join(created, ", ")
}

func describeWrite(wrote bool, what string) string {
	if wrote {
		return fmt.Sprintf("written (%s)", what)
	}
	return "already present, left untouched"
}
