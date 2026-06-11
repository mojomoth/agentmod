package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/handoff"
)

func runHandoffForTest(t *testing.T, env Env, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(append([]string{"handoff"}, args...), &out, &errBuf, env)
	return code, out.String(), errBuf.String()
}

func TestHandoffCreateDefaultOutput(t *testing.T) {
	root := makeProject(t, config.Default())
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}

	// fakeNow is fixed, so the default name is fully deterministic.
	wantName := filepath.Base(root) + "-" + fakeNow.Format("20060102-150405") + ".amod"
	wantPath := filepath.Join(root, ".agentmod", "snapshots", wantName)
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("default snapshot missing: %v", err)
	}
	wantContains(t, "stdout", stdout,
		"Created handoff snapshot: "+wantPath,
		"payload:",
		// The default output lives in snapshots/, which Create makes before
		// the walk, so the structural exclusion is always reported here.
		"excluded by default policy: 1 entry",
		".agentmod/snapshots/ (snapshots-output)",
		"secret scan: clean (no candidate patterns in packed files)",
	)

	zr, err := zip.OpenReader(wantPath)
	if err != nil {
		t.Fatalf("snapshot is not a valid zip: %v", err)
	}
	defer zr.Close()
	var m handoff.Manifest
	for _, f := range zr.File {
		if f.Name != handoff.ManifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			t.Fatal(err)
		}
		rc.Close()
	}
	if m.SchemaVersion != handoff.SchemaVersion {
		t.Errorf("manifest schema_version = %d, want %d", m.SchemaVersion, handoff.SchemaVersion)
	}
	if m.CreatedAt != fakeNow.Format(time.RFC3339) {
		t.Errorf("manifest created_at = %q, want %q", m.CreatedAt, fakeNow.Format(time.RFC3339))
	}
	if m.AgentmodVersion != Version {
		t.Errorf("manifest agentmod_version = %q, want %q", m.AgentmodVersion, Version)
	}
	// fakeEnv leaves GOOS "", reported as "unknown".
	if !strings.HasPrefix(m.Platform, "unknown/") {
		t.Errorf("manifest platform = %q, want unknown/<arch>", m.Platform)
	}
}

func TestHandoffCreateExplicitOutput(t *testing.T) {
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "custom.amod")
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("explicit output missing: %v", err)
	}
	wantContains(t, "stdout", stdout, "Created handoff snapshot: "+output)
}

func TestHandoffCreateReportsPolicyExclusions(t *testing.T) {
	// An auth file in the routed home must be dropped by the default
	// exclusion engine AND named on stdout with its rule, so the user sees
	// what the snapshot does not carry (T20; REDACTION.md renders the same
	// list in a later slice).
	root := makeProject(t, config.Default())
	claudeDir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		".credentials.json": `{"token":"sk-FAKE-fixture"}`,
		"settings.json":     "{}\n",
	} {
		if err := os.WriteFile(filepath.Join(claudeDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	output := filepath.Join(t.TempDir(), "snap.amod")
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"excluded by default policy: 1 entry",
		".agentmod/claude/.credentials.json (auth-file)",
	)

	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	keptSettings := false
	for _, f := range zr.File {
		if strings.Contains(f.Name, ".credentials.json") {
			t.Errorf("auth file leaked into snapshot: %q", f.Name)
		}
		if f.Name == "payload/.agentmod/claude/settings.json" {
			keptSettings = true
		}
	}
	if !keptSettings {
		t.Errorf("settings.json missing from snapshot payload")
	}
}

func TestHandoffCreateHardFindingRefusedThenAllowed(t *testing.T) {
	// Private-key material in a kept file refuses creation (exit 1, remedy
	// on stderr); --allow-findings packs it and marks the finding on stdout
	// and in REDACTION.md. Fixture value is obviously fake (CHECKS.md §5).
	root := makeProject(t, config.Default())
	codexDir := filepath.Join(root, ".agentmod", "codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeKey := "-----BEGIN FAKE PRIVATE KEY-----\nFAKE-fixture-not-a-real-key\n-----END FAKE PRIVATE KEY-----\n"
	if err := os.WriteFile(filepath.Join(codexDir, "deploy-key"), []byte(fakeKey), 0o600); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "snap.amod")
	code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr,
		"refusing to pack",
		".agentmod/codex/deploy-key line 1 (private-key)",
		"--allow-findings",
	)
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("refused create left output behind: %v", err)
	}

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output, "--allow-findings")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"secret scan: 1 candidate finding (details in REDACTION.md inside the snapshot)",
		".agentmod/codex/deploy-key line 1 (private-key, HARD — packed because --allow-findings was given)",
	)
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("allowed create missing output: %v", err)
	}
}

func TestHandoffCreateWarnFindingOnStdout(t *testing.T) {
	// Warn-level candidates never block: exit 0, finding named on stdout.
	root := makeProject(t, config.Default())
	claudeDir := filepath.Join(root, ".agentmod", "claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "notes.md"), []byte("# notes\napi_key = \"FAKE-fixture-value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"secret scan: 1 candidate finding (details in REDACTION.md inside the snapshot)",
		".agentmod/claude/notes.md line 2 (api-key)",
	)
	if strings.Contains(stdout, "FAKE-fixture-value") {
		t.Errorf("stdout reproduces the matched secret value:\n%s", stdout)
	}
}

func TestHandoffCreateSecondRunSameClockRefused(t *testing.T) {
	// fakeEnv's clock never advances, so the second default name collides;
	// the writer must refuse rather than overwrite the first snapshot.
	root := makeProject(t, config.Default())
	env := fakeEnv(root, nil)
	if code, _, stderr := runHandoffForTest(t, env, "create"); code != ExitOK {
		t.Fatalf("first create failed: %s", stderr)
	}
	code, _, stderr := runHandoffForTest(t, env, "create")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "already exists")
}

func TestHandoffCreateOutsideProject(t *testing.T) {
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "create")
	if code != ExitNotInProject {
		t.Fatalf("exit = %d, want %d", code, ExitNotInProject)
	}
	wantContains(t, "stderr", stderr, "requires an agentmod project", "agentmod init")
}

func TestHandoffArgumentValidation(t *testing.T) {
	root := makeProject(t, config.Default())
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no subcommand", nil, "requires a subcommand"},
		{"unknown subcommand", []string{"frobnicate"}, `unknown handoff subcommand "frobnicate"`},
		{"unsupported flag", []string{"create", "--frobnicate"}, `unsupported argument "--frobnicate"`},
		{"output without path", []string{"create", "--output"}, "--output requires a path"},
		{"restore without path", []string{"restore"}, "handoff restore takes exactly one argument"},
		{"restore extra args", []string{"restore", "a.amod", "b.amod"}, "handoff restore takes exactly one argument"},
		{"inspect without path", []string{"inspect"}, "handoff inspect takes exactly one argument"},
		{"inspect extra args", []string{"inspect", "a.amod", "b.amod"}, "handoff inspect takes exactly one argument"},
		{"verify without path", []string{"verify"}, "handoff verify takes exactly one argument"},
		{"list with args", []string{"list", "extra"}, "handoff list takes no arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), tc.args...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			wantContains(t, "stderr", stderr, tc.want)
			// makeProject creates no snapshots dir; a rejected invocation
			// must not have created it (or anything in it) either.
			entries, err := os.ReadDir(filepath.Join(root, ".agentmod", "snapshots"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("rejected invocation created snapshot entries: %d", len(entries))
			}
		})
	}
}

// readSnapshotManifest unmarshals manifest.json out of the snapshot at path.
func readSnapshotManifest(t *testing.T, path string) handoff.Manifest {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("snapshot is not a valid zip: %v", err)
	}
	defer zr.Close()
	var m handoff.Manifest
	for _, f := range zr.File {
		if f.Name != handoff.ManifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		return m
	}
	t.Fatalf("snapshot %s has no %s", path, handoff.ManifestName)
	return m
}

func TestHandoffCreateOutsideGitRepoOmitsMetadata(t *testing.T) {
	requireGit(t) // without git the note differs ("git binary not found")
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "git: metadata omitted (not a git repository)")
	if m := readSnapshotManifest(t, output); m.Git != nil {
		t.Errorf("manifest git = %+v, want nil outside a repository", m.Git)
	}
}

func TestHandoffCreateDirtyRepoRefusedThenAllowed(t *testing.T) {
	// §20: a dirty worktree refuses creation until the user explicitly
	// consents, because uncommitted source changes do not travel in a
	// snapshot. makeProject's .agentmod/ is untracked in the fresh repo, so
	// the tree is dirty without further setup (and the branch is unborn).
	requireGit(t)
	root := makeProject(t, config.Default())
	runGitFixture(t, root, "init", "--quiet", "-b", "main")
	output := filepath.Join(t.TempDir(), "snap.amod")

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout: %s", code, ExitError, stdout)
	}
	wantContains(t, "stderr", stderr,
		"the git worktree is dirty (1 untracked)",
		"--allow-dirty",
	)
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("refused create left an output file (stat err = %v)", err)
	}

	code, stdout, stderr = runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output, "--allow-dirty")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"git: branch main @ (no commits yet), DIRTY (1 untracked) — packed anyway (--allow-dirty)",
	)
	m := readSnapshotManifest(t, output)
	if m.Git == nil {
		t.Fatal("manifest git missing")
	}
	if !m.Git.Dirty || m.Git.StatusSummary != "1 untracked" {
		t.Errorf("manifest git = %+v, want dirty with '1 untracked'", m.Git)
	}
	if m.Git.Branch != "main" || m.Git.Head != "" {
		t.Errorf("manifest git = %+v, want branch main on an unborn HEAD", m.Git)
	}
	if m.Git.SourceIncluded {
		t.Error("manifest git source_included = true; no code path can include source yet")
	}
}

func TestHandoffCreateCleanRepoRecordsGitState(t *testing.T) {
	requireGit(t)
	root := makeProject(t, config.Default())
	runGitFixture(t, root, "init", "--quiet", "-b", "main")
	// Committing everything (including .agentmod/) makes the tree clean, so
	// no consent flag is needed and the stdout line reports clean.
	runGitFixture(t, root, "add", "-A")
	runGitFixture(t, root, "commit", "--quiet", "-m", "fixture")
	runGitFixture(t, root, "remote", "add", "origin", "https://user:sk-FAKE-fixture@example.com/org/repo.git")
	output := filepath.Join(t.TempDir(), "snap.amod")

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "git: branch main @ ", ", clean")
	m := readSnapshotManifest(t, output)
	if m.Git == nil {
		t.Fatal("manifest git missing")
	}
	if m.Git.Dirty || m.Git.StatusSummary != "clean" || m.Git.Branch != "main" || m.Git.Head == "" {
		t.Errorf("manifest git = %+v, want clean main with a commit hash", m.Git)
	}
	if m.Git.RemoteURL != "https://example.com/org/repo.git" {
		t.Errorf("manifest remote = %q, want credentials stripped", m.Git.RemoteURL)
	}
}

func TestHandoffInspectOutput(t *testing.T) {
	requireGit(t) // without git the omitted-metadata note differs
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "inspect", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"Snapshot: "+output,
		"schema version: 1",
		"created:        "+fakeNow.Format(time.RFC3339)+" by agentmod "+Version,
		"git:            no metadata recorded",
		"members:",
		"payload:",
		"--- REDACTION.md ---",
		"# Redaction report",
	)
	// Inspect must not extract anything next to the snapshot.
	entries, err := os.ReadDir(filepath.Dir(output))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("inspect created files next to the snapshot: %d entries", len(entries))
	}
}

func TestHandoffInspectShowsGitState(t *testing.T) {
	requireGit(t)
	root := makeProject(t, config.Default())
	runGitFixture(t, root, "init", "--quiet", "-b", "main")
	runGitFixture(t, root, "add", "-A")
	runGitFixture(t, root, "commit", "--quiet", "-m", "fixture")
	runGitFixture(t, root, "remote", "add", "origin", "https://user:sk-FAKE-fixture@example.com/org/repo.git")
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "inspect", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "git:            branch main @ ", ", clean", ", remote https://example.com/org/repo.git")
	if strings.Contains(stdout, "sk-FAKE-fixture") {
		t.Errorf("inspect reproduced remote credentials:\n%s", stdout)
	}
}

func TestHandoffInspectMissingFile(t *testing.T) {
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "inspect", filepath.Join(t.TempDir(), "nope.amod"))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "not a readable .amod snapshot")
}

func TestHandoffVerifyOK(t *testing.T) {
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}
	// Verify works anywhere — a recipient has the file, not the project.
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "verify", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "OK: "+output, "verified ", "checksums.txt; inventory matches the payload")
}

func TestHandoffVerifyTamperedMember(t *testing.T) {
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	victim := "payload/.agentmod/agentmod.toml"
	tamperSnapshotMember(t, output, tampered, victim, []byte("schema_version = 9\n"))

	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "verify", tampered)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d", code, ExitValidation)
	}
	wantContains(t, "stderr", stderr, "FAILED", "checksum mismatch for "+victim)
}

func TestHandoffVerifyNotASnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.amod")
	if err := os.WriteFile(path, []byte("not a zip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "verify", path)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d", code, ExitValidation)
	}
	wantContains(t, "stderr", stderr, "FAILED", "not a readable .amod snapshot")
}

func TestHandoffVerifyMissingFile(t *testing.T) {
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "verify", filepath.Join(t.TempDir(), "nope.amod"))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (a typo'd path is not a validation verdict)", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "no such file")
}

func TestHandoffListEmpty(t *testing.T) {
	root := makeProject(t, config.Default())
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(root, nil), "list")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "No handoff snapshots in ", "agentmod handoff create")
}

func TestHandoffListNewestFirst(t *testing.T) {
	root := makeProject(t, config.Default())
	snapsDir := filepath.Join(root, ".agentmod", "snapshots")
	env := fakeEnv(root, nil)
	for _, name := range []string{"older.amod", "newer.amod"} {
		code, _, stderr := runHandoffForTest(t, env, "create", "--output", filepath.Join(snapsDir, name))
		if code != ExitOK {
			t.Fatalf("create %s failed: %s", name, stderr)
		}
	}
	older := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 11, 9, 30, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(snapsDir, "older.amod"), older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(snapsDir, "newer.amod"), newer, newer); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runHandoffForTest(t, env, "list")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"Handoff snapshots in "+snapsDir+" (newest first):",
		"newer.amod", "older.amod", " bytes  modified ",
	)
	if iNew, iOld := strings.Index(stdout, "newer.amod"), strings.Index(stdout, "older.amod"); iNew > iOld {
		t.Errorf("newer.amod listed after older.amod:\n%s", stdout)
	}
}

func TestHandoffListOutsideProject(t *testing.T) {
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "list")
	if code != ExitNotInProject {
		t.Fatalf("exit = %d, want %d", code, ExitNotInProject)
	}
	wantContains(t, "stderr", stderr, "requires an agentmod project", "agentmod init")
}

func TestPackAliasCreates(t *testing.T) {
	// `agentmod pack` is `agentmod handoff create` with the same flags
	// (FABLE_PLAN §11 Alias).
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "packed.amod")
	var out, errBuf bytes.Buffer
	code := run([]string{"pack", "--output", output}, &out, &errBuf, fakeEnv(root, nil))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, errBuf.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("pack produced no snapshot: %v", err)
	}
	wantContains(t, "stdout", out.String(), "Created handoff snapshot: "+output)
}

func TestHandoffRestoreRoundTrip(t *testing.T) {
	// Create in project A, restore into project B: B's previous .agentmod/
	// is backed up (deterministically named — fakeNow is fixed), the
	// payload lands, and .gitignore gains the backup pattern (D042). B gets
	// a pre-existing .gitignore because an existing file is extended even
	// outside a git repository (D014).
	src := makeProject(t, config.Default())
	claudeDir := filepath.Join(src, ".agentmod", "claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "marker.md"), []byte("travels\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(src, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}

	dst := makeProject(t, config.Default())
	if err := os.WriteFile(filepath.Join(dst, ".gitignore"), []byte(".agentmod/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldConfig, err := os.ReadFile(filepath.Join(dst, ".agentmod", "agentmod.toml"))
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runHandoffForTest(t, fakeEnv(dst, nil), "restore", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantBackup := filepath.Join(dst, ".agentmod.backup-"+fakeNow.Format("20060102-150405"))
	wantContains(t, "stdout", stdout,
		"Restored handoff snapshot: "+output,
		"extracted: ",
		"— all under "+filepath.Join(dst, ".agentmod"),
		"previous environment backed up to: "+wantBackup,
		".gitignore: added .agentmod.backup-*/",
		// The snapshot carried no claude/settings.json, so the portability
		// pass wires the guard fresh for this machine (D044).
		"Claude guard: PreToolUse Bash hook written",
		"portability: no foreign absolute paths in restored agent configs",
		"agentmod doctor",
	)
	// The marker traveled; the old environment is intact in the backup.
	data, err := os.ReadFile(filepath.Join(dst, ".agentmod", "claude", "marker.md"))
	if err != nil || string(data) != "travels\n" {
		t.Errorf("restored marker = %q, %v", data, err)
	}
	backedUp, err := os.ReadFile(filepath.Join(wantBackup, "agentmod.toml"))
	if err != nil || !bytes.Equal(backedUp, oldConfig) {
		t.Errorf("backup config mismatch (err = %v)", err)
	}
	gi, err := os.ReadFile(filepath.Join(dst, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	wantContains(t, ".gitignore", string(gi), ".agentmod/\n", ".agentmod.backup-*/\n")
}

func TestHandoffRestoreRewritesGuardAndWarnsForeignPaths(t *testing.T) {
	// The portability pass (FABLE_PLAN §18, D044): a snapshot whose
	// settings.json was wired on the SOURCE machine gets its guard hook
	// command rewritten to THIS machine's binary, and absolute paths in
	// other restored configs that do not resolve here are warned about —
	// without changing the exit code.
	src := makeProject(t, config.Default())
	claudeDir := filepath.Join(src, ".agentmod", "claude")
	codexDir := filepath.Join(src, ".agentmod", "codex")
	for _, d := range []string{claudeDir, codexDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sourceCmd := guardHookCommand("/fake/source/agentmod")
	settings, err := marshalSettings(freshGuardSettings(sourceCmd))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), settings, 0o644); err != nil {
		t.Fatal(err)
	}
	codexCfg := "[mcp_servers.docs]\ncommand = \"/missing-on-this-machine/mcp\"\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(codexCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(src, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}

	dst := makeProject(t, config.Default())
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(dst, nil), "restore", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout,
		"Claude guard: guard hook binary path updated",
		"portability: 1 absolute path in restored agent configs needs attention:",
		".agentmod/codex/config.toml: /missing-on-this-machine/mcp — does not exist on this machine",
	)
	// The restored settings.json now points at this machine's binary, not
	// the source machine's.
	restored, err := os.ReadFile(filepath.Join(dst, ".agentmod", "claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantContains(t, "settings.json", string(restored), wantGuardCommand)
	if strings.Contains(string(restored), "/fake/source/agentmod") {
		t.Errorf("restored settings.json still carries the source machine's binary path:\n%s", restored)
	}
	// The codex config itself is user-owned and untouched (warn, not rewrite).
	after, err := os.ReadFile(filepath.Join(dst, ".agentmod", "codex", "config.toml"))
	if err != nil || string(after) != codexCfg {
		t.Errorf("codex/config.toml was modified (err = %v):\n%s", err, after)
	}
}

func TestHandoffRestoreGitignoreSkippedOutsideGitRepo(t *testing.T) {
	// No .gitignore and no git repository: the backup still happens, the
	// gitignore step reports the skip instead of creating a file (D014).
	src := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(src, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}
	dst := makeProject(t, config.Default())
	code, stdout, stderr := runHandoffForTest(t, fakeEnv(dst, nil), "restore", output)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, ".gitignore: skipped (not a git repository")
	if _, err := os.Stat(filepath.Join(dst, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("restore created a .gitignore outside a git repository (err = %v)", err)
	}
}

func TestHandoffRestoreSecondSameClockRefused(t *testing.T) {
	// fakeEnv's clock never advances, so a second restore's backup name
	// collides with the first's; BackupAgentmod refuses rather than
	// overwrite (D042) and nothing is extracted.
	src := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(src, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}
	dst := makeProject(t, config.Default())
	env := fakeEnv(dst, nil)
	if code, _, stderr := runHandoffForTest(t, env, "restore", output); code != ExitOK {
		t.Fatalf("first restore failed: %s", stderr)
	}
	code, _, stderr := runHandoffForTest(t, env, "restore", output)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "already exists")
}

func TestHandoffRestoreOutsideProject(t *testing.T) {
	code, _, stderr := runHandoffForTest(t, fakeEnv(t.TempDir(), nil), "restore", "whatever.amod")
	if code != ExitNotInProject {
		t.Fatalf("exit = %d, want %d", code, ExitNotInProject)
	}
	wantContains(t, "stderr", stderr, "requires an agentmod project", "agentmod init")
}

func TestHandoffRestoreMissingFile(t *testing.T) {
	root := makeProject(t, config.Default())
	code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "restore", filepath.Join(t.TempDir(), "nope.amod"))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (a typo'd path is not a validation verdict)", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "no such file")
}

func TestHandoffRestoreGarbageRefusedBeforeBackup(t *testing.T) {
	// Open fails → exit 3, and because validation precedes any disk move
	// (D042 pipeline), the existing .agentmod/ is untouched and unbacked-up.
	root := makeProject(t, config.Default())
	garbage := filepath.Join(t.TempDir(), "garbage.amod")
	if err := os.WriteFile(garbage, []byte("not a zip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "restore", garbage)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d", code, ExitValidation)
	}
	wantContains(t, "stderr", stderr, "refusing to restore", "not a readable .amod snapshot")
	assertNoBackupOrLoss(t, root)
}

func TestHandoffRestoreTamperedRefusedBeforeBackup(t *testing.T) {
	// Verify problems → exit 3 naming the mismatch; nothing on disk moved.
	root := makeProject(t, config.Default())
	output := filepath.Join(t.TempDir(), "snap.amod")
	if code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "create", "--output", output); code != ExitOK {
		t.Fatalf("create failed: %s", stderr)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.amod")
	victim := "payload/.agentmod/agentmod.toml"
	tamperSnapshotMember(t, output, tampered, victim, []byte("schema_version = 9\n"))

	code, _, stderr := runHandoffForTest(t, fakeEnv(root, nil), "restore", tampered)
	if code != ExitValidation {
		t.Fatalf("exit = %d, want %d", code, ExitValidation)
	}
	wantContains(t, "stderr", stderr, "integrity problems", "checksum mismatch for "+victim)
	assertNoBackupOrLoss(t, root)
}

// assertNoBackupOrLoss fails if root gained a backup entry or lost its
// .agentmod/agentmod.toml — the must-hold state after any refused restore.
func assertNoBackupOrLoss(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".agentmod.backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("refused restore created backup entries: %v", matches)
	}
	if _, err := os.Stat(filepath.Join(root, ".agentmod", "agentmod.toml")); err != nil {
		t.Errorf("refused restore disturbed .agentmod: %v", err)
	}
}

func TestUnpackNotImplemented(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"unpack"}, &out, &errBuf, fakeEnv(t.TempDir(), nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "unpack", "handoff restore", "not implemented yet")
}

// tamperSnapshotMember copies the zip at src to dst, replacing the named
// member's content and leaving everything else — including checksums.txt —
// untouched, so verify must flag the mismatch.
func tamperSnapshotMember(t *testing.T, src, dst, name string, content []byte) {
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
		hdr := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified}
		hdr.SetMode(f.FileInfo().Mode())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		data := content
		if f.Name != name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffCreateRealClockWhenNowNil(t *testing.T) {
	// osEnv always sets Now, but the field is optional by contract: a nil
	// Now falls back to the real clock instead of panicking.
	root := makeProject(t, config.Default())
	env := fakeEnv(root, nil)
	env.Now = nil
	code, _, stderr := runHandoffForTest(t, env, "create")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
}
