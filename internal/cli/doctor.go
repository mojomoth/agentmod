package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		findings = append(findings, shimFinding(proj.AgentmodDir))
	}

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
