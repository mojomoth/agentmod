package guard

import (
	"encoding/json"
	"testing"
)

// bashInput builds a §3.1-shaped PreToolUse payload for a Bash command.
func bashInput(t *testing.T, command string) []byte {
	t.Helper()
	payload := map[string]any{
		"session_id":      "test-session",
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd":             "/tmp/proj",
		"permission_mode": "default",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecideBashTable(t *testing.T) {
	const home = "/Users/tester"
	cases := []struct {
		name    string
		command string
		deny    bool
	}{
		// Writes into global agent homes — blocked.
		{"rm tilde claude", `rm -rf ~/.claude/skills/foo`, true},
		{"cp dollar home claude", `cp settings.json $HOME/.claude/settings.json`, true},
		{"mv braced home codex", `mv auth.json ${HOME}/.codex/auth.json`, true},
		{"mkdir claude plugins", `mkdir -p ~/.claude/plugins/new-plugin`, true},
		{"touch opencode config", `touch ~/.config/opencode/opencode.json`, true},
		{"ln into claude skills", `ln -s /tmp/x ~/.claude/skills/x`, true},
		{"rsync into claude skills", `rsync -a ./skills/ ~/.claude/skills/`, true},
		{"tee into claude", `cat new.md | tee ~/.claude/CLAUDE.md`, true},
		{"chained write after cd", `cd /tmp && rm -rf ~/.local/share/opencode`, true},
		{"absolute macos other user", `rm -rf /Users/other/.claude/skills`, true},
		{"absolute linux home", `rm -rf /home/other/.config/opencode`, true},
		{"git clone into claude skills", `git clone https://github.com/x/y ~/.claude/skills/y`, true},
		{"append redirect into claude", `echo foo >> ~/.claude/settings.json`, true},
		{"redirect into codex absolute", `echo foo > /Users/tester/.codex/config.toml`, true},
		{"multiline second line writes", "echo safe\nrm -rf ~/.codex/sessions", true},

		// sudo and HOME reassignment — blocked regardless of paths.
		{"sudo", `sudo rm -rf /tmp/whatever`, true},
		{"sudo after chain", `true && sudo make install`, true},
		{"home assign prefix", `HOME=/tmp/fake claude doctor`, true},
		{"home assign export", `export HOME=/tmp/fake`, true},

		// Reads of global homes — never blocked (§17).
		{"ls claude skills", `ls -la ~/.claude/skills`, false},
		{"cat codex auth", `cat ~/.codex/auth.json`, false},
		{"grep opencode", `grep -r provider ~/.config/opencode`, false},
		{"diff against routed copy", `diff ~/.claude/settings.json .agentmod/claude/settings.json`, false},
		{"read redirected to project", `cat ~/.claude/settings.json > ./local-copy.json`, false},
		{"echo mentioning global path", `echo "see ~/.claude for details"`, false},

		// Project-local writes — allowed.
		{"rm node_modules", `rm -rf node_modules`, false},
		{"mkdir under agentmod", `mkdir -p .agentmod/claude/skills`, false},
		{"cp local", `cp a.txt b.txt`, false},
		{"redirect local", `echo hi > notes.txt`, false},
		{"git clone into project", `git clone https://github.com/x/y ./vendor/y`, false},

		// Near-misses that must not trip the patterns.
		{"redirect to home non-global", `echo hi > ~/notes.txt`, false},
		{"claudette is not claude", `rm -rf ~/.claudette`, false},
		{"relative my.claude dir", `rm -rf ./my.claude/dir`, false},
		{"scp is not cp", `scp file host:~/.claude/`, false},
		{"codex_home var assign", `CODEX_HOME=/tmp/x codex --version`, false},
		{"saved home var assign", `AGENTMOD_SAVED_HOME=/tmp/x true`, false},
		{"empty command", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(bashInput(t, tc.command), home)
			if d.Deny != tc.deny {
				t.Fatalf("Decide(%q).Deny = %v, want %v (reason %q)", tc.command, d.Deny, tc.deny, d.Reason)
			}
			if d.Deny && d.Reason == "" {
				t.Errorf("deny without a reason for %q", tc.command)
			}
			if !d.Deny && d.Reason != "" {
				t.Errorf("allow carries reason %q for %q", d.Reason, tc.command)
			}
		})
	}
}

func TestDecideCustomHomeLiteral(t *testing.T) {
	// A HOME outside /Users and /home is only catchable via the injected
	// literal value.
	const home = "/srv/home1"
	if d := Decide(bashInput(t, `rm -rf /srv/home1/.claude/skills`), home); !d.Deny {
		t.Error("write into literal-$HOME global home not denied")
	}
	if d := Decide(bashInput(t, `ls /srv/home1/.claude/skills`), home); d.Deny {
		t.Errorf("read of literal-$HOME global home denied: %s", d.Reason)
	}
	// Without the injected home the standard spellings still work…
	if d := Decide(bashInput(t, `rm -rf ~/.claude`), ""); !d.Deny {
		t.Error("tilde write not denied when home is unknown")
	}
	// …and "/" must not poison the pattern into matching everything.
	if d := Decide(bashInput(t, `rm -rf ./scratch`), "/"); d.Deny {
		t.Errorf("project-local rm denied under home=/: %s", d.Reason)
	}
}

func TestDecideNonBashToolAllowed(t *testing.T) {
	input := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"~/.claude/settings.json","content":"x"}}`)
	if d := Decide(input, "/Users/tester"); d.Deny {
		t.Errorf("non-Bash tool denied: %s", d.Reason)
	}
}

func TestDecideUnparseableFailSafe(t *testing.T) {
	const home = "/Users/tester"
	// Garbage referencing a global home → deny.
	if d := Decide([]byte(`%%% not json rm -rf ~/.claude/skills`), home); !d.Deny {
		t.Error("unparseable input referencing a global home not denied")
	}
	// Garbage without a global reference → allow (never block everything).
	if d := Decide([]byte(`%%% not json at all`), home); d.Deny {
		t.Errorf("unparseable input without global reference denied: %s", d.Reason)
	}
	// Empty input → allow.
	if d := Decide(nil, home); d.Deny {
		t.Errorf("empty input denied: %s", d.Reason)
	}
	// Valid JSON but missing tool_name follows the same fail-safe split.
	if d := Decide([]byte(`{"tool_input":{"command":"rm -rf ~/.codex"}}`), home); !d.Deny {
		t.Error("tool_name-less input referencing a global home not denied")
	}
	if d := Decide([]byte(`{"tool_input":{"command":"rm -rf ./x"}}`), home); d.Deny {
		t.Errorf("tool_name-less input without global reference denied: %s", d.Reason)
	}
}
