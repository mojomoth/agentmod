package cli

// T11 — scripted-shell env-hygiene matrix (FABLE_PLAN §7): full sessions in
// real zsh and bash proving activation is a perfect inverse. The hook tests
// (T09/T10) cover single transitions per shell; this file drives repeated
// in/out and cross-project transitions through one session and diffs the
// entire exported environment before vs after.

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/routing"
)

// shellCase abstracts the zsh/bash scripting differences. bash has no chpwd
// and PROMPT_COMMAND never fires non-interactively (D018), so its cd snippet
// invokes the hook explicitly; zsh's chpwd fires on every cd.
type shellCase struct {
	name string
	run  func(t *testing.T, dir string, env []string, interactive bool, script string) (string, string)
	cd   func(dir string) string
}

func shellCases() []shellCase {
	return []shellCase{
		{
			name: "zsh",
			run:  runZsh,
			cd:   func(dir string) string { return "cd '" + dir + "'" },
		},
		{
			name: "bash",
			run:  runBash,
			cd:   func(dir string) string { return "cd '" + dir + "'\n_agentmod_hook" },
		},
	}
}

// sentinelEnv is a pre-existing user value for every variable the default
// config routes (D016 save/restore), plus XDG_CONFIG_HOME which partial
// isolation must never touch. Values carry spaces, a single quote and a $
// to prove the emitted quoting round-trips through a real shell. No value
// may contain a newline: the env dumps are compared line by line.
func sentinelEnv() map[string]string {
	return map[string]string{
		"CLAUDE_CONFIG_DIR": "/sentinel/claude",
		"CODEX_HOME":        "/sentinel/codex home with spaces",
		"OPENCODE_CONFIG":   "/sentinel/opencode.json",
		"NPM_CONFIG_CACHE":  "/sentinel/npm-cache",
		"NPM_CONFIG_PREFIX": "/sentinel/npm'prefix",
		"PNPM_HOME":         "/sentinel/pnpm$dollar",
		"BUN_INSTALL":       "/sentinel/bun",
		"XDG_CONFIG_HOME":   "/sentinel/xdg-config",
	}
}

// envSection parses the `env | sort` dump between ===<tag>=== and
// ===END<tag>=== markers into a name→value map, dropping the variables a
// session legitimately changes (cwd bookkeeping and the shells' `_`).
func envSection(t *testing.T, stdout, tag string) map[string]string {
	t.Helper()
	begin, end := "==="+tag+"===", "===END"+tag+"==="
	vars := make(map[string]string)
	in, found := false, false
	for _, line := range strings.Split(stdout, "\n") {
		switch line {
		case begin:
			in, found = true, true
			continue
		case end:
			in = false
			continue
		}
		if !in || line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("malformed env line in %s section: %q", tag, line)
		}
		switch name {
		case "PWD", "OLDPWD", "SHLVL", "_":
			continue
		}
		vars[name] = value
	}
	if !found {
		t.Fatalf("marker %s not found in:\n%s", begin, stdout)
	}
	return vars
}

func countPathEntries(path, entry string) int {
	n := 0
	for _, p := range strings.Split(path, sep) {
		if p == entry {
			n++
		}
	}
	return n
}

// diffTrees reports any file or directory created, removed or modified
// between two snapshotTree captures of dir.
func diffTrees(t *testing.T, dir string, before, after map[string]string) {
	t.Helper()
	for rel, b := range before {
		if a, ok := after[rel]; !ok {
			t.Errorf("%s: %s removed by the shell session", dir, rel)
		} else if a != b {
			t.Errorf("%s: %s changed by the shell session", dir, rel)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("%s: %s created by the shell session (shim?)", dir, rel)
		}
	}
}

func TestHookScriptedSessionEnvHygiene(t *testing.T) {
	for _, sh := range shellCases() {
		t.Run(sh.name, func(t *testing.T) {
			projA := makeProject(t, config.Default())
			projB := makeProject(t, config.Default())
			binDir := fakeAgentmodBin(t)
			startDir := t.TempDir()

			sentinels := sentinelEnv()
			names := make([]string, 0, len(sentinels))
			for name := range sentinels {
				names = append(names, name)
			}
			sort.Strings(names)
			env := childEnv(t, binDir)
			for _, name := range names {
				env = append(env, name+"="+sentinels[name])
			}

			// Any file the session creates in these trees would be a shim
			// (FABLE_PLAN §28); activation must have zero FS side effects.
			watched := []string{projA, projB, binDir, startDir}
			before := make([]map[string]string, len(watched))
			for i, dir := range watched {
				before[i] = snapshotTree(t, dir)
			}

			script := strings.Join([]string{
				`eval "$(agentmod hook ` + sh.name + `)"`,
				`printf '%s\n' "===ENV0==="`,
				`env | sort`,
				`printf '%s\n' "===ENDENV0==="`,
				sh.cd(projA),
				`printf '%s\n' "HOMEA:$HOME"`,
				`printf '%s\n' "CLAUDEA:${CLAUDE_CONFIG_DIR-unset}"`,
				`printf '%s\n' "SAVEDA:${AGENTMOD_SAVED_CLAUDE_CONFIG_DIR-unset}"`,
				`printf '%s\n' "PATHA1:$PATH"`,
				sh.cd("/"),
				`printf '%s\n' "MIDCLAUDE:${CLAUDE_CONFIG_DIR-unset}"`,
				`printf '%s\n' "MIDPATH:$PATH"`,
				sh.cd(projA),
				`printf '%s\n' "PATHA2:$PATH"`,
				sh.cd(projB),
				`printf '%s\n' "PATHB:$PATH"`,
				`printf '%s\n' "CLAUDEB:${CLAUDE_CONFIG_DIR-unset}"`,
				`printf '%s\n' "SAVEDB:${AGENTMOD_SAVED_CLAUDE_CONFIG_DIR-unset}"`,
				sh.cd(projA),
				`printf '%s\n' "PATHA3:$PATH"`,
				sh.cd("/"),
				`printf '%s\n' "===ENV1==="`,
				`env | sort`,
				`printf '%s\n' "===ENDENV1==="`,
			}, "\n") + "\n"

			stdout, stderr := sh.run(t, startDir, env, false, script)
			if stderr != "" {
				t.Errorf("unexpected stderr:\n%s", stderr)
			}

			env0 := envSection(t, stdout, "ENV0")
			env1 := envSection(t, stdout, "ENV1")

			// Perfect inverse: the exported environment after the session
			// matches before it, variable for variable (FABLE_PLAN §7).
			for name, v0 := range env0 {
				if v1, ok := env1[name]; !ok {
					t.Errorf("%s lost across the session (was %q)", name, v0)
				} else if v1 != v0 {
					t.Errorf("%s changed across the session: %q -> %q", name, v0, v1)
				}
			}
			for name, v1 := range env1 {
				if _, ok := env0[name]; !ok {
					t.Errorf("%s leaked by the session (= %q)", name, v1)
				}
				if strings.HasPrefix(name, "AGENTMOD") {
					t.Errorf("bookkeeping variable %s still exported after exit", name)
				}
			}
			for _, name := range names {
				if got := env1[name]; got != sentinels[name] {
					t.Errorf("%s = %q after session, want sentinel %q", name, got, sentinels[name])
				}
			}

			// HOME never changes, even while inside a project.
			if got := lineAfter(t, stdout, "HOMEA:"); got != env0["HOME"] {
				t.Errorf("HOME inside project = %q, want %q", got, env0["HOME"])
			}

			// Routing wins over the sentinel while inside; the saved value is
			// always the user's original — even right after the A→B switch,
			// which must never capture our own routing (D016).
			amA := filepath.Join(projA, ".agentmod")
			amB := filepath.Join(projB, ".agentmod")
			for prefix, want := range map[string]string{
				"CLAUDEA:":   filepath.Join(amA, "claude"),
				"SAVEDA:":    sentinels["CLAUDE_CONFIG_DIR"],
				"MIDCLAUDE:": sentinels["CLAUDE_CONFIG_DIR"],
				"CLAUDEB:":   filepath.Join(amB, "claude"),
				"SAVEDB:":    sentinels["CLAUDE_CONFIG_DIR"],
			} {
				if got := lineAfter(t, stdout, prefix); got != want {
					t.Errorf("%s = %q, want %q", prefix, got, want)
				}
			}

			// PATH hygiene: exactly one managed entry while inside — the
			// current project's — across repeated and cross-project
			// transitions; the original PATH in between and after.
			nodeA, nodeB := routing.NodeBinDir(amA), routing.NodeBinDir(amB)
			path0 := env0["PATH"]
			for prefix, want := range map[string][2]int{ // {nodeA, nodeB} counts
				"PATHA1:": {1, 0},
				"PATHA2:": {1, 0},
				"PATHB:":  {0, 1},
				"PATHA3:": {1, 0},
			} {
				got := lineAfter(t, stdout, prefix)
				if n := countPathEntries(got, nodeA); n != want[0] {
					t.Errorf("%s has %d %q entries, want %d\nPATH=%s", prefix, n, nodeA, want[0], got)
				}
				if n := countPathEntries(got, nodeB); n != want[1] {
					t.Errorf("%s has %d %q entries, want %d\nPATH=%s", prefix, n, nodeB, want[1], got)
				}
			}
			if got := lineAfter(t, stdout, "MIDPATH:"); got != path0 {
				t.Errorf("PATH between projects = %q, want original %q", got, path0)
			}
			if got := env1["PATH"]; got != path0 {
				t.Errorf("PATH after session = %q, want original %q", got, path0)
			}

			// No shims: nothing in the watched trees may change (§28).
			for i, dir := range watched {
				diffTrees(t, dir, before[i], snapshotTree(t, dir))
			}
		})
	}
}
