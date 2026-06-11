package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// matchDefaultRules reports the ID of the first DefaultRules entry matching
// the given path, or "" — the same first-match-wins the writer applies.
func matchDefaultRules(relPath string, isDir bool) string {
	base := relPath[strings.LastIndex(relPath, "/")+1:]
	for _, r := range DefaultRules() {
		if r.Matches(relPath, base, isDir) {
			return r.ID
		}
	}
	return ""
}

func TestDefaultRulesTable(t *testing.T) {
	cases := []struct {
		relPath string
		isDir   bool
		want    string // rule ID, "" = kept
	}{
		// Auth by NAME at any depth (D028: consent-copied paths are
		// claude/.credentials.json + codex/auth.json, but Claude writes
		// .credentials.json itself too — provenance must not matter).
		{".agentmod/claude/.credentials.json", false, "auth-file"},
		{".agentmod/codex/auth.json", false, "auth-file"},
		{".agentmod/opencode/xdg/data/.credentials.json", false, "auth-file"},
		{".agentmod/foo/credentials.json", false, "auth-file"},
		{".agentmod/foo/credentials", false, "auth-file"},
		{".agentmod/codex/auth.json.bak", false, ""}, // exact names only
		// .env family.
		{".agentmod/claude/.env", false, "env-file"},
		{".agentmod/claude/.env.local", false, "env-file"},
		{".agentmod/claude/prod.env", false, "env-file"},
		{".agentmod/claude/environment.md", false, ""},
		{".agentmod/claude/envrc.sample", false, ""},
		// SSH / TLS key material.
		{".agentmod/foo/id_ed25519", false, "ssh-key"},
		{".agentmod/foo/id_rsa.pub", false, "ssh-key"},
		{".agentmod/foo/server.pem", false, "ssh-key"},
		{".agentmod/foo/identity.md", false, ""},
		// Credential directories.
		{".agentmod/foo/.ssh", true, "credential-dir"},
		{".agentmod/foo/.aws", true, "credential-dir"},
		{".agentmod/foo/.docker", true, "credential-dir"},
		// OS credential store.
		{".agentmod/foo/login.keychain-db", false, "os-credential-store"},
		{".agentmod/foo/login.keychain", false, "os-credential-store"},
		// VCS: dir in normal clones, regular file in worktrees.
		{".agentmod/claude/skills/gstack/.git", true, "vcs-git"},
		{".agentmod/claude/skills/wt/.git", false, "vcs-git"},
		{".agentmod/claude/skills/gstack/.gitignore", false, ""},
		// Dependency trees and caches.
		{".agentmod/node/lib/node_modules", true, "node-modules"},
		{".agentmod/node/npm-cache", true, "cache"},
		{".agentmod/node/pnpm", true, "cache"},
		{".agentmod/node/bun", true, "cache"},
		{".agentmod/claude/.cache", true, "cache"},
		// Path-anchored: a dir merely NAMED like a cache target elsewhere
		// is user content.
		{".agentmod/claude/npm-cache", true, ""},
		{".agentmod/claude/pnpm", true, ""},
		// tmp (directories only).
		{".agentmod/codex/tmp", true, "tmp"},
		{".agentmod/codex/.tmp", true, "tmp"},
		{".agentmod/codex/tmp", false, ""},
		// Stays in for normal handoffs: sessions, logs, configs.
		{".agentmod/codex/sessions", true, ""},
		{".agentmod/logs", true, ""},
		{".agentmod/agentmod.toml", false, ""},
		{".agentmod/claude/settings.json", false, ""},
		{".agentmod/opencode/opencode.json", false, ""},
	}
	for _, tc := range cases {
		kind := "file"
		if tc.isDir {
			kind = "dir"
		}
		if got := matchDefaultRules(tc.relPath, tc.isDir); got != tc.want {
			t.Errorf("%s %s: matched %q, want %q", kind, tc.relPath, got, tc.want)
		}
	}
}

// matchForGitRules is matchDefaultRules over ForGitRules.
func matchForGitRules(relPath string, isDir bool) string {
	base := relPath[strings.LastIndex(relPath, "/")+1:]
	for _, r := range ForGitRules() {
		if r.Matches(relPath, base, isDir) {
			return r.ID
		}
	}
	return ""
}

func TestForGitRulesTable(t *testing.T) {
	cases := []struct {
		relPath string
		isDir   bool
		want    string // rule ID, "" = kept
	}{
		// Claude session/history artifacts (path-anchored to the routed home).
		{".agentmod/claude/projects", true, "session-data"},
		{".agentmod/claude/sessions", true, "session-data"},
		{".agentmod/claude/session-env", true, "session-data"},
		{".agentmod/claude/file-history", true, "session-data"},
		{".agentmod/claude/shell-snapshots", true, "session-data"},
		{".agentmod/claude/history.jsonl", false, "session-data"},
		// Codex session/history artifacts.
		{".agentmod/codex/sessions", true, "session-data"},
		{".agentmod/codex/shell_snapshots", true, "session-data"},
		{".agentmod/codex/history.jsonl", false, "session-data"},
		{".agentmod/codex/session_index.jsonl", false, "session-data"},
		// OpenCode XDG data/state (opt-in xdg_full_isolation routing).
		{".agentmod/opencode/xdg/data", true, "session-data"},
		{".agentmod/opencode/xdg/state", true, "session-data"},
		// Logs: ours, Codex's log dir, Codex's logs_<n>.sqlite databases.
		{".agentmod/logs", true, "log-data"},
		{".agentmod/codex/log", true, "log-data"},
		{".agentmod/codex/logs_2.sqlite", false, "log-data"},
		{".agentmod/codex/logs_2.sqlite-shm", false, "log-data"},
		{".agentmod/codex/logs_2.sqlite-wal", false, "log-data"},
		// Anchoring: same names elsewhere are user content and stay in.
		{".agentmod/claude/skills/gstack/sessions", true, ""},
		{".agentmod/codex/memories/history.jsonl", false, ""},
		{".agentmod/codex/vendor/logs_2.sqlite", false, ""},
		{".agentmod/opencode/xdg/config", true, ""},
		// Files where only directories match (and vice versa) stay in.
		{".agentmod/codex/log", false, ""},
		{".agentmod/claude/projects", false, ""},
		// The default rules still apply (and still win first-match).
		{".agentmod/claude/.credentials.json", false, "auth-file"},
		{".agentmod/codex/tmp", true, "tmp"},
		// Working context that must keep traveling in git handoffs.
		{".agentmod/agentmod.toml", false, ""},
		{".agentmod/claude/settings.json", false, ""},
		{".agentmod/codex/memories", true, ""},
	}
	for _, tc := range cases {
		kind := "file"
		if tc.isDir {
			kind = "dir"
		}
		if got := matchForGitRules(tc.relPath, tc.isDir); got != tc.want {
			t.Errorf("%s %s: matched %q, want %q", kind, tc.relPath, got, tc.want)
		}
	}
}

// mkHostileFixture extends the standard fixture with one representative of
// every default-excluded category next to content that must survive.
func mkHostileFixture(t *testing.T) string {
	t.Helper()
	root := mkFixtureProject(t)
	am := filepath.Join(root, ".agentmod")
	for _, d := range []string{
		filepath.Join("node", "npm-cache"),
		filepath.Join("node", "lib", "node_modules", "leftpad"),
		filepath.Join("claude", "skills", "gstack", ".git"),
		filepath.Join("codex", "sessions"),
		filepath.Join("codex", ".ssh"),
		filepath.Join("codex", "tmp"),
	} {
		if err := os.MkdirAll(filepath.Join(am, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for rel, content := range map[string]string{
		filepath.Join("claude", ".credentials.json"):                        `{"token":"sk-FAKE-fixture"}`,
		filepath.Join("codex", "auth.json"):                                 `{"token":"sk-FAKE-fixture"}`,
		filepath.Join("claude", ".env"):                                     "SECRET=sk-FAKE-fixture\n",
		filepath.Join("codex", "id_ed25519"):                                "FAKE-fixture-key\n",
		filepath.Join("node", "npm-cache", "blob"):                          "cached",
		filepath.Join("node", "lib", "node_modules", "leftpad", "index.js"): "module.exports = 1\n",
		filepath.Join("claude", "skills", "gstack", ".git", "config"):       "[core]\n",
		filepath.Join("claude", "skills", "gstack", "SKILL.md"):             "# gstack\n",
		filepath.Join("codex", "sessions", "rollout.jsonl"):                 `{"role":"user"}` + "\n",
		filepath.Join("codex", ".ssh", "id_rsa"):                            "FAKE-fixture-key\n",
		filepath.Join("codex", "tmp", "scratch"):                            "scratch",
		filepath.Join("logs", "agentmod.log"):                               "log line\n",
	} {
		if err := os.WriteFile(filepath.Join(am, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCreateAppliesDefaultExclusions(t *testing.T) {
	root := mkHostileFixture(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)

	zr := openSnapshot(t, output)
	names := memberNames(zr)
	for _, banned := range []string{
		".credentials.json", "auth.json", "/.env", "id_ed25519",
		"node_modules", "npm-cache", "/.git/", ".ssh", "/tmp/", "id_rsa",
	} {
		for _, n := range names {
			if strings.Contains(n, banned) {
				t.Errorf("excluded content leaked into snapshot: %q (matched %q)", n, banned)
			}
		}
	}
	// Siblings of excluded entries survive, sessions and logs stay in.
	for _, want := range []string{
		"payload/.agentmod/claude/skills/gstack/SKILL.md",
		"payload/.agentmod/codex/sessions/rollout.jsonl",
		"payload/.agentmod/logs/agentmod.log",
		"payload/.agentmod/claude/settings.json",
	} {
		readMember(t, zr, want) // fails the test if absent
	}

	wantExcluded := map[string]string{
		".agentmod/claude/.credentials.json":   "auth-file",
		".agentmod/codex/auth.json":            "auth-file",
		".agentmod/claude/.env":                "env-file",
		".agentmod/codex/id_ed25519":           "ssh-key",
		".agentmod/codex/.ssh/":                "credential-dir",
		".agentmod/claude/skills/gstack/.git/": "vcs-git",
		".agentmod/node/lib/node_modules/":     "node-modules",
		".agentmod/node/npm-cache/":            "cache",
		".agentmod/codex/tmp/":                 "tmp",
		".agentmod/snapshots/":                 "snapshots-output",
	}
	got := map[string]string{}
	for _, e := range res.Excluded {
		got[e.Path] = e.RuleID
		if e.Reason == "" {
			t.Errorf("excluded entry %s has an empty reason", e.Path)
		}
	}
	for path, rule := range wantExcluded {
		if got[path] != rule {
			t.Errorf("Excluded[%q] = %q, want %q", path, got[path], rule)
		}
	}
	if len(res.Excluded) != len(wantExcluded) {
		t.Errorf("Excluded has %d entries, want %d: %v", len(res.Excluded), len(wantExcluded), res.Excluded)
	}
}

func TestCreateExcludedDirRecordedOnceNotDescended(t *testing.T) {
	root := mkHostileFixture(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)

	for _, e := range res.Excluded {
		if strings.Contains(e.Path, "node_modules/") && e.Path != ".agentmod/node/lib/node_modules/" {
			t.Errorf("descendant of a pruned dir was recorded separately: %q", e.Path)
		}
	}
	// Pruned content is also absent from the inventory/payload counts:
	// the hostile fixture keeps the base fixture's 5 files plus SKILL.md,
	// rollout.jsonl, and agentmod.log.
	if res.PayloadFiles != 8 {
		t.Errorf("PayloadFiles = %d, want 8", res.PayloadFiles)
	}
}

func TestCreateEmptyRulesDisablesPolicyExclusions(t *testing.T) {
	// A non-nil empty Rules slice is the documented escape hatch: only the
	// structural snapshots/ skip remains. Pin it so a future change cannot
	// silently re-tighten or loosen the contract.
	root := mkHostileFixture(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res, err := Create(CreateOptions{
		ProjectRoot: root,
		OutputPath:  output,
		CreatedAt:   testNow,
		Rules:       []Rule{},
	})
	if err != nil {
		t.Fatal(err)
	}
	zr := openSnapshot(t, output)
	readMember(t, zr, "payload/.agentmod/claude/.credentials.json")
	if len(res.Excluded) != 1 || res.Excluded[0].RuleID != "snapshots-output" {
		t.Errorf("Excluded = %v, want only the structural snapshots entry", res.Excluded)
	}
}

func TestCreateDeterministicWithExclusions(t *testing.T) {
	root := mkHostileFixture(t)
	dir := t.TempDir()
	out1 := filepath.Join(dir, "a.amod")
	out2 := filepath.Join(dir, "b.amod")
	res1 := createForTest(t, root, out1)
	res2 := createForTest(t, root, out2)
	d1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if string(d1) != string(d2) {
		t.Errorf("two creates with identical inputs differ (%d vs %d bytes)", len(d1), len(d2))
	}
	if len(res1.Excluded) != len(res2.Excluded) {
		t.Fatalf("Excluded lengths differ: %d vs %d", len(res1.Excluded), len(res2.Excluded))
	}
	for i := range res1.Excluded {
		if res1.Excluded[i] != res2.Excluded[i] {
			t.Errorf("Excluded[%d] differs: %v vs %v", i, res1.Excluded[i], res2.Excluded[i])
		}
	}
}
