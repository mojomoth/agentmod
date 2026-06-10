package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
	"github.com/agentmod/agentmod/internal/routing"
)

// Env carries the parts of the process environment the CLI reads, so tests
// can substitute fakes without touching real process state. GOOS is the
// runtime.GOOS value, injected so platform-specific findings (the macOS
// Keychain note) are testable on any host; fakes leave it "" (treated as
// not-darwin) unless a test sets it explicitly.
type Env struct {
	Getwd     func() (string, error)
	LookupEnv func(key string) (string, bool)
	GOOS      string
}

func osEnv() Env {
	return Env{Getwd: os.Getwd, LookupEnv: os.LookupEnv, GOOS: runtime.GOOS}
}

// runStatus implements `agentmod status` (FABLE_PLAN §24): a brief report of
// whether AgentMod governs the current directory and, when it does, where
// each agent home routes. Inactive is a normal answer, not an error, so both
// states exit 0; only broken state (unreadable cwd, invalid config) errors.
//
// Until the shell hook lands, the paths shown are what WILL be routed; the
// "Shell routing" line reports whether the hook has actually applied them
// (AGENTMOD_ACTIVE) in this shell.
func runStatus(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "agentmod: status takes no arguments (got %q)\n", strings.Join(args, " "))
		return ExitError
	}
	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	proj, err := project.Discover(cwd)
	if errors.Is(err, project.ErrNotFound) {
		fmt.Fprint(stdout, ""+
			"AgentMod: inactive\n"+
			"  .agentmod/agentmod.toml not found in the current directory or any ancestor.\n"+
			"  Default global agent settings will be used.\n")
		return ExitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	cfg, err := config.Load(proj.ConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	opencodeLine := homeLine(cfg.OpenCode.Enabled, layout.OpencodeConfigPath(proj.AgentmodDir), "opencode.enabled = false")
	if cfg.OpenCode.Enabled && cfg.OpenCode.XDGFullIsolation {
		opencodeLine += " (+ XDG full isolation)"
	}

	fmt.Fprintln(stdout, "AgentMod: active")
	fmt.Fprintf(stdout, "  Project root:    %s\n", proj.Root)
	fmt.Fprintf(stdout, "  AgentMod root:   %s\n", proj.AgentmodDir)
	fmt.Fprintf(stdout, "  Shell routing:   %s\n", shellRoutingState(env, proj.Root))
	fmt.Fprintf(stdout, "  Claude home:     %s\n", homeLine(cfg.Claude.Enabled, filepath.Join(proj.AgentmodDir, layout.ClaudeDir), "claude.enabled = false"))
	fmt.Fprintf(stdout, "  Codex home:      %s\n", homeLine(cfg.Codex.Enabled, filepath.Join(proj.AgentmodDir, layout.CodexDir), "codex.enabled = false"))
	fmt.Fprintf(stdout, "  OpenCode config: %s\n", opencodeLine)
	fmt.Fprintf(stdout, "  Node dir:        %s\n", homeLine(cfg.Node.Enabled, filepath.Join(proj.AgentmodDir, layout.NodeDir), "node.enabled = false"))
	fmt.Fprintf(stdout, "  Recent handoff:  %s\n", recentHandoff(proj.AgentmodDir))
	return ExitOK
}

// homeLine renders a routed path, or names the config key that disabled it.
func homeLine(enabled bool, path, disabledKey string) string {
	if !enabled {
		return fmt.Sprintf("disabled (%s)", disabledKey)
	}
	return path
}

// routingEnvState reads the hook's bookkeeping vars: whether routing is
// applied in this shell and, when the hook recorded one, for which project
// root. status and doctor classify from this single source.
func routingEnvState(env Env) (active bool, root string, rootKnown bool) {
	if v, ok := env.LookupEnv(routing.EnvActive); !ok || v != "1" {
		return false, "", false
	}
	root, rootKnown = env.LookupEnv(routing.EnvProjectRoot)
	return true, root, rootKnown
}

// shellRoutingState reports whether the shell hook has applied routing in
// this shell, and for which project.
func shellRoutingState(env Env, root string) string {
	active, applied, known := routingEnvState(env)
	switch {
	case !active:
		return "not applied in this shell (hook inactive; paths below take effect once it runs)"
	case known && applied != root:
		return fmt.Sprintf("applied for a different project (%s) — stale environment?", applied)
	}
	return "applied (AGENTMOD_ACTIVE=1)"
}

// recentHandoff names the newest .amod under .agentmod/snapshots, or "none".
// A missing or unreadable snapshots directory simply means no handoffs yet.
func recentHandoff(agentmodDir string) string {
	entries, err := os.ReadDir(filepath.Join(agentmodDir, layout.SnapshotsDir))
	if err != nil {
		return "none"
	}
	var newestName string
	var newest time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".amod") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newestName == "" || info.ModTime().After(newest) {
			newest = info.ModTime()
			newestName = e.Name()
		}
	}
	if newestName == "" {
		return "none"
	}
	return fmt.Sprintf("%s (created %s)", newestName, newest.Format("2006-01-02 15:04"))
}
