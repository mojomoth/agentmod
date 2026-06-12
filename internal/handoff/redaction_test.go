package handoff

import (
	"testing"
	"time"
)

// TestRedactionFindingCounts round-trips rendered reports through the
// parser doctor uses: exclusion list items must never count as scan
// findings, and the HARD marker must be recovered exactly.
func TestRedactionFindingCounts(t *testing.T) {
	createdAt := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	// Exclusion entries render as "- `path` — rule: reason" list items in
	// the section ABOVE the scan heading; they must not inflate the counts.
	excluded := []ExcludedEntry{
		{Path: ".agentmod/claude/.credentials.json", RuleID: "auth-file", Reason: "never packed"},
		{Path: ".agentmod/node/npm-cache/", RuleID: "cache", Reason: "regenerated"},
	}
	cases := []struct {
		name      string
		findings  []ScanFinding
		wantTotal int
		wantHard  int
	}{
		{name: "clean", findings: nil, wantTotal: 0, wantHard: 0},
		{
			name: "warn only",
			findings: []ScanFinding{
				{Path: ".agentmod/claude/notes.txt", Pattern: "aws-access-key-id", Line: 3},
			},
			wantTotal: 1, wantHard: 0,
		},
		{
			name: "hard and warn",
			findings: []ScanFinding{
				{Path: ".agentmod/claude/notes.txt", Pattern: "private-key", Line: 1, Hard: true},
				{Path: ".agentmod/claude/notes.txt", Pattern: "aws-access-key-id", Line: 2},
				{Path: ".agentmod/codex/scratch.md", Pattern: "github-token", Line: 9},
			},
			wantTotal: 3, wantHard: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			report := renderRedaction(createdAt, "test", excluded, c.findings)
			total, hard := RedactionFindingCounts(report)
			if total != c.wantTotal || hard != c.wantHard {
				t.Errorf("counts = (%d, %d), want (%d, %d)\nreport:\n%s",
					total, hard, c.wantTotal, c.wantHard, report)
			}
		})
	}
}
