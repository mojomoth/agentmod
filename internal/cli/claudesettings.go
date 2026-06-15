package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mojomoth/agentmod/internal/layout"
)

// claudeSettingsFile is the settings file inside the ROUTED Claude home
// (.agentmod/claude/). FABLE_PLAN §17 placement decision: the guard's
// PreToolUse hook lives here so it is active exactly when routing is active.
// It must never be written to the project's own .claude/settings.json — that
// file is shared with collaborators via git.
const claudeSettingsFile = "settings.json"

// guardHookMarker identifies a hook command as ours, whatever binary path it
// carries. Matching on the subcommand (not the full command string) lets a
// re-init repair a stale binary path in place instead of appending a second
// hook entry.
const guardHookMarker = "guard claude-bash"

// guardHookCommand is the shell command Claude Code runs on every Bash
// PreToolUse event. The binary path is absolute (IMPLEMENTATION_PLAN §11) and
// single-quoted so paths with spaces survive the shell.
func guardHookCommand(binPath string) string {
	return shellQuote(binPath) + " " + guardHookMarker
}

// ensureClaudeGuardHook merges the Bash guard's PreToolUse hook entry into
// .agentmod/claude/settings.json (T17). Semantics, mirroring the rc-block
// editor's discipline (D019):
//   - file absent → created with exactly the hook config;
//   - hook present with the current binary path → no write at all, existing
//     bytes (including the user's formatting) untouched;
//   - hook present with a stale binary path → command rewritten in place;
//   - hook absent from an existing file → entry appended; every user key is
//     preserved, but the file is re-marshaled (stdlib JSON, keys sorted);
//   - unparseable JSON / wrong-type hooks keys → hard error, zero writes.
//
// Returns the status line for init's report.
func ensureClaudeGuardHook(agentmodDir string, env Env) (string, error) {
	if env.Executable == nil {
		return "skipped (cannot resolve the agentmod binary path)", nil
	}
	binPath, err := env.Executable()
	if err != nil {
		return fmt.Sprintf("skipped (cannot resolve the agentmod binary path: %v)", err), nil
	}
	// os.Executable can report the invocation-relative spelling (e.g.
	// ".../proj/../agentmod"); Clean canonicalizes it so re-init compares
	// equal regardless of how the binary was launched. Symlinks are kept
	// as-is: a version-managed symlink (homebrew) is the stabler reference.
	desired := guardHookCommand(filepath.Clean(binPath))
	path := filepath.Join(agentmodDir, layout.ClaudeDir, claudeSettingsFile)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, mErr := marshalSettings(freshGuardSettings(desired))
		if mErr != nil {
			return "", mErr
		}
		if wErr := os.WriteFile(path, data, 0o644); wErr != nil {
			return "", wErr
		}
		return "PreToolUse Bash hook written to .agentmod/claude/settings.json", nil
	}
	if err != nil {
		return "", err
	}

	settings, err := parseSettings(path, raw)
	if err != nil {
		return "", err
	}
	hooks, pre, err := guardHookSlice(path, settings)
	if err != nil {
		return "", err
	}

	found, changed := false, false
	for _, hookMap := range guardHookEntries(pre) {
		found = true
		if hookMap["command"].(string) != desired {
			hookMap["command"] = desired
			changed = true
		}
	}
	if !found {
		hooks["PreToolUse"] = append(pre, guardHookEntry(desired))
		settings["hooks"] = hooks
		changed = true
	}

	if !changed {
		return "guard hook already configured in .agentmod/claude/settings.json", nil
	}
	data, err := marshalSettings(settings)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	if found {
		return "guard hook binary path updated in .agentmod/claude/settings.json", nil
	}
	return "guard hook added to existing .agentmod/claude/settings.json", nil
}

// guardHookEntries returns, in document order, every hook map under the
// PreToolUse entries whose command string carries the ownership marker.
// Shared by the writer above (which mutates the returned maps in place) and
// the read-only inspector below; a returned map always has a string
// "command" containing guardHookMarker.
func guardHookEntries(pre []any) []map[string]any {
	var ours []map[string]any
	for _, entry := range pre {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hookMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hookMap["command"].(string); ok && strings.Contains(cmd, guardHookMarker) {
				ours = append(ours, hookMap)
			}
		}
	}
	return ours
}

// Wiring states inspectGuardHook distinguishes (doctor's subject; D029).
type guardHookState int

const (
	guardHookFileAbsent guardHookState = iota // settings.json does not exist
	guardHookMissing                          // file exists, no marker hook in it
	guardHookStale                            // marker hook present, command != desired
	guardHookCurrent                          // marker hook present with the desired command
)

// inspectGuardHook reports the guard hook's wiring state in the settings
// file at path, strictly read-only. desired is the command expected for the
// current binary; foundCmd carries the first marker hook's command so a
// stale finding can name what the file actually points at. Unreadable or
// structurally invalid files return the same hard errors the writer raises.
func inspectGuardHook(path, desired string) (state guardHookState, foundCmd string, err error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return guardHookFileAbsent, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	settings, err := parseSettings(path, raw)
	if err != nil {
		return 0, "", err
	}
	_, pre, err := guardHookSlice(path, settings)
	if err != nil {
		return 0, "", err
	}
	entries := guardHookEntries(pre)
	if len(entries) == 0 {
		return guardHookMissing, "", nil
	}
	for _, e := range entries {
		if e["command"].(string) == desired {
			return guardHookCurrent, desired, nil
		}
	}
	return guardHookStale, entries[0]["command"].(string), nil
}

// parseSettings decodes an existing settings.json. A whitespace-only file is
// treated as an empty document rather than a JSON error; anything else that
// fails to parse — or parses to a non-object — is the user's file in a state
// we must not guess about, so it is a hard error and nothing is written.
func parseSettings(path string, raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("%s is not a valid JSON object (%v); fix or remove it and re-run init", path, err)
	}
	return settings, nil
}

// guardHookSlice digs hooks.PreToolUse out of the settings document, creating
// missing levels and rejecting wrong-typed existing ones.
func guardHookSlice(path string, settings map[string]any) (map[string]any, []any, error) {
	hooks := map[string]any{}
	if existing, ok := settings["hooks"]; ok {
		hooksMap, ok := existing.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s has a non-object \"hooks\" key; fix or remove it and re-run init", path)
		}
		hooks = hooksMap
	} else {
		settings["hooks"] = hooks
	}
	var pre []any
	if existing, ok := hooks["PreToolUse"]; ok {
		preSlice, ok := existing.([]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s has a non-array \"hooks.PreToolUse\" key; fix or remove it and re-run init", path)
		}
		pre = preSlice
	}
	return hooks, pre, nil
}

func guardHookEntry(command string) map[string]any {
	return map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	}
}

func freshGuardSettings(command string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{guardHookEntry(command)},
		},
	}
}

func marshalSettings(settings map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
