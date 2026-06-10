// Secret-candidate content scan (IMPLEMENTATION_PLAN §12 create pipeline:
// collect → filter → scan remaining → write). Only files the exclusion
// engine KEPT are scanned; a finding records the file, pattern, and line —
// never the matched bytes themselves, so the redaction report and CLI
// output cannot leak what they warn about.
//
// Hard findings (private-key material) refuse creation unless
// CreateOptions.AllowFindings is set; everything else is a warning listed
// in REDACTION.md. Name-based exclusion (auth files, .env, …) is
// exclude.go's job — these patterns look at content only.

package handoff

import (
	"bytes"
	"regexp"
)

// ScanFinding records one secret-candidate pattern match in a kept payload
// file.
type ScanFinding struct {
	Path    string `json:"path"`    // project-root-relative, forward-slash (same shape as ExcludedEntry.Path)
	Pattern string `json:"pattern"` // pattern ID, e.g. "private-key"
	Line    int    `json:"line"`    // 1-based line of the pattern's first match
	Hard    bool   `json:"hard"`    // hard findings refuse creation unless AllowFindings
}

type scanPattern struct {
	id   string
	hard bool
	re   *regexp.Regexp
}

// scanPatterns are checked in order; each contributes at most one finding
// per file (the first match), so a wall of repeated hits stays readable.
// Bare words like "token" deliberately need assignment context (`token =`,
// `"token":`) — a file merely named or mentioning tokenizer must not warn.
var scanPatterns = []scanPattern{
	{id: "private-key", hard: true, re: regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----`)},
	{id: "aws-access-key-id", re: regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{id: "github-token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)},
	{id: "sk-token", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
	{id: "api-key", re: regexp.MustCompile(`(?i)api[_-]?key["']?\s*[:=]`)},
	{id: "token", re: regexp.MustCompile(`(?i)(auth|access|refresh|bearer|session|api)[_-]?token["']?\s*[:=]`)},
	{id: "secret", re: regexp.MustCompile(`(?i)secret["']?\s*[:=]`)},
}

// scanContent returns the findings for one kept file's content, in pattern
// order. relPath is the project-root-relative forward-slash path recorded
// on each finding.
func scanContent(relPath string, data []byte) []ScanFinding {
	var findings []ScanFinding
	for _, p := range scanPatterns {
		loc := p.re.FindIndex(data)
		if loc == nil {
			continue
		}
		findings = append(findings, ScanFinding{
			Path:    relPath,
			Pattern: p.id,
			Line:    1 + bytes.Count(data[:loc[0]], []byte("\n")),
			Hard:    p.hard,
		})
	}
	return findings
}
