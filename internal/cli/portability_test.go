package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassifyAbsoluteToken pins the warn/silent decision per token shape
// (FABLE_PLAN §22). Existing paths are created under temp dirs so the table
// is host-independent.
func TestClassifyAbsoluteToken(t *testing.T) {
	root := t.TempDir()
	agentmodDir := filepath.Join(root, ".agentmod")
	insideExists := filepath.Join(agentmodDir, "node", "bin")
	if err := os.MkdirAll(insideExists, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideExists := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(outsideExists, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "definitely-missing")

	cases := []struct {
		name, tok, wantSub string
		warn               bool
	}{
		{"relative", "relative/path", "", false},
		{"flag", "--flag", "", false},
		{"bare slash", "/", "", false},
		{"absolute existing", outsideExists, "", false},
		{"absolute missing", missing, "does not exist on this machine", true},
		{"inside this agentmod existing", insideExists, "", false},
		{"agentmod dir itself", agentmodDir, "", false},
		{"inside this agentmod missing", filepath.Join(agentmodDir, "node", "ghost"), "points inside this project's .agentmod but does not exist", true},
		{"foreign agentmod", "/Users/alice/proj/.agentmod/claude/x", "points at another machine's .agentmod", true},
		{"windows drive", `C:\tools\server.exe`, "Windows-style absolute path", true},
		{"windows drive forward", `c:/tools/server.exe`, "Windows-style absolute path", true},
		{"windows UNC", `\\srv\share\x`, "Windows-style absolute path", true},
		{"not a drive letter", `7:\x\y`, "", false},
	}
	for _, c := range cases {
		detail, warn := classifyAbsoluteToken(c.tok, agentmodDir)
		if warn != c.warn {
			t.Errorf("%s: warn = %v, want %v (detail %q)", c.name, warn, c.warn, detail)
			continue
		}
		if warn && !strings.Contains(detail, c.wantSub) {
			t.Errorf("%s: detail %q missing %q", c.name, detail, c.wantSub)
		}
	}
}

func TestLocalAgentmodEquivalent(t *testing.T) {
	got := localAgentmodEquivalent("/Users/alice/proj/.agentmod/claude/skills/gstack", "/here/.agentmod")
	want := filepath.Join("/here/.agentmod", "claude", "skills", "gstack")
	if got != want {
		t.Errorf("localAgentmodEquivalent = %q, want %q", got, want)
	}
}

// TestScanRestoredConfigs runs the scanner over a fixture .agentmod holding
// all four known config files: foreign paths in JSON values, JSON arrays,
// and TOML tables are warned; the agentmod-owned guard hook command and
// paths that resolve locally are not; an unparseable file degrades to a
// file-level warning instead of being skipped.
func TestScanRestoredConfigs(t *testing.T) {
	root := t.TempDir()
	agentmodDir := filepath.Join(root, ".agentmod")
	for _, d := range []string{"claude", "codex", "opencode"} {
		if err := os.MkdirAll(filepath.Join(agentmodDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	localTool := filepath.Join(agentmodDir, "claude", "local-tool")
	if err := os.WriteFile(localTool, []byte("ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// settings.json: our guard command (foreign binary — exempt) + a user
	// hook with a missing absolute path (warned). The same missing path
	// twice proves dedup.
	settings := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "'/fake/source/agentmod' guard claude-bash"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/missing/hook.sh"}, {"type": "command", "command": "/missing/hook.sh"}]}
    ]
  }
}`
	writeFixture(t, filepath.Join(agentmodDir, "claude", "settings.json"), settings)
	writeFixture(t, filepath.Join(agentmodDir, "claude", ".claude.json"),
		`{"mcpServers": {"docs": {"command": "node", "args": ["/missing/server.js", "--cwd", "`+localTool+`"]}}}`)
	writeFixture(t, filepath.Join(agentmodDir, "codex", "config.toml"),
		"[mcp_servers.docs]\ncommand = \"/missing/codex-mcp\"\nargs = [\"--root\", \"/Users/alice/p/.agentmod/node\"]\n")
	writeFixture(t, filepath.Join(agentmodDir, "opencode", "opencode.json"), "{not json")

	warns := scanRestoredConfigs(agentmodDir)
	type key struct{ file, path string }
	got := map[key]string{}
	for _, w := range warns {
		got[key{w.File, w.Path}] = w.Detail
	}
	wantSubs := map[key]string{
		{".agentmod/claude/settings.json", "/missing/hook.sh"}:           "does not exist on this machine",
		{".agentmod/claude/.claude.json", "/missing/server.js"}:          "does not exist on this machine",
		{".agentmod/codex/config.toml", "/missing/codex-mcp"}:            "does not exist on this machine",
		{".agentmod/codex/config.toml", "/Users/alice/p/.agentmod/node"}: "points at another machine's .agentmod",
		{".agentmod/opencode/opencode.json", ""}:                         "could not be parsed",
	}
	for k, sub := range wantSubs {
		detail, ok := got[k]
		if !ok {
			t.Errorf("missing warning for %v\ngot: %v", k, warns)
			continue
		}
		if !strings.Contains(detail, sub) {
			t.Errorf("%v detail %q missing %q", k, detail, sub)
		}
	}
	if len(warns) != len(wantSubs) {
		t.Errorf("got %d warnings, want %d (dedup + exemptions):\n%v", len(warns), len(wantSubs), warns)
	}
	// Deterministic order: sorted by file, then path.
	for i := 1; i < len(warns); i++ {
		if warns[i-1].File > warns[i].File {
			t.Errorf("warnings not sorted by file: %q before %q", warns[i-1].File, warns[i].File)
		}
	}
}

// TestScanRestoredConfigsAllAbsent: a payload with none of the known config
// files scans clean — absence is not a finding.
func TestScanRestoredConfigsAllAbsent(t *testing.T) {
	agentmodDir := filepath.Join(t.TempDir(), ".agentmod")
	if err := os.MkdirAll(agentmodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if warns := scanRestoredConfigs(agentmodDir); len(warns) != 0 {
		t.Errorf("expected no warnings, got %v", warns)
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
