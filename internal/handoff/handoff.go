// Package handoff implements .amod snapshot creation (FABLE_PLAN §18/§21,
// IMPLEMENTATION_PLAN §12). A .amod file is a zip whose members are
// manifest.json, inventory.json, REDACTION.md, HANDOFF.md, RESTORE.md,
// checksums.txt, and the
// payload tree under payload/ with forward-slash project-root-relative
// names (payload/.agentmod/...), so restore maps members back onto the
// project root directly. Inspect/verify/restore consume the same
// structures.
package handoff

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mojomoth/agentmod/internal/layout"
	"github.com/mojomoth/agentmod/internal/project"
)

// SchemaVersion is the .amod manifest schema this build writes and the
// newest restore will accept.
const SchemaVersion = 1

// Member names at the zip root. RedactionName lives in redaction.go.
const (
	ManifestName  = "manifest.json"
	InventoryName = "inventory.json"
	ChecksumsName = "checksums.txt"
	PayloadPrefix = "payload/"
)

// Manifest is manifest.json. Later slices extend it (policy flags);
// restore must tolerate the absence of optional fields in
// schema-version-1 snapshots.
type Manifest struct {
	SchemaVersion   int    `json:"schema_version"`
	CreatedAt       string `json:"created_at"` // RFC3339, UTC
	AgentmodVersion string `json:"agentmod_version"`
	Platform        string `json:"platform"` // "<GOOS>/<GOARCH>"
	// Git is nil — and the key absent from manifest.json — when the project
	// is not inside a git repository or no git binary was available at
	// create time. Restore must tolerate its absence.
	Git *GitState `json:"git,omitempty"`
	// ForGit marks a git-storable tree package created with --for-git
	// (FABLE_PLAN §19). Absent from regular .amod snapshots.
	ForGit bool `json:"for_git,omitempty"`
}

// GitState is the manifest's record of the project's git repository at
// create time. The CALLER collects it (the cli executes git, D030
// precedent); this package stays exec-free so snapshot writing is
// deterministic under test. Restore compares these fields against the
// target machine's repository (FABLE_PLAN §18).
type GitState struct {
	Branch        string `json:"branch,omitempty"` // empty when HEAD is detached
	Head          string `json:"head,omitempty"`   // commit hash; empty on an unborn branch
	Dirty         bool   `json:"dirty"`
	StatusSummary string `json:"status_summary"`       // "clean" or counts, e.g. "1 staged, 2 untracked"
	RemoteURL     string `json:"remote_url,omitempty"` // origin URL with credentials redacted
	// SourceIncluded records whether project source code traveled in the
	// snapshot (FABLE_PLAN §20). Always false in this version — patch
	// inclusion is a future explicit option; the field exists so a reader
	// of an old manifest never has to guess.
	SourceIncluded bool `json:"source_included"`
}

// InventoryEntry describes one non-directory payload member.
type InventoryEntry struct {
	Path          string `json:"path"`   // zip member name (payload/...)
	Size          int64  `json:"size"`   // bytes of zip member content
	SHA256        string `json:"sha256"` // hex of zip member content
	Mode          string `json:"mode"`   // octal permission bits, e.g. "0755"
	SymlinkTarget string `json:"symlink_target,omitempty"`
}

// Inventory is inventory.json.
type Inventory struct {
	Files []InventoryEntry `json:"files"` // sorted by Path
}

// CreateOptions parameterizes Create. The clock and identity fields are
// injected so output is deterministic under test.
type CreateOptions struct {
	ProjectRoot string    // directory containing .agentmod/
	OutputPath  string    // where the .amod file is written; must not exist
	CreatedAt   time.Time // manifest timestamp + zip member mtimes
	Version     string    // agentmod version for the manifest
	Platform    string    // "<GOOS>/<GOARCH>" for the manifest
	// Rules is the exclusion policy: nil means DefaultRules(). A non-nil
	// slice is used as-is — an explicitly empty one disables every policy
	// exclusion (the structural snapshots/ skip still applies), so the
	// caller owns the secret-safety consequences.
	Rules []Rule
	// AllowFindings packs the snapshot even when the secret scan hits a
	// HARD finding (private-key material in a kept file). Warn-level
	// findings never block; both kinds are listed in REDACTION.md.
	AllowFindings bool
	// Git is the project's git state, collected by the CALLER — the cli
	// executes git (D030 precedent) so this package stays exec-free and
	// deterministic under test. nil means no repository or no git binary;
	// manifest.json omits the key then. The dirty-worktree consent gate
	// (--allow-dirty) is also the caller's: by the time Create runs, packing
	// has been approved.
	Git *GitState
	// ForGit marks a git-storable tree package (FABLE_PLAN §19): the
	// manifest records it and the human-readable documents describe the
	// tree format instead of the .amod file. The FORMAT owns the flag —
	// CreateForGit forces it true, Create forces it false — so a caller
	// can never mislabel a package.
	ForGit bool
}

// Result reports what Create wrote.
type Result struct {
	OutputPath   string
	PayloadFiles int   // non-directory payload members
	PayloadBytes int64 // total content bytes of those members
	// Excluded lists every entry the exclusion engine dropped (plus the
	// structural snapshots/ skip), in walk (lexical) order. A pruned
	// directory is recorded once, not per descendant. The redaction report
	// renders this list.
	Excluded []ExcludedEntry
	// Findings lists every secret-candidate match the content scan made in
	// KEPT files, in walk order (pattern order within a file). The
	// redaction report renders this list too; hard findings only appear
	// here when AllowFindings let creation proceed.
	Findings []ScanFinding
}

// Create packs the project's .agentmod/ directory into a .amod snapshot.
//
// The payload is everything under .agentmod/ except .agentmod/snapshots
// (structural: it is the default OUTPUT directory, so packing it would nest
// prior snapshots — and, mid-write, the partially-written one — inside the
// new one) and whatever opts.Rules exclude (DefaultRules when nil: auth
// files, .env, ssh/cloud credentials, .git, node_modules, caches, tmp).
// Everything dropped is recorded in Result.Excluded.
//
// The output file never exists in a partial state: Create writes a dot-
// prefixed temp file in the output directory and renames it over
// OutputPath only after the zip is complete; any error removes the temp.
func Create(opts CreateOptions) (*Result, error) {
	opts.ForGit = false // the .amod format owns the flag
	agentmodDir := filepath.Join(opts.ProjectRoot, project.DirName)
	if fi, err := os.Lstat(agentmodDir); err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("handoff create: %s is not a directory", agentmodDir)
	}
	if _, err := os.Lstat(opts.OutputPath); err == nil {
		return nil, fmt.Errorf("handoff create: %s already exists (choose another --output)", opts.OutputPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("handoff create: %w", err)
	}

	outDir := filepath.Dir(opts.OutputPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	tmp, err := os.CreateTemp(outDir, ".amod-partial-")
	if err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	defer func() {
		if tmp != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	sink := &zipSink{zw: zip.NewWriter(tmp), modified: opts.CreatedAt.UTC()}
	res, err := writeSnapshot(sink, agentmodDir, []string{tmp.Name(), opts.OutputPath}, opts)
	if err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		tmp = nil
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	if err := os.Rename(tmp.Name(), opts.OutputPath); err != nil {
		os.Remove(tmp.Name())
		tmp = nil
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	tmp = nil // committed; nothing left to clean up
	res.OutputPath = opts.OutputPath
	return res, nil
}

// memberSink receives the snapshot's members in write order. zipSink
// renders them as .amod zip members; treeSink (gitpack.go) renders them as
// plain files for the git-storable tree variant — both are fed by the same
// writeSnapshot walk, so the two formats can never drift in content.
type memberSink interface {
	// Dir records a payload directory; name has no trailing slash.
	Dir(name string, perm fs.FileMode) error
	// Symlink records a symlink member whose content IS the target string.
	Symlink(name string, perm fs.FileMode, target string) error
	// File records a regular member with its full content.
	File(name string, perm fs.FileMode, data []byte) error
	// Close finalizes the sink's output.
	Close() error
}

// zipSink renders members as .amod zip members, all stamped with the
// snapshot's creation time so identical inputs produce byte-identical zips.
type zipSink struct {
	zw       *zip.Writer
	modified time.Time
}

func (s *zipSink) Dir(name string, perm fs.FileMode) error {
	hdr := &zip.FileHeader{Name: name + "/", Method: zip.Store, Modified: s.modified}
	hdr.SetMode(fs.ModeDir | perm)
	_, err := s.zw.CreateHeader(hdr)
	return err
}

func (s *zipSink) Symlink(name string, perm fs.FileMode, target string) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: s.modified}
	hdr.SetMode(fs.ModeSymlink | perm)
	w, err := s.zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(target))
	return err
}

func (s *zipSink) File(name string, perm fs.FileMode, data []byte) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: s.modified}
	hdr.SetMode(perm)
	w, err := s.zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (s *zipSink) Close() error { return s.zw.Close() }

// writeSnapshot walks the payload into sink — payload members first (hashing
// as it copies), then inventory/manifest/checksums derived from those
// hashes. skipPaths are in-progress output files skipped if the walk meets
// them (the zip temp file and OutputPath when they sit inside .agentmod/).
func writeSnapshot(sink memberSink, agentmodDir string, skipPaths []string, opts CreateOptions) (*Result, error) {
	modified := opts.CreatedAt.UTC()
	res := &Result{}
	var entries []InventoryEntry

	rules := opts.Rules
	if rules == nil {
		rules = DefaultRules()
	}

	skipAbs := map[string]bool{}
	for _, p := range skipPaths {
		if abs, err := filepath.Abs(p); err == nil {
			skipAbs[abs] = true
		}
	}
	snapshotsDir := filepath.Join(agentmodDir, layout.SnapshotsDir)

	walkErr := filepath.WalkDir(agentmodDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == snapshotsDir && d.IsDir() {
			res.Excluded = append(res.Excluded, snapshotsExclusion)
			return filepath.SkipDir
		}
		if abs, aerr := filepath.Abs(path); aerr == nil && skipAbs[abs] {
			return nil
		}
		rel, err := filepath.Rel(agentmodDir, path)
		if err != nil {
			return err
		}
		relProj := project.DirName
		if rel != "." {
			relProj += "/" + filepath.ToSlash(rel)

			// Policy exclusions: the rule check precedes the member-kind
			// switch, so even an irregular file with a matching name is
			// silently dropped rather than a create-time error.
			for _, r := range rules {
				if !r.Matches(relProj, d.Name(), d.IsDir()) {
					continue
				}
				excludedPath := relProj
				if d.IsDir() {
					excludedPath += "/"
				}
				res.Excluded = append(res.Excluded, ExcludedEntry{
					Path:   excludedPath,
					RuleID: r.ID,
					Reason: r.Reason,
				})
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		name := PayloadPrefix + relProj

		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return sink.Dir(name, info.Mode().Perm())
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := sink.Symlink(name, info.Mode().Perm(), target); err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(target))
			entries = append(entries, InventoryEntry{
				Path:          name,
				Size:          int64(len(target)),
				SHA256:        hex.EncodeToString(sum[:]),
				Mode:          fmt.Sprintf("%04o", info.Mode().Perm()),
				SymlinkTarget: target,
			})
			res.PayloadFiles++
			res.PayloadBytes += int64(len(target))
			return nil
		case d.Type().IsRegular():
			// Read fully so the bytes that are scanned for secret
			// candidates are exactly the bytes that land in the zip.
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			res.Findings = append(res.Findings, scanContent(relProj, data)...)
			if err := sink.File(name, info.Mode().Perm(), data); err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			entries = append(entries, InventoryEntry{
				Path:   name,
				Size:   int64(len(data)),
				SHA256: hex.EncodeToString(sum[:]),
				Mode:   fmt.Sprintf("%04o", info.Mode().Perm()),
			})
			res.PayloadFiles++
			res.PayloadBytes += int64(len(data))
			return nil
		default:
			return fmt.Errorf("%s is neither a regular file, directory, nor symlink (%s); remove it or hand-pack", path, info.Mode().Type())
		}
	})
	if walkErr != nil {
		return nil, fmt.Errorf("handoff create: %w", walkErr)
	}

	// §12 pipeline gate: hard findings refuse creation (the caller's defer
	// removes the partial temp file) unless explicitly allowed. All hard
	// findings are listed at once so the user fixes them in one pass.
	if !opts.AllowFindings {
		var refusal strings.Builder
		for _, f := range res.Findings {
			if !f.Hard {
				continue
			}
			if refusal.Len() == 0 {
				refusal.WriteString("handoff create: refusing to pack: the secret scan found private-key material in files the exclusion policy keeps:\n")
			}
			fmt.Fprintf(&refusal, "  %s line %d (%s)\n", f.Path, f.Line, f.Pattern)
		}
		if refusal.Len() > 0 {
			refusal.WriteString("remove or exclude those files, or re-run with --allow-findings to pack them anyway")
			return nil, errors.New(refusal.String())
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	manifest, err := marshalJSON(Manifest{
		SchemaVersion:   SchemaVersion,
		CreatedAt:       modified.Format(time.RFC3339),
		AgentmodVersion: opts.Version,
		Platform:        opts.Platform,
		Git:             opts.Git,
		ForGit:          opts.ForGit,
	})
	if err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	inventory, err := marshalJSON(Inventory{Files: entries})
	if err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	redaction := renderRedaction(modified, opts.Version, res.Excluded, res.Findings)
	handoffDoc := renderHandoffDoc(modified, opts.Version, opts.Platform,
		filepath.Base(filepath.Clean(opts.ProjectRoot)), opts.ForGit, res)
	restoreDoc := renderRestoreDoc(opts.Version, opts.ForGit)

	// checksums.txt: "<sha256>  <member>" (sha256sum format) for every
	// content-bearing member — manifest, inventory, redaction report, the
	// human-readable documents, then payload in path order. checksums.txt
	// cannot list itself.
	var checksums strings.Builder
	writeSum := func(name string, data []byte) {
		sum := sha256.Sum256(data)
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	writeSum(ManifestName, manifest)
	writeSum(InventoryName, inventory)
	writeSum(RedactionName, redaction)
	writeSum(HandoffDocName, handoffDoc)
	writeSum(RestoreDocName, restoreDoc)
	for _, e := range entries {
		fmt.Fprintf(&checksums, "%s  %s\n", e.SHA256, e.Path)
	}

	for _, m := range []struct {
		name string
		data []byte
	}{
		{ManifestName, manifest},
		{InventoryName, inventory},
		{RedactionName, redaction},
		{HandoffDocName, handoffDoc},
		{RestoreDocName, restoreDoc},
		{ChecksumsName, []byte(checksums.String())},
	} {
		if err := sink.File(m.name, 0o644, m.data); err != nil {
			return nil, fmt.Errorf("handoff create: %w", err)
		}
	}
	if err := sink.Close(); err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	return res, nil
}

// marshalJSON renders v indented with a trailing newline, so members are
// readable when extracted by hand.
func marshalJSON(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
