// Restore-side validation (FABLE_PLAN §18 "Handoff restore", §21 Critical,
// §22, §25; IMPLEMENTATION_PLAN §12 restore pipeline). PlanRestore walks
// every archive member, refuses anything that could write outside the
// project's .agentmod/ — zip-slip names, absolute paths, escaping symlink
// targets, protected path elements — and returns the extraction plan the
// restore slices execute. It never touches the filesystem.
//
// PlanRestore is deliberately orthogonal to Verify: Verify judges integrity
// (checksums, inventory agreement), PlanRestore judges path safety. Restore
// must run Open, then Verify, then PlanRestore, and refuse on any problem
// from either (§21 "Never trust external snapshots").

package handoff

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/mojomoth/agentmod/internal/project"
)

// maxSymlinkTarget bounds how many bytes PlanRestore will read as a symlink
// target. Real targets are short paths; anything larger is a hostile or
// corrupt archive, not a link.
const maxSymlinkTarget = 4096

// protectedElements are path elements no restored entry may contain,
// verbatim from FABLE_PLAN §21: "Never write to .ssh, .aws, .docker, .git".
// (Create's exclusion rules cover a broader credential-dir set; on restore
// every write is already confined to .agentmod/, so only these four —
// notably .git, which could plant hooks — need an explicit deny.)
var protectedElements = map[string]bool{
	".git": true, ".ssh": true, ".aws": true, ".docker": true,
}

// PlanEntry is one planned extraction target.
type PlanEntry struct {
	ZipName string      // archive member name (payload/...)
	RelPath string      // project-root-relative, slash-separated (.agentmod/...)
	Mode    fs.FileMode // permission bits only (setuid/setgid/sticky stripped)
	Target  string      // symlink target, Links entries only
}

// RestorePlan is the validated set of extraction actions, each slice sorted
// by RelPath (so parent directories precede children). The extraction slice
// should create Dirs, then Files, then Links — links last so no file write
// can ever pass through a just-restored symlink.
type RestorePlan struct {
	Dirs  []PlanEntry
	Files []PlanEntry
	Links []PlanEntry
}

// PlanRestore validates every archive member for safe extraction and
// returns the plan. Problems (human sentences, detection order) are
// collected rather than stopping at the first, so a hostile archive is
// reported in one pass; any problem means no plan. Checks per §21/§22/§25:
//
//   - manifest schema_version must be 1..SchemaVersion (restore hard-refuses
//     newer snapshots; inspect/verify merely warn)
//   - every member is either a §21 root member or under payload/
//   - payload paths are canonical, relative, forward-slash, no ".." escape,
//     no Windows drive prefix, first element .agentmod, no protected
//     elements, no duplicates
//   - symlink targets are non-empty, relative, and resolve (lexically)
//     inside .agentmod/
//   - members are regular files, directories, or symlinks — nothing else
func (s *Snapshot) PlanRestore() (*RestorePlan, []string) {
	var problems []string
	problemf := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if p := s.schemaProblem(); p != "" {
		problems = append(problems, p)
	}

	rootMembers := map[string]bool{}
	for _, name := range requiredMembers {
		rootMembers[name] = true
	}

	plan := &RestorePlan{}
	seen := map[string]bool{}
	for _, f := range s.zr.File {
		name := f.Name
		if rootMembers[name] {
			continue
		}
		if !strings.HasPrefix(name, PayloadPrefix) {
			problemf("unexpected member %s outside %s", name, PayloadPrefix)
			continue
		}
		mode := f.FileInfo().Mode()
		rel := strings.TrimPrefix(name, PayloadPrefix)
		if mode.IsDir() {
			rel = strings.TrimSuffix(rel, "/")
		}
		if rel == "" {
			continue // the payload/ container directory itself
		}

		switch {
		case strings.Contains(rel, `\`):
			problemf("member %s contains a backslash; snapshot paths are forward-slash relative", name)
			continue
		case strings.HasPrefix(rel, "/"):
			problemf("member %s would restore to an absolute path", name)
			continue
		case hasDrivePrefix(rel):
			problemf("member %s uses a Windows drive path", name)
			continue
		}
		cleaned := path.Clean(rel)
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			problemf("member %s escapes the project root (zip-slip)", name)
			continue
		}
		if cleaned != rel {
			problemf("member %s is not a canonical relative path", name)
			continue
		}
		elems := strings.Split(rel, "/")
		if elems[0] != project.DirName {
			problemf("member %s would restore outside %s/", name, project.DirName)
			continue
		}
		protected := ""
		for _, elem := range elems[1:] {
			if protectedElements[elem] {
				protected = elem
				break
			}
		}
		if protected != "" {
			problemf("member %s contains the protected path element %q", name, protected)
			continue
		}
		if seen[rel] {
			problemf("archive contains payload path %s more than once", name)
			continue
		}
		seen[rel] = true

		entry := PlanEntry{ZipName: name, RelPath: rel, Mode: mode.Perm()}
		switch {
		case mode.IsDir():
			plan.Dirs = append(plan.Dirs, entry)
		case mode&fs.ModeSymlink != 0:
			if f.UncompressedSize64 > maxSymlinkTarget {
				problemf("symlink %s target is implausibly large (%d bytes)", name, f.UncompressedSize64)
				continue
			}
			data, err := readZipMember(f)
			if err != nil {
				problemf("unreadable symlink %s (%v)", name, err)
				continue
			}
			target := string(data)
			switch {
			case target == "":
				problemf("symlink %s has an empty target", name)
				continue
			case strings.Contains(target, `\`):
				problemf("symlink %s target contains a backslash", name)
				continue
			case strings.HasPrefix(target, "/") || hasDrivePrefix(target):
				problemf("symlink %s has an absolute target (%s)", name, target)
				continue
			}
			// Lexical resolution: every target is confined to .agentmod/, so
			// (by induction) chains of in-payload links cannot escape either.
			resolved := path.Join(path.Dir(rel), target)
			if resolved != project.DirName && !strings.HasPrefix(resolved, project.DirName+"/") {
				problemf("symlink %s target escapes %s/ (resolves to %s)", name, project.DirName, resolved)
				continue
			}
			entry.Target = target
			plan.Links = append(plan.Links, entry)
		case mode.IsRegular():
			plan.Files = append(plan.Files, entry)
		default:
			problemf("member %s is neither a regular file, directory, nor symlink (%s)", name, mode.Type())
		}
	}

	if len(problems) > 0 {
		return nil, problems
	}
	byRel := func(entries []PlanEntry) func(i, j int) bool {
		return func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath }
	}
	sort.Slice(plan.Dirs, byRel(plan.Dirs))
	sort.Slice(plan.Files, byRel(plan.Files))
	sort.Slice(plan.Links, byRel(plan.Links))
	return plan, nil
}

// hasDrivePrefix reports whether p starts like a Windows drive path ("C:").
func hasDrivePrefix(p string) bool {
	return len(p) >= 2 && p[1] == ':' &&
		(('a' <= p[0] && p[0] <= 'z') || ('A' <= p[0] && p[0] <= 'Z'))
}
