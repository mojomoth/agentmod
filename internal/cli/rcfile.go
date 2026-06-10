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

// shellHookResult is what ensureShellHook reports back to init: the text
// for the "Shell hook:" line, the rc action taken (rcInstalled / rcUpdated /
// rcUnchanged / rcSkipped), and the shell name when one was recognized.
// hookActivationNotice keys off Action and Shell.
type shellHookResult struct {
	Line   string
	Action string
	Shell  string
}

// ensureShellHook installs (or refreshes) the agentmod block in the user's
// rc file (D019). Conditions that simply mean "nothing to do here" — the
// flag, an exotic $SHELL, missing $HOME — are reported as skips, not errors;
// only a damaged fence or an unwritable rc file is an error.
func ensureShellHook(opts initOptions, env Env) (shellHookResult, error) {
	if opts.NoShellHook {
		return shellHookResult{Line: "skipped (--no-shell-hook)", Action: rcSkipped}, nil
	}
	shell, rcPath, skip := shellHookTarget(env)
	if skip != "" {
		return shellHookResult{Line: skip, Action: rcSkipped}, nil
	}
	action, err := ensureRCBlock(rcPath, rcBlockFor(shell))
	if err != nil {
		return shellHookResult{}, err
	}
	res := shellHookResult{Action: action, Shell: shell}
	display := abbrevHome(rcPath, env)
	switch action {
	case rcInstalled:
		res.Line = fmt.Sprintf("installed in %s (takes effect in new shells)", display)
	case rcUpdated:
		res.Line = fmt.Sprintf("updated in %s (takes effect in new shells)", display)
	default:
		res.Line = "already installed in " + display
	}
	return res, nil
}

// hookActivationNotice diagnoses whether the shell hook is routing the
// shell that invoked init (FABLE_PLAN §12: init cannot change its parent
// shell's environment and must say so precisely). A live hook exports
// AGENTMOD_ACTIVE / AGENTMOD_PROJECT_ROOT, which reach init through the
// injected Env; everything else is inference from the rc-file outcome.
// An empty result means the "Shell hook:" line already says it all.
func hookActivationNotice(res shellHookResult, projectRoot string, env Env) string {
	if v, ok := env.LookupEnv("AGENTMOD_ACTIVE"); ok && v == "1" {
		root, _ := env.LookupEnv("AGENTMOD_PROJECT_ROOT")
		if root == projectRoot {
			return "The shell hook is live in this shell and already routing this project.\n"
		}
		return fmt.Sprintf("The shell hook is live in this shell (routing %s).\nIt will switch to this project at your next prompt.\n", root)
	}
	if res.Action == rcSkipped {
		// No hook was installed and none is live; the skip reason on the
		// "Shell hook:" line already says why and what to do instead.
		return ""
	}
	var b strings.Builder
	b.WriteString("Note: the hook is NOT active in this shell session — a process cannot\n")
	b.WriteString("modify its parent shell's environment. To start routing, either\n")
	b.WriteString("open a new terminal, run 'exec $SHELL',\n")
	b.WriteString(fmt.Sprintf("or run: eval \"$(agentmod hook %s)\"   (this shell only, effective immediately)\n", res.Shell))
	if res.Action != rcInstalled {
		// The block predates this init run, so the hook may already be
		// loaded here — it just hasn't seen this project yet (a freshly
		// created marker is only noticed at the next prompt).
		b.WriteString("(If the hook is already loaded in this shell, it will pick this project\nup at your next prompt instead.)\n")
	}
	return b.String()
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

// Actions a shellHookResult can carry: the three ensureRCBlock outcomes,
// plus rcSkipped when no rc file was touched at all.
const (
	rcInstalled = "installed"
	rcUpdated   = "updated"
	rcUnchanged = "unchanged"
	rcSkipped   = "skipped"
)

// locateRCBlock scans rc-file lines (in strings.SplitAfter form) for the
// agentmod fence. begin/end are line indices (-1 when absent); beginCount
// counts start markers so callers can detect a corrupt fence.
func locateRCBlock(lines []string) (begin, end, beginCount int) {
	begin, end = -1, -1
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
	return begin, end, beginCount
}

// rcFenceError reports the fence shapes that cannot be safely rewritten
// (start without end, or several starts) — guessing risks eating user
// config, so both ensureRCBlock and doctor treat them as hard errors.
func rcFenceError(path string, begin, end, beginCount int) error {
	switch {
	case beginCount > 1:
		return fmt.Errorf("%s: found %d agentmod blocks; remove the extra %q fences and re-run init", path, beginCount, rcBeginMarker)
	case begin != -1 && end == -1:
		return fmt.Errorf("%s: agentmod block has a start marker but no %q line; repair or delete the block and re-run init", path, rcEndMarker)
	}
	return nil
}

// rcBlockState is inspectRCBlock's read-only classification of the fence.
type rcBlockState int

const (
	rcBlockAbsent  rcBlockState = iota // no fence (or no rc file at all)
	rcBlockCurrent                     // fence present, content up to date
	rcBlockStale                       // fence present, content differs
)

// inspectRCBlock reports what ensureRCBlock would find at path — without
// writing anything. doctor uses it to diagnose hook installation state.
func inspectRCBlock(path, block string) (rcBlockState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rcBlockAbsent, nil
		}
		return rcBlockAbsent, err
	}
	lines := strings.SplitAfter(string(data), "\n")
	begin, end, beginCount := locateRCBlock(lines)
	if err := rcFenceError(path, begin, end, beginCount); err != nil {
		return rcBlockAbsent, err
	}
	if begin == -1 {
		return rcBlockAbsent, nil
	}
	if strings.Join(lines[begin:end+1], "") == block {
		return rcBlockCurrent, nil
	}
	return rcBlockStale, nil
}

// ensureRCBlock makes path contain exactly one copy of block, fenced by the
// agentmod markers. Absent → append (creating the file if needed); present
// but stale → replace in place; present and current → no write at all.
// Bytes outside the fence are never altered.
func ensureRCBlock(path, block string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	content := string(data)
	lines := strings.SplitAfter(content, "\n")
	begin, end, beginCount := locateRCBlock(lines)
	if err := rcFenceError(path, begin, end, beginCount); err != nil {
		return "", err
	}
	if begin == -1 {
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
