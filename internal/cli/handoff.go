package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/agentmod/agentmod/internal/handoff"
	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
)

// runHandoff dispatches `agentmod handoff <subcommand>` (FABLE_PLAN §18).
func runHandoff(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "agentmod: handoff requires a subcommand (try 'agentmod handoff create')\n")
		return ExitError
	}
	switch args[0] {
	case "create":
		return runHandoffCreate(args[1:], stdout, stderr, env)
	case "inspect":
		return runHandoffInspect(args[1:], stdout, stderr)
	case "verify":
		return runHandoffVerify(args[1:], stdout, stderr)
	case "list":
		return runHandoffList(args[1:], stdout, stderr, env)
	case "restore":
		return runHandoffRestore(args[1:], stdout, stderr, env)
	}
	fmt.Fprintf(stderr, "agentmod: unknown handoff subcommand %q (try 'agentmod handoff create')\n", args[0])
	return ExitError
}

// runHandoffCreate implements `agentmod handoff create [--output PATH]
// [--allow-findings] [--allow-dirty]`: pack this project's .agentmod/ into
// a .amod snapshot (handoff.Create). The default output is
// .agentmod/snapshots/<project>-<timestamp>.amod.
func runHandoffCreate(args []string, stdout, stderr io.Writer, env Env) int {
	output := ""
	allowFindings := false
	allowDirty := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "agentmod: handoff create: --output requires a path\n")
				return ExitError
			}
			i++
			output = args[i]
		case args[i] == "--allow-findings":
			allowFindings = true
		case args[i] == "--allow-dirty":
			allowDirty = true
		default:
			fmt.Fprintf(stderr, "agentmod: handoff create: unsupported argument %q (supported: --output PATH, --allow-findings, --allow-dirty)\n", args[i])
			return ExitError
		}
	}

	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	proj, err := project.Discover(cwd)
	if errors.Is(err, project.ErrNotFound) {
		fmt.Fprintf(stderr, "agentmod: handoff create requires an agentmod project; run 'agentmod init' first (%v)\n", err)
		return ExitNotInProject
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	// §20 dirty-worktree gate: a snapshot carries the agent environment,
	// not source changes, so a handoff cut from a dirty tree silently loses
	// work unless the user explicitly accepts that.
	gitState, gitNote := collectGitState(proj.Root)
	if gitState != nil && gitState.Dirty && !allowDirty {
		fmt.Fprintf(stderr, "agentmod: handoff create: refusing to pack: the git worktree is dirty (%s); uncommitted source changes do not travel in a snapshot — commit or stash them so the handoff matches a commit, or re-run with --allow-dirty to pack anyway\n", gitState.StatusSummary)
		return ExitError
	}

	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}
	if output == "" {
		name := fmt.Sprintf("%s-%s.amod", filepath.Base(proj.Root), now.UTC().Format("20060102-150405"))
		output = filepath.Join(proj.AgentmodDir, layout.SnapshotsDir, name)
	}

	goos := env.GOOS
	if goos == "" {
		goos = "unknown"
	}
	res, err := handoff.Create(handoff.CreateOptions{
		ProjectRoot:   proj.Root,
		OutputPath:    output,
		CreatedAt:     now,
		Version:       Version,
		Platform:      goos + "/" + runtime.GOARCH,
		AllowFindings: allowFindings,
		Git:           gitState,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "Created handoff snapshot: %s\n", res.OutputPath)
	fmt.Fprintf(stdout, "  payload: %d files, %d bytes (manifest, inventory, and checksums included)\n", res.PayloadFiles, res.PayloadBytes)
	switch {
	case gitState == nil:
		fmt.Fprintf(stdout, "  git: metadata omitted (%s)\n", gitNote)
	case gitState.Dirty:
		fmt.Fprintf(stdout, "  git: %s, DIRTY (%s) — packed anyway (--allow-dirty); uncommitted source changes do not travel in a snapshot\n", gitIdentity(gitState), gitState.StatusSummary)
	default:
		fmt.Fprintf(stdout, "  git: %s, clean\n", gitIdentity(gitState))
	}
	switch n := len(res.Excluded); n {
	case 0:
		fmt.Fprintf(stdout, "  excluded by default policy: nothing\n")
	case 1:
		fmt.Fprintf(stdout, "  excluded by default policy: 1 entry\n")
	default:
		fmt.Fprintf(stdout, "  excluded by default policy: %d entries\n", n)
	}
	for _, e := range res.Excluded {
		fmt.Fprintf(stdout, "    %s (%s)\n", e.Path, e.RuleID)
	}
	switch n := len(res.Findings); n {
	case 0:
		fmt.Fprintf(stdout, "  secret scan: clean (no candidate patterns in packed files)\n")
	case 1:
		fmt.Fprintf(stdout, "  secret scan: 1 candidate finding (details in REDACTION.md inside the snapshot)\n")
	default:
		fmt.Fprintf(stdout, "  secret scan: %d candidate findings (details in REDACTION.md inside the snapshot)\n", n)
	}
	for _, f := range res.Findings {
		mark := ""
		if f.Hard {
			mark = ", HARD — packed because --allow-findings was given"
		}
		fmt.Fprintf(stdout, "    %s line %d (%s%s)\n", f.Path, f.Line, f.Pattern, mark)
	}
	fmt.Fprintf(stdout, "Verify it anywhere with 'agentmod handoff verify'; restore it on the target machine with 'agentmod handoff restore'.\n")
	return ExitOK
}

// gitignoreBackupEntry is the .gitignore pattern covering restore backups
// (D042): without it an untracked backup directory makes the worktree dirty
// and trips the next handoff create's --allow-dirty gate.
const gitignoreBackupEntry = handoff.BackupPrefix + "*/"

// runHandoffRestore implements `agentmod handoff restore <file.amod>`:
// replace this project's .agentmod/ with the snapshot's payload. The
// pipeline is pinned by D042 — Open, Verify, PlanRestore, refusing on any
// problem from any of them BEFORE anything on disk moves — then
// handoff.Restore backs up the existing .agentmod/ and extracts the
// validated plan (rolling back on failure). Nothing from the snapshot is
// ever executed; every write lands under .agentmod/ (FABLE_PLAN §18/§21).
func runHandoffRestore(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "agentmod: handoff restore takes exactly one argument: the .amod path (see 'agentmod handoff list')\n")
		return ExitError
	}
	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	// RESTORE.md's step order is init first, then restore from the project
	// root — so restore requires a project, like create and list.
	proj, err := project.Discover(cwd)
	if errors.Is(err, project.ErrNotFound) {
		fmt.Fprintf(stderr, "agentmod: handoff restore requires an agentmod project; run 'agentmod init' first (%v)\n", err)
		return ExitNotInProject
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	if _, err := os.Stat(args[0]); err != nil {
		// A typo'd path is not a validation verdict (D040).
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	snap, err := handoff.Open(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: handoff restore: refusing to restore %s:\n  %v\n", args[0], err)
		return ExitValidation
	}
	defer snap.Close()
	if vres := snap.Verify(); len(vres.Problems) > 0 {
		fmt.Fprintf(stderr, "agentmod: handoff restore: refusing to restore %s (integrity problems):\n", snap.Path)
		for _, p := range vres.Problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return ExitValidation
	}
	plan, problems := snap.PlanRestore()
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "agentmod: handoff restore: refusing to restore %s (unsafe to extract):\n", snap.Path)
		for _, p := range problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return ExitValidation
	}

	now := time.Now()
	if env.Now != nil {
		now = env.Now()
	}
	res, err := snap.Restore(proj.Root, plan, now)
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}

	fmt.Fprintf(stdout, "Restored handoff snapshot: %s\n", snap.Path)
	fmt.Fprintf(stdout, "  extracted: %d directories, %d files, %d symlinks — all under %s\n", res.Dirs, res.Files, res.Links, proj.AgentmodDir)
	if res.BackupPath != "" {
		fmt.Fprintf(stdout, "  previous environment backed up to: %s (delete it once the restore checks out)\n", res.BackupPath)
		line, gerr := ensureGitignore(proj.Root, gitignoreBackupEntry)
		if gerr != nil {
			// The restore itself succeeded; a .gitignore hiccup is a warning,
			// not a failure — the next handoff create's dirty gate surfaces it.
			fmt.Fprintf(stderr, "agentmod: warning: could not add %s to .gitignore: %v\n", gitignoreBackupEntry, gerr)
		} else {
			fmt.Fprintf(stdout, "  .gitignore: %s\n", line)
			if data, rerr := os.ReadFile(filepath.Join(proj.Root, ".gitignore")); rerr == nil && !gitignoreCovers(data, gitignoreEntry) {
				// D043 wrinkle: a project initialized before 'git init' never
				// got the .agentmod/ entry, so this restore's edit alone
				// would leave .agentmod/ committable.
				fmt.Fprintf(stdout, "  note: .gitignore does not cover %s yet — re-run 'agentmod init' to add it\n", gitignoreEntry)
			}
		}
	}
	// Portability pass (FABLE_PLAN §18, D044): re-wire the guard hook for
	// this machine's binary and warn about config paths that still point at
	// the source machine. Restore already succeeded — nothing here changes
	// the exit code.
	reportPortability(stdout, stderr, proj.AgentmodDir, env)

	// §18 "Notice of secrets-excluded items (and re-login guidance)": auth
	// and credentials never travel (default exclusion policy, D035), so
	// every agent needs a fresh login here. Same canonical wording as
	// RESTORE.md and doctor (internal/handoff owns the strings, D037).
	fmt.Fprintf(stdout, "Re-login (auth and credentials never travel in a snapshot; the full exclusion list is the snapshot's REDACTION.md, via 'agentmod handoff inspect'):\n")
	fmt.Fprintf(stdout, "  Claude Code: %s.\n", handoff.ClaudeReloginRemedy)
	fmt.Fprintf(stdout, "  Codex CLI: %s.\n", handoff.CodexReloginRemedy)
	fmt.Fprintf(stdout, "  OpenCode: log in to your provider again if it asks.\n")
	if env.GOOS == "darwin" {
		fmt.Fprintf(stdout, "  macOS: Claude Code auth lives in the shared user Keychain — one login covers every project on this machine (D025 limitation).\n")
	}

	// §18 "Run doctor after restore": run it inline so the restored
	// environment is checked immediately (D045). Doctor's findings are
	// advisory here — the restore itself succeeded, so its exit code never
	// changes this command's (doctor routinely warns about routing vars in
	// a shell that has not re-activated yet, see the D044 smoke note).
	fmt.Fprintf(stdout, "Checking the restored environment with 'agentmod doctor':\n")
	if dcode := runDoctor(nil, stdout, stderr, env); dcode != ExitOK {
		fmt.Fprintf(stdout, "doctor reported findings above; the restore itself succeeded — fix what applies and re-run 'agentmod doctor'.\n")
	}
	return ExitOK
}

// runHandoffInspect implements `agentmod handoff inspect <file.amod>`:
// print the manifest, member/payload counts, and the snapshot's own
// redaction report without extracting anything to disk. It works anywhere —
// inspecting a received snapshot must not require a project.
func runHandoffInspect(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "agentmod: handoff inspect takes exactly one argument: the .amod path (see 'agentmod handoff list')\n")
		return ExitError
	}
	snap, err := handoff.Open(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	defer snap.Close()

	m := snap.Manifest
	fmt.Fprintf(stdout, "Snapshot: %s\n", snap.Path)
	fmt.Fprintf(stdout, "  schema version: %d\n", m.SchemaVersion)
	if m.SchemaVersion > handoff.SchemaVersion {
		fmt.Fprintf(stdout, "  WARNING: newer than this build supports (%d); upgrade agentmod before restoring\n", handoff.SchemaVersion)
	}
	fmt.Fprintf(stdout, "  created:        %s by agentmod %s (%s)\n", m.CreatedAt, m.AgentmodVersion, m.Platform)
	switch {
	case m.Git == nil:
		fmt.Fprintf(stdout, "  git:            no metadata recorded (created outside a git repository or without git)\n")
	case m.Git.Dirty:
		fmt.Fprintf(stdout, "  git:            %s, DIRTY at create time (%s)%s\n", gitIdentity(m.Git), m.Git.StatusSummary, gitRemoteSuffix(m.Git))
	default:
		fmt.Fprintf(stdout, "  git:            %s, clean%s\n", gitIdentity(m.Git), gitRemoteSuffix(m.Git))
	}
	var payloadBytes int64
	for _, e := range snap.Inventory.Files {
		payloadBytes += e.Size
	}
	fmt.Fprintf(stdout, "  members:        %d zip members; payload: %d files, %d bytes, %d directories\n",
		snap.Members, len(snap.Inventory.Files), payloadBytes, snap.PayloadDirs)
	fmt.Fprintf(stdout, "\n--- %s ---\n%s", handoff.RedactionName, snap.Redaction)
	return ExitOK
}

// gitRemoteSuffix renders the manifest's redacted remote, if one was
// recorded, for inspect's git line.
func gitRemoteSuffix(st *handoff.GitState) string {
	if st.RemoteURL == "" {
		return ""
	}
	return ", remote " + st.RemoteURL
}

// runHandoffVerify implements `agentmod handoff verify <file.amod>`:
// re-hash every content-bearing member against checksums.txt and
// cross-check the inventory, reading the zip only — nothing is written.
// Integrity problems (including an unreadable snapshot format) exit 3;
// a path that cannot be read at all exits 1.
func runHandoffVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "agentmod: handoff verify takes exactly one argument: the .amod path (see 'agentmod handoff list')\n")
		return ExitError
	}
	if _, err := os.Stat(args[0]); err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	snap, err := handoff.Open(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: handoff verify: %s FAILED:\n  %v\n", args[0], err)
		return ExitValidation
	}
	defer snap.Close()

	res := snap.Verify()
	if len(res.Problems) > 0 {
		fmt.Fprintf(stderr, "agentmod: handoff verify: %s FAILED:\n", snap.Path)
		for _, p := range res.Problems {
			fmt.Fprintf(stderr, "  %s\n", p)
		}
		return ExitValidation
	}
	fmt.Fprintf(stdout, "OK: %s\n", snap.Path)
	fmt.Fprintf(stdout, "  verified %d members against %s; inventory matches the payload\n", res.Checked, handoff.ChecksumsName)
	return ExitOK
}

// runHandoffList implements `agentmod handoff list`: name every .amod in
// this project's .agentmod/snapshots/, newest first. Snapshots written
// elsewhere via --output are outside its view by design.
func runHandoffList(args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "agentmod: handoff list takes no arguments\n")
		return ExitError
	}
	cwd, err := env.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	proj, err := project.Discover(cwd)
	if errors.Is(err, project.ErrNotFound) {
		fmt.Fprintf(stderr, "agentmod: handoff list requires an agentmod project; run 'agentmod init' first (%v)\n", err)
		return ExitNotInProject
	}
	if err != nil {
		fmt.Fprintf(stderr, "agentmod: %v\n", err)
		return ExitError
	}
	dir := filepath.Join(proj.AgentmodDir, layout.SnapshotsDir)
	files := listSnapshotFiles(dir)
	if len(files) == 0 {
		fmt.Fprintf(stdout, "No handoff snapshots in %s (create one with 'agentmod handoff create').\n", dir)
		return ExitOK
	}
	fmt.Fprintf(stdout, "Handoff snapshots in %s (newest first):\n", dir)
	for _, f := range files {
		fmt.Fprintf(stdout, "  %s  %d bytes  modified %s\n", f.Name, f.Size, f.ModTime.Format("2006-01-02 15:04"))
	}
	return ExitOK
}

// snapshotFile is one .amod in the snapshots directory.
type snapshotFile struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// listSnapshotFiles returns the .amod files in dir sorted newest first
// (ties alphabetically). A missing or unreadable directory simply means no
// snapshots yet — status and list both treat that as an answer, not an
// error.
func listSnapshotFiles(dir string) []snapshotFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []snapshotFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".amod") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, snapshotFile{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].ModTime.After(files[j].ModTime)
		}
		return files[i].Name < files[j].Name
	})
	return files
}
