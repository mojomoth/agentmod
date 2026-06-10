// Package routing defines which environment variables agentmod routes while
// a project is active (IMPLEMENTATION_PLAN §8) and the names of agentmod's
// own bookkeeping variables (FABLE_PLAN §14). `agentmod env` emits these on
// activation; doctor later audits shells against the same single source.
//
// layout defines what init CREATES on disk; this package defines what the
// shell hook ROUTES to those locations. Paths that exist only as routing
// targets (node/bin, npm-cache, the XDG roots) are named here.
package routing

import (
	"path/filepath"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
)

// Bookkeeping variables owned by agentmod (uppercase per FABLE_PLAN §14).
const (
	// EnvActive is "1" while a project's routing is applied in this shell.
	EnvActive = "AGENTMOD_ACTIVE"
	// EnvProjectRoot is the project root the routing was applied for.
	EnvProjectRoot = "AGENTMOD_PROJECT_ROOT"
	// EnvRoot is the .agentmod directory of that project.
	EnvRoot = "AGENTMOD_ROOT"
	// EnvVarsList records, space-separated, which routed variables were set
	// at activation. Deactivation restores exactly this list, so it stays
	// correct even if agentmod.toml changes (or disappears) while inside
	// the project.
	EnvVarsList = "AGENTMOD_VARS"
	// SavedPrefix prefixes the saved pre-activation value of a routed
	// variable (D006). Absence of AGENTMOD_SAVED_<VAR> means <VAR> was
	// unset before activation, so deactivation unsets it.
	SavedPrefix = "AGENTMOD_SAVED_"
)

// Var is one routed environment variable.
type Var struct {
	Name  string
	Value string
}

// Vars returns the variables to route for a project with the given .agentmod
// directory and config, in stable emission order. Disabled agents contribute
// nothing. PATH is handled separately (prepend/strip, not set/restore).
func Vars(agentmodDir string, cfg config.Config) []Var {
	var vars []Var
	if cfg.Claude.Enabled {
		vars = append(vars, Var{"CLAUDE_CONFIG_DIR", filepath.Join(agentmodDir, layout.ClaudeDir)})
	}
	if cfg.Codex.Enabled {
		vars = append(vars, Var{"CODEX_HOME", filepath.Join(agentmodDir, layout.CodexDir)})
	}
	if cfg.OpenCode.Enabled {
		vars = append(vars, Var{"OPENCODE_CONFIG", layout.OpencodeConfigPath(agentmodDir)})
		if cfg.OpenCode.XDGFullIsolation {
			xdg := filepath.Join(agentmodDir, layout.OpencodeDir, layout.OpencodeXDGDir)
			vars = append(vars,
				Var{"XDG_CONFIG_HOME", filepath.Join(xdg, "config")},
				Var{"XDG_DATA_HOME", filepath.Join(xdg, "data")},
				Var{"XDG_CACHE_HOME", filepath.Join(xdg, "cache")},
				Var{"XDG_STATE_HOME", filepath.Join(xdg, "state")},
			)
		}
	}
	if cfg.Node.Enabled {
		node := filepath.Join(agentmodDir, layout.NodeDir)
		vars = append(vars,
			Var{"NPM_CONFIG_CACHE", filepath.Join(node, layout.NodeNPMCacheDir)},
			// Prefix is node/ itself so npm's global bin is node/bin —
			// the one PATH entry the hook manages.
			Var{"NPM_CONFIG_PREFIX", node},
			Var{"PNPM_HOME", filepath.Join(node, layout.NodePnpmDir)},
			Var{"BUN_INSTALL", filepath.Join(node, layout.NodeBunDir)},
		)
	}
	return vars
}

// NodeBinDir is the single PATH entry agentmod manages: prepended once on
// activation (when node routing is enabled), stripped on deactivation.
func NodeBinDir(agentmodDir string) string {
	return filepath.Join(agentmodDir, layout.NodeDir, "bin")
}

// RoutedNames returns the name of every variable agentmod can ever route,
// independent of any particular project's config (single source: Vars with
// every agent and XDG opt-in enabled). Doctor uses it to detect routed
// values lingering in shells that are outside any project.
func RoutedNames() []string {
	cfg := config.Default()
	cfg.Claude.Enabled = true
	cfg.Codex.Enabled = true
	cfg.OpenCode.Enabled = true
	cfg.OpenCode.XDGFullIsolation = true
	cfg.Node.Enabled = true
	vars := Vars("", cfg)
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name
	}
	return names
}
