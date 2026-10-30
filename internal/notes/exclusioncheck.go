package notes

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"mint/internal/ai"
	"mint/internal/bytesize"
)

// CheckNarration is told when the exclusion check's call starts and when it ends, with
// what it found, when the keep list covers every changed path so there is nothing to
// call about, and when the check failed: after the call ends when the call failed or
// its answer was unreadable, and with no call at all when the call would not fit under
// the cap. output is the AI's captured output, or its unreadable answer, verbatim but
// for trailing whitespace, and empty when there is none. CheckDone is told, once a
// call has started, that the check is over and the notes go on: after the call where
// the check asks nothing, after the last pause's answers are recorded where it does;
// never where Check returns an error.
type CheckNarration interface {
	Checking()
	Checked(result CheckResult)
	AllKept()
	CheckFailed(output string)
	CheckDone()
}

// CheckResult is what the check's call found: how it ended and, for a call whose
// answer suggested paths to leave out, how many of those the run goes on to ask about
// or report, and whether it asks. With pauses wired, a suggestion matching only paths
// the keep list or a suggestion before it covers is never asked about, whatever the
// answers before it, so it is not counted; a report counts every usable suggestion.
type CheckResult struct {
	Outcome CheckOutcome
	Flagged int
	Asked   bool
}

// CheckOutcome is how the check's call ended.
type CheckOutcome int

const (
	// CheckAnswered is a call whose answer suggested paths to leave out, or one
	// cancelled.
	CheckAnswered CheckOutcome = iota
	// CheckClear is a call answered "none": nothing to leave out.
	CheckClear
	// CheckUnusable is a call that failed or whose answer yielded no usable suggestion,
	// its failure narrated next.
	CheckUnusable
)

type noCheckNarration struct{}

func (noCheckNarration) Checking()           {}
func (noCheckNarration) Checked(CheckResult) {}
func (noCheckNarration) AllKept()            {}
func (noCheckNarration) CheckFailed(string)  {}
func (noCheckNarration) CheckDone()          {}

// ExclusionCheck asks the AI which of a release's changed paths look out of place in
// its notes. Its call carries no diff, only the changed paths the notes diff carries
// that the keep list does not cover.
type ExclusionCheck struct {
	transport Transport
	keep      []string
	narration CheckNarration
	pauses    SuggestionPauses
	report    func(Suggestion, Matches)
	record    func(context.Context, Answers) error
}

// Suggestion is a glob the check's answer suggests leaving out of the notes, with the
// AI's reason for it.
type Suggestion struct {
	Glob   string
	Reason string
}

// Answer is the operator's decision at a suggestion's pause.
type Answer int

const (
	AnswerKeep Answer = iota
	AnswerExclude
)

// Matches is what an Exclude answer to a suggestion would take out of the notes.
type Matches struct {
	Paths []string
	// DiffBytes is the total of Paths' diff sizes.
	DiffBytes int
	// Kept is those of Paths the keep list, or a Keep answer earlier in the run, covers.
	Kept []string
}

// Answers are a run's answers at its pauses, each list's globs in the order answered.
type Answers struct {
	Exclude []string
	Keep    []string
}

// SuggestionPauses asks the operator about one suggestion and returns the answer.
type SuggestionPauses interface {
	Pause(ctx context.Context, s Suggestion, m Matches) (Answer, error)
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

// WithPauses returns a copy of c that pauses on each suggestion the answer yields; c
// itself is unchanged.
func (c *ExclusionCheck) WithPauses(pauses SuggestionPauses) *ExclusionCheck {
	d := *c
	d.pauses = pauses
	return &d
}

// WithReport returns a copy of c that, unless pauses are wired, hands report each
// suggestion matching a path the list covers, in the answer's order, with what it matches
// against the keep list alone; c itself is unchanged.
func (c *ExclusionCheck) WithReport(report func(Suggestion, Matches)) *ExclusionCheck {
	d := *c
	d.report = report
	return &d
}

// WithRecord returns a copy of c that, once its last pause is answered, hands record
// the run's answers, unless no pause was answered; c itself is unchanged. An error
// record returns is Check's.
func (c *ExclusionCheck) WithRecord(record func(context.Context, Answers) error) *ExclusionCheck {
	d := *c
	d.record = record
	return &d
}

// Check makes the check's call over diff, the post-exclusion notes diff, unless the
// keep list covers every path in it: then it makes no call and narrates only that. The call's
// list condenses by folder past a tenth of maxPromptBytes, and a call that would still
// exceed maxPromptBytes is never made. A failed check, the call not fitting or failing
// or its answer neither "none" nor yielding one usable suggestion, is narrated and
// leaves the release to carry on, so only a cancel is returned. With pauses wired, each
// suggestion matching a path the list covers is paused on in the answer's order, unless
// the suggestions before it already cover every such path it matches; a pause's error, or a
// cancel while it waits, is returned and ends the pauses. It returns the globs answered
// Exclude, in the order answered; a report excludes nothing.
func (c *ExclusionCheck) Check(ctx context.Context, diff string, maxPromptBytes int) ([]string, error) {
	files := diffPaths(diff)
	paths := slices.DeleteFunc(slices.Clone(files), func(p checkedPath) bool { return coveredBy(c.keep, p.path) })
	if len(paths) == 0 {
		c.narration.AllKept()
		return nil, nil
	}
	prompt := composeCheckPrompt(checkList(paths, maxPromptBytes))
	if len(prompt) > maxPromptBytes {
		c.narration.CheckFailed("")
		return nil, nil
	}
	c.narration.Checking()
	excluded, err := c.call(ctx, prompt, paths, files)
	if err == nil {
		c.narration.CheckDone()
	}
	return excluded, err
}

// call makes the check's call with prompt over paths, of files the notes diff's
// changed paths, and pauses on or reports what its answer suggests.
func (c *ExclusionCheck) call(ctx context.Context, prompt string, paths, files []checkedPath) ([]string, error) {
	answer, err := c.transport.Generate(ctx, prompt)
	outcome, suggestions := checkOutcome(answer, err, paths)
	if c.pauses != nil {
		suggestions = c.undecided(suggestions, files)
	}
	c.narration.Checked(CheckResult{Outcome: outcome, Flagged: len(suggestions), Asked: c.pauses != nil})
	switch {
	case errors.Is(err, context.Canceled):
		return nil, err
	case err != nil:
		c.narration.CheckFailed(failureOutput(err))
		return nil, nil
	case outcome == CheckUnusable:
		c.narration.CheckFailed(strings.TrimRightFunc(answer, unicode.IsSpace))
		return nil, nil
	case outcome == CheckClear:
		return nil, nil
	}
	switch {
	case c.pauses != nil:
		return c.pause(ctx, suggestions, files)
	case c.report != nil:
		c.reportAll(suggestions, files)
	}
	return nil, nil
}

// checkOutcome is how the check's call over paths ended, answer or err, with the
// usable suggestions of an answer that has any.
func checkOutcome(answer string, err error, paths []checkedPath) (CheckOutcome, []Suggestion) {
	switch {
	case errors.Is(err, context.Canceled):
		return CheckAnswered, nil
	case err != nil:
		return CheckUnusable, nil
	case strings.EqualFold(strings.TrimSpace(answer), "none"):
		return CheckClear, nil
	}
	if suggestions := usableSuggestions(answer, paths); len(suggestions) > 0 {
		return CheckAnswered, suggestions
	}
	return CheckUnusable, nil
}

func failureOutput(err error) string {
	var genErr *ai.GenerationError
	if errors.As(err, &genErr) {
		return genErr.Output()
	}
	return ""
}

func (c *ExclusionCheck) reportAll(suggestions []Suggestion, files []checkedPath) {
	unanswered := runAnswers{keep: c.keep}
	for _, s := range suggestions {
		c.report(s, unanswered.matches(s.Glob, files))
	}
}

// undecided is those of suggestions the pauses ask about, in order: each matching a
// path of files neither the keep list nor a suggestion before it covers. Whatever a
// pause answers, the paths its glob matches are decided from then on, so which are
// asked is known before the first is.
func (c *ExclusionCheck) undecided(suggestions []Suggestion, files []checkedPath) []Suggestion {
	decided := slices.Clip(c.keep)
	var asked []Suggestion
	for _, s := range suggestions {
		if slices.ContainsFunc(files, func(p checkedPath) bool { return pathspecMatches(s.Glob, p.path) && !coveredBy(decided, p.path) }) {
			asked = append(asked, s)
		}
		decided = append(decided, s.Glob)
	}
	return asked
}

// pause asks about each of suggestions, those undecided, in order.
func (c *ExclusionCheck) pause(ctx context.Context, suggestions []Suggestion, files []checkedPath) ([]string, error) {
	decided := runAnswers{keep: slices.Clip(c.keep)}
	for _, s := range suggestions {
		answer, err := c.pauses.Pause(ctx, s, decided.matches(s.Glob, files))
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			return nil, err
		}
		decided.record(s.Glob, answer)
	}
	if c.record != nil && decided.answered() {
		if err := c.record(ctx, Answers{Exclude: decided.excluded, Keep: decided.kept}); err != nil {
			return nil, err
		}
	}
	return decided.excluded, nil
}

// runAnswers is the keep list grown by this run's Keep answers, and the globs of its
// Exclude answers and its Keep answers.
type runAnswers struct {
	keep     []string
	excluded []string
	kept     []string
}

func (a *runAnswers) record(glob string, answer Answer) {
	if answer == AnswerExclude {
		a.excluded = append(a.excluded, glob)
		return
	}
	a.keep = append(a.keep, glob)
	a.kept = append(a.kept, glob)
}

func (a *runAnswers) answered() bool {
	return len(a.excluded) > 0 || len(a.kept) > 0
}

func (a *runAnswers) matches(glob string, files []checkedPath) Matches {
	var m Matches
	for _, p := range files {
		if !pathspecMatches(glob, p.path) || coveredBy(a.excluded, p.path) {
			continue
		}
		m.Paths = append(m.Paths, p.path)
		m.DiffBytes += p.diffBytes
		if coveredBy(a.keep, p.path) {
			m.Kept = append(m.Kept, p.path)
		}
	}
	return m
}

func coveredBy(specs []string, path string) bool {
	return slices.ContainsFunc(specs, func(spec string) bool { return pathspecMatches(spec, path) })
}

// usableSuggestions reads answer's "glob: reason" lines, keeping each suggestion whose
// glob matches at least one of paths.
func usableSuggestions(answer string, paths []checkedPath) []Suggestion {
	var suggestions []Suggestion
	for line := range strings.Lines(answer) {
		glob, reason, ok := strings.Cut(strings.TrimSpace(line), ": ")
		glob = strings.TrimSpace(glob)
		if !ok || glob == "" {
			continue
		}
		if !slices.ContainsFunc(paths, func(p checkedPath) bool { return pathspecMatches(glob, p.path) }) {
			continue
		}
		suggestions = append(suggestions, Suggestion{Glob: glob, Reason: strings.TrimSpace(reason)})
	}
	return suggestions
}

// checkedPath is one changed path the check lists.
type checkedPath struct {
	path string
	// change is how the path changed, in the words the list gives it.
	change    string
	diffBytes int
}

func diffPaths(diff string) []checkedPath {
	var paths []checkedPath
	for _, file := range splitFileDiffs(diff) {
		paths = append(paths, checkedPath{path: headerPath(file), change: fileChange(file), diffBytes: len(file)})
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

// checkList lists paths one per line, condensed by folder once the list would take more
// than its share of maxPromptBytes.
func checkList(paths []checkedPath, maxPromptBytes int) pathList {
	lines := make([]foldLine, len(paths))
	for i, p := range paths {
		lines[i] = foldLine{
			text:  p.path + " (" + p.change + ", " + bytesize.Format(p.diffBytes) + ")",
			run:   runOf("", p.path),
			bytes: p.diffBytes,
		}
	}
	full := renderCheckList(lines)
	share := condenseShare(maxPromptBytes)
	if len(full) <= share {
		return pathList{text: full}
	}
	return pathList{text: renderCheckList(condenseRuns(lines, len(full)-share, summarizeCheckRun)), condensed: true}
}

// pathList is the check's rendered list, and whether it took more than its share, so
// folder summaries may stand in for some of its paths.
type pathList struct {
	text      string
	condensed bool
}

func summarizeCheckRun(key runKey, count, bytes int) string {
	return key.folder + "/: " + groupThousands(count) + " files, " + bytesize.Format(bytes)
}

func renderCheckList(lines []foldLine) string {
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = "- " + l.text
	}
	return strings.Join(texts, "\n")
}

func composeCheckPrompt(list pathList) string {
	intro := "Each line below is one changed path, with its change and the size of its diff"
	if list.condensed {
		intro += ", or one folder's changed paths summed up as how many and the total size of their diffs"
	}
	return "A software release's notes are about to be written from its diff. Release notes are a public list of what the release holds, for the people who use it. " +
		"Before they are written, check the release's changed paths for any that do not belong in its notes: " +
		"files about how the software is made rather than what it does, such as design documents, specs and plans, " +
		"tests and their fixtures, agent and tooling configuration, transcripts, and generated or vendored files.\n" +
		"\n" +
		intro + ":\n" +
		"\n" +
		list.text + "\n" +
		"\n" +
		"Suggest git pathspec globs that cover the paths that do not belong. A glob ending in / covers a whole folder, " +
		"and * matches across folders. Prefer one folder glob to a glob per file where a folder holds them.\n" +
		"\n" +
		"Answer with one suggestion per line, written as the glob, a colon and a space, then a short reason, and nothing else. " +
		"If every path belongs in the notes, answer with exactly: none"
}
