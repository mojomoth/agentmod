// Default exclusion engine (FABLE_PLAN §18 "Excluded by default",
// IMPLEMENTATION_PLAN §12). Every rule carries a human-readable Reason so
// the redaction report can explain each exclusion verbatim; Create records
// what each rule dropped in Result.Excluded.
//
// Deliberately NOT excluded by DefaultRules: sessions and logs/ stay in
// normal handoffs and are dropped only by ForGitRules (FABLE_PLAN §19);
// the secret-candidate content scan is its own layer — these rules match
// on names and paths only.

package handoff

import (
	"strings"

	"github.com/mojomoth/agentmod/internal/layout"
	"github.com/mojomoth/agentmod/internal/project"
)

// Rule is one name/path-based exclusion policy. Matches receives the
// project-root-relative forward-slash path (e.g.
// ".agentmod/claude/.credentials.json"), its base name, and whether the
// entry is a directory; matching a directory prunes the whole subtree.
type Rule struct {
	ID      string // stable identifier, e.g. "auth-file"
	Reason  string // human-readable; rendered into the redaction report
	Matches func(relPath, base string, isDir bool) bool
}

// ExcludedEntry records one entry the exclusion engine dropped. Directory
// paths carry a trailing "/" (the whole subtree was pruned).
type ExcludedEntry struct {
	Path   string `json:"path"` // project-root-relative, forward-slash
	RuleID string `json:"rule_id"`
	Reason string `json:"reason"`
}

// snapshotsExclusion is the structural snapshots/ skip (D034): not a policy
// rule — snapshots/ is the default output directory, so packing it would
// nest prior snapshots inside the new one. Recorded in Result.Excluded all
// the same, so the redaction report can explain its absence.
var snapshotsExclusion = ExcludedEntry{
	Path:   project.DirName + "/" + layout.SnapshotsDir + "/",
	RuleID: "snapshots-output",
	Reason: "snapshot output directory; packing it would nest prior snapshots inside the new one",
}

// DefaultRules returns the §18 default exclusion list. Auth/secret rules
// come first so an entry matched by several rules is reported under the
// most security-relevant one.
func DefaultRules() []Rule {
	// Consent-copied auth lands at claude/.credentials.json and
	// codex/auth.json (D028), but Claude also writes .credentials.json
	// itself on Linux login, so auth is matched by NAME at any depth, not
	// by consent-copy provenance. Bare "credentials" is the AWS-style file.
	authBases := map[string]bool{
		".credentials.json": true, // Claude Code
		"auth.json":         true, // Codex CLI
		"credentials.json":  true,
		"credentials":       true,
	}
	sshKeyBases := map[string]bool{
		"id_rsa": true, "id_dsa": true, "id_ecdsa": true,
		"id_ed25519": true, "id_ecdsa_sk": true, "id_ed25519_sk": true,
	}
	credDirBases := map[string]bool{
		".ssh": true, ".aws": true, ".azure": true, ".gcloud": true,
		".kube": true, ".gnupg": true, ".docker": true,
	}
	// Path-anchored: exactly the cache targets routing.Vars points the
	// package managers at. A directory merely NAMED npm-cache elsewhere is
	// user content and stays in.
	nodeDir := project.DirName + "/" + layout.NodeDir
	cachePaths := map[string]bool{
		nodeDir + "/" + layout.NodeNPMCacheDir: true,
		nodeDir + "/" + layout.NodePnpmDir:     true,
		nodeDir + "/" + layout.NodeBunDir:      true,
	}

	return []Rule{
		{
			ID:     "auth-file",
			Reason: "agent authentication material is never packed; re-log-in on the target machine (FABLE_PLAN §18)",
			Matches: func(_, base string, isDir bool) bool {
				return !isDir && authBases[base]
			},
		},
		{
			ID:     "env-file",
			Reason: ".env files commonly hold secrets and are never packed",
			Matches: func(_, base string, isDir bool) bool {
				return !isDir && (strings.HasSuffix(base, ".env") || strings.HasPrefix(base, ".env."))
			},
		},
		{
			ID:     "ssh-key",
			Reason: "SSH/TLS key material is never packed",
			Matches: func(_, base string, isDir bool) bool {
				if isDir {
					return false
				}
				return sshKeyBases[strings.TrimSuffix(base, ".pub")] ||
					strings.HasSuffix(base, ".pem")
			},
		},
		{
			ID:     "credential-dir",
			Reason: "SSH/cloud credential directory is never packed",
			Matches: func(_, base string, isDir bool) bool {
				return isDir && credDirBases[base]
			},
		},
		{
			ID:     "os-credential-store",
			Reason: "OS credential store databases are never packed",
			Matches: func(_, base string, isDir bool) bool {
				return !isDir && (strings.HasSuffix(base, ".keychain") || strings.HasSuffix(base, ".keychain-db"))
			},
		},
		{
			ID:     "vcs-git",
			Reason: "version control data; repository history travels via git, not handoffs ('agentmod install gstack --force' restores the gstack clone's .git)",
			Matches: func(_, base string, _ bool) bool {
				// .git is a directory in a normal clone but a regular file
				// in worktrees/submodules — both are excluded.
				return base == ".git"
			},
		},
		{
			ID:     "node-modules",
			Reason: "installed dependency tree; reinstall from manifests after restore",
			Matches: func(_, base string, isDir bool) bool {
				return isDir && base == "node_modules"
			},
		},
		{
			ID:     "cache",
			Reason: "cache data; regenerated on demand",
			Matches: func(relPath, base string, isDir bool) bool {
				return isDir && (cachePaths[relPath] || base == ".cache")
			},
		},
		{
			ID:     "tmp",
			Reason: "temporary files",
			Matches: func(_, base string, isDir bool) bool {
				return isDir && (base == "tmp" || base == ".tmp")
			},
		},
	}
}

// ForGitRules returns the exclusion policy for git-storable handoff
// packages (FABLE_PLAN §19): everything DefaultRules drops, plus agent
// sessions/history and log files — a committed package is published with
// the repository, so per-machine conversation history must never travel in
// it. The default rules come first so an entry matched by both (an auth
// file inside a session dir's parent, say) is still reported under the
// most security-relevant ID (D035). CreateForGit applies these when
// CreateOptions.Rules is nil.
func ForGitRules() []Rule {
	return append(DefaultRules(), sessionDataRule(), logDataRule())
}

// GitPublishRules returns only the session/log rules that ForGitRules adds
// on top of DefaultRules. doctor applies them to an existing
// .agentmod-handoff/ payload to detect session or log material a commit
// would publish (FABLE_PLAN §23) — agentmod's own CreateForGit can never
// pack such entries, so a hit means the tree was edited by hand or written
// by another tool.
func GitPublishRules() []Rule {
	return []Rule{sessionDataRule(), logDataRule()}
}

// sessionDataRule matches the session/history locations each agent
// actually uses inside its routed home (verified against real installs:
// claude 2.x, codex-cli 0.13x, opencode 1.4 — D048). Path-anchored so a
// user directory merely NAMED "sessions" elsewhere stays in.
func sessionDataRule() Rule {
	claude := project.DirName + "/" + layout.ClaudeDir
	codex := project.DirName + "/" + layout.CodexDir
	xdg := project.DirName + "/" + layout.OpencodeDir + "/" + layout.OpencodeXDGDir
	sessionDirs := map[string]bool{
		claude + "/projects":        true, // per-project session transcripts
		claude + "/sessions":        true,
		claude + "/session-env":     true,
		claude + "/file-history":    true,
		claude + "/shell-snapshots": true,
		codex + "/sessions":         true, // rollout files
		codex + "/shell_snapshots":  true,
		// OpenCode keeps sessions/storage in the XDG data dir and state in
		// the XDG state dir (opt-in xdg_full_isolation mode routes them
		// here; in default partial isolation these simply do not exist).
		xdg + "/data":  true,
		xdg + "/state": true,
	}
	sessionFiles := map[string]bool{
		claude + "/history.jsonl":      true,
		codex + "/history.jsonl":       true,
		codex + "/session_index.jsonl": true,
	}
	return Rule{
		ID:     "session-data",
		Reason: "agent sessions and history never travel in a git handoff — a committed package is published with the repository (FABLE_PLAN §19); pack a regular .amod snapshot to carry them privately",
		Matches: func(relPath, _ string, isDir bool) bool {
			if isDir {
				return sessionDirs[relPath]
			}
			return sessionFiles[relPath]
		},
	}
}

// logDataRule matches agentmod's own logs/ plus Codex's log dir and its
// logs_<n>.sqlite databases (with -shm/-wal sidecars) directly under the
// Codex home.
func logDataRule() Rule {
	codex := project.DirName + "/" + layout.CodexDir
	logDirs := map[string]bool{
		project.DirName + "/" + layout.LogsDir: true,
		codex + "/log":                         true,
	}
	return Rule{
		ID:     "log-data",
		Reason: "log files never travel in a git handoff (FABLE_PLAN §19)",
		Matches: func(relPath, base string, isDir bool) bool {
			if isDir {
				return logDirs[relPath]
			}
			return relPath == codex+"/"+base &&
				strings.HasPrefix(base, "logs_") && strings.Contains(base, ".sqlite")
		},
	}
}
