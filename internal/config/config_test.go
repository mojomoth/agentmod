package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestDefaults pins every mandatory default of FABLE_PLAN §13.
func TestDefaults(t *testing.T) {
	d := Default()

	if d.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", d.SchemaVersion)
	}
	if d.Mode != "standard" {
		t.Errorf("Mode = %q, want %q", d.Mode, "standard")
	}
	if d.Isolation.ChangeHome {
		t.Error("Isolation.ChangeHome = true, must default to false")
	}
	if !d.Isolation.BlockGlobalWrites {
		t.Error("Isolation.BlockGlobalWrites = false, must default to true")
	}
	if !d.Claude.Enabled {
		t.Error("Claude.Enabled = false, must default to true")
	}
	if !d.Claude.BashGuard {
		t.Error("Claude.BashGuard = false, must default to true")
	}
	if !d.Codex.Enabled {
		t.Error("Codex.Enabled = false, must default to true")
	}
	if !d.OpenCode.Enabled {
		t.Error("OpenCode.Enabled = false, must default to true")
	}
	if d.OpenCode.XDGFullIsolation {
		t.Error("OpenCode.XDGFullIsolation = true, XDG routing must be opt-in (default false)")
	}
	if !d.Node.Enabled {
		t.Error("Node.Enabled = false, must default to true")
	}
	if !d.Gstack.AutoDoctorCheck {
		t.Error("Gstack.AutoDoctorCheck = false, must default to true")
	}
	if !d.Snapshot.ExcludeSource {
		t.Error("Snapshot.ExcludeSource = false, must default to true")
	}
	if !d.Snapshot.ExcludeSecrets {
		t.Error("Snapshot.ExcludeSecrets = false, must default to true")
	}
	if d.Handoff.Git.IncludeSessions {
		t.Error("Handoff.Git.IncludeSessions = true, must default to false")
	}
	if d.Handoff.Git.IncludeLogs {
		t.Error("Handoff.Git.IncludeLogs = true, must default to false")
	}

	if err := d.Validate(); err != nil {
		t.Errorf("Default() must validate, got: %v", err)
	}
}

// TestParseEmptyDocumentYieldsDefaults: a file that sets nothing keeps every
// default.
func TestParseEmptyDocumentYieldsDefaults(t *testing.T) {
	got, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(empty) error: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("Parse(empty) = %+v, want Default() %+v", got, Default())
	}
}

// TestParsePartialOverride: setting one key leaves all others at default.
func TestParsePartialOverride(t *testing.T) {
	got, err := Parse([]byte("[opencode]\nxdg_full_isolation = true\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !got.OpenCode.XDGFullIsolation {
		t.Error("xdg_full_isolation = true not applied")
	}
	want := Default()
	want.OpenCode.XDGFullIsolation = true
	if !reflect.DeepEqual(got, want) {
		t.Errorf("partial override disturbed other fields:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestParseDisableAgent(t *testing.T) {
	got, err := Parse([]byte("[codex]\nenabled = false\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if got.Codex.Enabled {
		t.Error("codex.enabled = false not applied")
	}
	if !got.Claude.Enabled || !got.OpenCode.Enabled {
		t.Error("disabling codex disturbed other agents")
	}
}

func TestParseRejectsUnknownSchemaVersion(t *testing.T) {
	for _, version := range []string{"0", "2", "99"} {
		_, err := Parse([]byte("schema_version = " + version + "\n"))
		if !errors.Is(err, ErrSchemaVersion) {
			t.Errorf("schema_version = %s: err = %v, want ErrSchemaVersion", version, err)
		}
	}
}

func TestParseRejectsChangeHome(t *testing.T) {
	_, err := Parse([]byte("[isolation]\nchange_home = true\n"))
	if !errors.Is(err, ErrChangeHome) {
		t.Errorf("change_home = true: err = %v, want ErrChangeHome", err)
	}
}

func TestParseRejectsIncludeSessions(t *testing.T) {
	_, err := Parse([]byte("[handoff.git]\ninclude_sessions = true\n"))
	if !errors.Is(err, ErrSessionsNeedEncryption) {
		t.Errorf("include_sessions = true: err = %v, want ErrSessionsNeedEncryption", err)
	}
	if err == nil || !strings.Contains(err.Error(), "encryption") {
		t.Errorf("error must explain the encryption requirement, got: %v", err)
	}
}

func TestParseRejectsUnknownMode(t *testing.T) {
	_, err := Parse([]byte("mode = \"yolo\"\n"))
	if err == nil || !strings.Contains(err.Error(), "yolo") {
		t.Errorf("mode = \"yolo\": err = %v, want unknown-mode error naming it", err)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	for _, doc := range []string{
		"[claud]\nenabled = true\n",         // misspelled table
		"[claude]\nbash_gaurd = false\n",    // misspelled key
		"[isolation]\nchnage_home = true\n", // misspelled hard-policy key
	} {
		_, err := Parse([]byte(doc))
		if err == nil || !strings.Contains(err.Error(), "unknown key") {
			t.Errorf("Parse(%q): err = %v, want unknown-key error", doc, err)
		}
	}
}

func TestParseRejectsMalformedTOML(t *testing.T) {
	_, err := Parse([]byte("schema_version = = 1\n"))
	if err == nil {
		t.Error("malformed TOML must error")
	}
}

func TestParseRejectsWrongType(t *testing.T) {
	_, err := Parse([]byte("[claude]\nenabled = \"yes\"\n"))
	if err == nil {
		t.Error("string for bool field must error")
	}
}

// TestRoundTrip: Marshal(Default()) parses back to exactly Default(), and a
// modified valid config survives the same trip.
func TestRoundTrip(t *testing.T) {
	cases := map[string]Config{
		"default":  Default(),
		"modified": func() Config { c := Default(); c.OpenCode.XDGFullIsolation = true; c.Node.Enabled = false; return c }(),
	}
	for name, in := range cases {
		data, err := Marshal(in)
		if err != nil {
			t.Fatalf("%s: Marshal error: %v", name, err)
		}
		out, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: Parse(Marshal()) error: %v\ndocument:\n%s", name, err, data)
		}
		if !reflect.DeepEqual(out, in) {
			t.Errorf("%s: round-trip mismatch:\nin   %+v\nout  %+v\ndocument:\n%s", name, in, out, data)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentmod.toml")
	if err := os.WriteFile(path, []byte("schema_version = 1\n[node]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.Node.Enabled {
		t.Error("node.enabled = false not applied via Load")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "agentmod.toml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load(missing) err = %v, want os.ErrNotExist", err)
	}
}

// TestLoadErrorNamesFile: parse/validation failures must identify the file.
func TestLoadErrorNamesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentmod.toml")
	if err := os.WriteFile(path, []byte("[isolation]\nchange_home = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrChangeHome) {
		t.Fatalf("Load err = %v, want ErrChangeHome", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Load error must name the file, got: %v", err)
	}
}
