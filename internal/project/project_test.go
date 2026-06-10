package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mkProject creates a valid .agentmod/agentmod.toml marker under root.
func mkProject(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("schema_version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverFoundInCwd(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root)

	p, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover(%q) error: %v", root, err)
	}
	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if want := filepath.Join(root, DirName); p.AgentmodDir != want {
		t.Errorf("AgentmodDir = %q, want %q", p.AgentmodDir, want)
	}
	if want := filepath.Join(root, DirName, ConfigFileName); p.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", p.ConfigPath, want)
	}
}

func TestDiscoverFoundInAncestor(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root)
	deep := filepath.Join(root, "src", "pkg", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover(%q) error: %v", deep, err)
	}
	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
}

func TestDiscoverNearestWinsWithNestedProjects(t *testing.T) {
	outer := t.TempDir()
	mkProject(t, outer)
	inner := filepath.Join(outer, "vendor", "subproj")
	mkProject(t, inner)
	below := filepath.Join(inner, "cmd")
	if err := os.MkdirAll(below, 0o755); err != nil {
		t.Fatal(err)
	}

	// From inside the inner project, the inner root wins.
	p, err := Discover(below)
	if err != nil {
		t.Fatalf("Discover(%q) error: %v", below, err)
	}
	if p.Root != inner {
		t.Errorf("Root = %q, want inner %q", p.Root, inner)
	}

	// From between the two roots, the outer project applies.
	p, err = Discover(filepath.Join(outer, "vendor"))
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if p.Root != outer {
		t.Errorf("Root = %q, want outer %q", p.Root, outer)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	// t.TempDir() ancestors (/tmp, /var/folders, …) are not agentmod
	// projects, so the walk reaches the filesystem root and stops there.
	p, err := Discover(t.TempDir())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (project: %+v)", err, p)
	}
}

func TestDiscoverStopsAtFilesystemRoot(t *testing.T) {
	// Starting at the root itself must terminate (no infinite parent walk)
	// and report not-found rather than erroring.
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Discover(%q) err = %v, want ErrNotFound", root, err)
	}
}

func TestDiscoverRelativeStartDir(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root)
	t.Chdir(root)

	p, err := Discover(".")
	if err != nil {
		t.Fatalf("Discover(\".\") error: %v", err)
	}
	// On macOS t.TempDir() can sit behind a symlink (/tmp → /private/tmp);
	// Discover is lexical, so compare against the lexical absolute path.
	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != want {
		t.Errorf("Root = %q, want %q", p.Root, want)
	}
}

func TestDiscoverMarkerMustBeRegularFile(t *testing.T) {
	// A directory named agentmod.toml is not a valid marker.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, DirName, ConfigFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for directory marker", err)
	}

	// A bare .agentmod/ without agentmod.toml does not activate either.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(bare); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for bare .agentmod dir", err)
	}
}
