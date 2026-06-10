// Default exclusion engine (FABLE_PLAN §18 "Excluded by default",
// IMPLEMENTATION_PLAN §12). Every rule carries a human-readable Reason so
// the redaction report can explain each exclusion verbatim; Create records
// what each rule dropped in Result.Excluded.
//
// Deliberately NOT excluded here: sessions and logs/ stay in normal
// handoffs (they are excluded only in --for-git mode, Phase 7), and the
// secret-candidate content scan is its own slice — these rules match on
// names and paths only.

package handoff

import (
	"strings"

	"github.com/agentmod/agentmod/internal/layout"
	"github.com/agentmod/agentmod/internal/project"
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
