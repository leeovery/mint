package notes

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"mint/internal/bytesize"
)

// CheckNarration is told when the exclusion check's call starts and when it ends.
type CheckNarration interface {
	Checking()
	Checked()
}

type noCheckNarration struct{}

func (noCheckNarration) Checking() {}
func (noCheckNarration) Checked()  {}

// ExclusionCheck asks the AI which of a release's changed paths look out of place in
// its notes. Its call carries no diff, only the changed paths the notes diff carries
// that the keep list does not cover, each with its change and the size of its diff.
type ExclusionCheck struct {
	transport Transport
	keep      []string
	narration CheckNarration
}

// NewExclusionCheck builds the check over transport, leaving out the paths the keep
// list's pathspec globs cover and narrating its call to narration. A nil narration
// narrates nowhere.
func NewExclusionCheck(transport Transport, keep []string, narration CheckNarration) *ExclusionCheck {
	if narration == nil {
		narration = noCheckNarration{}
	}
	return &ExclusionCheck{transport: transport, keep: keep, narration: narration}
}

// Check makes the check's call over diff, the post-exclusion notes diff. A failed call
// leaves the release to carry on, so only a cancel is returned.
func (c *ExclusionCheck) Check(ctx context.Context, diff string) error {
	c.narration.Checking()
	_, err := c.transport.Generate(ctx, composeCheckPrompt(c.uncoveredPaths(diff)))
	c.narration.Checked()
	if errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// checkedPath is one changed path the check lists.
type checkedPath struct {
	path string
	// change is how the path changed, in the words the list gives it.
	change    string
	diffBytes int
}

func (c *ExclusionCheck) uncoveredPaths(diff string) []checkedPath {
	var paths []checkedPath
	for _, file := range splitFileDiffs(diff) {
		path := headerPath(file)
		if slices.ContainsFunc(c.keep, func(spec string) bool { return pathspecMatches(spec, path) }) {
			continue
		}
		paths = append(paths, checkedPath{path: path, change: fileChange(file), diffBytes: len(file)})
	}
	return paths
}

// fileChange reads a file section's change from its extended header lines, which
// precede its first hunk.
func fileChange(fileDiff string) string {
	_, rest, _ := strings.Cut(fileDiff, "\n")
	for rest != "" && !strings.HasPrefix(rest, "@@") {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		switch {
		case strings.HasPrefix(line, "new file mode "):
			return "added"
		case strings.HasPrefix(line, "deleted file mode "):
			return "deleted"
		case strings.HasPrefix(line, "rename from "):
			from := strings.TrimPrefix(line, "rename from ")
			if unquoted, err := strconv.Unquote(from); err == nil {
				from = unquoted
			}
			return "renamed from " + from
		}
	}
	return "modified"
}

func composeCheckPrompt(paths []checkedPath) string {
	lines := make([]string, len(paths))
	for i, p := range paths {
		lines[i] = "- " + p.path + " (" + p.change + ", " + bytesize.Format(p.diffBytes) + ")"
	}
	return "A software release's notes are about to be written from its diff. Release notes are a public list of what the release holds, for the people who use it. " +
		"Before they are written, check the release's changed paths for any that do not belong in its notes: " +
		"files about how the software is made rather than what it does, such as design documents, specs and plans, " +
		"tests and their fixtures, agent and tooling configuration, transcripts, and generated or vendored files.\n" +
		"\n" +
		"Each line below is one changed path, with its change and the size of its diff:\n" +
		"\n" +
		strings.Join(lines, "\n") + "\n" +
		"\n" +
		"Suggest git pathspec globs that cover the paths that do not belong. A glob ending in / covers a whole folder, " +
		"and * matches across folders. Prefer one folder glob to a glob per file where a folder holds them.\n" +
		"\n" +
		"Answer with one suggestion per line, written as the glob, a colon and a space, then a short reason, and nothing else. " +
		"If every path belongs in the notes, answer with exactly: none"
}
