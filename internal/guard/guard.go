// Package guard implements the decision engine for `agentmod guard
// claude-bash`, the PreToolUse hook that stops Claude Code Bash commands
// from polluting global agent homes (FABLE_PLAN §17). The engine is the
// pure function Decide so it is table-testable; stdin reading and exit-code
// plumbing live in internal/cli.
//
// Verified hook contract (FABLE_PLAN §3.1): stdin carries JSON with
// tool_name and tool_input.command; a hook blocks by exiting 2 (stderr is
// fed back to Claude) or by emitting a permissionDecision "deny" JSON.
package guard

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Decision is the verdict on one hook invocation. An empty Decision allows.
type Decision struct {
	Deny   bool
	Reason string
}

// hookInput is the subset of the §3.1 PreToolUse stdin contract the guard
// needs. Unknown fields are ignored by encoding/json, so contract additions
// stay compatible.
type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// protectedHomes are the global agent homes the guard defends, relative to
// the user home (FABLE_PLAN §17, IMPLEMENTATION_PLAN §11). Credential dirs
// like ~/.ssh are the dev-harness guard's concern, not the product's.
const protectedHomes = `\.claude|\.codex|\.config/opencode|\.local/share/opencode`

// homePrefixes are the spellings of "the user home" recognized without
// knowing $HOME: tilde, $HOME, ${HOME}, and the standard macOS/Linux
// absolute layouts. A non-standard literal $HOME value is appended by
// globalPathPattern.
const homePrefixes = `~|\$HOME\b|\$\{HOME\}|/Users/[A-Za-z0-9._-]+|/home/[A-Za-z0-9._-]+`

// pathEnd terminates a protected-home match so ~/.claudette never counts.
const pathEnd = `(/|["'\s=:]|$)`

var (
	// Command-position write-likelihood. (?m) so a command on any line of a
	// multiline Bash block is seen; space alone is NOT a boundary, so
	// "scp" never matches "cp".
	writeCmdRe = regexp.MustCompile(`(?m)(^|[;&|]|\$\()\s*(cp|mv|rm|mkdir|rmdir|touch|tee|ln|rsync|install|unzip|chmod|chown|truncate|dd)(\s|$)`)
	gitCloneRe = regexp.MustCompile(`git\s+clone(\s|$)`)
	sudoRe     = regexp.MustCompile(`(?m)(^|[;&|]|\$\()\s*sudo(\s|$)`)
	// HOME reassignment: HOME= at a word boundary (start, separator, or
	// whitespace), optionally via export. CODEX_HOME=… must not match.
	homeAssignRe = regexp.MustCompile(`(?m)(^|[;&|(]|\s)(export\s+)?HOME=\S`)
)

// globalPathPattern returns the regexp matching any reference to a protected
// global agent home. home, when non-empty, is added literally so custom
// $HOME locations (outside /Users, /home) are still covered.
func globalPathPattern(home string) *regexp.Regexp {
	prefixes := homePrefixes
	if home != "" && home != "/" {
		prefixes += `|` + regexp.QuoteMeta(strings.TrimRight(home, "/"))
	}
	return regexp.MustCompile(`(` + prefixes + `)/(` + protectedHomes + `)` + pathEnd)
}

// redirectToGlobalPattern matches output redirection whose TARGET is a
// protected home (IMPLEMENTATION_PLAN §11: redirection elsewhere — even
// elsewhere under $HOME — is not the guard's business).
func redirectToGlobalPattern(home string) *regexp.Regexp {
	prefixes := homePrefixes
	if home != "" && home != "/" {
		prefixes += `|` + regexp.QuoteMeta(strings.TrimRight(home, "/"))
	}
	return regexp.MustCompile(`>>?\s*["']?(` + prefixes + `)/(` + protectedHomes + `)` + pathEnd)
}

const remedy = "agent state for this project is routed into .agentmod/; use the routed project-local paths instead of the global agent homes (~/.claude, ~/.codex, ~/.config/opencode, ~/.local/share/opencode)"

// Decide inspects one PreToolUse stdin payload and returns the guard's
// verdict. home is the user's home directory ("" when unknown).
//
// Fail-safe contract (FABLE_PLAN §17): unparseable input denies ONLY when
// the raw bytes reference a protected global home; it never blocks
// everything. Reads are never blanket-blocked — only command-position
// write-likelihood, git clone, and redirection targeting a protected home.
func Decide(input []byte, home string) Decision {
	globalRe := globalPathPattern(home)

	var in hookInput
	if err := json.Unmarshal(input, &in); err != nil || in.ToolName == "" {
		if globalRe.Match(input) {
			return Decision{Deny: true, Reason: "unparseable hook input references a global agent home; refusing to risk it. " + remedy}
		}
		return Decision{}
	}
	if in.ToolName != "Bash" {
		return Decision{}
	}
	cmd := in.ToolInput.Command
	if cmd == "" {
		return Decision{}
	}

	if sudoRe.MatchString(cmd) {
		return Decision{Deny: true, Reason: "sudo is blocked inside an agentmod project; run privileged commands outside the agent"}
	}
	if homeAssignRe.MatchString(cmd) {
		return Decision{Deny: true, Reason: "reassigning HOME is blocked inside an agentmod project; agentmod isolates agent homes without changing HOME"}
	}
	if globalRe.MatchString(cmd) {
		if writeCmdRe.MatchString(cmd) {
			return Decision{Deny: true, Reason: "write-like command targets a global agent home. " + remedy}
		}
		if gitCloneRe.MatchString(cmd) {
			return Decision{Deny: true, Reason: "git clone into a global agent home is blocked. " + remedy}
		}
		if redirectToGlobalPattern(home).MatchString(cmd) {
			return Decision{Deny: true, Reason: "output redirection targets a global agent home. " + remedy}
		}
	}
	return Decision{}
}
