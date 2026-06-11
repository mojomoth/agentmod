package handoff

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rewriteSnapshot copies the zip at src to dst with surgical tampering:
// mutations are applied by member name (a nil return drops the member),
// extra members are appended as regular files, and — when fixChecksums —
// every checksums.txt hash is regenerated from the post-mutation contents
// so ONLY the deliberate inconsistency remains detectable.
func rewriteSnapshot(t *testing.T, src, dst string, mutations map[string]func([]byte) []byte, extra map[string][]byte, fixChecksums bool) {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	type member struct {
		name string
		mode fs.FileMode
		dir  bool
		data []byte
	}
	var members []member
	for _, f := range zr.File {
		m := member{name: f.Name, mode: f.FileInfo().Mode(), dir: f.FileInfo().IsDir()}
		if !m.dir {
			data, err := readZipMember(f)
			if err != nil {
				t.Fatal(err)
			}
			m.data = data
		}
		if fn, ok := mutations[f.Name]; ok {
			m.data = fn(m.data)
			if m.data == nil {
				continue
			}
		}
		members = append(members, m)
	}
	for name, data := range extra {
		members = append(members, member{name: name, mode: 0o644, data: data})
	}

	if fixChecksums {
		byName := map[string][]byte{}
		for _, m := range members {
			byName[m.name] = m.data
		}
		for i, m := range members {
			if m.name != ChecksumsName {
				continue
			}
			var b strings.Builder
			for _, line := range strings.Split(string(m.data), "\n") {
				_, name, ok := strings.Cut(line, "  ")
				if !ok {
					continue
				}
				sum := sha256.Sum256(byName[name])
				fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), name)
			}
			members[i].data = []byte(b.String())
		}
	}

	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, m := range members {
		hdr := &zip.FileHeader{Name: m.name, Method: zip.Deflate, Modified: testNow}
		hdr.SetMode(m.mode)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if !m.dir {
			if _, err := w.Write(m.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenReadsSnapshot(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)

	snap, err := Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	if snap.Manifest.SchemaVersion != SchemaVersion {
		t.Errorf("schema version = %d, want %d", snap.Manifest.SchemaVersion, SchemaVersion)
	}
	if snap.Manifest.CreatedAt != testNow.Format(time.RFC3339) {
		t.Errorf("created_at = %q, want %q", snap.Manifest.CreatedAt, testNow.Format(time.RFC3339))
	}
	if snap.Manifest.AgentmodVersion != "test-version" {
		t.Errorf("agentmod_version = %q, want test-version", snap.Manifest.AgentmodVersion)
	}
	if got, want := len(snap.Inventory.Files), res.PayloadFiles; got != want {
		t.Errorf("inventory entries = %d, want %d (Create's PayloadFiles)", got, want)
	}
	if !strings.Contains(string(snap.Redaction), "# Redaction report") {
		t.Errorf("redaction content missing header:\n%s", snap.Redaction)
	}
	// Fixture payload dirs: .agentmod + claude/codex/opencode/node/logs
	// (snapshots/ is structurally excluded).
	if snap.PayloadDirs != 6 {
		t.Errorf("payload dirs = %d, want 6", snap.PayloadDirs)
	}
	// 6 dirs + payload files + 6 root members.
	if want := 6 + res.PayloadFiles + 6; snap.Members != want {
		t.Errorf("members = %d, want %d", snap.Members, want)
	}
}

func TestOpenMissingRequiredMember(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	broken := filepath.Join(t.TempDir(), "broken.amod")
	rewriteSnapshot(t, output, broken, map[string]func([]byte) []byte{
		InventoryName: func([]byte) []byte { return nil },
	}, nil, false)

	if _, err := Open(broken); err == nil {
		t.Fatal("Open succeeded on a snapshot without inventory.json")
	} else if !strings.Contains(err.Error(), "missing member(s) "+InventoryName) {
		t.Errorf("error = %v, want it to name the missing member", err)
	}
}

func TestOpenNotAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.amod")
	if err := os.WriteFile(path, []byte("this is not a zip archive\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("Open succeeded on garbage")
	} else if !strings.Contains(err.Error(), "not a readable .amod snapshot") {
		t.Errorf("error = %v, want the not-a-snapshot wording", err)
	}
}

func TestVerifyCleanSnapshot(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	created := createForTest(t, root, output)

	snap, err := Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	if len(res.Problems) != 0 {
		t.Fatalf("clean snapshot reported problems: %v", res.Problems)
	}
	// Every content-bearing member: manifest, inventory, REDACTION.md,
	// HANDOFF.md, RESTORE.md + the payload files (checksums.txt cannot
	// list itself).
	if want := 5 + created.PayloadFiles; res.Checked != want {
		t.Errorf("checked = %d, want %d", res.Checked, want)
	}
}

func TestVerifyTamperedPayload(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	victim := PayloadPrefix + ".agentmod/agentmod.toml"
	rewriteSnapshot(t, output, tampered, map[string]func([]byte) []byte{
		victim: func([]byte) []byte { return []byte("schema_version = 9\n") },
	}, nil, false)

	snap, err := Open(tampered)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	wantProblem(t, res.Problems, "checksum mismatch for "+victim)
	wantProblem(t, res.Problems, "inventory sha256 mismatch for "+victim)
}

func TestVerifyMissingMember(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	victim := PayloadPrefix + ".agentmod/claude/settings.json"
	rewriteSnapshot(t, output, tampered, map[string]func([]byte) []byte{
		victim: func([]byte) []byte { return nil },
	}, nil, false)

	snap, err := Open(tampered)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	wantProblem(t, res.Problems, victim+" is listed in "+ChecksumsName+" but missing from the archive")
	wantProblem(t, res.Problems, "inventory lists "+victim+" but the archive has no such member")
}

func TestVerifyExtraMember(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	intruder := PayloadPrefix + ".agentmod/claude/intruder.txt"
	rewriteSnapshot(t, output, tampered, nil, map[string][]byte{
		intruder: []byte("smuggled\n"),
	}, false)

	snap, err := Open(tampered)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	wantProblem(t, res.Problems, "member "+intruder+" is not listed in "+ChecksumsName)
	wantProblem(t, res.Problems, "payload member "+intruder+" is missing from "+InventoryName)
}

func TestVerifyNewerSchemaVersion(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	rewriteSnapshot(t, output, tampered, map[string]func([]byte) []byte{
		ManifestName: func(data []byte) []byte {
			return []byte(strings.Replace(string(data),
				fmt.Sprintf(`"schema_version": %d`, SchemaVersion),
				`"schema_version": 99`, 1))
		},
	}, nil, true) // fixChecksums: the schema bump is the ONLY inconsistency

	snap, err := Open(tampered)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	if len(res.Problems) != 1 {
		t.Fatalf("problems = %v, want exactly the schema-version one", res.Problems)
	}
	wantProblem(t, res.Problems, "schema_version 99 is newer than this build supports")
}

func TestVerifyMalformedChecksumsLine(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	rewriteSnapshot(t, output, tampered, map[string]func([]byte) []byte{
		ChecksumsName: func(data []byte) []byte {
			return append(data, []byte("nonsense without a separator\n")...)
		},
	}, nil, false)

	snap, err := Open(tampered)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	res := snap.Verify()
	wantProblem(t, res.Problems, "malformed "+ChecksumsName+" line")
}

// wantProblem asserts that one of the problems contains want.
func wantProblem(t *testing.T, problems []string, want string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(p, want) {
			return
		}
	}
	t.Errorf("problems missing %q; got: %v", want, problems)
}
