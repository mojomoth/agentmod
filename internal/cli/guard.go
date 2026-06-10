package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/agentmod/agentmod/internal/guard"
)

// exitGuardDeny is the PreToolUse hook protocol's blocking exit code
// (FABLE_PLAN §3.1): exit 2 with the reason on stderr, which Claude Code
// feeds back to the model. It coincides with ExitNotInProject numerically
// but belongs to the hook contract, not agentmod's exit-code table.
const exitGuardDeny = 2

// guardJSONOutput is the §3.1 alternative deny form, emitted on stdout with
// exit 0 when --json is given.
type guardJSONOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// runGuard implements `agentmod guard claude-bash [--json]` (FABLE_PLAN
// §17): read the PreToolUse JSON from stdin, decide via the pure engine in
// internal/guard, and report the verdict in whichever §3.1 deny form the
// caller selected. Allow is always silent exit 0.
func runGuard(args []string, stdout, stderr io.Writer, env Env) int {
	jsonMode := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) != 1 || rest[0] != "claude-bash" {
		fmt.Fprintf(stderr, "agentmod: usage: agentmod guard claude-bash [--json]\n")
		if len(rest) > 0 && rest[0] != "claude-bash" {
			fmt.Fprintf(stderr, "agentmod: unknown guard target %q (only claude-bash exists)\n", rest[0])
		}
		return ExitError
	}

	// A failing guard must degrade safely (§17): a stdin read error is not
	// a reason to block everything — decide on whatever bytes arrived, and
	// the engine's fail-safe path handles the rest.
	var input []byte
	if env.Stdin != nil {
		input, _ = io.ReadAll(env.Stdin)
	}
	home, _ := env.LookupEnv("HOME")

	d := guard.Decide(input, home)
	if !d.Deny {
		return ExitOK
	}
	if jsonMode {
		var out guardJSONOutput
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.PermissionDecision = "deny"
		out.HookSpecificOutput.PermissionDecisionReason = d.Reason
		data, err := json.Marshal(out)
		if err != nil {
			// Unreachable with this struct; degrade to the exit-2 form
			// rather than approving by accident.
			fmt.Fprintf(stderr, "agentmod guard: BLOCKED: %s\n", d.Reason)
			return exitGuardDeny
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return ExitOK
	}
	fmt.Fprintf(stderr, "agentmod guard: BLOCKED: %s\n", d.Reason)
	return exitGuardDeny
}
