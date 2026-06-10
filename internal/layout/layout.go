// Package layout defines the on-disk layout of an .agentmod/ directory
// (IMPLEMENTATION_PLAN §4). Commands that create or resolve paths inside a
// project (init, status, routing, handoff) share these names so the layout
// is defined exactly once.
package layout

import "path/filepath"

// Entry names under .agentmod/.
const (
	ClaudeDir          = "claude"   // CLAUDE_CONFIG_DIR
	CodexDir           = "codex"    // CODEX_HOME
	OpencodeDir        = "opencode" // holds the OPENCODE_CONFIG target
	OpencodeConfigFile = "opencode.json"
	OpencodeXDGDir     = "xdg" // under opencode/; XDG roots, opt-in mode only
	NodeDir            = "node"
	SnapshotsDir       = "snapshots"
	LogsDir            = "logs"
)

// Subdirs lists the directories init creates under .agentmod/, relative to
// it. opencode/xdg is absent: it belongs to the opt-in XDG full-isolation
// mode and is created only when that mode is enabled.
func Subdirs() []string {
	return []string{
		ClaudeDir,
		CodexDir,
		OpencodeDir,
		NodeDir,
		SnapshotsDir,
		LogsDir,
	}
}

// OpencodeConfigPath returns the OPENCODE_CONFIG target for an .agentmod dir.
func OpencodeConfigPath(agentmodDir string) string {
	return filepath.Join(agentmodDir, OpencodeDir, OpencodeConfigFile)
}
