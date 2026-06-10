package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentmod/agentmod/internal/layout"
)

// Global agent home directory names under $HOME (FABLE_PLAN §3/§12). The
// copy-on-consent flow reads auth files from these — read-only; agentmod
// never writes into a global home.
const (
	globalClaudeDirName = ".claude"
	globalCodexDirName  = ".codex"
)

// authSpec describes one agent's §12 copy-on-consent flow. The consent-copied
// target (localHome/authFile) is permanently on the snapshot/handoff
// exclusion list — Phase 5's exclusion engine must cover
// claude/.credentials.json and codex/auth.json (§18, T20).
type authSpec struct {
	label     string // init summary label, e.g. "Claude auth"
	agentName string // human name used in prompts, e.g. "Claude"
	globalDir string // directory under $HOME holding the global auth file
	localHome string // project-local home the file would be copied into
	authFile  string // auth file name, identical globally and locally
	remedy    string // exact re-login instruction (§12) when not copying
	keychain  bool   // darwin Claude: auth is the shared Keychain, not a file
}

// bootstrapAuth implements init's auth copy-on-consent (FABLE_PLAN §12,
// IMPLEMENTATION_PLAN §9): for each file-auth agent whose project-local home
// lacks its auth file while the global one exists, explicitly ask before
// copying. Decline, EOF, or non-interactive mode prints the exact re-login
// instruction instead. No other global file is ever copied. Prompts read
// env.Stdin (D026: never os.Stdin). Prints both the prompts and the aligned
// summary lines.
func bootstrapAuth(agentmodDir string, opts initOptions, stdout io.Writer, env Env) error {
	p := newAuthPrompter(stdout, env)
	specs := []authSpec{
		{
			label:     "Claude auth",
			agentName: "Claude",
			globalDir: globalClaudeDirName,
			localHome: filepath.Join(agentmodDir, layout.ClaudeDir),
			authFile:  claudeAuthFile,
			remedy:    claudeReloginRemedy,
			keychain:  env.GOOS == "darwin",
		},
		{
			label:     "Codex auth",
			agentName: "Codex",
			globalDir: globalCodexDirName,
			localHome: filepath.Join(agentmodDir, layout.CodexDir),
			authFile:  codexAuthFile,
			remedy:    codexReloginRemedy,
		},
	}
	for _, spec := range specs {
		line, err := bootstrapOneAuth(spec, opts, p, env)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "  %-17s%s\n", spec.label+":", line)
	}
	return nil
}

// bootstrapOneAuth runs the §12 decision ladder for one agent and returns
// the summary line. Only the consented copy writes anything, and only under
// the project-local home.
func bootstrapOneAuth(spec authSpec, opts initOptions, p *authPrompter, env Env) (string, error) {
	if spec.keychain {
		return "stored in the shared macOS Keychain — nothing to copy; accounts are shared across all projects (per-project isolation is not possible on macOS, no re-login needed)", nil
	}
	localPath := filepath.Join(spec.localHome, spec.authFile)
	if _, err := os.Lstat(localPath); err == nil {
		return fmt.Sprintf("already present (%s), left untouched", spec.authFile), nil
	}
	home, ok := env.LookupEnv("HOME")
	if !ok || home == "" {
		return fmt.Sprintf("cannot locate the global %s home (HOME unset); %s", spec.agentName, spec.remedy), nil
	}
	globalPath := filepath.Join(home, spec.globalDir, spec.authFile)
	info, err := os.Stat(globalPath)
	if err != nil {
		return fmt.Sprintf("no global auth to copy (%s not found); %s", globalPath, spec.remedy), nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("global %s is not a regular file — not copying; %s", globalPath, spec.remedy), nil
	}
	if opts.NonInteractive {
		return fmt.Sprintf("not copied (non-interactive mode never copies auth); %s", spec.remedy), nil
	}
	question := fmt.Sprintf("Copy global %s auth (%s) into this project's %s home?", spec.agentName, globalPath, spec.agentName)
	if !p.consent(question) {
		return fmt.Sprintf("not copied (declined); %s", spec.remedy), nil
	}
	if err := copyAuthFile(globalPath, localPath); err != nil {
		return "", err
	}
	return fmt.Sprintf("copied from %s (mode 0600)", globalPath), nil
}

// authPrompter asks [y/N] questions on stdout and reads answers from one
// shared buffered reader, so consecutive prompts in a single init run never
// lose buffered input between them.
type authPrompter struct {
	stdout io.Writer
	reader *bufio.Reader // nil when env.Stdin is nil: every prompt declines
}

func newAuthPrompter(stdout io.Writer, env Env) *authPrompter {
	p := &authPrompter{stdout: stdout}
	if env.Stdin != nil {
		p.reader = bufio.NewReader(env.Stdin)
	}
	return p
}

// consent returns true only for an explicit yes answer. EOF, read errors,
// and anything other than y/yes (case-insensitive) decline — consent must
// never be the accident path.
func (p *authPrompter) consent(question string) bool {
	fmt.Fprintf(p.stdout, "%s [y/N] ", question)
	if p.reader == nil {
		fmt.Fprintln(p.stdout)
		return false
	}
	line, err := p.reader.ReadString('\n')
	if line == "" && err != nil {
		fmt.Fprintln(p.stdout)
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// copyAuthFile copies a global auth file into the project-local home with
// 0600 permissions. O_EXCL: the caller already verified the target is
// absent, and a racing creation must not be truncated.
func copyAuthFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
