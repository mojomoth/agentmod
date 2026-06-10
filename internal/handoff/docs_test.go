package handoff

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateHandoffDocContents proves HANDOFF.md lands in the snapshot and
// carries the §12 trio: what this is (project, timestamp, version,
// platform, payload size), how to restore (incl. the honest restore-not-
// implemented note), and what's missing (exclusion + scan summary and the
// D035 gstack/.git and node/bin notes).
func TestCreateHandoffDocContents(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	doc := string(readMember(t, zr, HandoffDocName))

	for _, want := range []string{
		"# Agent environment handoff",
		"`" + filepath.Base(root) + "`",
		"Created 2026-06-11T12:30:45Z by agentmod test-version on testos/testarch.",
		"- 5 files (",
		"agentmod handoff restore",
		"the agentmod build that created this snapshot (test-version) does not\nimplement restore yet",
		// The fixture's pre-existing snapshot triggers exactly the
		// structural snapshots-output exclusion.
		"- 1 entry was excluded by the redaction policy",
		"`REDACTION.md`",
		"- Secret scan: clean (no candidate patterns in packed files).\n",
		"every agent needs a fresh login",
		"`agentmod install gstack --force`",
		"`.agentmod/node/bin`",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("HANDOFF.md missing %q\n--- document ---\n%s", want, doc)
		}
	}
}

// TestCreateRestoreDocContents proves RESTORE.md lands in the snapshot with
// the step-by-step restore flow, the verbatim canonical re-login remedies,
// and the D035 reinstall guidance.
func TestCreateRestoreDocContents(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	doc := string(readMember(t, zr, RestoreDocName))

	for _, want := range []string{
		"# Restoring this snapshot",
		"Written by agentmod test-version at create time.",
		"does not implement\nrestore yet",
		"`agentmod init`",
		"`agentmod handoff restore <file>.amod`",
		"never executes anything from the snapshot",
		"`agentmod doctor`",
		"- Claude Code: " + ClaudeReloginRemedy + ".\n",
		"- Codex CLI: " + CodexReloginRemedy + ".\n",
		"OpenCode: log in to your provider again",
		"Keychain",
		"`agentmod install gstack --force`",
		"`lib/node_modules`",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("RESTORE.md missing %q\n--- document ---\n%s", want, doc)
		}
	}
}

// TestRenderHandoffDocStates pins the singular/plural and empty-state
// renderings of the "What is missing" summary lines.
func TestRenderHandoffDocStates(t *testing.T) {
	base := Result{PayloadFiles: 1, PayloadBytes: 7}

	clean := string(renderHandoffDoc(testNow, "v", "os/arch", "proj", &base))
	for _, want := range []string{
		"- 1 file (7 bytes) under `payload/`",
		"- Nothing was excluded by the redaction policy.\n",
		"- Secret scan: clean (no candidate patterns in packed files).\n",
	} {
		if !strings.Contains(clean, want) {
			t.Errorf("clean HANDOFF.md missing %q\n--- document ---\n%s", want, clean)
		}
	}

	busy := base
	busy.PayloadFiles = 2
	busy.Excluded = []ExcludedEntry{
		{Path: ".agentmod/a.env", RuleID: "env-file", Reason: "r"},
		{Path: ".agentmod/.git/", RuleID: "vcs-git", Reason: "r"},
	}
	busy.Findings = []ScanFinding{{Path: ".agentmod/x", Pattern: "sk-token", Line: 1}}
	doc := string(renderHandoffDoc(testNow, "v", "os/arch", "proj", &busy))
	for _, want := range []string{
		"- 2 files (7 bytes) under `payload/`",
		"- 2 entries were excluded by the redaction policy",
		"- Secret scan: 1 candidate finding in packed files",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("busy HANDOFF.md missing %q\n--- document ---\n%s", want, doc)
		}
	}

	busy.Findings = append(busy.Findings, ScanFinding{Path: ".agentmod/y", Pattern: "sk-token", Line: 2})
	doc = string(renderHandoffDoc(testNow, "v", "os/arch", "proj", &busy))
	if !strings.Contains(doc, "- Secret scan: 2 candidate findings in packed files") {
		t.Errorf("two-finding HANDOFF.md missing plural scan line\n--- document ---\n%s", doc)
	}
}
