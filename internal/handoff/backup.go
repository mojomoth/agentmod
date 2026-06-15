package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mojomoth/agentmod/internal/project"
)

// BackupPrefix is the project-root-relative name prefix of restore backups:
// every backup is <BackupPrefix><utc-stamp> next to where .agentmod/ was
// (IMPLEMENTATION_PLAN §12). The prefix is exported so the restore command
// can name the pattern in its output and gitignore handling.
const BackupPrefix = project.DirName + ".backup-"

// backupTimeFormat is the UTC stamp in backup names. It matches the default
// snapshot-name stamp so the two artifacts date themselves the same way.
const backupTimeFormat = "20060102-150405"

// BackupAgentmod moves projectRoot/.agentmod aside to
// .agentmod.backup-<utc-stamp> so a restore can extract a fresh tree without
// destroying the current environment (FABLE_PLAN §18/§25). The move is a
// single rename: atomic, preserves every mode/symlink/session byte without
// reading any of them, and keeps the original recoverable — if extraction
// later fails, renaming the backup back is the complete rollback. Whatever
// occupies the .agentmod name is backed up as-is, even a stray regular file;
// judging it is doctor's job, losing it is never acceptable here.
//
// The returned path names the backup; "" with a nil error means nothing
// existed to back up. An existing entry at the backup name refuses the
// backup (D034's collision discipline) — nothing is ever overwritten.
func BackupAgentmod(projectRoot string, now time.Time) (string, error) {
	source := filepath.Join(projectRoot, project.DirName)
	if _, err := os.Lstat(source); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("backup of %s: %w", source, err)
	}
	target := filepath.Join(projectRoot, BackupPrefix+now.UTC().Format(backupTimeFormat))
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("backup target %s already exists (another restore in the same second?); remove or rename it, then retry", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("backup of %s: %w", source, err)
	}
	if err := os.Rename(source, target); err != nil {
		return "", fmt.Errorf("backup of %s: %w", source, err)
	}
	return target, nil
}
