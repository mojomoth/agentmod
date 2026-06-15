package cli

// Post-restore portability pass (FABLE_PLAN §18 "OS path portability
// handling" + "MCP absolute-path warning or rewriting", §22;
// IMPLEMENTATION_PLAN §14). Separator normalization and exec-bit restoration
// are structural — PlanRestore refuses non-forward-slash names (D041) and
// extraction chmods recorded modes umask-proof (D043) — so the runtime half
// is the config problem: restored agent configs may carry absolute paths
// that meant something on the SOURCE machine. agentmod rewrites the one
// file it owns (the guard hook command in claude/settings.json, via
// ensureClaudeGuardHook) and WARNS about everything else: re-marshaling a
// user-owned JSON/TOML document to rewrite one string risks corrupting a
// file we do not own (D044). Warnings never change restore's exit code.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/mojomoth/agentmod/internal/layout"
	"github.com/mojomoth/agentmod/internal/project"
)

// portabilityWarning is one machine-specific absolute path (or unscannable
// file) found in a restored agent config.
type portabilityWarning struct {
	File   string // project-root-relative, forward-slash, e.g. ".agentmod/codex/config.toml"
	Path   string // the absolute path string found; "" for a file-level problem
	Detail string // why it needs attention + what to do
}

// restoredConfigRelPaths lists the agent-config files inside .agentmod/ that
// can carry MCP server definitions or hook commands with machine-specific
// absolute paths. Exact MCP schemas are deliberately NOT depended on
// (FABLE_PLAN §31): the scan walks every string value in each file, so any
// key layout is covered; files absent from a snapshot are simply skipped.
func restoredConfigRelPaths() []string {
	return []string{
		path.Join(layout.ClaudeDir, claudeSettingsFile),          // Claude hooks (incl. our guard)
		path.Join(layout.ClaudeDir, ".claude.json"),              // Claude Code state + MCP servers under CLAUDE_CONFIG_DIR
		path.Join(layout.CodexDir, "config.toml"),                // Codex CLI config incl. [mcp_servers.*]
		path.Join(layout.OpencodeDir, layout.OpencodeConfigFile), // OpenCode config incl. "mcp"
	}
}

// reportPortability runs the portability pass after a successful restore:
// re-wire the Claude guard hook for THIS machine's binary, then scan the
// restored configs and print what still points elsewhere. A guard-wiring
// failure (e.g. the snapshot shipped an unparseable settings.json) is a
// stderr warning, not a failure — the restore itself succeeded and doctor
// reports the same condition (D029).
func reportPortability(stdout, stderr io.Writer, agentmodDir string, env Env) {
	guardLine, err := ensureClaudeGuardHook(agentmodDir, env)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: warning: could not re-wire the Claude guard hook for this machine: %v\n", err)
	} else {
		fmt.Fprintf(stdout, "  Claude guard: %s\n", guardLine)
	}

	warns := scanRestoredConfigs(agentmodDir)
	if len(warns) == 0 {
		fmt.Fprintf(stdout, "  portability: no foreign absolute paths in restored agent configs\n")
		return
	}
	if len(warns) == 1 {
		fmt.Fprintf(stdout, "  portability: 1 absolute path in restored agent configs needs attention:\n")
	} else {
		fmt.Fprintf(stdout, "  portability: %d absolute paths in restored agent configs need attention:\n", len(warns))
	}
	for _, w := range warns {
		if w.Path == "" {
			fmt.Fprintf(stdout, "    %s: %s\n", w.File, w.Detail)
		} else {
			fmt.Fprintf(stdout, "    %s: %s — %s\n", w.File, w.Path, w.Detail)
		}
	}
}

// scanRestoredConfigs walks every string value in the known restored config
// files and returns the absolute paths that look machine-specific, sorted
// and deduplicated. Read-only.
func scanRestoredConfigs(agentmodDir string) []portabilityWarning {
	var warns []portabilityWarning
	seen := map[portabilityWarning]bool{}
	add := func(w portabilityWarning) {
		if !seen[w] {
			seen[w] = true
			warns = append(warns, w)
		}
	}

	for _, rel := range restoredConfigRelPaths() {
		display := path.Join(project.DirName, rel)
		raw, err := os.ReadFile(filepath.Join(agentmodDir, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			add(portabilityWarning{File: display, Detail: fmt.Sprintf("could not be read (%v); review absolute paths in it manually", err)})
			continue
		}
		var doc any
		if strings.HasSuffix(rel, ".toml") {
			var table map[string]any
			err = toml.Unmarshal(raw, &table)
			doc = table
		} else {
			err = json.Unmarshal(raw, &doc)
		}
		if err != nil {
			add(portabilityWarning{File: display, Detail: fmt.Sprintf("could not be parsed (%v); review absolute paths in it manually", err)})
			continue
		}
		collectStrings(doc, func(s string) {
			if strings.Contains(s, guardHookMarker) {
				// agentmod's own guard hook command: reportPortability just
				// re-resolved it to this machine's binary, and doctor owns
				// its staleness (D029).
				return
			}
			for _, tok := range strings.Fields(s) {
				tok = strings.Trim(tok, `"'`)
				if detail, warn := classifyAbsoluteToken(tok, agentmodDir); warn {
					add(portabilityWarning{File: display, Path: tok, Detail: detail})
				}
			}
		})
	}

	sort.Slice(warns, func(i, j int) bool {
		if warns[i].File != warns[j].File {
			return warns[i].File < warns[j].File
		}
		if warns[i].Path != warns[j].Path {
			return warns[i].Path < warns[j].Path
		}
		return warns[i].Detail < warns[j].Detail
	})
	return warns
}

// collectStrings calls fn for every string value reachable in a decoded
// JSON/TOML document. Map keys are skipped — paths live in values.
// BurntSushi decodes arrays of tables as []map[string]any, hence the extra
// case stdlib JSON never produces.
func collectStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case string:
		fn(x)
	case map[string]any:
		for _, vv := range x {
			collectStrings(vv, fn)
		}
	case []any:
		for _, vv := range x {
			collectStrings(vv, fn)
		}
	case []map[string]any:
		for _, vv := range x {
			collectStrings(vv, fn)
		}
	}
}

// classifyAbsoluteToken decides whether one whitespace-separated token from
// a config value is a machine-specific absolute path worth warning about
// (FABLE_PLAN §22 "Rewrite or warn on absolute paths at restore"). Absolute
// paths that resolve on THIS machine are silent — they work. Relative paths
// are always silent: they travel correctly by construction.
func classifyAbsoluteToken(tok, agentmodDir string) (detail string, warn bool) {
	if isWindowsAbsPath(tok) {
		return "Windows-style absolute path; rewrite it for this machine", true
	}
	if len(tok) < 2 || tok[0] != '/' {
		return "", false
	}
	cleaned := filepath.Clean(tok)
	if cleaned == agentmodDir || strings.HasPrefix(cleaned, agentmodDir+string(filepath.Separator)) {
		if _, err := os.Stat(cleaned); err != nil {
			return "points inside this project's .agentmod but does not exist (excluded from the snapshot or never created); recreate it or update the config", true
		}
		return "", false
	}
	if hasAgentmodElement(cleaned) {
		return fmt.Sprintf("points at another machine's .agentmod (this project's copy would be %s)", localAgentmodEquivalent(cleaned, agentmodDir)), true
	}
	if _, err := os.Stat(cleaned); err != nil {
		return "does not exist on this machine; install it or update the config", true
	}
	return "", false
}

// isWindowsAbsPath reports UNC (\\server\share) and drive-letter (C:\ or
// C:/) spellings — never resolvable on this platform, always worth a warn.
func isWindowsAbsPath(tok string) bool {
	if strings.HasPrefix(tok, `\\`) {
		return true
	}
	if len(tok) >= 3 && tok[1] == ':' && (tok[2] == '\\' || tok[2] == '/') {
		c := tok[0]
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	}
	return false
}

// localAgentmodEquivalent maps a foreign path's tail after its .agentmod
// element onto this project's agentmod dir, so the warning can name where
// the restored copy of the same entry lives.
func localAgentmodEquivalent(p, agentmodDir string) string {
	els := strings.Split(filepath.ToSlash(p), "/")
	for i, el := range els {
		if el == project.DirName {
			return filepath.Join(append([]string{agentmodDir}, els[i+1:]...)...)
		}
	}
	return agentmodDir
}
