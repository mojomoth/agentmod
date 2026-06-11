package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// createGitForTest runs CreateForGit with the fixed test clock/identity.
func createGitForTest(t *testing.T, root string) *Result {
	t.Helper()
	res, err := CreateForGit(CreateOptions{
		ProjectRoot: root,
		CreatedAt:   testNow,
		Version:     "test-version",
		Platform:    "testos/testarch",
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// rootEntries lists the project root's entry names, so tests can prove the
// package build leaves no partial/old temp directories behind.
func rootEntries(t *testing.T, root string) []string {
	t.Helper()
	des, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, de := range des {
		names = append(names, de.Name())
	}
	sort.Strings(names)
	return names
}

func TestCreateForGitTreeMembersAndPayload(t *testing.T) {
	root := mkFixtureProject(t)
	res := createGitForTest(t, root)

	target := filepath.Join(root, GitDirName)
	if res.OutputPath != target {
		t.Errorf("OutputPath = %q, want %q", res.OutputPath, target)
	}
	for _, name := range []string{ManifestName, InventoryName, ChecksumsName, RedactionName, HandoffDocName, RestoreDocName} {
		fi, err := os.Lstat(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("root member %s: %v", name, err)
		}
		if !fi.Mode().IsRegular() {
			t.Errorf("root member %s is not a regular file (%s)", name, fi.Mode())
		}
	}

	var m Manifest
	if err := json.Unmarshal(mustRead(t, filepath.Join(target, ManifestName)), &m); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if !m.ForGit {
		t.Error("manifest for_git = false, want true")
	}
	if m.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %d, want %d", m.SchemaVersion, SchemaVersion)
	}
	if m.CreatedAt != testNow.Format(time.RFC3339) {
		t.Errorf("created_at = %q, want %q", m.CreatedAt, testNow.Format(time.RFC3339))
	}

	// Payload spot checks: verbatim file content, preserved exec bit,
	// symlink stored as a symlink, structural snapshots/ exclusion, and an
	// empty directory traveling as a directory.
	toml := mustRead(t, filepath.Join(target, "payload", ".agentmod", "agentmod.toml"))
	if string(toml) != "schema_version = 1\n" {
		t.Errorf("payload agentmod.toml = %q", toml)
	}
	if fi, err := os.Stat(filepath.Join(target, "payload", ".agentmod", "claude", "run.sh")); err != nil {
		t.Errorf("payload run.sh: %v", err)
	} else if fi.Mode().Perm() != 0o755 {
		t.Errorf("payload run.sh mode = %v, want 0755", fi.Mode().Perm())
	}
	link := filepath.Join(target, "payload", ".agentmod", "claude", "link.toml")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("payload link.toml: err=%v mode=%v, want symlink", err, fi.Mode())
	} else if got, _ := os.Readlink(link); got != "../agentmod.toml" {
		t.Errorf("payload link.toml target = %q, want ../agentmod.toml", got)
	}
	if _, err := os.Lstat(filepath.Join(target, "payload", ".agentmod", "snapshots")); !os.IsNotExist(err) {
		t.Errorf("payload snapshots/ should be structurally excluded (err=%v)", err)
	}
	if fi, err := os.Stat(filepath.Join(target, "payload", ".agentmod", "codex")); err != nil || !fi.IsDir() {
		t.Errorf("payload codex/ empty dir: err=%v", err)
	}

	// Every checksums.txt line must hash the on-disk member correctly —
	// the tree must be verifiable by hand with `shasum -a 256 -c`.
	lines := strings.Split(strings.TrimRight(string(mustRead(t, filepath.Join(target, ChecksumsName))), "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("checksums.txt is empty")
	}
	for _, line := range lines {
		hexSum, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("malformed checksums line %q", line)
		}
		p := filepath.Join(target, filepath.FromSlash(name))
		var data []byte
		if fi, err := os.Lstat(p); err != nil {
			t.Errorf("checksums names %s but it is missing: %v", name, err)
			continue
		} else if fi.Mode()&fs.ModeSymlink != 0 {
			// A symlink member's content is its target string (D034).
			tgt, err := os.Readlink(p)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(tgt)
		} else {
			data = mustRead(t, p)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != hexSum {
			t.Errorf("checksum mismatch for %s", name)
		}
	}

	// Inventory entries must all exist in the tree.
	var inv Inventory
	if err := json.Unmarshal(mustRead(t, filepath.Join(target, InventoryName)), &inv); err != nil {
		t.Fatalf("inventory.json: %v", err)
	}
	if len(inv.Files) == 0 {
		t.Fatal("inventory is empty")
	}
	for _, e := range inv.Files {
		if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(e.Path))); err != nil {
			t.Errorf("inventory lists %s but it is missing: %v", e.Path, err)
		}
	}

	if got, want := rootEntries(t, root), []string{".agentmod", GitDirName}; !slicesEqualGit(got, want) {
		t.Errorf("project root entries = %v, want %v (no partial/old leftovers)", got, want)
	}
}

func slicesEqualGit(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCreateForGitManifestFlagOnlyInTreePackage(t *testing.T) {
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	zr := openSnapshot(t, output)
	if strings.Contains(string(readMember(t, zr, ManifestName)), "for_git") {
		t.Error(".amod manifest must not carry the for_git key")
	}

	createGitForTest(t, root)
	tree := mustRead(t, filepath.Join(root, GitDirName, ManifestName))
	if !strings.Contains(string(tree), `"for_git": true`) {
		t.Errorf("tree manifest missing for_git marker:\n%s", tree)
	}
}

func TestCreateForGitDeterministicAndReplacesPrevious(t *testing.T) {
	root := mkFixtureProject(t)
	target := filepath.Join(root, GitDirName)

	createGitForTest(t, root)
	first := digestTree(t, target)

	// A stale entry from a previous package must not survive a re-create.
	if err := os.WriteFile(filepath.Join(target, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	createGitForTest(t, root)
	second := digestTree(t, target)
	if diffs := diffDigests(first, second); len(diffs) != 0 {
		t.Errorf("re-created package differs from the first (stale entries kept or output nondeterministic):\n  %s", strings.Join(diffs, "\n  "))
	}
	if got, want := rootEntries(t, root), []string{".agentmod", GitDirName}; !slicesEqualGit(got, want) {
		t.Errorf("project root entries = %v, want %v (no partial/old leftovers)", got, want)
	}
}

func TestCreateForGitRefusesForeignDirectory(t *testing.T) {
	root := mkFixtureProject(t)
	target := filepath.Join(root, GitDirName)
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "user-notes.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := CreateForGit(CreateOptions{ProjectRoot: root, CreatedAt: testNow})
	if err == nil || !strings.Contains(err.Error(), "does not look like an agentmod git handoff package") {
		t.Fatalf("err = %v, want foreign-directory refusal", err)
	}
	if got := mustRead(t, filepath.Join(target, "user-notes.md")); string(got) != "mine" {
		t.Errorf("user file changed: %q", got)
	}
	if got, want := rootEntries(t, root), []string{".agentmod", GitDirName}; !slicesEqualGit(got, want) {
		t.Errorf("project root entries = %v, want %v", got, want)
	}
}

func TestCreateForGitRefusesNonDirectoryTarget(t *testing.T) {
	root := mkFixtureProject(t)
	target := filepath.Join(root, GitDirName)
	if err := os.WriteFile(target, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CreateForGit(CreateOptions{ProjectRoot: root, CreatedAt: testNow})
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("err = %v, want non-directory refusal", err)
	}
	if got := mustRead(t, target); string(got) != "not a dir" {
		t.Errorf("user file changed: %q", got)
	}
}

func TestCreateForGitHardFindingRefusesAndKeepsPrevious(t *testing.T) {
	root := mkFixtureProject(t)
	target := filepath.Join(root, GitDirName)
	createGitForTest(t, root)
	before := digestTree(t, target)

	keyFile := filepath.Join(root, ".agentmod", "claude", "notes.txt")
	if err := os.WriteFile(keyFile, []byte(fakePrivateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := CreateForGit(CreateOptions{ProjectRoot: root, CreatedAt: testNow})
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("err = %v, want private-key refusal", err)
	}
	if diffs := diffDigests(before, digestTree(t, target)); len(diffs) != 0 {
		t.Errorf("failed re-create changed the previous package:\n  %s", strings.Join(diffs, "\n  "))
	}
	if got, want := rootEntries(t, root), []string{".agentmod", GitDirName}; !slicesEqualGit(got, want) {
		t.Errorf("project root entries = %v, want %v (no partial leftovers)", got, want)
	}
}

func TestCreateForGitHardFindingFreshRunLeavesNothing(t *testing.T) {
	root := mkFixtureProject(t)
	keyFile := filepath.Join(root, ".agentmod", "claude", "notes.txt")
	if err := os.WriteFile(keyFile, []byte(fakePrivateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateForGit(CreateOptions{ProjectRoot: root, CreatedAt: testNow}); err == nil {
		t.Fatal("want private-key refusal")
	}
	if got, want := rootEntries(t, root), []string{".agentmod"}; !slicesEqualGit(got, want) {
		t.Errorf("project root entries = %v, want %v (no package, no leftovers)", got, want)
	}
}

func TestCreateForGitDocsDescribeTreePackage(t *testing.T) {
	root := mkFixtureProject(t)
	createGitForTest(t, root)
	target := filepath.Join(root, GitDirName)

	handoffDoc := string(mustRead(t, filepath.Join(target, HandoffDocName)))
	for _, want := range []string{
		"git-storable handoff package",
		"`" + GitDirName + "/`",
		"commit",
		"deliberately not gitignored",
	} {
		if !strings.Contains(handoffDoc, want) {
			t.Errorf("git HANDOFF.md missing %q\n--- document ---\n%s", want, handoffDoc)
		}
	}
	if strings.Contains(handoffDoc, "This `.amod` snapshot packs") {
		t.Error("git HANDOFF.md still describes the .amod file format")
	}

	restoreDoc := string(mustRead(t, filepath.Join(target, RestoreDocName)))
	for _, want := range []string{
		"cannot yet restore a directory tree",
		"payload/.agentmod/",
		"nothing in a package is ever executed",
	} {
		if !strings.Contains(restoreDoc, want) {
			t.Errorf("git RESTORE.md missing %q\n--- document ---\n%s", want, restoreDoc)
		}
	}
	if strings.Contains(restoreDoc, "restore <file>.amod") {
		t.Error("git RESTORE.md still tells the user to restore a .amod file")
	}
}
