package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fence markers for the rc-file block (FABLE_PLAN §12). Only the bytes
// between (and including) these lines are ever written by agentmod; the
// user's shell config around them is preserved exactly.
const (
	rcBeginMarker = "# >>> agentmod >>>"
	rcEndMarker   = "# <<< agentmod <<<"
)

// rcBlockFor returns the full fenced block for a shell, trailing newline
// included. The `command -v` guard keeps shells quiet when agentmod is
// uninstalled or off PATH (the rc file outlives the binary).
func rcBlockFor(shell string) string {
	return rcBeginMarker + "\n" +
		"# Managed by 'agentmod init'. Manual edits inside this block are overwritten.\n" +
		`command -v agentmod >/dev/null 2>&1 && eval "$(agentmod hook ` + shell + `)"` + "\n" +
		rcEndMarker + "\n"
}

// ensureShellHook installs (or refreshes) the agentmod block in the user's
// rc file and returns the human-readable status for init's "Shell hook:"
// line (D019). Conditions that simply mean "nothing to do here" — the flag,
// an exotic $SHELL, missing $HOME — are reported as skips, not errors; only
// a damaged fence or an unwritable rc file is an error.
func ensureShellHook(opts initOptions, env Env) (string, error) {
	if opts.NoShellHook {
		return "skipped (--no-shell-hook)", nil
	}
	shell, rcPath, skip := shellHookTarget(env)
	if skip != "" {
		return skip, nil
	}
	action, err := ensureRCBlock(rcPath, rcBlockFor(shell))
	if err != nil {
		return "", err
	}
	display := abbrevHome(rcPath, env)
	switch action {
	case rcInstalled:
		return fmt.Sprintf("installed in %s (takes effect in new shells)", display), nil
	case rcUpdated:
		return fmt.Sprintf("updated in %s (takes effect in new shells)", display), nil
	default:
		return "already installed in " + display, nil
	}
}

// shellHookTarget picks the rc file for the user's login shell. zsh honors
// ZDOTDIR; bash prefers an existing ~/.bashrc, then an existing
// ~/.bash_profile, and creates ~/.bashrc when neither exists (D019). A
// non-empty skip reason means init should report and move on.
func shellHookTarget(env Env) (shell, rcPath, skip string) {
	shellVar, _ := env.LookupEnv("SHELL")
	if shellVar == "" {
		return "", "", "skipped ($SHELL is not set; add the block from 'agentmod hook <shell>' to your rc file manually)"
	}
	shell = filepath.Base(shellVar)
	switch shell {
	case "zsh", "bash":
	default:
		return "", "", fmt.Sprintf("skipped (unsupported shell %q; supported: zsh, bash — wire 'agentmod hook' manually)", shell)
	}
	home, _ := env.LookupEnv("HOME")
	if home == "" {
		return "", "", "skipped ($HOME is not set; cannot locate your rc file)"
	}
	if shell == "zsh" {
		dir := home
		if zdot, ok := env.LookupEnv("ZDOTDIR"); ok && zdot != "" {
			dir = zdot
		}
		return shell, filepath.Join(dir, ".zshrc"), ""
	}
	bashrc := filepath.Join(home, ".bashrc")
	profile := filepath.Join(home, ".bash_profile")
	switch {
	case fileExists(bashrc):
		return shell, bashrc, ""
	case fileExists(profile):
		return shell, profile, ""
	default:
		return shell, bashrc, ""
	}
}

// Actions ensureRCBlock can report.
const (
	rcInstalled = "installed"
	rcUpdated   = "updated"
	rcUnchanged = "unchanged"
)

// ensureRCBlock makes path contain exactly one copy of block, fenced by the
// agentmod markers. Absent → append (creating the file if needed); present
// but stale → replace in place; present and current → no write at all.
// Bytes outside the fence are never altered. A fence we cannot safely
// rewrite (start without end, or several starts) is an error — guessing
// risks eating user config.
func ensureRCBlock(path, block string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	content := string(data)
	lines := strings.SplitAfter(content, "\n")
	begin, end, beginCount := -1, -1, 0
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case rcBeginMarker:
			beginCount++
			if begin == -1 {
				begin = i
			}
		case rcEndMarker:
			if begin != -1 && end == -1 {
				end = i
			}
		}
	}
	switch {
	case beginCount > 1:
		return "", fmt.Errorf("%s: found %d agentmod blocks; remove the extra %q fences and re-run init", path, beginCount, rcBeginMarker)
	case begin != -1 && end == -1:
		return "", fmt.Errorf("%s: agentmod block has a start marker but no %q line; repair or delete the block and re-run init", path, rcEndMarker)
	case begin == -1:
		var b strings.Builder
		b.WriteString(content)
		if content != "" && !strings.HasSuffix(content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString(block)
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return "", err
		}
		return rcInstalled, nil
	}
	current := strings.Join(lines[begin:end+1], "")
	if current == block {
		return rcUnchanged, nil
	}
	updated := strings.Join(lines[:begin], "") + block + strings.Join(lines[end+1:], "")
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return rcUpdated, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// abbrevHome shortens a path under $HOME to ~/… for display.
func abbrevHome(path string, env Env) string {
	home, _ := env.LookupEnv("HOME")
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return "~" + string(filepath.Separator) + rel
}
