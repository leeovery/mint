package notes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// BuildChangeMap assembles the diff-derived salience preamble for lastTag..HEAD —
// the Change Map. It is the fix for the "glosses over the big feature" failure: a
// salience problem, not a missing-data problem. The map is compact METADATA that a
// later task prepends before the (capped) diff to tell the AI what to prioritize;
// it is NOT a restatement of the diff and this task does NOT do the prepend.
//
// Two cheap git calls feed it, run cwd-relative like the other engine git calls and
// AFTER the SAME exclusion set the diff uses — the built-in :(exclude)CHANGELOG.md
// plus one :(exclude)<glob> per configured diff_exclude glob plus, in PLAIN mode, the
// strategy-aware :(exclude)<version_file> (the shared excludePathspecs) rides on BOTH —
// so excluded churn never appears in the map:
//
//   - `git diff --name-status {lastTag}..HEAD -- . {excludePathspecs}` —
//     A/M/D/R status per path, the STRUCTURAL NOVELTY signal.
//   - `git diff --numstat {lastTag}..HEAD -- . {excludePathspecs}` —
//     {added}\t{removed}\t{path} per file, the MAGNITUDE signal.
//
// The rendered map orders salience NOVELTY-FIRST (weighted above magnitude): new
// packages/directories and renamed/removed paths lead, then the per-area churn
// rollup as supporting context, then individually-notable files (new top-level
// entries and the single largest file).
//
// A missing git binary is surfaced as a condition matching runner.ErrCommandNotFound
// (via errors.Is); any other non-zero git exit is wrapped so an unexpected failure
// is never mistaken for a degenerate map.
func (a *Assembler) BuildChangeMap(ctx context.Context, lastTag string) (string, error) {
	return a.BuildChangeMapForRange(ctx, forwardRange(lastTag))
}

// BuildChangeMapForRange is BuildChangeMap over an ARBITRARY git range — the
// regenerate fresh source (Phase 5) feeds it `{PreviousTag}..{Tag}` (5-3's DiffRange).
// It runs the SAME two git calls (--name-status, --numstat) over the range with the
// SAME post-exclusion set the diff uses (the shared excludePathspecs: CHANGELOG.md +
// configured globs + strategy version_file), so the map is computed AFTER exclusion
// and an excluded path NEVER appears — the regenerate map matches the forward map's
// ordering and exclusion exactly. BuildChangeMap is the forward wrapper that builds
// `{lastTag}..HEAD` and delegates here.
func (a *Assembler) BuildChangeMapForRange(ctx context.Context, diffRange string) (string, error) {
	excludes := a.excludePathspecs()
	nameStatusArgs := append([]string{"diff", "--name-status", diffRange, "--", "."}, excludes...)
	nameStatusRes, err := a.runner.Run(ctx, "git", nameStatusArgs...)
	if err != nil {
		return "", fmt.Errorf("building change map name-status for %s: %w", diffRange, err)
	}

	numstatArgs := append([]string{"diff", "--numstat", diffRange, "--", "."}, excludes...)
	numstatRes, err := a.runner.Run(ctx, "git", numstatArgs...)
	if err != nil {
		return "", fmt.Errorf("building change map numstat for %s: %w", diffRange, err)
	}

	changes := parseNameStatus(nameStatusRes.Stdout)
	churn := parseNumstat(numstatRes.Stdout)

	return renderChangeMap(changes, churn), nil
}

// changeStatus is the structural status of one changed path from name-status.
type changeStatus int

const (
	statusOther    changeStatus = iota // unclassified (defensive default)
	statusAdded                        // A — a brand-new path
	statusModified                     // M — an edit to a pre-existing path
	statusDeleted                      // D — a removed path
	statusRenamed                      // R — a path moved (old -> new)
)

// pathChange is one structural change: a destination path, its status, and (for
// renames only) the source path it moved from. Path is always the NEW/current path
// so directory rollup and novelty grouping key off where the file lives now.
type pathChange struct {
	Path    string
	OldPath string // populated for renames only; "" otherwise.
	Status  changeStatus
}

// fileChurn is one file's magnitude from numstat: total added+removed lines. Binary
// files (numstat "-\t-\tpath") contribute zero churn — they carry no line signal.
type fileChurn struct {
	Path  string
	Lines int
}

// parseNameStatus turns `git diff --name-status` stdout into structural changes.
// Each line is tab-separated: "A\tpath", "M\tpath", "D\tpath", or for a rename
// "R{score}\told\tnew" (two paths). Blank lines and unparseable lines are skipped
// defensively so a stray git artifact never aborts the cheap map.
func parseNameStatus(out string) []pathChange {
	var changes []pathChange
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		code := fields[0]
		switch code[0] {
		case 'A':
			changes = append(changes, pathChange{Path: fields[1], Status: statusAdded})
		case 'M':
			changes = append(changes, pathChange{Path: fields[1], Status: statusModified})
		case 'D':
			changes = append(changes, pathChange{Path: fields[1], Status: statusDeleted})
		case 'R':
			if len(fields) < 3 {
				continue
			}
			changes = append(changes, pathChange{Path: fields[2], OldPath: fields[1], Status: statusRenamed})
		}
	}
	return changes
}

// parseNumstat turns `git diff --numstat` stdout into per-file churn. Each line is
// "{added}\t{removed}\t{path}"; a rename renders the path as "old\tnew" so the path
// is everything after the two count fields rejoined. Binary files show "-\t-\tpath"
// and contribute zero churn. Unparseable lines are skipped defensively.
func parseNumstat(out string) []fileChurn {
	var churn []fileChurn
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}

		added := parseCount(fields[0])
		removed := parseCount(fields[1])
		path := numstatPath(fields[2:])
		churn = append(churn, fileChurn{Path: path, Lines: added + removed})
	}
	return churn
}

// parseCount parses a numstat count field. "-" (binary file) parses as 0; any other
// non-numeric value defensively parses as 0 so a stray field never aborts the map.
func parseCount(s string) int {
	if s == "-" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// numstatPath resolves the path for a numstat row. For a normal row that is a single
// field; for a rename git emits "old\tnew" so the row carries two path fields — the
// NEW (current) path is the last one, matching name-status which keys off the new path.
func numstatPath(pathFields []string) string {
	return pathFields[len(pathFields)-1]
}

// renderChangeMap composes the compact preamble: NOVELTY first (new packages, then
// renames and removals), then the MAGNITUDE rollup (per-area churn ranked desc),
// then NOTABLE files (new top-level entries, single largest file). Sections present
// only when they have content, so a small change set yields a small map.
func renderChangeMap(changes []pathChange, churn []fileChurn) string {
	var sections []string

	if novelty := renderNovelty(changes); novelty != "" {
		sections = append(sections, novelty)
	}
	if magnitude := renderMagnitude(churn); magnitude != "" {
		sections = append(sections, magnitude)
	}
	if notable := renderNotable(changes, churn); notable != "" {
		sections = append(sections, notable)
	}

	return strings.Join(sections, "\n")
}

// renderNovelty is the PRIMARY section: structural novelty weighted above magnitude.
// New packages/directories headline first (the strongest language-agnostic signal),
// then renamed and removed paths reported as structural changes.
func renderNovelty(changes []pathChange) string {
	var lines []string

	for _, dir := range newDirectories(changes) {
		lines = append(lines, "New package/dir: "+dir)
	}
	for _, c := range changes {
		switch c.Status {
		case statusRenamed:
			lines = append(lines, "Renamed: "+c.OldPath+" -> "+c.Path)
		case statusDeleted:
			lines = append(lines, "Removed: "+c.Path)
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return "Structural novelty:\n" + indentLines(lines)
}

// newDirectories applies the documented novelty heuristic: group changed paths by
// their LEADING directory segment; a directory is NEW/novel when it has at least one
// path under it AND every changed path under it has status Added (none Modified,
// Deleted, or Renamed) — i.e. nothing pre-existing in that area was touched. So a
// brand-new auth/ package (all files added) headlines, while an existing area that
// is merely modified produces no false novelty headline. Top-level files (no
// directory segment) are NOT directories and are handled as notable entries instead.
// Result is sorted for deterministic output.
func newDirectories(changes []pathChange) []string {
	allAdded := make(map[string]bool)
	for _, c := range changes {
		dir, ok := leadingDir(c.Path)
		if !ok {
			continue // top-level file, not a directory.
		}
		if _, seen := allAdded[dir]; !seen {
			allAdded[dir] = true
		}
		if c.Status != statusAdded {
			allAdded[dir] = false
		}
	}

	var dirs []string
	for dir, novel := range allAdded {
		if novel {
			dirs = append(dirs, dir+"/")
		}
	}
	sort.Strings(dirs)
	return dirs
}

// renderMagnitude is the SECONDARY section: per-file churn rolled up to area
// (leading-directory) granularity and ranked by total churn descending — the
// salience-preserving form, since a flat per-file list is itself mush on big
// releases. Top-level files roll up under their own filename as their "area".
// Ties break alphabetically for deterministic output.
func renderMagnitude(churn []fileChurn) string {
	totals := make(map[string]int)
	for _, fc := range churn {
		totals[areaOf(fc.Path)] += fc.Lines
	}
	if len(totals) == 0 {
		return ""
	}

	areas := make([]string, 0, len(totals))
	for area := range totals {
		areas = append(areas, area)
	}
	sort.Slice(areas, func(i, j int) bool {
		if totals[areas[i]] != totals[areas[j]] {
			return totals[areas[i]] > totals[areas[j]]
		}
		return areas[i] < areas[j]
	})

	lines := make([]string, 0, len(areas))
	for _, area := range areas {
		lines = append(lines, fmt.Sprintf("%s: %d lines", area, totals[area]))
	}
	return "Churn by area (largest first):\n" + indentLines(lines)
}

// renderNotable is the NOTABLE-files section: new TOP-LEVEL entries (added paths
// with no directory segment) and the SINGLE largest file by churn, each called out
// individually so a standout file is not buried in the area rollup.
func renderNotable(changes []pathChange, churn []fileChurn) string {
	var lines []string

	for _, c := range changes {
		if c.Status == statusAdded {
			if _, isDir := leadingDir(c.Path); !isDir {
				lines = append(lines, "New top-level: "+c.Path)
			}
		}
	}
	if largest, ok := largestFile(churn); ok {
		lines = append(lines, fmt.Sprintf("Largest file: %s (%d lines)", largest.Path, largest.Lines))
	}

	if len(lines) == 0 {
		return ""
	}
	return "Notable files:\n" + indentLines(lines)
}

// largestFile returns the single file with the most churn (added+removed). Ties
// break alphabetically for deterministic output. The bool is false when there is no
// churn at all (nothing to call out).
func largestFile(churn []fileChurn) (fileChurn, bool) {
	var best fileChurn
	found := false
	for _, fc := range churn {
		switch {
		case !found, fc.Lines > best.Lines, fc.Lines == best.Lines && fc.Path < best.Path:
			best = fc
			found = true
		}
	}
	return best, found
}

// areaOf is the rollup key for a path: its leading directory segment, or — for a
// top-level file with no directory — the file itself (a top-level file is its own
// area for magnitude purposes).
func areaOf(path string) string {
	if dir, ok := leadingDir(path); ok {
		return dir + "/"
	}
	return path
}

// leadingDir returns the leading directory segment of path (everything before the
// first "/") and true, or "" and false when the path has no directory segment (a
// top-level file). "auth/login.go" -> "auth"; "README.md" -> ("", false).
func leadingDir(path string) (string, bool) {
	i := strings.IndexByte(path, '/')
	if i < 0 {
		return "", false
	}
	return path[:i], true
}

// indentLines joins lines with a leading "  " indent each, producing the compact
// two-space-indented body under a section header.
func indentLines(lines []string) string {
	return "  " + strings.Join(lines, "\n  ")
}
