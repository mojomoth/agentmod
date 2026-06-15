package routing

import (
	"reflect"
	"testing"

	"github.com/mojomoth/agentmod/internal/config"
)

func names(vars []Var) []string {
	out := make([]string, len(vars))
	for i, v := range vars {
		out[i] = v.Name
	}
	return out
}

func TestVarsDefaults(t *testing.T) {
	got := Vars("/p/.agentmod", config.Default())
	want := []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "OPENCODE_CONFIG",
		"NPM_CONFIG_CACHE", "NPM_CONFIG_PREFIX", "PNPM_HOME", "BUN_INSTALL",
	}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
	byName := map[string]string{}
	for _, v := range got {
		byName[v.Name] = v.Value
	}
	if byName["CLAUDE_CONFIG_DIR"] != "/p/.agentmod/claude" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q", byName["CLAUDE_CONFIG_DIR"])
	}
	if byName["OPENCODE_CONFIG"] != "/p/.agentmod/opencode/opencode.json" {
		t.Errorf("OPENCODE_CONFIG = %q", byName["OPENCODE_CONFIG"])
	}
	// npm prefix is node/ itself so its global bin lands in node/bin, the
	// single PATH entry the hook manages.
	if byName["NPM_CONFIG_PREFIX"] != "/p/.agentmod/node" {
		t.Errorf("NPM_CONFIG_PREFIX = %q", byName["NPM_CONFIG_PREFIX"])
	}
}

func TestVarsXDGOptIn(t *testing.T) {
	cfg := config.Default()
	cfg.OpenCode.XDGFullIsolation = true
	got := names(Vars("/p/.agentmod", cfg))
	want := []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "OPENCODE_CONFIG",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME",
		"NPM_CONFIG_CACHE", "NPM_CONFIG_PREFIX", "PNPM_HOME", "BUN_INSTALL",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

func TestVarsDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Enabled = false
	cfg.Codex.Enabled = false
	cfg.OpenCode.Enabled = false
	cfg.OpenCode.XDGFullIsolation = true // ignored: opencode disabled
	cfg.Node.Enabled = false
	if got := Vars("/p/.agentmod", cfg); len(got) != 0 {
		t.Errorf("all disabled should route nothing, got %v", names(got))
	}
}

func TestNodeBinDir(t *testing.T) {
	if got := NodeBinDir("/p/.agentmod"); got != "/p/.agentmod/node/bin" {
		t.Errorf("NodeBinDir = %q", got)
	}
}

func TestRoutedNamesIsTheFullSuperset(t *testing.T) {
	got := RoutedNames()
	want := []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "OPENCODE_CONFIG",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME",
		"NPM_CONFIG_CACHE", "NPM_CONFIG_PREFIX", "PNPM_HOME", "BUN_INSTALL",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RoutedNames = %v, want %v", got, want)
	}
}
