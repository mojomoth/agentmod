// Read-side .amod access (FABLE_PLAN §18 "Handoff inspect / verify / list",
// §21). Open loads a snapshot's identity members — manifest, inventory, and
// the redaction report — without extracting anything to disk; Verify
// re-hashes every content-bearing member against checksums.txt and
// cross-checks the inventory. Restore (Phase 6) builds on these structures;
// per §21 it must never trust a snapshot, so Verify reports problems rather
// than stopping at the first one.

package handoff

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// requiredMembers are the root zip members every .amod must contain
// (FABLE_PLAN §21; checksums.txt is content-bearing but cannot list itself).
var requiredMembers = []string{
	ManifestName,
	InventoryName,
	ChecksumsName,
	RedactionName,
	HandoffDocName,
	RestoreDocName,
}

// Snapshot is an opened .amod file. Close releases the underlying file
// handle. Open succeeds on any structurally complete snapshot regardless of
// its schema version — the CALLER decides what to do with a newer one
// (inspect prints a warning, Verify records a problem, restore will refuse).
type Snapshot struct {
	Path        string
	Manifest    Manifest
	Inventory   Inventory
	Redaction   []byte // REDACTION.md content, verbatim
	Members     int    // total zip members, including directory entries
	PayloadDirs int    // payload directory members (empty dirs restore too)

	zr *zip.ReadCloser
}

// Open reads the snapshot at path. It fails when the file is not a zip or
// any §21-required root member is missing or unparseable; it does NOT hash
// anything — integrity is Verify's job.
func Open(path string) (*Snapshot, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("handoff: %s is not a readable .amod snapshot (%v)", path, err)
	}
	s := &Snapshot{Path: path, zr: zr}
	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		s.Members++
		if f.FileInfo().IsDir() {
			if strings.HasPrefix(f.Name, PayloadPrefix) {
				s.PayloadDirs++
			}
			continue
		}
		if _, dup := byName[f.Name]; !dup {
			byName[f.Name] = f
		}
	}
	var missing []string
	for _, name := range requiredMembers {
		if byName[name] == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		zr.Close()
		return nil, fmt.Errorf("handoff: %s is not a valid .amod snapshot: missing member(s) %s", path, strings.Join(missing, ", "))
	}

	manifest, err := readZipMember(byName[ManifestName])
	if err == nil {
		err = json.Unmarshal(manifest, &s.Manifest)
	}
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("handoff: %s: unreadable %s (%v)", path, ManifestName, err)
	}
	inventory, err := readZipMember(byName[InventoryName])
	if err == nil {
		err = json.Unmarshal(inventory, &s.Inventory)
	}
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("handoff: %s: unreadable %s (%v)", path, InventoryName, err)
	}
	if s.Redaction, err = readZipMember(byName[RedactionName]); err != nil {
		zr.Close()
		return nil, fmt.Errorf("handoff: %s: unreadable %s (%v)", path, RedactionName, err)
	}
	return s, nil
}

// Close releases the snapshot's file handle.
func (s *Snapshot) Close() error { return s.zr.Close() }

// VerifyResult is Verify's report. An empty Problems means every
// content-bearing member hashed to its checksums.txt entry and the
// inventory matches the payload exactly.
type VerifyResult struct {
	Checked  int      // content-bearing members hashed against checksums.txt
	Problems []string // human-readable integrity failures, in detection order
}

// Verify re-hashes every content-bearing member (everything except
// directory entries and checksums.txt itself) against checksums.txt, then
// cross-checks inventory.json against the payload members: presence both
// ways, size, sha256, permission mode, and that a recorded symlink target
// hashes to the recorded sha256 (the member content IS the target string,
// D034). Read failures become problems, not errors, so one bad member never
// hides the rest.
func (s *Snapshot) Verify() *VerifyResult {
	res := &VerifyResult{}
	problemf := func(format string, args ...any) {
		res.Problems = append(res.Problems, fmt.Sprintf(format, args...))
	}

	if s.Manifest.SchemaVersion > SchemaVersion {
		problemf("manifest schema_version %d is newer than this build supports (%d); upgrade agentmod", s.Manifest.SchemaVersion, SchemaVersion)
	} else if s.Manifest.SchemaVersion < 1 {
		problemf("manifest schema_version %d is not a valid version", s.Manifest.SchemaVersion)
	}

	// checksums.txt: "<64-hex>  <member>" per line (sha256sum format).
	want := map[string]string{}
	var checksums []byte
	for _, f := range s.zr.File {
		if f.Name == ChecksumsName && !f.FileInfo().IsDir() {
			data, err := readZipMember(f)
			if err != nil {
				problemf("unreadable %s (%v)", ChecksumsName, err)
			}
			checksums = data
			break
		}
	}
	for _, line := range strings.Split(string(checksums), "\n") {
		if line == "" {
			continue
		}
		hexSum, name, ok := strings.Cut(line, "  ")
		if !ok || len(hexSum) != sha256.Size*2 || name == "" {
			problemf("malformed %s line %q", ChecksumsName, line)
			continue
		}
		if _, dup := want[name]; dup {
			problemf("%s lists %s twice", ChecksumsName, name)
			continue
		}
		want[name] = hexSum
	}

	// Pass over the archive: hash every content-bearing member.
	type digest struct {
		size int64
		sum  string
		mode string
	}
	digests := map[string]digest{}
	for _, f := range s.zr.File {
		if f.FileInfo().IsDir() || f.Name == ChecksumsName {
			continue
		}
		data, err := readZipMember(f)
		if err != nil {
			problemf("unreadable member %s (%v)", f.Name, err)
			continue
		}
		sum := sha256.Sum256(data)
		hexSum := hex.EncodeToString(sum[:])
		if _, seen := digests[f.Name]; seen {
			problemf("archive contains member %s more than once", f.Name)
			continue
		}
		digests[f.Name] = digest{
			size: int64(len(data)),
			sum:  hexSum,
			mode: fmt.Sprintf("%04o", f.FileInfo().Mode().Perm()),
		}
		expected, listed := want[f.Name]
		switch {
		case !listed:
			problemf("member %s is not listed in %s", f.Name, ChecksumsName)
		case expected != hexSum:
			problemf("checksum mismatch for %s", f.Name)
			res.Checked++
		default:
			res.Checked++
		}
		delete(want, f.Name)
	}
	// Anything still in want was promised by checksums.txt but never found.
	// Report in checksums.txt line order for determinism.
	for _, line := range strings.Split(string(checksums), "\n") {
		_, name, ok := strings.Cut(line, "  ")
		if !ok {
			continue
		}
		if _, stillMissing := want[name]; stillMissing {
			problemf("%s is listed in %s but missing from the archive", name, ChecksumsName)
			delete(want, name)
		}
	}

	// Inventory ↔ payload cross-check.
	inInventory := map[string]bool{}
	for _, e := range s.Inventory.Files {
		inInventory[e.Path] = true
		d, present := digests[e.Path]
		if !present {
			problemf("inventory lists %s but the archive has no such member", e.Path)
			continue
		}
		if d.size != e.Size {
			problemf("size mismatch for %s: inventory says %d bytes, archive has %d", e.Path, e.Size, d.size)
		}
		if d.sum != e.SHA256 {
			problemf("inventory sha256 mismatch for %s", e.Path)
		}
		if d.mode != e.Mode {
			problemf("mode mismatch for %s: inventory says %s, archive has %s", e.Path, e.Mode, d.mode)
		}
		if e.SymlinkTarget != "" {
			tsum := sha256.Sum256([]byte(e.SymlinkTarget))
			if hex.EncodeToString(tsum[:]) != e.SHA256 {
				problemf("symlink target for %s does not hash to its recorded sha256", e.Path)
			}
		}
	}
	for _, f := range s.zr.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, PayloadPrefix) {
			continue
		}
		if !inInventory[f.Name] {
			problemf("payload member %s is missing from %s", f.Name, InventoryName)
		}
	}
	return res
}

// readZipMember returns the member's full content.
func readZipMember(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
