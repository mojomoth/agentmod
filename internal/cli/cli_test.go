package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // required substring of stdout ("" = must be empty)
		wantStderr string // required substring of stderr ("" = must be empty)
	}{
		{"no args prints usage", nil, ExitOK, "Usage:", ""},
		{"version flag", []string{"--version"}, ExitOK, "agentmod " + Version, ""},
		{"version subcommand", []string{"version"}, ExitOK, "agentmod " + Version, ""},
		{"help subcommand", []string{"help"}, ExitOK, "Usage:", ""},
		{"help flag", []string{"--help"}, ExitOK, "Usage:", ""},
		{"unknown command", []string{"frobnicate"}, ExitError, "", `unknown command "frobnicate"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("Run(%v) exit code = %d, want %d", tt.args, code, tt.wantCode)
			}
			checkStream(t, "stdout", stdout.String(), tt.wantStdout)
			checkStream(t, "stderr", stderr.String(), tt.wantStderr)
		})
	}
}

func checkStream(t *testing.T, name, got, wantSubstring string) {
	t.Helper()
	if wantSubstring == "" {
		if got != "" {
			t.Errorf("%s = %q, want empty", name, got)
		}
		return
	}
	if !strings.Contains(got, wantSubstring) {
		t.Errorf("%s = %q, want substring %q", name, got, wantSubstring)
	}
}

func TestResolveVersion(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	// An ldflags-injected value always wins over the build-info fallback.
	Version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion() with ldflags = %q, want %q", got, "v9.9.9")
	}

	// With the dev sentinel and no usable build-info version (the usual case
	// under `go test`), it must fall back gracefully — never empty, never the
	// "(devel)" placeholder.
	Version = devVersion
	got := resolveVersion()
	if got == "" || got == "(devel)" {
		t.Errorf("resolveVersion() fallback = %q, want a non-empty real version", got)
	}
}

func TestExitCodeContract(t *testing.T) {
	// IMPLEMENTATION_PLAN §3 fixes these values; downstream packages and
	// shell hooks will rely on them.
	if ExitOK != 0 || ExitError != 1 || ExitNotInProject != 2 || ExitValidation != 3 {
		t.Fatalf("exit code contract broken: OK=%d Error=%d NotInProject=%d Validation=%d",
			ExitOK, ExitError, ExitNotInProject, ExitValidation)
	}
}
