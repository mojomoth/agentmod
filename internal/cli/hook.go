package cli

import (
	"fmt"
	"io"

	"github.com/agentmod/agentmod/internal/shellhook"
)

// runHook implements `agentmod hook <shell>`: it prints the shell hook script
// to stdout for the rc file to eval (IMPLEMENTATION_PLAN §7). Editing rc
// files is init's job; this command only emits the script, so users can also
// wire it manually.
func runHook(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "agentmod: hook requires exactly one shell argument (supported: zsh, bash)\n")
		return ExitError
	}
	switch args[0] {
	case "zsh":
		io.WriteString(stdout, shellhook.Zsh())
		return ExitOK
	case "bash":
		io.WriteString(stdout, shellhook.Bash())
		return ExitOK
	default:
		fmt.Fprintf(stderr, "agentmod: unsupported shell %q for hook (supported: zsh, bash)\n", args[0])
		return ExitError
	}
}
