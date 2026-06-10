// Package handoff implements .amod snapshot creation (FABLE_PLAN §18/§21,
// IMPLEMENTATION_PLAN §12). A .amod file is a zip whose members are
// manifest.json, inventory.json, checksums.txt, and the payload tree under
// payload/ with forward-slash project-root-relative names
// (payload/.agentmod/...), so restore maps members back onto the project
// root directly. Inspect/verify/restore consume the same structures.
package handoff

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

// SchemaVersion is the .amod manifest schema this build writes and the
// newest restore will accept.
const SchemaVersion = 1

// Member names at the zip root.
const (
	ManifestName  = "manifest.json"
	InventoryName = "inventory.json"
	ChecksumsName = "checksums.txt"
	PayloadPrefix = "payload/"
)

// Manifest is manifest.json. Later slices extend it (git state metadata,
// policy flags); restore must tolerate the absence of those future fields
// in schema-version-1 snapshots.
type Manifest struct {
	SchemaVersion   int    `json:"schema_version"`
	CreatedAt       string `json:"created_at"` // RFC3339, UTC
	AgentmodVersion string `json:"agentmod_version"`
	Platform        string `json:"platform"` // "<GOOS>/<GOARCH>"
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
}

// Result reports what Create wrote.
type Result struct {
	OutputPath   string
	PayloadFiles int   // non-directory payload members
	PayloadBytes int64 // total content bytes of those members
}

// Create packs the project's .agentmod/ directory into a .amod snapshot.
//
// Scope (this slice): the payload is everything under .agentmod/ except
// .agentmod/snapshots — that exclusion is structural, not policy: snapshots
// is the default OUTPUT directory, so packing it would nest prior snapshots
// (and, mid-write, the partially-written one) inside the new one. The
// policy exclusion engine (auth files, caches, .env, …) is a separate
// slice that filters further.
//
// The output file never exists in a partial state: Create writes a dot-
// prefixed temp file in the output directory and renames it over
// OutputPath only after the zip is complete; any error removes the temp.
func Create(opts CreateOptions) (*Result, error) {
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

	res, err := writeSnapshot(tmp, agentmodDir, tmp.Name(), opts)
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

// writeSnapshot streams the zip into w: payload members first (hashing as
// it copies), then inventory/manifest/checksums derived from those hashes.
// tmpName is the in-progress file's own path, skipped if the walk meets it.
func writeSnapshot(w io.Writer, agentmodDir, tmpName string, opts CreateOptions) (*Result, error) {
	zw := zip.NewWriter(w)
	modified := opts.CreatedAt.UTC()
	res := &Result{}
	var entries []InventoryEntry

	skipAbs := map[string]bool{}
	for _, p := range []string{tmpName, opts.OutputPath} {
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
			return filepath.SkipDir
		}
		if abs, aerr := filepath.Abs(path); aerr == nil && skipAbs[abs] {
			return nil
		}
		rel, err := filepath.Rel(agentmodDir, path)
		if err != nil {
			return err
		}
		name := PayloadPrefix + project.DirName
		if rel != "." {
			name += "/" + filepath.ToSlash(rel)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			hdr := &zip.FileHeader{Name: name + "/", Method: zip.Store, Modified: modified}
			hdr.SetMode(fs.ModeDir | info.Mode().Perm())
			_, err := zw.CreateHeader(hdr)
			return err
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: modified}
			hdr.SetMode(fs.ModeSymlink | info.Mode().Perm())
			mw, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(target))
			if _, err := mw.Write([]byte(target)); err != nil {
				return err
			}
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
			hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified}
			hdr.SetMode(info.Mode().Perm())
			mw, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			h := sha256.New()
			n, err := io.Copy(io.MultiWriter(mw, h), f)
			f.Close()
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			entries = append(entries, InventoryEntry{
				Path:   name,
				Size:   n,
				SHA256: hex.EncodeToString(h.Sum(nil)),
				Mode:   fmt.Sprintf("%04o", info.Mode().Perm()),
			})
			res.PayloadFiles++
			res.PayloadBytes += n
			return nil
		default:
			return fmt.Errorf("%s is neither a regular file, directory, nor symlink (%s); remove it or hand-pack", path, info.Mode().Type())
		}
	})
	if walkErr != nil {
		return nil, fmt.Errorf("handoff create: %w", walkErr)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	manifest, err := marshalJSON(Manifest{
		SchemaVersion:   SchemaVersion,
		CreatedAt:       modified.Format(time.RFC3339),
		AgentmodVersion: opts.Version,
		Platform:        opts.Platform,
	})
	if err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}
	inventory, err := marshalJSON(Inventory{Files: entries})
	if err != nil {
		return nil, fmt.Errorf("handoff create: %w", err)
	}

	// checksums.txt: "<sha256>  <member>" (sha256sum format) for every
	// content-bearing member — manifest, inventory, then payload in path
	// order. checksums.txt cannot list itself.
	var checksums strings.Builder
	writeSum := func(name string, data []byte) {
		sum := sha256.Sum256(data)
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	writeSum(ManifestName, manifest)
	writeSum(InventoryName, inventory)
	for _, e := range entries {
		fmt.Fprintf(&checksums, "%s  %s\n", e.SHA256, e.Path)
	}

	for _, m := range []struct {
		name string
		data []byte
	}{
		{ManifestName, manifest},
		{InventoryName, inventory},
		{ChecksumsName, []byte(checksums.String())},
	} {
		hdr := &zip.FileHeader{Name: m.name, Method: zip.Deflate, Modified: modified}
		hdr.SetMode(0o644)
		mw, err := zw.CreateHeader(hdr)
		if err != nil {
			return nil, fmt.Errorf("handoff create: %w", err)
		}
		if _, err := mw.Write(m.data); err != nil {
			return nil, fmt.Errorf("handoff create: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
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
