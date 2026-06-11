package handoff

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pipelineForRestore runs the pinned pre-extraction pipeline (Open is the
// caller's via openForPlan; this adds Verify + PlanRestore) and fails the
// test on any problem — the state every legitimate restore starts from.
func pipelineForRestore(t *testing.T, snap *Snapshot) *RestorePlan {
	t.Helper()
	if res := snap.Verify(); len(res.Problems) > 0 {
		t.Fatalf("fixture snapshot failed verify: %v", res.Problems)
	}
	plan, problems := snap.PlanRestore()
	if len(problems) > 0 {
		t.Fatalf("fixture snapshot failed PlanRestore: %v", problems)
	}
	return plan
}

// digestTree walks dir and returns rel path → "kind mode content" for every
// entry, so two trees can be compared exactly (used to prove rollbacks).
func digestTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			tree[rel] = "dir " + info.Mode().Perm().String()
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			tree[rel] = "link " + target
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tree[rel] = "file " + info.Mode().Perm().String() + " " + string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// diffDigests reports the keys that differ between two digestTree results.
func diffDigests(before, after map[string]string) []string {
	var diffs []string
	for k, v := range before {
		if got, ok := after[k]; !ok {
			diffs = append(diffs, "removed "+k)
		} else if got != v {
			diffs = append(diffs, "changed "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			diffs = append(diffs, "created "+k)
		}
	}
	return diffs
}

// backupEntries returns the .agentmod.backup-* entries under root.
func backupEntries(t *testing.T, root string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, BackupPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestRestoreRoundTripFreshRoot(t *testing.T) {
	src := mkFixtureProject(t)
	// Extra mode-bearing entries beyond the shared fixture: a 0700 dir and a
	// 0600 file whose permission bits must survive the round trip.
	am := filepath.Join(src, ".agentmod")
	if err := os.Mkdir(filepath.Join(am, "claude", "private"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(am, "claude", "private", "notes.md"), []byte("quiet\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, src, output)

	snap := openForPlan(t, output)
	plan := pipelineForRestore(t, snap)
	target := t.TempDir()
	res, err := snap.Restore(target, plan, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if res.BackupPath != "" {
		t.Errorf("BackupPath = %q, want empty on a fresh root", res.BackupPath)
	}
	if res.Dirs != len(plan.Dirs) || res.Files != len(plan.Files) || res.Links != len(plan.Links) {
		t.Errorf("counts = %d/%d/%d, want %d/%d/%d (plan)",
			res.Dirs, res.Files, res.Links, len(plan.Dirs), len(plan.Files), len(plan.Links))
	}

	// Every write landed under .agentmod/ — the target root gained nothing else.
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".agentmod" {
		t.Fatalf("target root entries = %v, want exactly .agentmod", entries)
	}

	got := filepath.Join(target, ".agentmod")
	tree := digestTree(t, got)
	for rel, want := range map[string]string{
		"agentmod.toml":           "file -rw-r--r-- schema_version = 1\n",
		"claude/settings.json":    "file -rw-r--r-- {\"hooks\":{}}\n",
		"claude/run.sh":           "file -rwxr-xr-x #!/bin/sh\necho hi\n",
		"claude/private":          "dir -rwx------",
		"claude/private/notes.md": "file -rw------- quiet\n",
		"claude/link.toml":        "link ../agentmod.toml",
		"opencode/opencode.json":  "file -rw-r--r-- {}\n",
	} {
		if tree[filepath.FromSlash(rel)] != want {
			t.Errorf("%s = %q, want %q", rel, tree[filepath.FromSlash(rel)], want)
		}
	}
	// The symlink resolves to restored content, proving link and target both
	// landed (and that the link was created as a link, not followed).
	data, err := os.ReadFile(filepath.Join(got, "claude", "link.toml"))
	if err != nil || string(data) != "schema_version = 1\n" {
		t.Errorf("link.toml resolves to %q, %v", data, err)
	}
	// Empty payload dirs restore; snapshots/ never travels but is recreated
	// (with the rest of the standard layout) so doctor finds a complete tree.
	for _, d := range []string{"codex", "node", "logs", "snapshots"} {
		info, err := os.Stat(filepath.Join(got, d))
		if err != nil || !info.IsDir() {
			t.Errorf("standard dir %s missing after restore: %v", d, err)
		}
	}
	if ents, _ := os.ReadDir(filepath.Join(got, "snapshots")); len(ents) != 0 {
		t.Errorf("recreated snapshots/ is not empty: %v", ents)
	}
}

func TestRestoreBacksUpExistingTree(t *testing.T) {
	snap := openForPlan(t, planFixture(t))
	plan := pipelineForRestore(t, snap)

	target := t.TempDir()
	oldAM := filepath.Join(target, ".agentmod")
	if err := os.MkdirAll(filepath.Join(oldAM, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldAM, "claude", "sentinel.md"), []byte("old life\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := snap.Restore(target, plan, testNow)
	if err != nil {
		t.Fatal(err)
	}
	wantBackup := filepath.Join(target, BackupPrefix+testNow.UTC().Format(backupTimeFormat))
	if res.BackupPath != wantBackup {
		t.Errorf("BackupPath = %q, want %q", res.BackupPath, wantBackup)
	}
	data, err := os.ReadFile(filepath.Join(wantBackup, "claude", "sentinel.md"))
	if err != nil || string(data) != "old life\n" {
		t.Errorf("backup sentinel = %q, %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(oldAM, "claude", "sentinel.md")); !os.IsNotExist(err) {
		t.Errorf("old sentinel still present in the restored tree (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(oldAM, "agentmod.toml")); err != nil {
		t.Errorf("restored tree missing agentmod.toml: %v", err)
	}
}

func TestRestoreNilPlanRefused(t *testing.T) {
	snap := openForPlan(t, planFixture(t))
	target := t.TempDir()
	oldAM := filepath.Join(target, ".agentmod")
	if err := os.MkdirAll(oldAM, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldAM, "sentinel"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := digestTree(t, target)

	if _, err := snap.Restore(target, nil, testNow); err == nil {
		t.Fatal("Restore accepted a nil plan")
	} else if !strings.Contains(err.Error(), "no extraction plan") {
		t.Errorf("error = %v, want it to name the missing plan", err)
	}
	if diffs := diffDigests(before, digestTree(t, target)); len(diffs) != 0 {
		t.Errorf("nil-plan refusal touched the tree: %v", diffs)
	}
}

func TestRestoreFailureRollsBackToPrevious(t *testing.T) {
	snap := openForPlan(t, planFixture(t))
	plan := pipelineForRestore(t, snap)
	// Sabotage: a planned member the archive does not contain makes
	// extraction fail partway through (dirs already created).
	plan.Files = append(plan.Files, PlanEntry{
		ZipName: PayloadPrefix + ".agentmod/ghost",
		RelPath: ".agentmod/ghost",
		Mode:    0o644,
	})

	target := t.TempDir()
	oldAM := filepath.Join(target, ".agentmod")
	if err := os.MkdirAll(filepath.Join(oldAM, "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldAM, "codex", "history.md"), []byte("precious\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := digestTree(t, target)

	_, err := snap.Restore(target, plan, testNow)
	if err == nil {
		t.Fatal("Restore succeeded despite a vanished planned member")
	}
	if !strings.Contains(err.Error(), "vanished") || !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error = %v, want vanished-member cause and rollback statement", err)
	}
	if diffs := diffDigests(before, digestTree(t, target)); len(diffs) != 0 {
		t.Errorf("rollback did not restore the previous tree exactly: %v", diffs)
	}
	if leftovers := backupEntries(t, target); len(leftovers) != 0 {
		t.Errorf("rollback left backup entries behind: %v", leftovers)
	}
}

func TestRestoreFailureFreshRootLeavesNothing(t *testing.T) {
	snap := openForPlan(t, planFixture(t))
	plan := pipelineForRestore(t, snap)
	plan.Files = append(plan.Files, PlanEntry{
		ZipName: PayloadPrefix + ".agentmod/ghost",
		RelPath: ".agentmod/ghost",
		Mode:    0o644,
	})

	target := t.TempDir()
	if _, err := snap.Restore(target, plan, testNow); err == nil {
		t.Fatal("Restore succeeded despite a vanished planned member")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed restore on a fresh root left entries: %v", entries)
	}
}

func TestRestoreNeverOverwritesThroughDuplicateEntry(t *testing.T) {
	// O_EXCL is the no-overwrite guarantee: a plan that targets the same
	// path twice (only a hostile or buggy caller can produce one —
	// PlanRestore refuses duplicates) fails on the second write and rolls
	// back rather than clobbering.
	snap := openForPlan(t, planFixture(t))
	plan := pipelineForRestore(t, snap)
	plan.Files = append(plan.Files, plan.Files[0])

	target := t.TempDir()
	_, err := snap.Restore(target, plan, testNow)
	if err == nil {
		t.Fatal("Restore succeeded despite a duplicate plan target")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error = %v, want rollback statement", err)
	}
	entries, readErr := os.ReadDir(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Errorf("failed restore left entries: %v", entries)
	}
}

func TestRestoreSnapshotInsideOwnSnapshotsDir(t *testing.T) {
	// The .amod commonly lives in .agentmod/snapshots/ of the very tree
	// being replaced. BackupAgentmod renames that tree away mid-restore; the
	// zip handle Open established stays valid (POSIX rename keeps open fds),
	// so extraction still reads every member.
	root := mkFixtureProject(t)
	output := filepath.Join(root, ".agentmod", "snapshots", "self.amod")
	createForTest(t, root, output)

	snap := openForPlan(t, output)
	plan := pipelineForRestore(t, snap)
	res, err := snap.Restore(root, plan, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if res.BackupPath == "" {
		t.Fatal("no backup created despite an existing .agentmod")
	}
	// The snapshot traveled into the backup with the old tree...
	if _, err := os.Stat(filepath.Join(res.BackupPath, "snapshots", "self.amod")); err != nil {
		t.Errorf("snapshot not in the backup: %v", err)
	}
	// ...and the restored tree is complete, with an empty snapshots/.
	if _, err := os.Stat(filepath.Join(root, ".agentmod", "agentmod.toml")); err != nil {
		t.Errorf("restored tree missing agentmod.toml: %v", err)
	}
	if ents, _ := os.ReadDir(filepath.Join(root, ".agentmod", "snapshots")); len(ents) != 0 {
		t.Errorf("restored snapshots/ is not empty: %v", ents)
	}
}
