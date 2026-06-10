package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
	"github.com/agentmod/agentmod/internal/routing"
)

// Severity levels of a doctor finding. ok is informational; warn and error
// both count as problems for the exit code — warn means degraded-but-
// recoverable, error means something agentmod cannot work around.
const (
	diagOK    = "ok"
	diagWarn  = "warn"
	diagError = "error"
)

type finding struct {
	level  string
	label  string
	detail string
}

// runDoctor implements the FABLE_PLAN §23 checks that have shippable
// subjects so far: project discovery, config validity, .agentmod layout,
// shell + rc-hook installation, and this shell's routing env state. It is
// strictly read-only: doctor never creates, repairs, or rewrites anything.
//
// Exit codes: 0 all checks ok · 3 any warn/error finding (ExitValidation) ·
// 1 only when doctor itself cannot run (unreadable cwd, bad arguments).
func runDoctor(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "agentmod: doctor takes no arguments (got %q)\n", strings.Join(args, " "))
		return ExitError
	}
	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	var findings []finding
	add := func(level, label, detail string) {
		findings = append(findings, finding{level, label, detail})
	}

	// Project discovery + agentmod root. Not being in a project is a normal
	// answer (doctor also serves the "did routing leak?" case), not a warn.
	proj, err := project.Discover(cwd)
	inProject := err == nil
	switch {
	case errors.Is(err, project.ErrNotFound):
		add(diagOK, "Project", "not inside an agentmod project (no .agentmod/agentmod.toml in this directory or any ancestor)")
	case err != nil:
		add(diagError, "Project", err.Error())
	default:
		add(diagOK, "Project", fmt.Sprintf("root %s (agentmod root %s)", proj.Root, proj.AgentmodDir))
	}

	var cfg config.Config
	cfgOK := false
	if inProject {
		if cfg, err = config.Load(proj.ConfigPath); err != nil {
			// config.Load names the file; routing checks below fall back to
			// the var-independent classification.
			add(diagError, "Config", err.Error())
		} else {
			cfgOK = true
			add(diagOK, "Config", proj.ConfigPath+" is valid")
		}
		findings = append(findings, layoutFinding(proj.AgentmodDir))
		findings = append(findings, agentHomeFindings(proj, cfg, cfgOK)...)
		findings = append(findings, keychainFindings(env, cfg, cfgOK)...)
		findings = append(findings, guardFinding(proj.AgentmodDir, env))
		findings = append(findings, opencodeIsolationFindings(cfg, cfgOK, env)...)
		findings = append(findings, shimFinding(proj.AgentmodDir))
		findings = append(findings, gstackProjectFinding(proj.AgentmodDir))
	}
	findings = append(findings, agentBinariesFinding(env))
	findings = append(findings, gstackGlobalFinding(env))

	// Shell type + rc-hook block. The skip reasons (exotic shell, no
	// SHELL/HOME) reuse init's wording; inside a project they are warnings
	// because routing can never activate, outside they are informational.
	shell, rcPath, skip := shellHookTarget(env)
	hookInstalled := false
	if skip != "" {
		level := diagOK
		if inProject {
			level = diagWarn
		}
		add(level, "Shell hook", skip)
	} else {
		display := abbrevHome(rcPath, env)
		add(diagOK, "Shell", fmt.Sprintf("%s (rc file %s)", shell, display))
		state, err := inspectRCBlock(rcPath, rcBlockFor(shell))
		switch {
		case err != nil:
			add(diagError, "Shell hook", err.Error())
		case state == rcBlockCurrent:
			hookInstalled = true
			add(diagOK, "Shell hook", "installed in "+display)
		case state == rcBlockStale:
			hookInstalled = true
			add(diagWarn, "Shell hook", fmt.Sprintf("block in %s is outdated — re-run 'agentmod init' to refresh it", display))
		case inProject:
			add(diagWarn, "Shell hook", fmt.Sprintf("not installed in %s — run 'agentmod init'", display))
		default:
			add(diagOK, "Shell hook", fmt.Sprintf("not installed in %s (run 'agentmod init' inside a project to set it up)", display))
		}
	}

	// Routing env vars in this shell. Inside a project: applied / stale /
	// drifted classification plus the PATH-entry audit. Outside: the §23
	// lingering-vars check (the hook must leave nothing behind).
	if inProject {
		findings = append(findings, routingFinding(env, proj, cfg, cfgOK, hookInstalled, shell))
		findings = append(findings, pathFinding(env, proj, cfg, cfgOK))
	} else {
		findings = append(findings, lingeringFinding(env, shell))
	}
	findings = append(findings, homeFinding(env))

	problems := 0
	for _, f := range findings {
		fmt.Fprintf(stdout, "%5s  %s: %s\n", f.level, f.label, f.detail)
		if f.level != diagOK {
			problems++
		}
	}
	if problems == 0 {
		fmt.Fprintln(stdout, "doctor: all checks passed")
		return ExitOK
	}
	fmt.Fprintf(stdout, "doctor: %d problem(s) found\n", problems)
	return ExitValidation
}

// layoutFinding verifies the directories init creates still exist under
// .agentmod/. Missing entries are recoverable (re-init recreates them), so
// they warn; an entry that is not a directory blocks routing and errors.
func layoutFinding(agentmodDir string) finding {
	var missing, notDir []string
	for _, sub := range layout.Subdirs() {
		info, err := os.Stat(filepath.Join(agentmodDir, sub))
		switch {
		case err == nil && info.IsDir():
		case err == nil || !os.IsNotExist(err):
			notDir = append(notDir, sub)
		default:
			missing = append(missing, sub)
		}
	}
	switch {
	case len(notDir) > 0:
		return finding{diagError, "Layout", fmt.Sprintf("not a directory under .agentmod/: %s — move it aside and re-run 'agentmod init'", strings.Join(notDir, ", "))}
	case len(missing) > 0:
		return finding{diagWarn, "Layout", fmt.Sprintf("missing under .agentmod/: %s — re-run 'agentmod init' to recreate", strings.Join(missing, ", "))}
	}
	return finding{diagOK, "Layout", fmt.Sprintf("all %d directories present under .agentmod/", len(layout.Subdirs()))}
}

// Per-agent auth files inside the project-local homes (FABLE_PLAN §12/§15):
// Codex keeps auth.json under CODEX_HOME; Claude keeps .credentials.json
// under CLAUDE_CONFIG_DIR on Linux/Windows (macOS uses the shared Keychain —
// keychainFindings states that platform note).
const (
	claudeAuthFile = ".credentials.json"
	codexAuthFile  = "auth.json"
)

// Exact re-login instructions (§12), shared by doctor's auth findings and
// init's copy-on-consent decline/non-interactive paths (auth.go).
const (
	claudeReloginRemedy = "claude may ask you to log in here; complete it once by running 'claude' inside this project"
	codexReloginRemedy  = "re-login needed: run 'codex login' inside this project"
)

// agentHomeFindings reports each agent's project-local home state (§23),
// including auth present / re-login needed per §12. Auth absence is ok-level
// (D023): a fresh project legitimately has no auth yet, so the finding
// carries the exact re-login instruction instead of warning. OpenCode has no
// project-local auth in partial-isolation mode (§15.3) — its subject is the
// routed config file; the session/merge-chain warnings are a separate item.
func agentHomeFindings(proj *project.Project, cfg config.Config, cfgOK bool) []finding {
	return []finding{
		agentAuthFinding("Claude home",
			filepath.Join(proj.AgentmodDir, layout.ClaudeDir), claudeAuthFile,
			claudeReloginRemedy,
			!cfgOK || cfg.Claude.Enabled, "claude.enabled = false"),
		agentAuthFinding("Codex home",
			filepath.Join(proj.AgentmodDir, layout.CodexDir), codexAuthFile,
			codexReloginRemedy,
			!cfgOK || cfg.Codex.Enabled, "codex.enabled = false"),
		opencodeConfigFinding(proj.AgentmodDir, !cfgOK || cfg.OpenCode.Enabled),
	}
}

// agentAuthFinding inspects one agent's project-local home for its auth
// file. Strictly read-only: copying global auth into the home is the
// consent-based bootstrap (§12), a separate command path, never doctor's.
func agentAuthFinding(label, home, authFile, remedy string, enabled bool, disabledKey string) finding {
	if !enabled {
		return finding{diagOK, label, fmt.Sprintf("routing disabled (%s)", disabledKey)}
	}
	info, err := os.Stat(filepath.Join(home, authFile))
	switch {
	case err == nil && info.Mode().IsRegular():
		return finding{diagOK, label, fmt.Sprintf("%s — auth present (%s)", home, authFile)}
	case err == nil:
		return finding{diagWarn, label, fmt.Sprintf(
			"%s exists but is not a regular file — move it aside, then log in again", filepath.Join(home, authFile))}
	}
	return finding{diagOK, label, fmt.Sprintf("%s — no auth file (%s); %s", home, authFile, remedy)}
}

// opencodeConfigFinding checks the OPENCODE_CONFIG target init stubs out.
// Without it OpenCode silently falls back to the global merge chain alone,
// but re-init recreates it — degraded-recoverable, so missing warns.
func opencodeConfigFinding(agentmodDir string, enabled bool) finding {
	path := layout.OpencodeConfigPath(agentmodDir)
	if !enabled {
		return finding{diagOK, "OpenCode config", "routing disabled (opencode.enabled = false)"}
	}
	info, err := os.Stat(path)
	switch {
	case err == nil && info.Mode().IsRegular():
		return finding{diagOK, "OpenCode config", path + " present"}
	case err == nil:
		return finding{diagError, "OpenCode config", path + " is not a regular file — move it aside and re-run 'agentmod init'"}
	}
	return finding{diagWarn, "OpenCode config", path + " missing — re-run 'agentmod init' to recreate it"}
}

// Global OpenCode locations (FABLE_PLAN §3.3, verified): sessions, storage,
// and auth.json live under ${XDG_DATA_HOME:-~/.local/share}/opencode; the
// config merge chain starts at ${XDG_CONFIG_HOME:-~/.config}/opencode/opencode.json.
func globalOpencodeDataDir(env Env) string {
	if base, ok := env.LookupEnv("XDG_DATA_HOME"); ok && base != "" {
		return filepath.Join(base, "opencode")
	}
	if home, ok := env.LookupEnv("HOME"); ok && home != "" {
		return filepath.Join(home, ".local", "share", "opencode")
	}
	return ""
}

func globalOpencodeConfigPath(env Env) string {
	if base, ok := env.LookupEnv("XDG_CONFIG_HOME"); ok && base != "" {
		return filepath.Join(base, "opencode", "opencode.json")
	}
	if home, ok := env.LookupEnv("HOME"); ok && home != "" {
		return filepath.Join(home, ".config", "opencode", "opencode.json")
	}
	return ""
}

// opencodeIsolationFindings covers the two §15.3 leak subjects: sessions
// staying global in partial-isolation mode, and the global opencode.json
// merge chain. Both are moot when opencode routing is disabled, and both are
// resolved by the XDG full-isolation opt-in (XDG_CONFIG_HOME/XDG_DATA_HOME
// are then routed into the project while inside it). A broken config (cfgOK
// false) is treated as the defaults: opencode enabled, partial mode.
func opencodeIsolationFindings(cfg config.Config, cfgOK bool, env Env) []finding {
	if cfgOK && !cfg.OpenCode.Enabled {
		return nil
	}
	if cfgOK && cfg.OpenCode.XDGFullIsolation {
		return []finding{
			{diagOK, "OpenCode sessions", "routed into the project while inside it (opencode.xdg_full_isolation = true)"},
			{diagOK, "OpenCode merge chain", "global config is not merged while routing is active (opencode.xdg_full_isolation = true)"},
		}
	}
	return []finding{opencodeSessionsFinding(env), opencodeMergeFinding(env)}
}

// opencodeSessionsFinding is the §15.3 partial-isolation warning. It warns
// only when the global data dir exists — i.e. OpenCode is actually in use
// and sessions ARE accumulating outside the project (D024); on a machine
// that has never run OpenCode the same limitation is stated at ok level so
// a fresh default-config project still exits 0.
func opencodeSessionsFinding(env Env) finding {
	dir := globalOpencodeDataDir(env)
	limitation := "partial-isolation mode keeps OpenCode sessions, storage, and auth in the global data dir"
	remedy := "set opencode.xdg_full_isolation = true in .agentmod/agentmod.toml for full isolation"
	if dir == "" {
		return finding{diagOK, "OpenCode sessions", fmt.Sprintf(
			"cannot locate the global data dir (HOME and XDG_DATA_HOME unset); %s — %s", limitation, remedy)}
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return finding{diagWarn, "OpenCode sessions", fmt.Sprintf(
			"not project-isolated: %s (%s) — %s", limitation, dir, remedy)}
	}
	return finding{diagOK, "OpenCode sessions", fmt.Sprintf(
		"%s (%s; nothing stored there yet) — %s", limitation, dir, remedy)}
}

// opencodeMergeFinding is §23's "OpenCode global config/plugins leaking into
// the project view": in partial mode the global opencode.json is still first
// in the merge chain, so it warns when that file carries any setting (any
// top-level key besides $schema). Unreadable or unparseable content warns
// too — OpenCode may still merge it (the format tolerates JSONC), and doctor
// cannot prove it leaks nothing.
func opencodeMergeFinding(env Env) finding {
	path := globalOpencodeConfigPath(env)
	if path == "" {
		return finding{diagOK, "OpenCode merge chain",
			"cannot locate the global config (HOME and XDG_CONFIG_HOME unset); nothing known to merge into the project view"}
	}
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return finding{diagOK, "OpenCode merge chain", "no global config at " + path + "; nothing leaks into the project view"}
	case err != nil:
		return finding{diagWarn, "OpenCode merge chain", fmt.Sprintf(
			"cannot inspect the global config %s (%v) — it is merged into the project view; review it manually", path, err)}
	case !info.Mode().IsRegular():
		return finding{diagWarn, "OpenCode merge chain", path + " is not a regular file — cannot tell what it merges into the project view"}
	}
	keys, err := opencodeConfigKeys(path)
	switch {
	case err != nil:
		return finding{diagWarn, "OpenCode merge chain", fmt.Sprintf(
			"global %s could not be parsed (%v) — it is merged into the project view and may leak settings; review it manually or set opencode.xdg_full_isolation = true", path, err)}
	case len(keys) > 0:
		return finding{diagWarn, "OpenCode merge chain", fmt.Sprintf(
			"global %s is merged into the project view and will leak: %s — remove or trim those settings, or set opencode.xdg_full_isolation = true", path, strings.Join(keys, ", "))}
	}
	return finding{diagOK, "OpenCode merge chain", "global " + path + " carries no settings (nothing leaks into the project view)"}
}

// opencodeConfigKeys returns the sorted top-level keys of a JSON object
// file, minus $schema (metadata that changes no behavior). A top-level
// non-object is reported as an error: whatever it is, doctor cannot vouch
// for what OpenCode does with it.
func opencodeConfigKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	var keys []string
	for k := range obj {
		if k != "$schema" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// keychainFindings states the §15.1 macOS credential limitation: Claude auth
// lives in the shared system Keychain, which every config dir reads, so
// per-project account isolation is impossible on macOS. A platform fact, not
// a problem — ok-level, and doctor states it rather than pretending the
// project-local home isolates accounts. Skipped off darwin and when claude
// routing is disabled (the limitation is moot); broken config = defaults
// (claude enabled), matching the other per-agent findings.
func keychainFindings(env Env, cfg config.Config, cfgOK bool) []finding {
	if env.GOOS != "darwin" {
		return nil
	}
	if cfgOK && !cfg.Claude.Enabled {
		return nil
	}
	return []finding{{diagOK, "Claude auth (macOS)",
		"stored in the shared macOS Keychain, not in the project-local home — accounts are shared across all config dirs; per-project account isolation is not possible on macOS (no per-project re-login needed)"}}
}

// guardFinding reports the Bash guard's wiring state in the routed Claude
// home's settings.json (FABLE_PLAN §17; IMPLEMENTATION_PLAN §11: the hook's
// binary path is re-resolved by doctor — init repairs it, doctor only
// reports). Not gated on claude.enabled, matching D027: init wires the guard
// unconditionally, so its absence is a finding regardless of routing config.
// The hook-missing cases warn (re-init rewires); a settings file doctor
// cannot parse is an error, same as the writer treats it. An unresolvable
// current binary makes the path comparison impossible — ok-level note, since
// a wired-looking hook is the best doctor can verify then.
func guardFinding(agentmodDir string, env Env) finding {
	path := filepath.Join(agentmodDir, layout.ClaudeDir, claudeSettingsFile)
	desired := ""
	var binErr error
	if env.Executable == nil {
		binErr = errors.New("no resolver available")
	} else if bin, err := env.Executable(); err != nil {
		binErr = err
	} else {
		desired = guardHookCommand(filepath.Clean(bin))
	}

	state, foundCmd, err := inspectGuardHook(path, desired)
	switch {
	case err != nil:
		return finding{diagError, "Claude guard", err.Error()}
	case state == guardHookFileAbsent:
		return finding{diagWarn, "Claude guard",
			path + " missing — the Bash guard is not wired; re-run 'agentmod init'"}
	case state == guardHookMissing:
		return finding{diagWarn, "Claude guard",
			"no guard hook in " + path + " — re-run 'agentmod init' to wire it"}
	case desired == "":
		return finding{diagOK, "Claude guard", fmt.Sprintf(
			"hook present in %s, but the current agentmod binary cannot be resolved (%v) — binary path not verified", path, binErr)}
	case state == guardHookStale:
		return finding{diagWarn, "Claude guard", fmt.Sprintf(
			"hook in %s runs %s, but the current binary expects %s — re-run 'agentmod init' to repair it", path, foundCmd, desired)}
	}
	return finding{diagOK, "Claude guard", "PreToolUse Bash hook wired in " + path + " with the current binary"}
}

// Where gstack hardcodes its global install (FABLE_PLAN §3.4), relative to
// HOME, and where agentmod installs it project-locally (§16), relative to the
// agentmod root.
var gstackRelGlobal = filepath.Join(".claude", "skills", "gstack")
var gstackRelProject = filepath.Join(layout.ClaudeDir, "skills", "gstack")

// gstackGlobalFinding is §23's "Global ~/.claude/skills/gstack exists" must-
// warn, checked in and out of projects: a global gstack install affects every
// project on the machine, so its presence warns unconditionally (it is a real
// pollution risk, not a fresh-machine default). Lstat, not Stat: a symlink or
// stray file at that path is just as much a global install.
func gstackGlobalFinding(env Env) finding {
	home, ok := env.LookupEnv("HOME")
	if !ok || home == "" {
		return finding{diagOK, "gstack (global)", "cannot locate the global Claude home (HOME unset); no global install to check"}
	}
	path := filepath.Join(home, gstackRelGlobal)
	if _, err := os.Lstat(path); err == nil {
		return finding{diagWarn, "gstack (global)", fmt.Sprintf(
			"%s exists — a global gstack affects every project on this machine; remove it and use 'agentmod install gstack' for a project-local install", path)}
	}
	return finding{diagOK, "gstack (global)", "no global install at " + path}
}

// gstackProjectFinding reports §23's "gstack project-local install state".
// Present and absent are both ok-level — installing gstack is optional —
// and the state is reported even when claude routing is disabled (whatever
// sits in the project-local skills dir is a fact either way).
func gstackProjectFinding(agentmodDir string) finding {
	path := filepath.Join(agentmodDir, gstackRelProject)
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return finding{diagOK, "gstack (project)", "installed at " + path}
	case err == nil:
		return finding{diagWarn, "gstack (project)", path + " exists but is not a directory — move it aside, then run 'agentmod install gstack'"}
	}
	return finding{diagOK, "gstack (project)", "not installed ('agentmod install gstack' installs it project-locally)"}
}

// agentBinariesFinding reports which agent CLIs are reachable (§23 "Claude /
// Codex / OpenCode binaries present"). Stat-based PATH walk — doctor never
// executes anything. Always ok-level: not every project uses all three
// agents, so an absent binary is information, not a problem.
func agentBinariesFinding(env Env) finding {
	path, _ := env.LookupEnv("PATH")
	var parts []string
	for _, name := range agentNames {
		if found := statBinaryOnPath(name, path); found != "" {
			parts = append(parts, name+" at "+found)
		} else {
			parts = append(parts, name+" not found on PATH")
		}
	}
	return finding{diagOK, "Agent binaries", strings.Join(parts, "; ")}
}

// statBinaryOnPath is a stat-only exec.LookPath: first PATH entry holding an
// executable regular file named name. exec.LookPath itself is unusable here
// because it reads the process's real PATH, not the injected Env's.
func statBinaryOnPath(name, path string) string {
	for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

// routingFinding classifies this shell's routing state against the project
// (§23: warn when inside a project with routing unset, applied for another
// root, or applied with drifted variable values).
func routingFinding(env Env, proj *project.Project, cfg config.Config, cfgOK, hookInstalled bool, shell string) finding {
	active, root, rootKnown := routingEnvState(env)
	switch {
	case !active && hookInstalled:
		// shell is always known here: hookInstalled is only set after shell
		// detection succeeded.
		return finding{diagWarn, "Routing env", fmt.Sprintf(
			"shell hook installed but not active in this shell — open a new terminal, run 'exec $SHELL', or run: eval \"$(agentmod hook %s)\"", shell)}
	case !active:
		return finding{diagWarn, "Routing env",
			"not applied in this shell and no shell hook is installed — run 'agentmod init', then open a new terminal"}
	case rootKnown && root != proj.Root:
		return finding{diagWarn, "Routing env", fmt.Sprintf(
			"applied for a different project (%s) — stale; if this persists at the next prompt, the shell hook is not running", root)}
	}
	if cfgOK {
		if bad := misroutedVars(env, proj.AgentmodDir, cfg); len(bad) > 0 {
			return finding{diagWarn, "Routing env", fmt.Sprintf(
				"applied for this project, but routed variable(s) do not match the expected paths: %s — cd out of the project and back in", strings.Join(bad, ", "))}
		}
	}
	return finding{diagOK, "Routing env", "applied for this project (AGENTMOD_ACTIVE=1)"}
}

// misroutedVars lists routed variables whose current value differs from what
// activation would set (unset counts as a mismatch). PATH is excluded here:
// duplicate/missing PATH entries are pathFinding's job.
func misroutedVars(env Env, agentmodDir string, cfg config.Config) []string {
	var bad []string
	for _, v := range routing.Vars(agentmodDir, cfg) {
		if got, ok := env.LookupEnv(v.Name); !ok || got != v.Value {
			bad = append(bad, v.Name)
		}
	}
	return bad
}

// agentNames are the binaries an attacker-or-tool-created shim would shadow.
var agentNames = []string{"claude", "codex", "opencode"}

// shimFinding audits .agentmod/node/bin (the one PATH dir agentmod manages)
// for agent-named entries. A symlink resolving inside .agentmod is a
// legitimate project-local install (npm's bin layout); anything else shadows
// the real binary while routing is active, and agentmod itself never creates
// such entries (FABLE_PLAN §2), so it warns.
func shimFinding(agentmodDir string) finding {
	binDir := routing.NodeBinDir(agentmodDir)
	var local, shims []string
	for _, name := range agentNames {
		path := filepath.Join(binDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if target, err := filepath.EvalSymlinks(path); err == nil && insideDir(target, agentmodDir) {
				local = append(local, name)
				continue
			}
		}
		shims = append(shims, name)
	}
	switch {
	case len(shims) > 0:
		return finding{diagWarn, "Shims", fmt.Sprintf(
			"agent-named executable(s) in .agentmod/node/bin shadow the real binaries: %s — agentmod never creates shims; remove them (project-local installs via npm are symlinks into .agentmod)", strings.Join(shims, ", "))}
	case len(local) > 0:
		return finding{diagOK, "Shims", fmt.Sprintf(
			"none — %s in .agentmod/node/bin are project-local installs (symlinks into .agentmod)", strings.Join(local, ", "))}
	}
	return finding{diagOK, "Shims", "none in .agentmod/node/bin (agentmod never creates shims)"}
}

// insideDir reports whether path is dir or lies underneath it, resolving
// symlinks in dir first (macOS temp dirs: /var vs /private/var).
func insideDir(path, dir string) bool {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// pathFinding audits PATH against the project (§23 "Duplicate agentmod PATH
// entries"): the managed node/bin entry must appear at most once, exactly
// once while routing is active with node enabled, and no entry from another
// project's .agentmod may linger. A single entry while routing is NOT active
// is left to routingFinding's warning — same root cause, one remedy.
func pathFinding(env Env, proj *project.Project, cfg config.Config, cfgOK bool) finding {
	path, _ := env.LookupEnv("PATH")
	binDir := routing.NodeBinDir(proj.AgentmodDir)
	count := 0
	var foreign []string
	for _, entry := range strings.Split(path, string(os.PathListSeparator)) {
		switch {
		case entry == binDir:
			count++
		case entry != "" && hasAgentmodElement(entry):
			foreign = append(foreign, entry)
		}
	}
	active, root, rootKnown := routingEnvState(env)
	activeHere := active && rootKnown && root == proj.Root

	var issues []string
	if count > 1 {
		issues = append(issues, fmt.Sprintf("%s appears %d times on PATH (must be at most once)", binDir, count))
	}
	if len(foreign) > 0 {
		issues = append(issues, "entries from another .agentmod linger: "+strings.Join(foreign, ", "))
	}
	if count == 0 && activeHere && cfgOK && cfg.Node.Enabled {
		issues = append(issues, binDir+" is missing from PATH while routing is active")
	}
	if len(issues) > 0 {
		return finding{diagWarn, "PATH", strings.Join(issues, "; ") + " — open a new terminal or cd out of the project and back in"}
	}
	if count == 1 {
		return finding{diagOK, "PATH", binDir + " on PATH once"}
	}
	return finding{diagOK, "PATH", "no agentmod entries (node routing not applied)"}
}

// lingeringFinding is the outside-a-project half of the routing audit (§23
// "agentmod env vars lingering in a folder without .agentmod"): deactivation
// must leave no bookkeeping vars, no saved values, no routed value pointing
// into an .agentmod, and no .agentmod PATH entry.
func lingeringFinding(env Env, shell string) finding {
	var lingering []string
	for _, name := range []string{routing.EnvActive, routing.EnvProjectRoot, routing.EnvRoot, routing.EnvVarsList} {
		if _, ok := env.LookupEnv(name); ok {
			lingering = append(lingering, name)
		}
	}
	for _, name := range routing.RoutedNames() {
		if v, ok := env.LookupEnv(name); ok && hasAgentmodElement(v) {
			lingering = append(lingering, name)
		}
		if _, ok := env.LookupEnv(routing.SavedPrefix + name); ok {
			lingering = append(lingering, routing.SavedPrefix+name)
		}
	}
	if path, ok := env.LookupEnv("PATH"); ok {
		for _, entry := range strings.Split(path, string(os.PathListSeparator)) {
			if entry != "" && hasAgentmodElement(entry) {
				lingering = append(lingering, "PATH entry "+entry)
			}
		}
	}
	if len(lingering) == 0 {
		return finding{diagOK, "Routing env", "no agentmod variables lingering in this shell"}
	}
	remedy := " — open a new terminal"
	if shell != "" {
		remedy += fmt.Sprintf(`, or run: eval "$(agentmod env --shell %s --deactivate)"`, shell)
	}
	return finding{diagWarn, "Routing env",
		"agentmod environment lingering outside any project: " + strings.Join(lingering, ", ") + remedy}
}

// homeFinding checks §23 "HOME changed": agentmod never saves, sets, or
// routes HOME, so a saved copy or a HOME inside an .agentmod directory means
// some other tool (or a tampered hook) changed it.
func homeFinding(env Env) finding {
	if v, ok := env.LookupEnv(routing.SavedPrefix + "HOME"); ok {
		return finding{diagWarn, "HOME", fmt.Sprintf(
			"%sHOME is set (%s) — agentmod never saves or changes HOME; unset it and check what modified your environment", routing.SavedPrefix, v)}
	}
	home, ok := env.LookupEnv("HOME")
	if !ok {
		return finding{diagOK, "HOME", "not set in this shell (not agentmod's doing — agentmod never modifies HOME)"}
	}
	if hasAgentmodElement(home) {
		return finding{diagWarn, "HOME", fmt.Sprintf(
			"points inside an .agentmod directory (%s) — agentmod never changes HOME; restore your real home directory", home)}
	}
	return finding{diagOK, "HOME", "no signs of tampering (agentmod never modifies HOME)"}
}

// hasAgentmodElement reports whether one of p's path elements is exactly
// ".agentmod" — the marker that a value points into some project's agentmod
// root, whosever it is.
func hasAgentmodElement(p string) bool {
	for _, el := range strings.Split(filepath.ToSlash(p), "/") {
		if el == ".agentmod" {
			return true
		}
	}
	return false
}
