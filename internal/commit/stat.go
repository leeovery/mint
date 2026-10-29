package commit

import (
	"fmt"
	"strconv"
	"strings"
)

// diffStat is what a commit carries: the files it changes and the lines it adds and
// removes, a binary file counted among the files and not the lines.
type diffStat struct {
	files, added, removed int
}

func (s diffStat) plus(o diffStat) diffStat {
	return diffStat{files: s.files + o.files, added: s.added + o.added, removed: s.removed + o.removed}
}

// String is the count as the diff's row reads it: "4 files · +86 −12".
func (s diffStat) String() string {
	noun := "files"
	if s.files == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d %s · +%d −%d", s.files, noun, s.added, s.removed)
}

// parseNumstat counts `git diff --numstat` output: a line per file, its lines added,
// then removed, then its path, the counts "-" for a binary file. A line is a file
// whatever its path, a rename's "old => new" included.
func parseNumstat(numstat string) diffStat {
	var s diffStat
	for _, line := range strings.Split(numstat, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		s.files++
		fields := strings.SplitN(line, "\t", 3)
		s.added += count(fields, 0)
		s.removed += count(fields, 1)
	}
	return s
}

// count is the line count in fields[i], 0 where it is missing or not a number, as a
// binary file's "-" is not.
func count(fields []string, i int) int {
	if i >= len(fields) {
		return 0
	}
	n, err := strconv.Atoi(fields[i])
	if err != nil {
		return 0
	}
	return n
}

// diffDetail is the diff's row: what the commit carries, then what it covers under
// mode and when that is staged.
func diffDetail(stat diffStat, mode StagingMode) string {
	return stat.String() + " · " + stagingDetail(mode)
}
