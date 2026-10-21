package notes

import (
	"sort"
	"strconv"
)

// condenseShare is the bytes a folder-condensable text may take before it condenses:
// a tenth of the per-call cap.
func condenseShare(maxPromptBytes int) int {
	return maxPromptBytes / 10
}

// foldLine is one line of a folder-condensable text. run is the folder run the line can
// condense into; the zero runKey marks a line that never condenses. bytes is the size
// the line contributes to its run's summary.
type foldLine struct {
	text  string
	run   runKey
	bytes int
}

// runKey names a condensable run: one kind of entry under one leading directory.
type runKey struct {
	kind   string
	folder string
}

// runOf keys path's line into its leading-directory run of kind; a top-level path has
// no folder and so no run.
func runOf(kind, path string) runKey {
	dir, ok := leadingDir(path)
	if !ok {
		return runKey{}
	}
	return runKey{kind: kind, folder: dir}
}

// summarizeRun writes the one line a collapsed run of count lines, totalling bytes,
// condenses into.
type summarizeRun func(key runKey, count, bytes int) string

// runSummary is one candidate collapse: the run, its size, its summary line, and the
// bytes collapsing it saves.
type runSummary struct {
	key   runKey
	count int
	text  string
	saved int
}

// condenseRuns collapses runs in rankRuns order until at least excess bytes are saved
// or none is left. Each collapsed run's summary line takes the place of the run's
// first line.
func condenseRuns(lines []foldLine, excess int, summarize summarizeRun) []foldLine {
	collapsed := make(map[runKey]string)
	saved := 0
	for _, run := range rankRuns(lines, summarize) {
		if saved >= excess {
			break
		}
		collapsed[run.key] = run.text
		saved += run.saved
	}

	out := make([]foldLine, 0, len(lines))
	placed := make(map[runKey]bool)
	for _, l := range lines {
		summary, ok := collapsed[l.run]
		if !ok {
			out = append(out, l)
			continue
		}
		if !placed[l.run] {
			out = append(out, foldLine{text: summary})
			placed[l.run] = true
		}
	}
	return out
}

// rankRuns returns the runs of two or more lines whose collapse would shrink the text,
// in collapse order: largest first, ties by kind then folder so the order never
// depends on map iteration.
func rankRuns(lines []foldLine, summarize summarizeRun) []runSummary {
	counts := make(map[runKey]int)
	costs := make(map[runKey]int)
	sizes := make(map[runKey]int)
	for _, l := range lines {
		if l.run == (runKey{}) {
			continue
		}
		counts[l.run]++
		costs[l.run] += len(l.text) + lineOverhead
		sizes[l.run] += l.bytes
	}

	runs := make([]runSummary, 0, len(counts))
	for key, count := range counts {
		if count < 2 {
			continue
		}
		text := summarize(key, count, sizes[key])
		if saved := costs[key] - (len(text) + lineOverhead); saved > 0 {
			runs = append(runs, runSummary{key: key, count: count, text: text, saved: saved})
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].count != runs[j].count {
			return runs[i].count > runs[j].count
		}
		if runs[i].key.kind != runs[j].key.kind {
			return runs[i].key.kind < runs[j].key.kind
		}
		return runs[i].key.folder < runs[j].key.folder
	})
	return runs
}

// lineOverhead is the bytes a condensable text adds per line beyond its text: a
// two-character lead and the joining newline.
const lineOverhead = 3

// groupThousands renders n with comma thousands separators.
func groupThousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
