package handoff

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 6, 11, 12, 30, 45, 0, time.UTC)

// mkFixtureProject builds a project root with an .agentmod tree exercising
// every member kind: nested regular files, an executable, an empty
// directory, a relative symlink, and a pre-existing snapshot that must be
// excluded from the payload.
func mkFixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	am := filepath.Join(root, ".agentmod")
	for _, d := range []string{"claude", "codex", "opencode", "node", "snapshots", "logs"} {
		if err := os.MkdirAll(filepath.Join(am, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func(rel, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(am, rel), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("agentmod.toml", "schema_version = 1\n", 0o644)
	writeFile(filepath.Join("claude", "settings.json"), `{"hooks":{}}`+"\n", 0o644)
	writeFile(filepath.Join("claude", "run.sh"), "#!/bin/sh\necho hi\n", 0o755)
	writeFile(filepath.Join("opencode", "opencode.json"), "{}\n", 0o644)
	writeFile(filepath.Join("snapshots", "old.amod"), "must not be packed", 0o644)
	if err := os.Symlink("../agentmod.toml", filepath.Join(am, "claude", "link.toml")); err != nil {
		t.Fatal(err)
	}
	return root
}

func createForTest(t *testing.T, root, output string) *Result {
	t.Helper()
	res, err := Create(CreateOptions{
		ProjectRoot: root,
		OutputPath:  output,
		CreatedAt:   testNow,
		Version:     "test-version",
		Platform:    "testos/testarch",
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// readMember returns the named member's content, failing if absent.
func readMember(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	}
	t.Fatalf("member %q not in snapshot; members: %v", name, memberNames(zr))
	return nil
}

func memberNames(zr *zip.Reader) []string {
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func openSnapshot(t *testing.T, path string) *zip.Reader {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("snapshot is not a valid zip: %v", err)
	}
	return zr
}

func TestCreateMembersAndPayload(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)

	if res.OutputPath != output {
		t.Errorf("OutputPath = %q, want %q", res.OutputPath, output)
	}
	// agentmod.toml, settings.json, run.sh, opencode.json, link.toml
	if res.PayloadFiles != 5 {
		t.Errorf("PayloadFiles = %d, want 5", res.PayloadFiles)
	}

	zr := openSnapshot(t, output)
	names := memberNames(zr)
	for _, want := range []string{
		ManifestName, InventoryName, ChecksumsName, RedactionName,
		HandoffDocName, RestoreDocName,
		"payload/.agentmod/",
		"payload/.agentmod/agentmod.toml",
		"payload/.agentmod/claude/",
		"payload/.agentmod/claude/settings.json",
		"payload/.agentmod/claude/run.sh",
		"payload/.agentmod/claude/link.toml",
		"payload/.agentmod/opencode/opencode.json",
		"payload/.agentmod/logs/", // empty dir is preserved
	} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("snapshot missing member %q\nmembers: %v", want, names)
		}
	}
	for _, n := range names {
		if strings.Contains(n, "snapshots") {
			t.Errorf("snapshots dir leaked into payload: %q", n)
		}
	}

	got := readMember(t, zr, "payload/.agentmod/claude/settings.json")
	if string(got) != `{"hooks":{}}`+"\n" {
		t.Errorf("settings.json content = %q", got)
	}
}

func TestCreateManifest(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	var m Manifest
	if err := json.Unmarshal(readMember(t, zr, ManifestName), &m); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if m.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %d, want %d", m.SchemaVersion, SchemaVersion)
	}
	if m.CreatedAt != testNow.Format(time.RFC3339) {
		t.Errorf("created_at = %q, want %q", m.CreatedAt, testNow.Format(time.RFC3339))
	}
	if m.AgentmodVersion != "test-version" {
		t.Errorf("agentmod_version = %q", m.AgentmodVersion)
	}
	if m.Platform != "testos/testarch" {
		t.Errorf("platform = %q", m.Platform)
	}
}

func TestCreateInventoryMatchesPayload(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	var inv Inventory
	if err := json.Unmarshal(readMember(t, zr, InventoryName), &inv); err != nil {
		t.Fatalf("inventory.json: %v", err)
	}

	byPath := map[string]InventoryEntry{}
	for i, e := range inv.Files {
		byPath[e.Path] = e
		if i > 0 && inv.Files[i-1].Path >= e.Path {
			t.Errorf("inventory not sorted: %q before %q", inv.Files[i-1].Path, e.Path)
		}
	}

	// Every non-directory payload member appears with matching size+sha256.
	payloadMembers := 0
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, PayloadPrefix) || strings.HasSuffix(f.Name, "/") {
			continue
		}
		payloadMembers++
		e, ok := byPath[f.Name]
		if !ok {
			t.Errorf("payload member %q missing from inventory", f.Name)
			continue
		}
		content := readMember(t, zr, f.Name)
		if e.Size != int64(len(content)) {
			t.Errorf("%s: inventory size %d, member size %d", f.Name, e.Size, len(content))
		}
		sum := sha256.Sum256(content)
		if e.SHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("%s: inventory sha256 mismatch", f.Name)
		}
	}
	if payloadMembers != len(inv.Files) {
		t.Errorf("inventory lists %d files, zip has %d payload members", len(inv.Files), payloadMembers)
	}

	exe := byPath["payload/.agentmod/claude/run.sh"]
	if exe.Mode != "0755" {
		t.Errorf("run.sh mode = %q, want %q", exe.Mode, "0755")
	}
	link := byPath["payload/.agentmod/claude/link.toml"]
	if link.SymlinkTarget != "../agentmod.toml" {
		t.Errorf("link.toml symlink_target = %q, want %q", link.SymlinkTarget, "../agentmod.toml")
	}
	if string(readMember(t, zr, link.Path)) != "../agentmod.toml" {
		t.Errorf("symlink member content is not its target")
	}
	plain := byPath["payload/.agentmod/agentmod.toml"]
	if plain.SymlinkTarget != "" {
		t.Errorf("regular file has symlink_target %q", plain.SymlinkTarget)
	}
}

func TestCreateExecBitInZipMode(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	for _, f := range zr.File {
		switch f.Name {
		case "payload/.agentmod/claude/run.sh":
			if f.Mode().Perm() != 0o755 {
				t.Errorf("run.sh zip mode = %v, want 0755", f.Mode().Perm())
			}
		case "payload/.agentmod/claude/link.toml":
			if f.Mode()&os.ModeSymlink == 0 {
				t.Errorf("link.toml zip mode %v lacks symlink bit", f.Mode())
			}
		}
	}
}

func TestCreateChecksumsCoverAllContentMembers(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)

	zr := openSnapshot(t, output)
	lines := strings.Split(strings.TrimRight(string(readMember(t, zr, ChecksumsName)), "\n"), "\n")
	sums := map[string]string{}
	for _, line := range lines {
		sum, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("malformed checksums line %q", line)
		}
		sums[name] = sum
	}

	// Re-hash every content-bearing member and compare; checksums.txt
	// itself is the only one not listed.
	covered := 0
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") || f.Name == ChecksumsName {
			continue
		}
		covered++
		want, ok := sums[f.Name]
		if !ok {
			t.Errorf("checksums.txt missing %q", f.Name)
			continue
		}
		sum := sha256.Sum256(readMember(t, zr, f.Name))
		if want != hex.EncodeToString(sum[:]) {
			t.Errorf("checksums.txt wrong for %q", f.Name)
		}
	}
	if covered != len(sums) {
		t.Errorf("checksums.txt lists %d members, zip has %d content members", len(sums), covered)
	}
}

func TestCreateDeterministic(t *testing.T) {
	root := mkFixtureProject(t)
	dir := t.TempDir()
	out1 := filepath.Join(dir, "a.amod")
	out2 := filepath.Join(dir, "b.amod")
	createForTest(t, root, out1)
	createForTest(t, root, out2)
	d1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d1, d2) {
		t.Errorf("two creates with identical inputs differ (%d vs %d bytes)", len(d1), len(d2))
	}
}

func TestCreateRefusesExistingOutput(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	if err := os.WriteFile(output, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Create(CreateOptions{ProjectRoot: root, OutputPath: output, CreatedAt: testNow})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already-exists", err)
	}
	got, rerr := os.ReadFile(output)
	if rerr != nil || string(got) != "precious" {
		t.Errorf("existing output was modified: %q, %v", got, rerr)
	}
}

func TestCreateDefaultOutputInsideSnapshotsDirNotSelfPacked(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(root, ".agentmod", "snapshots", "new.amod")
	createForTest(t, root, output)
	zr := openSnapshot(t, output)
	for _, n := range memberNames(zr) {
		if strings.Contains(n, ".amod") {
			t.Errorf("snapshot contains a snapshot: %q", n)
		}
	}
}

func TestCreateMissingAgentmodDir(t *testing.T) {
	root := t.TempDir()
	_, err := Create(CreateOptions{ProjectRoot: root, OutputPath: filepath.Join(t.TempDir(), "x.amod"), CreatedAt: testNow})
	if err == nil || !strings.Contains(err.Error(), ".agentmod") {
		t.Fatalf("err = %v, want missing .agentmod", err)
	}
}

func TestCreateUnreadableFileLeavesNoPartialOutput(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; chmod 0 does not block reads")
	}
	root := mkFixtureProject(t)
	blocked := filepath.Join(root, ".agentmod", "claude", "settings.json")
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o644) })

	outDir := t.TempDir()
	output := filepath.Join(outDir, "snap.amod")
	_, err := Create(CreateOptions{ProjectRoot: root, OutputPath: output, CreatedAt: testNow})
	if err == nil {
		t.Fatal("Create succeeded with an unreadable payload file")
	}
	if !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("err %v does not name the unreadable file", err)
	}
	entries, derr := os.ReadDir(outDir)
	if derr != nil {
		t.Fatal(derr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("output dir not clean after failure: %v", names)
	}
}

func TestCreateIrregularFileRefused(t *testing.T) {
	root := mkFixtureProject(t)
	fifo := filepath.Join(root, ".agentmod", "codex", "pipe")
	if err := mkfifo(fifo); err != nil {
		t.Skipf("cannot create fifo on this platform: %v", err)
	}
	outDir := t.TempDir()
	_, err := Create(CreateOptions{ProjectRoot: root, OutputPath: filepath.Join(outDir, "x.amod"), CreatedAt: testNow})
	if err == nil {
		t.Fatal("Create succeeded with a fifo in the payload")
	}
	if !strings.Contains(err.Error(), "pipe") || !strings.Contains(err.Error(), "neither a regular file") {
		t.Errorf("err = %v, want refusal naming the fifo", err)
	}
	entries, derr := os.ReadDir(outDir)
	if derr != nil {
		t.Fatal(derr)
	}
	if len(entries) != 0 {
		t.Errorf("output dir not clean after refusal: %d entries", len(entries))
	}
}

// mkfifo shells out to the POSIX mkfifo utility so this file needs no
// syscall/unix dependency; non-unix platforms simply skip the test.
func mkfifo(path string) error {
	mkfifoBin, err := exec.LookPath("mkfifo")
	if err != nil {
		return fmt.Errorf("mkfifo not available: %w", err)
	}
	out, err := exec.Command(mkfifoBin, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkfifo: %v: %s", err, out)
	}
	return nil
}
