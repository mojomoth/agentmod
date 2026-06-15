// Git-storable handoff packaging (FABLE_PLAN §19, IMPLEMENTATION_PLAN §13).
// CreateForGit renders the same members as a .amod snapshot — manifest,
// inventory, redaction report, the two human-readable documents, checksums,
// and the payload tree — as PLAIN FILES under the project's
// .agentmod-handoff/ directory, so the package can be committed, reviewed,
// and diffed like any other repository content. agentmod never commits it;
// git moves it (D047).

package handoff

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mojomoth/agentmod/internal/project"
)

// GitDirName is the git-storable package directory at the project root
// (FABLE_PLAN §10). It is deliberately NOT gitignored — committing it is
// the point.
const GitDirName = ".agentmod-handoff"

// treeSink renders members as plain files under root. Directory permission
// bits are recorded during the walk and applied deepest-first at Close, so
// a read-only directory can never block writing its own children (the D043
// extraction precedent); file modes are set at create time plus an explicit
// Chmod so recorded exec bits survive the process umask. Symlink permission
// bits are not portably settable (and git does not store them), so links
// keep the platform default; the inventory still records the source bits.
type treeSink struct {
	root     string
	dirPerms []treeDirPerm
}

type treeDirPerm struct {
	path string
	perm fs.FileMode
}

func (s *treeSink) path(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

func (s *treeSink) Dir(name string, perm fs.FileMode) error {
	p := s.path(name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return err
	}
	s.dirPerms = append(s.dirPerms, treeDirPerm{path: p, perm: perm})
	return nil
}

func (s *treeSink) Symlink(name string, perm fs.FileMode, target string) error {
	return os.Symlink(target, s.path(name))
}

func (s *treeSink) File(name string, perm fs.FileMode, data []byte) error {
	p := s.path(name)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(p, perm)
}

func (s *treeSink) Close() error {
	// The walk appends parents before children, so the reverse order is
	// deepest-first.
	for i := len(s.dirPerms) - 1; i >= 0; i-- {
		if err := os.Chmod(s.dirPerms[i].path, s.dirPerms[i].perm); err != nil {
			return err
		}
	}
	return nil
}

// CreateForGit packs the project's .agentmod/ directory into the
// git-storable tree package at <ProjectRoot>/.agentmod-handoff/
// (opts.OutputPath is ignored — the destination is fixed so the package
// always lands where the repository expects it).
//
// .agentmod-handoff/ is a publishing area, so a previous package —
// recognized by its manifest.json — is REPLACED; anything else at that path
// belongs to the user and is refused untouched. The new tree is built in a
// dot-prefixed temp directory next to the target and swapped in only after
// it is complete (the D031 install --force pattern), so the package never
// exists in a partial state and a failed run leaves the previous one
// intact.
func CreateForGit(opts CreateOptions) (*Result, error) {
	opts.ForGit = true // the tree format owns the flag
	if opts.Rules == nil {
		// §19 default for the committable format: sessions/logs excluded on
		// top of the regular policy. An explicitly non-nil Rules slice is
		// honored as-is, same as Create (the pinned escape hatch, D035).
		opts.Rules = ForGitRules()
	}
	agentmodDir := filepath.Join(opts.ProjectRoot, project.DirName)
	if fi, err := os.Lstat(agentmodDir); err != nil {
		return nil, fmt.Errorf("handoff create --for-git: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("handoff create --for-git: %s is not a directory", agentmodDir)
	}

	target := filepath.Join(opts.ProjectRoot, GitDirName)
	replacing := false
	switch fi, err := os.Lstat(target); {
	case err == nil && fi.IsDir():
		if _, merr := os.Lstat(filepath.Join(target, ManifestName)); merr != nil {
			return nil, fmt.Errorf("handoff create --for-git: %s exists but does not look like an agentmod git handoff package (no %s); move it aside first", target, ManifestName)
		}
		replacing = true
	case err == nil:
		return nil, fmt.Errorf("handoff create --for-git: %s exists and is not a directory; move it aside first", target)
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("handoff create --for-git: %w", err)
	}

	tmp, err := os.MkdirTemp(opts.ProjectRoot, GitDirName+"-partial-")
	if err != nil {
		return nil, fmt.Errorf("handoff create --for-git: %w", err)
	}
	defer os.RemoveAll(tmp) // no-op once the tree is renamed onto the target

	res, err := writeSnapshot(&treeSink{root: tmp}, agentmodDir, nil, opts)
	if err != nil {
		return nil, err
	}

	if replacing {
		// Reserve a sibling name for the old package; Darwin's rename(2)
		// refuses an existing destination directory even when empty (the
		// D031 gotcha), so the reservation is removed just before renaming
		// onto it.
		old, err := os.MkdirTemp(opts.ProjectRoot, GitDirName+"-old-")
		if err != nil {
			return nil, fmt.Errorf("handoff create --for-git: %w", err)
		}
		if err := os.Remove(old); err != nil {
			return nil, fmt.Errorf("handoff create --for-git: %w", err)
		}
		if err := os.Rename(target, old); err != nil {
			return nil, fmt.Errorf("handoff create --for-git: %w", err)
		}
		if err := os.Rename(tmp, target); err != nil {
			if rerr := os.Rename(old, target); rerr != nil {
				return nil, fmt.Errorf("handoff create --for-git: could not move the new package in (%v) and could not put the previous one back (%v); the previous package is preserved at %s", err, rerr, old)
			}
			return nil, fmt.Errorf("handoff create --for-git: %v (the previous package was left in place)", err)
		}
		if err := os.RemoveAll(old); err != nil {
			return nil, fmt.Errorf("handoff create --for-git: the new package is in place but the previous one could not be removed from %s: %w", old, err)
		}
	} else if err := os.Rename(tmp, target); err != nil {
		return nil, fmt.Errorf("handoff create --for-git: %w", err)
	}
	res.OutputPath = target
	return res, nil
}
