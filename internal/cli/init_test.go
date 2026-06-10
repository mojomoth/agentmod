package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentmod/agentmod/internal/config"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

func runInitForTest(t *testing.T, cwd string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run([]string{"init"}, &out, &errBuf, fakeEnv(cwd, nil))
	return code, out.String(), errBuf.String()
}

func TestInitFresh(t *testing.T) {
	root := t.TempDir()
	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	wantContains(t, "stdout", stdout, "AgentMod: initialized", "agentmod status")

	agentmodDir := filepath.Join(root, ".agentmod")
	for _, rel := range layout.Subdirs() {
		info, err := os.Stat(filepath.Join(agentmodDir, rel))
		if err != nil || !info.IsDir() {
			t.Errorf(".agentmod/%s: want directory, got err=%v", rel, err)
		}
	}

	// The written config must round-trip to exactly the defaults.
	cfg, err := config.Load(filepath.Join(agentmodDir, "agentmod.toml"))
	if err != nil {
		t.Fatalf("written agentmod.toml does not load: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("written config = %+v, want defaults %+v", cfg, config.Default())
	}

	stub, err := os.ReadFile(layout.OpencodeConfigPath(agentmodDir))
	if err != nil {
		t.Fatalf("opencode.json not written: %v", err)
	}
	if !strings.Contains(string(stub), "opencode.ai/config.json") {
		t.Errorf("opencode.json stub = %q, want $schema reference", stub)
	}

	// And the result must be a discoverable project that status reports active.
	if _, err := project.Discover(root); err != nil {
		t.Errorf("Discover after init: %v", err)
	}
	code, statusOut, _ := runStatusForTest(t, fakeEnv(root, nil))
	if code != ExitOK || !strings.Contains(statusOut, "AgentMod: active") {
		t.Errorf("status after init: exit=%d output:\n%s", code, statusOut)
	}
}

func TestInitReinitNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	agentmodDir := filepath.Join(root, ".agentmod")

	// Pre-existing user-edited files must come through byte-identical.
	customToml := []byte("schema_version = 1\n\n[claude]\nenabled = false\n")
	if err := os.MkdirAll(filepath.Join(agentmodDir, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentmodDir, "agentmod.toml"), customToml, 0o644); err != nil {
		t.Fatal(err)
	}
	customStub := []byte("{\"theme\":\"user-edited\"}\n")
	if err := os.WriteFile(layout.OpencodeConfigPath(agentmodDir), customStub, 0o644); err != nil {
		t.Fatal(err)
	}
	// A file the layout knows nothing about must survive too.
	stray := filepath.Join(agentmodDir, "claude")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	strayFile := filepath.Join(stray, "settings.json")
	if err := os.WriteFile(strayFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "already initialized", "left untouched")

	for path, want := range map[string][]byte{
		filepath.Join(agentmodDir, "agentmod.toml"): customToml,
		layout.OpencodeConfigPath(agentmodDir):      customStub,
		strayFile:                                   []byte("{}"),
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed by re-init:\ngot:  %q\nwant: %q", path, got, want)
		}
	}

	// Missing layout dirs are filled in.
	for _, rel := range layout.Subdirs() {
		if _, err := os.Stat(filepath.Join(agentmodDir, rel)); err != nil {
			t.Errorf(".agentmod/%s not created on re-init: %v", rel, err)
		}
	}
}

func TestInitAgentmodIsAFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".agentmod"), []byte("oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runInitForTest(t, root)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", stderr, "not a directory")
	got, err := os.ReadFile(filepath.Join(root, ".agentmod"))
	if err != nil || string(got) != "oops" {
		t.Errorf("file in the way modified: %q, %v", got, err)
	}
}

func TestInitNestedWarnsAndCreates(t *testing.T) {
	outer := makeProject(t, config.Default())
	inner := filepath.Join(outer, "sub", "dir")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runInitForTest(t, inner)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, ExitOK, stderr)
	}
	wantContains(t, "stdout", stdout, "already inside the agentmod project at "+outer, "shadows")
	proj, err := project.Discover(inner)
	if err != nil {
		t.Fatal(err)
	}
	if proj.Root != inner {
		t.Errorf("after nested init, Discover root = %s, want %s", proj.Root, inner)
	}
}

func TestInitAtExistingRootDoesNotWarnNested(t *testing.T) {
	root := makeProject(t, config.Default())
	code, stdout, _ := runInitForTest(t, root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if strings.Contains(stdout, "already inside") {
		t.Errorf("re-init at root warned about nesting:\n%s", stdout)
	}
	wantContains(t, "stdout", stdout, "already initialized")
}

func TestInitRejectsArguments(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"init", "--bogus"}, &out, &errBuf, fakeEnv(t.TempDir(), nil))
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	wantContains(t, "stderr", errBuf.String(), "--bogus")
}
