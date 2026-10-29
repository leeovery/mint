package commit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	return fmt.Sprintf("%s · +%d −%d", s.filesText(), s.added, s.removed)
}

// filesText is how many files the count holds: "4 files", "1 file".
func (s diffStat) filesText() string {
	if s.files == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", s.files)
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

// countUntracked counts the untracked files listing names, NUL-terminated paths
// relative to root, as `git add -A` would stage each: as the additions git's numstat
// counts, read from the disk once rather than by a git call per file. The count is
// display only, so it never fails: a path it can't read, as one gone since it was
// listed, is left out.
func countUntracked(root, listing string) diffStat {
	var s diffStat
	for _, path := range strings.Split(listing, "\x00") {
		if path == "" {
			continue
		}
		if counted, ok := countAddition(filepath.Join(root, path), strings.HasSuffix(path, "/")); ok {
			s = s.plus(counted)
		}
	}
	return s
}

// countAddition counts one untracked file at path as an addition, ok false where it
// can't be read. A nested repo, which ls-files lists with a trailing slash and `git add
// -A` stages as a gitlink, is one file with no lines.
func countAddition(path string, nestedRepo bool) (diffStat, bool) {
	if nestedRepo {
		return diffStat{files: 1}, true
	}
	f, err := os.Open(path)
	if err != nil {
		return diffStat{}, false
	}
	defer func() { _ = f.Close() }()
	lines, binary, err := countLines(f)
	if err != nil {
		return diffStat{}, false
	}
	if binary {
		return diffStat{files: 1}, true
	}
	return diffStat{files: 1, added: lines}, true
}

// binarySniff is how much of a file git reads to judge it binary, and a NUL in it
// makes it so: git's own heuristic, which counts no lines for a binary file.
const binarySniff = 8000

// countLines counts the lines r holds as git's numstat does: each newline, and an
// unterminated last line as one more. binary reports a NUL within its first
// binarySniff bytes, and then no lines are counted.
func countLines(r io.Reader) (lines int, binary bool, err error) {
	buf := make([]byte, 64*1024)
	read, last := 0, byte('\n')
	for {
		n, rerr := r.Read(buf)
		chunk := buf[:n]
		if read < binarySniff && bytes.IndexByte(chunk[:min(n, binarySniff-read)], 0) >= 0 {
			return 0, true, nil
		}
		lines += bytes.Count(chunk, []byte{'\n'})
		if n > 0 {
			last = chunk[n-1]
		}
		read += n
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return 0, false, rerr
		}
	}
	if last != '\n' {
		lines++
	}
	return lines, false, nil
}

// diffDetail is the diff's row: what the commit carries, then what it covers under
// mode and when that is staged.
func diffDetail(stat diffStat, mode StagingMode) string {
	return stat.String() + " · " + stagingDetail(mode)
}
