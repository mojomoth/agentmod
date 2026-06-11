package handoff

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planFixture creates a snapshot from the standard fixture project and
// returns its path; tests tamper with it via rewriteSnapshot/addZipMember.
func planFixture(t *testing.T) string {
	t.Helper()
	root := mkFixtureProject(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	createForTest(t, root, output)
	return output
}

// openForPlan opens the snapshot at path, failing the test on error.
func openForPlan(t *testing.T, path string) *Snapshot {
	t.Helper()
	snap, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { snap.Close() })
	return snap
}

// addZipMember copies the zip at src to dst and appends one member with an
// explicit mode — the shapes rewriteSnapshot's fixed-mode extras cannot
// produce (hostile symlinks, irregular files, setuid bits).
func addZipMember(t *testing.T, src, dst, name string, data []byte, mode fs.FileMode) {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		hdr := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: testNow}
		hdr.SetMode(f.FileInfo().Mode())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if !f.FileInfo().IsDir() {
			content, err := readZipMember(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(content); err != nil {
				t.Fatal(err)
			}
		}
	}
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: testNow}
	hdr.SetMode(mode)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// wantNoPlan asserts that PlanRestore refused (nil plan, >=1 problem
// containing want).
func wantNoPlan(t *testing.T, snap *Snapshot, want string) {
	t.Helper()
	plan, problems := snap.PlanRestore()
	if plan != nil {
		t.Errorf("PlanRestore returned a plan despite problems %v", problems)
	}
	if len(problems) == 0 {
		t.Fatalf("PlanRestore reported no problems, want one containing %q", want)
	}
	wantProblem(t, problems, want)
}

func TestPlanRestoreCleanSnapshot(t *testing.T) {
	snap := openForPlan(t, planFixture(t))
	plan, problems := snap.PlanRestore()
	if len(problems) != 0 {
		t.Fatalf("clean snapshot reported problems: %v", problems)
	}

	wantDirs := []string{
		".agentmod", ".agentmod/claude", ".agentmod/codex",
		".agentmod/logs", ".agentmod/node", ".agentmod/opencode",
	}
	if got := relPaths(plan.Dirs); !slicesEqualPlan(got, wantDirs) {
		t.Errorf("plan dirs = %v, want %v", got, wantDirs)
	}
	wantFiles := []string{
		".agentmod/agentmod.toml", ".agentmod/claude/run.sh",
		".agentmod/claude/settings.json", ".agentmod/opencode/opencode.json",
	}
	if got := relPaths(plan.Files); !slicesEqualPlan(got, wantFiles) {
		t.Errorf("plan files = %v, want %v", got, wantFiles)
	}
	if len(plan.Links) != 1 || plan.Links[0].RelPath != ".agentmod/claude/link.toml" {
		t.Fatalf("plan links = %+v, want exactly .agentmod/claude/link.toml", plan.Links)
	}
	if plan.Links[0].Target != "../agentmod.toml" {
		t.Errorf("link target = %q, want ../agentmod.toml", plan.Links[0].Target)
	}
	for _, f := range plan.Files {
		want := fs.FileMode(0o644)
		if f.RelPath == ".agentmod/claude/run.sh" {
			want = 0o755
		}
		if f.Mode != want {
			t.Errorf("%s mode = %v, want %v", f.RelPath, f.Mode, want)
		}
		if f.ZipName != PayloadPrefix+f.RelPath {
			t.Errorf("%s zip name = %q, want payload-prefixed rel path", f.RelPath, f.ZipName)
		}
	}
}

func TestPlanRestoreZipSlipNames(t *testing.T) {
	src := planFixture(t)
	for _, tc := range []struct {
		name   string
		member string
		want   string
	}{
		{"dotdot inside payload", "payload/.agentmod/../../evil.txt", "escapes the project root (zip-slip)"},
		{"dotdot at payload root", "payload/../evil.txt", "escapes the project root (zip-slip)"},
		{"bare dotdot", "payload/..", "escapes the project root (zip-slip)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tampered := filepath.Join(t.TempDir(), "tampered.amod")
			rewriteSnapshot(t, src, tampered, nil, map[string][]byte{tc.member: []byte("x")}, false)
			wantNoPlan(t, openForPlan(t, tampered), tc.want)
		})
	}
}

func TestPlanRestoreAbsoluteAndMalformedNames(t *testing.T) {
	src := planFixture(t)
	for _, tc := range []struct {
		name   string
		member string
		want   string
	}{
		{"absolute path", "payload//etc/passwd", "would restore to an absolute path"},
		{"windows drive", "payload/C:/evil.txt", "uses a Windows drive path"},
		{"backslash", `payload/.agentmod\evil.txt`, "contains a backslash"},
		{"non-canonical double slash", "payload/.agentmod//double.txt", "not a canonical relative path"},
		{"non-canonical dot segment", "payload/.agentmod/./x.txt", "not a canonical relative path"},
		{"outside whitelist", "payload/src/main.go", "would restore outside .agentmod/"},
		{"root smuggle", "../evil.txt", "unexpected member ../evil.txt outside payload/"},
		{"unexpected root file", "evil.sh", "unexpected member evil.sh outside payload/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tampered := filepath.Join(t.TempDir(), "tampered.amod")
			rewriteSnapshot(t, src, tampered, nil, map[string][]byte{tc.member: []byte("x")}, false)
			wantNoPlan(t, openForPlan(t, tampered), tc.want)
		})
	}
}

func TestPlanRestoreProtectedElements(t *testing.T) {
	src := planFixture(t)
	for _, tc := range []struct {
		member string
		elem   string
	}{
		{"payload/.agentmod/.git/hooks/pre-commit", ".git"},
		{"payload/.agentmod/.ssh/id_rsa", ".ssh"},
		{"payload/.agentmod/claude/.aws/credentials", ".aws"},
		{"payload/.agentmod/.docker/config.json", ".docker"},
	} {
		t.Run(tc.elem, func(t *testing.T) {
			tampered := filepath.Join(t.TempDir(), "tampered.amod")
			rewriteSnapshot(t, src, tampered, nil, map[string][]byte{tc.member: []byte("x")}, false)
			wantNoPlan(t, openForPlan(t, tampered),
				"contains the protected path element \""+tc.elem+"\"")
		})
	}
}

func TestPlanRestoreHostileSymlinkTargets(t *testing.T) {
	src := planFixture(t)
	link := PayloadPrefix + ".agentmod/claude/link.toml"
	for _, tc := range []struct {
		name   string
		target string
		want   string
	}{
		{"absolute", "/etc/passwd", "has an absolute target (/etc/passwd)"},
		{"escape via dotdot", "../../outside.txt", "target escapes .agentmod/ (resolves to outside.txt)"},
		{"escape to sibling", "../../../other/.ssh/id_rsa", "target escapes .agentmod/"},
		{"empty", "", "has an empty target"},
		{"backslash", `..\agentmod.toml`, "target contains a backslash"},
		{"windows drive", "C:/evil", "has an absolute target (C:/evil)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tampered := filepath.Join(t.TempDir(), "tampered.amod")
			rewriteSnapshot(t, src, tampered, map[string]func([]byte) []byte{
				link: func([]byte) []byte { return []byte(tc.target) },
			}, nil, true) // fixChecksums: the hostile target is the ONLY inconsistency
			wantNoPlan(t, openForPlan(t, tampered), tc.want)
		})
	}
}

func TestPlanRestoreSymlinkChainStaysConfined(t *testing.T) {
	// A link whose target resolves inside .agentmod/ is fine even when it
	// points at another link (lexical containment is inductive).
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	addZipMember(t, src, tampered, PayloadPrefix+".agentmod/link2",
		[]byte("claude/link.toml"), fs.ModeSymlink|0o777)
	plan, problems := openForPlan(t, tampered).PlanRestore()
	if len(problems) != 0 {
		t.Fatalf("confined symlink chain reported problems: %v", problems)
	}
	if len(plan.Links) != 2 {
		t.Errorf("plan links = %+v, want 2 entries", plan.Links)
	}
}

func TestPlanRestoreNewerSchemaRefused(t *testing.T) {
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	rewriteSnapshot(t, src, tampered, map[string]func([]byte) []byte{
		ManifestName: func(data []byte) []byte {
			return []byte(strings.Replace(string(data), `"schema_version": 1`, `"schema_version": 99`, 1))
		},
	}, nil, true)
	snap := openForPlan(t, tampered)
	plan, problems := snap.PlanRestore()
	if plan != nil {
		t.Error("PlanRestore returned a plan for a newer-schema snapshot")
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly the schema-version one", problems)
	}
	wantProblem(t, problems, "schema_version 99 is newer than this build supports")
}

func TestPlanRestoreDuplicatePayloadPath(t *testing.T) {
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	dup := PayloadPrefix + ".agentmod/agentmod.toml"
	addZipMember(t, src, tampered, dup, []byte("schema_version = 9\n"), 0o644)
	wantNoPlan(t, openForPlan(t, tampered),
		"archive contains payload path "+dup+" more than once")
}

func TestPlanRestoreIrregularMember(t *testing.T) {
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	addZipMember(t, src, tampered, PayloadPrefix+".agentmod/pipe",
		nil, fs.ModeNamedPipe|0o644)
	wantNoPlan(t, openForPlan(t, tampered),
		"neither a regular file, directory, nor symlink")
}

func TestPlanRestoreStripsSetuid(t *testing.T) {
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	victim := PayloadPrefix + ".agentmod/claude/sneaky.sh"
	addZipMember(t, src, tampered, victim, []byte("#!/bin/sh\n"), fs.ModeSetuid|0o755)
	plan, problems := openForPlan(t, tampered).PlanRestore()
	if len(problems) != 0 {
		t.Fatalf("setuid member reported problems %v; the bit should be stripped, not refused", problems)
	}
	for _, f := range plan.Files {
		if f.ZipName != victim {
			continue
		}
		if f.Mode != 0o755 {
			t.Errorf("planned mode = %v, want 0755 (setuid stripped)", f.Mode)
		}
		return
	}
	t.Errorf("plan files missing %s: %+v", victim, plan.Files)
}

func TestPlanRestoreOversizedSymlinkTarget(t *testing.T) {
	src := planFixture(t)
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	addZipMember(t, src, tampered, PayloadPrefix+".agentmod/biglink",
		[]byte(strings.Repeat("a/", maxSymlinkTarget)), fs.ModeSymlink|0o777)
	wantNoPlan(t, openForPlan(t, tampered), "target is implausibly large")
}

func relPaths(entries []PlanEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.RelPath
	}
	return out
}

func slicesEqualPlan(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
