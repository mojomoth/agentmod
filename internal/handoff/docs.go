// HANDOFF.md / RESTORE.md rendering (IMPLEMENTATION_PLAN §12: the two
// human-readable members — "what this is, how to restore, what's missing"
// and "step-by-step restore + re-login guidance"). Both are root zip
// members next to manifest.json so a recipient can read them without
// restoring, and both render deterministically from create-time data, so
// identical snapshots stay byte-identical.

package handoff

import (
	"fmt"
	"strings"
	"time"
)

// Zip member names of the two human-readable documents.
const (
	HandoffDocName = "HANDOFF.md"
	RestoreDocName = "RESTORE.md"
)

// Canonical re-login instructions (§12). doctor's auth findings and init's
// copy-on-consent flow print the same strings (internal/cli aliases these),
// and RESTORE.md embeds them, so the wording cannot drift between the live
// tool and the document that travels with a snapshot.
const (
	ClaudeReloginRemedy = "claude may ask you to log in here; complete it once by running 'claude' inside this project"
	CodexReloginRemedy  = "re-login needed: run 'codex login' inside this project"
)

// renderHandoffDoc produces the HANDOFF.md member: what the snapshot is,
// how to restore it, and what is deliberately missing from it. forGit
// switches the wording from the .amod file format to the git-storable tree
// under .agentmod-handoff/ (D047).
func renderHandoffDoc(createdAt time.Time, version, platform, projectName string, forGit bool, res *Result) []byte {
	var b strings.Builder
	b.WriteString("# Agent environment handoff\n\n")
	what := "This `.amod` snapshot packs"
	if forGit {
		what = "This git-storable handoff package (a plain-file tree under\n`" + GitDirName + "/`) packs"
	}
	fmt.Fprintf(&b, "%s the per-project agent environment of\n`%s` — the `.agentmod/` tree holding project-local Claude Code,\nCodex CLI, and OpenCode configuration, skills, sessions, and working\ncontext. Created %s by agentmod %s on %s.\n",
		what, projectName, createdAt.UTC().Format(time.RFC3339), version, platform)

	b.WriteString("\n## What is in it\n\n")
	fmt.Fprintf(&b, "- %s (%d bytes) under `payload/`, named by project-root-relative path.\n",
		countNoun(res.PayloadFiles, "file", "files"), res.PayloadBytes)
	b.WriteString("- `inventory.json` lists every payload file with size, mode, and\n  sha256; `checksums.txt` (sha256sum format) covers every\n  content-bearing member.\n")

	b.WriteString("\n## How to restore\n\n")
	if forGit {
		b.WriteString("This package travels with the repository itself: commit\n`" + GitDirName + "/` (it is deliberately not gitignored) and the\nrecipient receives it by pulling. `RESTORE.md` next to this document has\nthe restore steps, including re-login guidance.\n")
	} else {
		b.WriteString("Run `agentmod handoff restore <this file>` inside the target project;\n`RESTORE.md` (packed next to this document) has the full steps,\nincluding re-login guidance.\n")
	}

	b.WriteString("\n## What is missing\n\n")
	if len(res.Excluded) == 0 {
		b.WriteString("- Nothing was excluded by the redaction policy.\n")
	} else {
		fmt.Fprintf(&b, "- %s excluded by the redaction policy (auth files, `.env` files,\n  ssh/cloud credentials, caches, `.git`, `node_modules`, tmp). The full\n  list with per-entry reasons is in `REDACTION.md`.\n",
			countNoun(len(res.Excluded), "entry was", "entries were"))
	}
	if len(res.Findings) == 0 {
		b.WriteString("- Secret scan: clean (no candidate patterns in packed files).\n")
	} else {
		fmt.Fprintf(&b, "- Secret scan: %s in packed files — review `REDACTION.md`\n  before sharing this snapshot.\n",
			countNoun(len(res.Findings), "candidate finding", "candidate findings"))
	}
	b.WriteString("- Auth and credentials never travel; every agent needs a fresh login\n  on the target machine (see `RESTORE.md`).\n")
	b.WriteString("- A gstack install travels without its `.git` directory (excluded);\n  run `agentmod install gstack --force` after restoring.\n")
	b.WriteString("- npm global-tool symlinks under `.agentmod/node/bin` may dangle\n  because `lib/node_modules` is excluded; reinstall those tools after\n  restoring.\n")
	return []byte(b.String())
}

// renderRestoreDoc produces the RESTORE.md member: step-by-step restore
// instructions plus the re-login and reinstall guidance the target machine
// needs once the snapshot is unpacked. forGit swaps step 3 for the tree
// package's honest state: the build that writes a tree cannot yet restore
// one (the directory reader is a later Phase 7 item, D047 — the D034
// honesty precedent).
func renderRestoreDoc(version string, forGit bool) []byte {
	var b strings.Builder
	b.WriteString("# Restoring this snapshot\n\n")
	fmt.Fprintf(&b, "Written by agentmod %s at create time. `agentmod handoff restore`\nfollows these steps; the re-login and reinstall guidance below applies\nafter any restore.\n", version)

	b.WriteString("\n## Steps\n\n")
	b.WriteString("1. Install agentmod on the target machine, plus the coding agents you\n   use (claude, codex, opencode).\n")
	b.WriteString("2. Enter (or create) the target project directory and run\n   `agentmod init` — it builds the `.agentmod/` layout, wires the\n   Claude guard, edits `.gitignore`, and installs the shell hook.\n")
	if forGit {
		b.WriteString("3. Restore the package. NOTE: the agentmod build that wrote this\n   package cannot yet restore a directory tree — check whether your\n   build's `agentmod handoff restore` accepts `" + GitDirName + "`;\n   otherwise copy what you need from `payload/.agentmod/` into the\n   project's `.agentmod/` by hand (every file is stored verbatim, and\n   nothing in a package is ever executed).\n")
	} else {
		b.WriteString("3. From the project root run `agentmod handoff restore <file>.amod`.\n   Restore verifies the schema version and checksums, backs up any\n   existing `.agentmod/` first, and extracts only under `.agentmod/` —\n   it never executes anything from the snapshot.\n")
	}
	b.WriteString("4. Run `agentmod doctor` and fix whatever it reports.\n")

	b.WriteString("\n## Re-login (auth never travels)\n\n")
	b.WriteString("Auth files, credentials, and tokens are excluded from snapshots, so\neach agent needs a fresh login in the restored project:\n\n")
	fmt.Fprintf(&b, "- Claude Code: %s.\n", ClaudeReloginRemedy)
	fmt.Fprintf(&b, "- Codex CLI: %s.\n", CodexReloginRemedy)
	b.WriteString("- OpenCode: log in to your provider again if it asks.\n")
	b.WriteString("- macOS: Claude Code keeps auth in the user Keychain, which agentmod\n  does not isolate per project — logging in once covers every project\n  on the machine.\n")

	b.WriteString("\n## Reinstall after restore\n\n")
	b.WriteString("- gstack: the packed clone has no `.git` directory (excluded), so run\n  `agentmod install gstack --force` for a fresh, updatable clone.\n")
	b.WriteString("- npm global tools: symlinks under `.agentmod/node/bin` may dangle\n  because `lib/node_modules` is excluded; re-run the npm installs\n  inside the project to recreate them.\n")
	return []byte(b.String())
}

// countNoun renders "1 file" / "3 files" style counts.
func countNoun(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
