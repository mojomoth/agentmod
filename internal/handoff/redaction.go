// REDACTION.md rendering (IMPLEMENTATION_PLAN §12: "what was excluded and
// why; secret-candidate scan results"). The report is a root zip member
// next to manifest.json so a recipient can read it without restoring; it
// renders Result.Excluded reasons verbatim and the scan findings by
// file/line/pattern only — the matched content never appears.

package handoff

import (
	"fmt"
	"strings"
	"time"
)

// RedactionName is the redaction report's zip member name.
const RedactionName = "REDACTION.md"

// scanHeading is the report section renderRedaction writes the scan
// findings under; RedactionFindingCounts keys off it when parsing.
const scanHeading = "## Secret-candidate scan"

// RedactionFindingCounts parses a rendered REDACTION.md back into its
// secret-candidate counts: every finding listed under the scan heading,
// and how many of them were HARD findings packed under --allow-findings.
// doctor uses it to re-surface a snapshot's create-time scan without
// re-reading payload content; it counts only list items in the scan
// section, so the exclusion list above it never inflates the numbers.
func RedactionFindingCounts(report []byte) (total, hard int) {
	inScan := false
	for _, line := range strings.Split(string(report), "\n") {
		if strings.HasPrefix(line, "## ") {
			inScan = strings.TrimSpace(line) == scanHeading
			continue
		}
		if inScan && strings.HasPrefix(line, "- ") {
			total++
			if strings.Contains(line, "(HARD finding") {
				hard++
			}
		}
	}
	return total, hard
}

// renderRedaction produces the REDACTION.md member. All inputs are already
// in deterministic (walk/pattern) order, so identical snapshots render
// byte-identical reports.
func renderRedaction(createdAt time.Time, version string, excluded []ExcludedEntry, findings []ScanFinding) []byte {
	var b strings.Builder
	b.WriteString("# Redaction report\n\n")
	fmt.Fprintf(&b, "Snapshot created %s by agentmod %s.\n\n", createdAt.UTC().Format(time.RFC3339), version)

	b.WriteString("## Excluded from the payload\n\n")
	if len(excluded) == 0 {
		b.WriteString("Nothing was excluded; the payload carries the complete agent\nenvironment tree.\n")
	} else {
		b.WriteString("These entries exist in the project's agent environment but were NOT\npacked. A path ending in \"/\" is a directory whose whole subtree was\nexcluded.\n\n")
		for _, e := range excluded {
			fmt.Fprintf(&b, "- `%s` — %s: %s\n", e.Path, e.RuleID, e.Reason)
		}
	}
	b.WriteString("\n" + scanHeading + "\n\n")
	if len(findings) == 0 {
		b.WriteString("No secret candidates were found in the packed files.\n")
	} else {
		b.WriteString("Packed files were scanned for secret-candidate patterns. Each match\nnames the file, line, and pattern only; the matched text itself is\nnever reproduced here. Review these files before sharing the snapshot:\n\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- `%s` line %d — %s", f.Path, f.Line, f.Pattern)
			// A hard finding can only reach the report when the caller
			// passed AllowFindings — otherwise create refused already.
			if f.Hard {
				b.WriteString(" (HARD finding; packed because --allow-findings was given)")
			}
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}
