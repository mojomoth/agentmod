// Package config defines the agentmod.toml schema (v1), its defaults, and
// validation. Loading starts from Default() and overlays only the keys the
// document actually sets, so a partial file keeps the mandatory defaults of
// FABLE_PLAN §13. Validation rejects configs that would violate hard policy
// (change_home, unencrypted sessions in git handoff) regardless of how they
// were produced.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// CurrentSchemaVersion is the only schema this binary understands. Documents
// declaring any other version are rejected so a newer config is never
// silently misread.
const CurrentSchemaVersion = 1

// ModeStandard is the only mode defined in schema v1.
const ModeStandard = "standard"

// Sentinel errors for policy violations callers may want to distinguish.
var (
	ErrSchemaVersion          = errors.New("unsupported schema_version")
	ErrChangeHome             = errors.New("isolation.change_home must be false: agentmod never changes HOME")
	ErrSessionsNeedEncryption = errors.New("handoff.git.include_sessions = true requires encryption, which this version does not support; sessions stay excluded from git handoffs")
)

// Config is the decoded agentmod.toml.
type Config struct {
	SchemaVersion int       `toml:"schema_version"`
	Mode          string    `toml:"mode"`
	Isolation     Isolation `toml:"isolation"`
	Claude        Claude    `toml:"claude"`
	Codex         Codex     `toml:"codex"`
	OpenCode      OpenCode  `toml:"opencode"`
	Node          Node      `toml:"node"`
	Gstack        Gstack    `toml:"gstack"`
	Snapshot      Snapshot  `toml:"snapshot"`
	Handoff       Handoff   `toml:"handoff"`
}

// Isolation holds the global-pollution policy.
type Isolation struct {
	ChangeHome        bool `toml:"change_home"`
	BlockGlobalWrites bool `toml:"block_global_writes"`
}

// Claude controls CLAUDE_CONFIG_DIR routing and the Bash guard hook.
type Claude struct {
	Enabled   bool `toml:"enabled"`
	BashGuard bool `toml:"bash_guard"`
}

// Codex controls CODEX_HOME routing.
type Codex struct {
	Enabled bool `toml:"enabled"`
}

// OpenCode controls OPENCODE_CONFIG routing; XDGFullIsolation additionally
// routes the XDG_* roots (off by default — partial isolation, §15.3).
type OpenCode struct {
	Enabled          bool `toml:"enabled"`
	XDGFullIsolation bool `toml:"xdg_full_isolation"`
}

// Node controls npm/pnpm/bun cache+prefix routing and the PATH entry.
type Node struct {
	Enabled bool `toml:"enabled"`
}

// Gstack holds project-local gstack installer settings.
type Gstack struct {
	AutoDoctorCheck bool `toml:"auto_doctor_check"`
}

// Snapshot holds .amod creation defaults. These set the exclusion baseline;
// the exclusion engine treats secret/auth entries as protected and never
// removable at create time regardless of these values.
type Snapshot struct {
	ExcludeSource  bool `toml:"exclude_source"`
	ExcludeSecrets bool `toml:"exclude_secrets"`
}

// Handoff holds handoff defaults.
type Handoff struct {
	Git GitHandoff `toml:"git"`
}

// GitHandoff holds git-handoff (--for-git) defaults.
type GitHandoff struct {
	IncludeSessions bool `toml:"include_sessions"`
	IncludeLogs     bool `toml:"include_logs"`
}

// Default returns the configuration written by init: the mandatory defaults
// of FABLE_PLAN §13.
func Default() Config {
	return Config{
		SchemaVersion: CurrentSchemaVersion,
		Mode:          ModeStandard,
		Isolation: Isolation{
			ChangeHome:        false,
			BlockGlobalWrites: true,
		},
		Claude: Claude{
			Enabled:   true,
			BashGuard: true,
		},
		Codex: Codex{
			Enabled: true,
		},
		OpenCode: OpenCode{
			Enabled:          true,
			XDGFullIsolation: false,
		},
		Node: Node{
			Enabled: true,
		},
		Gstack: Gstack{
			AutoDoctorCheck: true,
		},
		Snapshot: Snapshot{
			ExcludeSource:  true,
			ExcludeSecrets: true,
		},
		Handoff: Handoff{
			Git: GitHandoff{
				IncludeSessions: false,
				IncludeLogs:     false,
			},
		},
	}
}

// Parse decodes an agentmod.toml document over the defaults and validates
// the result. Keys the document does not set keep their default values.
// Unknown keys are rejected — within a schema version they can only be typos,
// and a misspelled policy key silently reverting to a default is worse than
// an error.
func Parse(data []byte) (Config, error) {
	cfg := Default()
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parse agentmod.toml: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return Config{}, fmt.Errorf("agentmod.toml: unknown key(s): %s", strings.Join(keys, ", "))
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load reads and parses the agentmod.toml at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Validate enforces the hard policies of FABLE_PLAN §13 that no config may
// override.
func (c Config) Validate() error {
	if c.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: %d (this agentmod understands %d)", ErrSchemaVersion, c.SchemaVersion, CurrentSchemaVersion)
	}
	if c.Mode != ModeStandard {
		return fmt.Errorf("unknown mode %q (schema v1 defines only %q)", c.Mode, ModeStandard)
	}
	if c.Isolation.ChangeHome {
		return ErrChangeHome
	}
	if c.Handoff.Git.IncludeSessions {
		return ErrSessionsNeedEncryption
	}
	return nil
}

// Marshal renders the config as TOML. Marshal(Default()) is what init writes;
// Parse(Marshal(c)) round-trips for any valid c.
func Marshal(c Config) ([]byte, error) {
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return nil, fmt.Errorf("encode agentmod.toml: %w", err)
	}
	return []byte(b.String()), nil
}
