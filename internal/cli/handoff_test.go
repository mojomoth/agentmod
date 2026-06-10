package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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
		{"restore not implemented", []string{"restore"}, "handoff restore is not implemented yet"},
		{"inspect not implemented", []string{"inspect"}, "handoff inspect is not implemented yet"},
		{"verify not implemented", []string{"verify"}, "handoff verify is not implemented yet"},
		{"list not implemented", []string{"list"}, "handoff list is not implemented yet"},
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
