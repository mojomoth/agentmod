package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mojomoth/agentmod/internal/config"
	"github.com/mojomoth/agentmod/internal/project"
	"github.com/mojomoth/agentmod/internal/routing"
)

// envOptions carries the parsed `agentmod env` flags.
type envOptions struct {
	Shell        string // "zsh" or "bash"
	ActivateRoot string // project root to activate; "" when deactivating
	Deactivate   bool
}

// parseEnvFlags accepts: --shell <zsh|bash> and exactly one of
// --activate <ROOT> | --deactivate.
func parseEnvFlags(args []string) (envOptions, error) {
	var opts envOptions
	activate := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--shell requires a value (zsh or bash)")
			}
			i++
			opts.Shell = args[i]
		case "--activate":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--activate requires a project root path")
			}
			i++
			opts.ActivateRoot = args[i]
			activate = true
		case "--deactivate":
			opts.Deactivate = true
		default:
			return opts, fmt.Errorf("unknown argument %q for env (valid: --shell <zsh|bash>, --activate <root>, --deactivate)", args[i])
		}
	}
	switch opts.Shell {
	case "zsh", "bash":
	case "":
		return opts, fmt.Errorf("env requires --shell <zsh|bash>")
	default:
		return opts, fmt.Errorf("unsupported shell %q (supported: zsh, bash)", opts.Shell)
	}
	if activate == opts.Deactivate {
		return opts, fmt.Errorf("env requires exactly one of --activate <root> or --deactivate")
	}
	if activate && opts.ActivateRoot == "" {
		return opts, fmt.Errorf("--activate requires a non-empty project root path")
	}
	return opts, nil
}

// runEnv implements `agentmod env` (IMPLEMENTATION_PLAN §7): it prints shell
// code that the hook evals on activation/deactivation transitions. All
// decisions (what to save, restore, dedup) are made HERE, against the real
// environment of the calling shell; the emitted code is only plain
// `export NAME='value'` / `unset NAME` lines, identical for zsh and bash.
// Nothing but eval-able code ever goes to stdout.
func runEnv(args []string, stdout, stderr io.Writer, env Env) int {
	opts, err := parseEnvFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	ops := &opList{model: &envModel{lookup: env.LookupEnv, overrides: map[string]*string{}}}
	if opts.Deactivate {
		appendDeactivate(ops)
		io.WriteString(stdout, renderOps(ops.ops))
		return ExitOK
	}

	root := opts.ActivateRoot
	if !filepath.IsAbs(root) {
		cwd, err := env.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "agentmod: %v\n", err)
			return ExitError
		}
		root = filepath.Join(cwd, root)
	}
	root = filepath.Clean(root)
	agentmodDir := filepath.Join(root, project.DirName)
	configPath := filepath.Join(agentmodDir, project.ConfigFileName)
	if info, err := os.Stat(configPath); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(stderr, "agentmod: %s is not an agentmod project root (no regular file at %s)\n", root, configPath)
		return ExitNotInProject
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	// If routing is already applied (same project with edited config, or a
	// different project after a direct cd between trees), undo it first so
	// activation always starts from a clean slate and saved values never
	// capture our own routing.
	appendDeactivate(ops)
	appendActivate(ops, root, agentmodDir, cfg)
	io.WriteString(stdout, renderOps(ops.ops))
	return ExitOK
}

// envModel tracks the shell's environment as the emitted ops mutate it, so
// later ops (e.g. activation after an implicit deactivation) see the state
// the shell will actually be in when they run.
type envModel struct {
	lookup    func(key string) (string, bool)
	overrides map[string]*string // nil entry = unset
}

func (m *envModel) get(name string) (string, bool) {
	if v, ok := m.overrides[name]; ok {
		if v == nil {
			return "", false
		}
		return *v, true
	}
	return m.lookup(name)
}

// opList accumulates shell ops, applying each to the model as it is added.
type opList struct {
	model *envModel
	ops   []shellOp
}

type shellOp struct {
	name  string
	value string
	unset bool
}

func (l *opList) export(name, value string) {
	v := value
	l.model.overrides[name] = &v
	l.ops = append(l.ops, shellOp{name: name, value: value})
}

// unsetVar emits an unset only when the variable is currently set; unsetting
// nothing would only add noise to the eval'd script.
func (l *opList) unsetVar(name string) {
	if _, ok := l.model.get(name); !ok {
		return
	}
	l.model.overrides[name] = nil
	l.ops = append(l.ops, shellOp{name: name, unset: true})
}

// envVarName guards names interpolated into shell code. AGENTMOD_VARS comes
// from the inherited environment, so a name that is not a plain identifier
// is skipped rather than eval'd.
var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// appendDeactivate restores the pre-activation environment: each variable
// recorded in AGENTMOD_VARS gets its saved value back (or is unset when no
// value was saved — D006), the node bin entry is stripped from PATH, and the
// bookkeeping variables are removed. A perfect inverse of appendActivate;
// no-op when routing is not applied.
func appendDeactivate(l *opList) {
	if v, ok := l.model.get(routing.EnvActive); !ok || v != "1" {
		return
	}
	varsList, _ := l.model.get(routing.EnvVarsList)
	for _, name := range strings.Fields(varsList) {
		if !envVarName.MatchString(name) {
			continue
		}
		saved := routing.SavedPrefix + name
		if old, ok := l.model.get(saved); ok {
			l.export(name, old)
			l.unsetVar(saved)
		} else {
			l.unsetVar(name)
		}
	}
	if agentmodDir, ok := l.model.get(routing.EnvRoot); ok {
		if path, ok := l.model.get("PATH"); ok {
			stripped := stripPathEntry(path, routing.NodeBinDir(agentmodDir))
			if stripped != path {
				l.export("PATH", stripped)
			}
		}
	}
	l.unsetVar(routing.EnvVarsList)
	l.unsetVar(routing.EnvRoot)
	l.unsetVar(routing.EnvProjectRoot)
	l.unsetVar(routing.EnvActive)
}

// appendActivate routes the enabled agents into agentmodDir, saving any
// pre-existing value of each routed variable (D006), prepends the node bin
// dir to PATH exactly once, and records the bookkeeping variables.
func appendActivate(l *opList, root, agentmodDir string, cfg config.Config) {
	vars := routing.Vars(agentmodDir, cfg)
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		if old, ok := l.model.get(v.Name); ok {
			l.export(routing.SavedPrefix+v.Name, old)
		}
		l.export(v.Name, v.Value)
		names = append(names, v.Name)
	}
	if cfg.Node.Enabled {
		entry := routing.NodeBinDir(agentmodDir)
		path, _ := l.model.get("PATH")
		l.export("PATH", prependPathEntry(path, entry))
	}
	l.export(routing.EnvActive, "1")
	l.export(routing.EnvProjectRoot, root)
	l.export(routing.EnvRoot, agentmodDir)
	l.export(routing.EnvVarsList, strings.Join(names, " "))
}

// stripPathEntry removes every component equal to entry from a PATH value.
func stripPathEntry(path, entry string) string {
	parts := strings.Split(path, string(os.PathListSeparator))
	kept := parts[:0]
	for _, p := range parts {
		if p != entry {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// prependPathEntry puts entry first, removing any existing occurrence so
// repeated transitions never accumulate duplicates.
func prependPathEntry(path, entry string) string {
	rest := stripPathEntry(path, entry)
	if rest == "" {
		return entry
	}
	return entry + string(os.PathListSeparator) + rest
}

// renderOps turns ops into eval-able shell lines. Values are single-quoted
// with embedded quotes escaped, so arbitrary saved values round-trip; names
// are restricted to identifiers before reaching here.
func renderOps(ops []shellOp) string {
	var b strings.Builder
	for _, op := range ops {
		if op.unset {
			fmt.Fprintf(&b, "unset %s\n", op.name)
		} else {
			fmt.Fprintf(&b, "export %s=%s\n", op.name, shellQuote(op.value))
		}
	}
	return b.String()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
