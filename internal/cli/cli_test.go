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

func TestExitCodeContract(t *testing.T) {
	// IMPLEMENTATION_PLAN §3 fixes these values; downstream packages and
	// shell hooks will rely on them.
	if ExitOK != 0 || ExitError != 1 || ExitNotInProject != 2 || ExitValidation != 3 {
		t.Fatalf("exit code contract broken: OK=%d Error=%d NotInProject=%d Validation=%d",
			ExitOK, ExitError, ExitNotInProject, ExitValidation)
	}
}
