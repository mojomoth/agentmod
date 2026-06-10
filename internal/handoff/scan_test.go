package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All scan fixtures use obviously-fake values (CHECKS.md §5): the patterns
// they exercise match on shape, not on the FAKE payloads.

const fakePrivateKey = "-----BEGIN FAKE PRIVATE KEY-----\nFAKE-fixture-not-a-real-key\n-----END FAKE PRIVATE KEY-----\n"

func TestScanContentPatternTable(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		want     string // pattern ID, "" = clean
		wantLine int
		wantHard bool
	}{
		{"openssh private key", "comment\n" + fakePrivateKey, "private-key", 2, true},
		{"rsa private key", "-----BEGIN RSA PRIVATE KEY-----\nFAKE\n", "private-key", 1, true},
		{"pkcs8 private key", "-----BEGIN PRIVATE KEY-----\nFAKE\n", "private-key", 1, true},
		{"public key block is clean", "-----BEGIN PUBLIC KEY-----\nFAKE\n", "", 0, false},
		{"aws access key id", "key=AKIAFAKEFAKEFAKEFAKE\n", "aws-access-key-id", 1, false},
		{"aws id too short", "AKIA123\n", "", 0, false},
		{"github token", "x\nx\nghp_FAKEfixture0000000000000\n", "github-token", 3, false},
		{"sk token", "anthropic: sk-FAKEfixturevalue000000\n", "sk-token", 1, false},
		{"short sk value is clean", "sk-FAKE-fixture\n", "", 0, false},
		{"api_key assignment", "# config\napi_key = \"FAKE-fixture\"\n", "api-key", 2, false},
		{"apiKey json", "{\"apikey\": \"FAKE-fixture\"}\n", "api-key", 1, false},
		{"api key prose is clean", "The API key feature rocks.\n", "", 0, false},
		{"auth_token assignment", "{\"auth_token\": \"FAKE-fixture\"}\n", "token", 1, false},
		{"bare token mention is clean", "the tokenizer splits tokens: 5 of them\n", "", 0, false},
		{"secret assignment", "client_secret: FAKE-fixture\n", "secret", 1, false},
		{"secretary is clean", "the secretary's notes\n", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scanContent(".agentmod/x", []byte(tc.content))
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("findings = %v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("findings = %v, want exactly one (%s)", got, tc.want)
			}
			f := got[0]
			if f.Pattern != tc.want || f.Line != tc.wantLine || f.Hard != tc.wantHard || f.Path != ".agentmod/x" {
				t.Errorf("finding = %+v, want pattern %s line %d hard %v", f, tc.want, tc.wantLine, tc.wantHard)
			}
		})
	}
}

func TestScanContentMultiplePatternsOnePerPattern(t *testing.T) {
	content := "api_key = \"FAKE-1\"\napi_key = \"FAKE-2\"\n" + fakePrivateKey
	got := scanContent(".agentmod/multi", []byte(content))
	if len(got) != 2 {
		t.Fatalf("findings = %v, want 2 (one per pattern, first match only)", got)
	}
	// Pattern order, not file order: private-key (hard) is checked first.
	if got[0].Pattern != "private-key" || got[0].Line != 3 || !got[0].Hard {
		t.Errorf("findings[0] = %+v, want private-key line 3 hard", got[0])
	}
	if got[1].Pattern != "api-key" || got[1].Line != 1 || got[1].Hard {
		t.Errorf("findings[1] = %+v, want api-key line 1 (first of the two matches)", got[1])
	}
}

func TestCreateRedactionMemberOnCleanFixture(t *testing.T) {
	// The hostile fixture's KEPT files contain no secret candidates, so the
	// report renders the full exclusion list and an explicitly clean scan.
	root := mkHostileFixture(t)
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want none on the hostile fixture's kept files", res.Findings)
	}

	zr := openSnapshot(t, output)
	report := string(readMember(t, zr, RedactionName))
	for _, want := range []string{
		"# Redaction report",
		"agentmod test-version",
		"## Excluded from the payload",
		"- `.agentmod/claude/.credentials.json` — auth-file:",
		"agent authentication material is never packed",
		"- `.agentmod/snapshots/` — snapshots-output:",
		"## Secret-candidate scan",
		"No secret candidates were found in the packed files.",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("REDACTION.md missing %q\nreport:\n%s", want, report)
		}
	}
	// Excluded fixture content must not leak into the report beyond paths.
	if strings.Contains(report, "sk-FAKE-fixture") {
		t.Errorf("REDACTION.md reproduces excluded file content:\n%s", report)
	}
}

func TestCreateWarnFindingListedNotBlocking(t *testing.T) {
	root := mkFixtureProject(t)
	notes := filepath.Join(root, ".agentmod", "claude", "notes.md")
	if err := os.WriteFile(notes, []byte("# notes\napi_key = \"FAKE-fixture-value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)

	want := ScanFinding{Path: ".agentmod/claude/notes.md", Pattern: "api-key", Line: 2, Hard: false}
	if len(res.Findings) != 1 || res.Findings[0] != want {
		t.Fatalf("Findings = %v, want [%+v]", res.Findings, want)
	}

	zr := openSnapshot(t, output)
	report := string(readMember(t, zr, RedactionName))
	if !strings.Contains(report, "- `.agentmod/claude/notes.md` line 2 — api-key") {
		t.Errorf("REDACTION.md missing the warn finding:\n%s", report)
	}
	// The matched value never appears in the report.
	if strings.Contains(report, "FAKE-fixture-value") {
		t.Errorf("REDACTION.md reproduces the matched secret value:\n%s", report)
	}
	// Warn findings do not exclude the file: it is packed as-is.
	readMember(t, zr, "payload/.agentmod/claude/notes.md")
}

func TestCreateHardFindingRefused(t *testing.T) {
	root := mkFixtureProject(t)
	key := filepath.Join(root, ".agentmod", "codex", "deploy-key")
	if err := os.WriteFile(key, []byte(fakePrivateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	_, err := Create(CreateOptions{
		ProjectRoot: root,
		OutputPath:  filepath.Join(outDir, "snap.amod"),
		CreatedAt:   testNow,
	})
	if err == nil {
		t.Fatal("Create succeeded despite private-key material in a kept file")
	}
	for _, want := range []string{
		"refusing to pack",
		".agentmod/codex/deploy-key line 1 (private-key)",
		"--allow-findings",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q missing %q", err, want)
		}
	}
	entries, derr := os.ReadDir(outDir)
	if derr != nil {
		t.Fatal(derr)
	}
	if len(entries) != 0 {
		t.Errorf("output dir not clean after refusal: %d entries", len(entries))
	}
}

func TestCreateAllowFindingsPacksHardFinding(t *testing.T) {
	root := mkFixtureProject(t)
	key := filepath.Join(root, ".agentmod", "codex", "deploy-key")
	if err := os.WriteFile(key, []byte(fakePrivateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mkOpts := func(out string) CreateOptions {
		return CreateOptions{
			ProjectRoot:   root,
			OutputPath:    out,
			CreatedAt:     testNow,
			Version:       "test-version",
			AllowFindings: true,
		}
	}
	out1 := filepath.Join(dir, "a.amod")
	res, err := Create(mkOpts(out1))
	if err != nil {
		t.Fatal(err)
	}
	want := ScanFinding{Path: ".agentmod/codex/deploy-key", Pattern: "private-key", Line: 1, Hard: true}
	if len(res.Findings) != 1 || res.Findings[0] != want {
		t.Fatalf("Findings = %v, want [%+v]", res.Findings, want)
	}

	zr := openSnapshot(t, out1)
	readMember(t, zr, "payload/.agentmod/codex/deploy-key") // packed, not dropped
	report := string(readMember(t, zr, RedactionName))
	if !strings.Contains(report, "- `.agentmod/codex/deploy-key` line 1 — private-key (HARD finding; packed because --allow-findings was given)") {
		t.Errorf("REDACTION.md does not mark the allowed hard finding:\n%s", report)
	}

	// Findings change neither determinism nor atomicity.
	out2 := filepath.Join(dir, "b.amod")
	if _, err := Create(mkOpts(out2)); err != nil {
		t.Fatal(err)
	}
	d1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if string(d1) != string(d2) {
		t.Errorf("two creates with identical inputs differ (%d vs %d bytes)", len(d1), len(d2))
	}
}

func TestCreateExcludedFilesNotScanned(t *testing.T) {
	// §12 pipeline order is collect → filter → scan: private-key material
	// inside a policy-excluded file must not block creation or appear as a
	// finding — the file never reaches the scanner.
	root := mkFixtureProject(t)
	envFile := filepath.Join(root, ".agentmod", "claude", ".env")
	if err := os.WriteFile(envFile, []byte(fakePrivateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	res := createForTest(t, root, output)
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want none (the only candidate lives in an excluded file)", res.Findings)
	}
	found := false
	for _, e := range res.Excluded {
		if e.Path == ".agentmod/claude/.env" && e.RuleID == "env-file" {
			found = true
		}
	}
	if !found {
		t.Errorf("Excluded = %v, missing the .env exclusion", res.Excluded)
	}
}
