package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBackupAgentmodMovesTreeIntact proves the rename moves the whole tree —
// file bytes, exec bit, symlink target — frees the .agentmod name, and that
// renaming the backup back is a complete rollback (T25 "backup restorable").
func TestBackupAgentmodMovesTreeIntact(t *testing.T) {
	root := mkFixtureProject(t)
	source := filepath.Join(root, ".agentmod")

	got, err := BackupAgentmod(root, testNow)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".agentmod.backup-20260611-123045")
	if got != want {
		t.Fatalf("backup path = %q, want %q", got, want)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Errorf(".agentmod still present after backup (Lstat err = %v)", err)
	}

	data, err := os.ReadFile(filepath.Join(got, "agentmod.toml"))
	if err != nil || string(data) != "schema_version = 1\n" {
		t.Errorf("agentmod.toml in backup = %q, %v", data, err)
	}
	info, err := os.Stat(filepath.Join(got, "claude", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("run.sh lost its exec bit in the backup: mode %v", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(got, "claude", "link.toml"))
	if err != nil || target != "../agentmod.toml" {
		t.Errorf("link.toml target in backup = %q, %v; want ../agentmod.toml", target, err)
	}

	if err := os.Rename(got, source); err != nil {
		t.Fatalf("rolling the backup back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "claude", "settings.json")); err != nil {
		t.Errorf("rolled-back tree incomplete: %v", err)
	}
}

func TestBackupAgentmodNothingToBackUp(t *testing.T) {
	root := t.TempDir()
	got, err := BackupAgentmod(root, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("backup path = %q, want empty for an absent .agentmod", got)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("project root gained %d entries from a no-op backup", len(entries))
	}
}

func TestBackupAgentmodRefusesExistingTarget(t *testing.T) {
	root := mkFixtureProject(t)
	occupied := filepath.Join(root, ".agentmod.backup-20260611-123045")
	if err := os.Mkdir(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "sentinel"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := BackupAgentmod(root, testNow)
	if err == nil {
		t.Fatal("want an error when the backup name is already taken")
	}
	if !strings.Contains(err.Error(), occupied) || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q does not name the occupied target", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agentmod", "agentmod.toml")); err != nil {
		t.Errorf(".agentmod was disturbed by the refused backup: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(occupied, "sentinel"))
	if err != nil || string(data) != "mine" {
		t.Errorf("pre-existing entry at the backup name was disturbed: %q, %v", data, err)
	}
}

// A stray regular file at the .agentmod name is backed up as-is: losing user
// data is never acceptable, judging the entry is doctor's job.
func TestBackupAgentmodBacksUpStrayRegularFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".agentmod")
	if err := os.WriteFile(source, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := BackupAgentmod(root, testNow)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got)
	if err != nil || string(data) != "not a directory" {
		t.Errorf("backed-up file = %q, %v", data, err)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Errorf(".agentmod still present after backup (Lstat err = %v)", err)
	}
}

func TestBackupAgentmodStampIsUTC(t *testing.T) {
	root := mkFixtureProject(t)
	kst := time.FixedZone("KST", 9*60*60)
	got, err := BackupAgentmod(root, testNow.In(kst))
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(got); base != ".agentmod.backup-20260611-123045" {
		t.Errorf("backup name = %q; the stamp must render in UTC regardless of the clock's zone", base)
	}
}
