// Restore-side extraction (FABLE_PLAN §18 "Handoff restore", §25;
// IMPLEMENTATION_PLAN §12 restore pipeline). Restore executes a validated
// RestorePlan under a project root: back up the existing .agentmod/ first
// (BackupAgentmod, D042), extract directories, then files, then symlinks —
// links last so no file write can pass through a just-restored link — and
// roll the whole thing back on any failure. Nothing from the snapshot is
// ever executed; members are only written to disk.
//
// Callers must run Open → Verify → PlanRestore first and refuse on any
// problem from either (§21 "Never trust external snapshots"); Restore
// trusts its plan precisely because PlanRestore confined every path to
// .agentmod/.

package handoff

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

// RestoreResult reports what Restore did.
type RestoreResult struct {
	BackupPath string // where the previous .agentmod went; "" when none existed
	Dirs       int    // directories created from the plan
	Files      int    // regular files written
	Links      int    // symlinks created
}

// Restore executes plan under projectRoot. The existing .agentmod entry (if
// any) is moved aside by BackupAgentmod before the first write, so
// extraction always targets a fresh tree and O_EXCL writes cannot collide
// with current state. On any extraction failure the partial tree is removed
// and the backup renamed back — the rollback IS the rename (D042); when the
// rollback itself fails the error names both the partial tree and the
// backup so nothing is silently lost.
//
// After extraction the standard layout directories missing from the payload
// are recreated (snapshots/ never travels — it is structurally excluded at
// create time) so doctor finds a complete tree.
//
// The snapshot's zip handle stays valid even when the .amod file itself
// lives inside the .agentmod/snapshots/ tree being renamed away: members
// are read through the handle Open established, never re-opened by path.
func (s *Snapshot) Restore(projectRoot string, plan *RestorePlan, now time.Time) (*RestoreResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("restore of %s: no extraction plan (run PlanRestore and refuse on its problems first)", s.Path)
	}
	byName := map[string]*zip.File{}
	for _, f := range s.zr.File {
		if _, dup := byName[f.Name]; !dup {
			byName[f.Name] = f
		}
	}

	backup, err := BackupAgentmod(projectRoot, now)
	if err != nil {
		return nil, err
	}
	res := &RestoreResult{BackupPath: backup}
	agentmodDir := filepath.Join(projectRoot, project.DirName)

	if err := extractPlan(byName, projectRoot, plan, res); err != nil {
		// Full rollback: drop the partial tree, move the backup back.
		if rmErr := os.RemoveAll(agentmodDir); rmErr != nil {
			return nil, fmt.Errorf("restore of %s failed (%v) and the partial %s could not be removed (%v); the previous environment is intact at %s", s.Path, err, agentmodDir, rmErr, backupNote(backup))
		}
		if backup != "" {
			if rnErr := os.Rename(backup, agentmodDir); rnErr != nil {
				return nil, fmt.Errorf("restore of %s failed (%v) and the backup could not be moved back (%v); your previous environment is intact at %s", s.Path, err, rnErr, backup)
			}
		}
		return nil, fmt.Errorf("restore of %s failed (%v); the previous environment was rolled back", s.Path, err)
	}

	// snapshots/ is structurally excluded from every payload and other
	// standard directories may have been empty-but-policy-pruned; recreate
	// whatever the plan did not so the restored tree is doctor-complete.
	for _, d := range layout.Subdirs() {
		if err := os.MkdirAll(filepath.Join(agentmodDir, d), 0o755); err != nil {
			return nil, fmt.Errorf("restore of %s: recreating %s/%s: %w", s.Path, project.DirName, d, err)
		}
	}
	return res, nil
}

// extractPlan writes the plan's entries under projectRoot: Dirs, then
// Files, then Links (D041 ordering). Directories are created restrictively
// (0o700) so every later write succeeds, then chmodded to their recorded
// modes deepest-first once all content is in place; file modes are applied
// with an explicit Chmod so the umask cannot strip recorded exec bits.
func extractPlan(byName map[string]*zip.File, projectRoot string, plan *RestorePlan, res *RestoreResult) error {
	target := func(e PlanEntry) string {
		return filepath.Join(projectRoot, filepath.FromSlash(e.RelPath))
	}
	for _, e := range plan.Dirs {
		if err := os.MkdirAll(target(e), 0o700); err != nil {
			return err
		}
		res.Dirs++
	}
	for _, e := range plan.Files {
		f := byName[e.ZipName]
		if f == nil {
			return fmt.Errorf("planned member %s vanished from the archive", e.ZipName)
		}
		if err := writeFileMember(f, target(e), e.Mode); err != nil {
			return err
		}
		res.Files++
	}
	for _, e := range plan.Links {
		if err := os.Symlink(filepath.FromSlash(e.Target), target(e)); err != nil {
			return err
		}
		res.Links++
	}
	for i := len(plan.Dirs) - 1; i >= 0; i-- {
		e := plan.Dirs[i]
		if err := os.Chmod(target(e), e.Mode); err != nil {
			return err
		}
	}
	return nil
}

// writeFileMember streams one zip member to path. O_EXCL: extraction
// targets a tree that did not exist a moment ago, so any collision means a
// hostile or malformed archive (e.g. a dir member shadowing a file path)
// and must fail rather than overwrite.
func writeFileMember(f *zip.File, path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("reading member %s: %w", f.Name, err)
	}
	defer rc.Close()
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	// OpenFile's mode is filtered by the umask; recorded exec bits must
	// survive restore (IMPLEMENTATION_PLAN §14).
	return os.Chmod(path, mode)
}

// backupNote names the backup location for rollback-failure messages.
func backupNote(backup string) string {
	if backup == "" {
		return "(no backup was needed; nothing existed before)"
	}
	return backup
}
