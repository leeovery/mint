package presenter

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/muesli/termenv"
	"golang.org/x/term"

	"mint/internal/presenter/rail"
)

// spinnerFrame is how long the running spinner shows each frame.
const spinnerFrame = 80 * time.Millisecond

// The rail's own words for what its payloads leave to it.
const (
	// committedAs leads the subject of a commit a stage made, hung under its node.
	committedAs = "committed as "
	// unwindName names the unwind's node.
	unwindName = "unwind"
	// flaggedNotAsked leads a suggestion the exclusion check made that no pause asked
	// about.
	flaggedNotAsked = "flagged, not asked: "
)

// Ticker starts a tick every d, until stop is called: the spinner's clock.
type Ticker func(d time.Duration) (ticks <-chan time.Time, stop func())

func newTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// RailPresenter draws a run as the Rail (design/terminal-ui.md): from the station the
// run starts at, through a node per stage, to the station it lands at. It keeps the
// run as a model built from each event's payload, which the rail package draws, and
// a liveRegion puts it on the terminal: lines above the live region settle and never
// change, and the region, from the live node down to the end station waiting, is
// redrawn in place at each event and each spinner frame.
//
// One mutex holds every write to the terminal. Each method holds it while it changes
// the run and redraws, letting it go only while a question reads its answer; the
// spinner's ticker runs only while a stage runs and nothing else holds the terminal
// (no question waiting, no editor), and takes the mutex for each frame, drawing only
// while it is still the ticker running, so nothing writes once it is stopped.
type RailPresenter struct {
	mu  sync.Mutex
	out io.Writer
	// err gets the one-line failure and warning summaries, as today's presenters'
	// do, for a stream split from out; nil writes none.
	err io.Writer
	in  io.Reader
	// reader is the one buffered reader over in that every line read shares.
	reader           *bufio.Reader
	yes              bool
	stdinInteractive bool
	// echo is whether in is a terminal, which echoes a typed line where the cursor is.
	echo  bool
	level rail.Level
	// size is the terminal's columns and rows, 0 where it gives none, read at each
	// redraw.
	size func() (cols, rows int)
	tick Ticker
	now  func() time.Time

	region *liveRegion
	run    *runModel
	layout rail.Layout
	// fits is whether the terminal had room for a region when last measured.
	fits    bool
	ticking *ticker
	// suspended is the editor holding the terminal, asking a question waiting.
	suspended, asking bool
}

// Compile-time proof the rail presenter satisfies the seam it renders.
var _ Presenter = (*RailPresenter)(nil)

// RailOption configures a RailPresenter at construction.
type RailOption func(*RailPresenter)

// WithRailErr wires the stream the failure and warning summaries go to.
func WithRailErr(err io.Writer) RailOption { return func(p *RailPresenter) { p.err = err } }

// WithRailInput wires the stream questions read their answers from, os.Stdin
// without it.
func WithRailInput(in io.Reader) RailOption { return func(p *RailPresenter) { p.in = in } }

// WithRailGating sets the -y decision and whether stdin can answer a question, an
// interactive stdin without -y without it.
func WithRailGating(yes, stdinInteractive bool) RailOption {
	return func(p *RailPresenter) { p.yes, p.stdinInteractive = yes, stdinInteractive }
}

// WithRailProfile sets the colour the run is drawn in, NO_COLOR, COLORTERM and TERM's
// without it.
func WithRailProfile(profile termenv.Profile) RailOption {
	return func(p *RailPresenter) { p.level = rail.LevelOf(profile) }
}

// WithRailSize sets where the terminal's columns and rows are read from at each
// redraw, the out terminal's without it.
func WithRailSize(size func() (cols, rows int)) RailOption {
	return func(p *RailPresenter) { p.size = size }
}

// WithRailTicker sets the spinner's clock, a ticker of real time without it.
func WithRailTicker(tick Ticker) RailOption { return func(p *RailPresenter) { p.tick = tick } }

// WithRailClock sets the time a running stage's time is read from, the wall clock
// without it.
func WithRailClock(now func() time.Time) RailOption {
	return func(p *RailPresenter) { p.now = now }
}

// NewRailPresenter is a RailPresenter drawing to out, a terminal, configured by opts.
func NewRailPresenter(out io.Writer, opts ...RailOption) *RailPresenter {
	p := &RailPresenter{
		out:              out,
		in:               os.Stdin,
		stdinInteractive: true,
		level:            rail.LevelOf(termenv.EnvColorProfile()),
		size:             terminalSize(out),
		tick:             newTicker,
		now:              time.Now,
		region:           newLiveRegion(out),
		run:              newRunModel(),
	}
	for _, opt := range opts {
		opt(p)
	}
	_, p.echo = rawKeyFile(p.in)
	return p
}

// terminalSize reads out's size where out is a terminal, and 0 by 0 otherwise.
func terminalSize(out io.Writer) func() (int, int) {
	f, ok := out.(*os.File)
	return func() (int, int) {
		if !ok {
			return 0, 0
		}
		cols, rows, err := term.GetSize(int(f.Fd()))
		if err != nil {
			return 0, 0
		}
		return cols, rows
	}
}

// RunStarted opens the run: its header, then its start station.
func (p *RailPresenter) RunStarted(info RunInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.start(info)
	p.redraw()
}

// ShowPlan lays the plan's stages out on the rail to come; a plan with no stages
// draws nothing.
func (p *RailPresenter) ShowPlan(plan Plan) {
	if len(plan.Stages) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.plan(plan)
	p.redraw()
}

// StageStarted sets the stage running: the spinner in its mark's place, what it is
// doing in its detail and, for one that takes time, its time so far, which a stage
// started again while it runs (an `r` that turns into a run in parts) keeps counting.
// An open question's own activity, as the review's edit, draws nothing.
func (p *RailPresenter) StageStarted(s StageStart) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run.activity(s.Key) {
		return
	}
	pt := p.run.stage(s.Key, s.Name)
	p.run.reach(pt)
	if pt.node.State != rail.Running {
		pt.started = p.now()
	}
	pt.doing = firstOf(s.Text, pt.plan, s.Name)
	pt.node.State, pt.node.Detail, pt.node.Time, pt.node.Progress = rail.Running, pt.doing, "", nil
	pt.timed = s.Blocking
	p.redraw()
}

// StageSucceeded settles the stage done: its detail, its time where it takes time,
// and the subject of any commit it made and its note hung under it. An open
// question's own activity draws nothing.
func (p *RailPresenter) StageSucceeded(s StageSuccess) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run.activity(s.Key) {
		return
	}
	pt := p.run.stage(s.Key, s.Name)
	p.run.ended(s.Key, true)
	n := pt.node
	n.State, n.Detail, n.Time, n.Progress = rail.Done, firstOf(s.Row, s.Detail, pt.plan), "", nil
	if s.Blocking {
		n.Time = stageTime(s.Elapsed)
	}
	if s.Commit != "" {
		n.Lines = append(n.Lines, rail.Hang{Kind: rail.Note, Text: committedAs + s.Commit})
	}
	if s.Note != "" {
		n.Lines = append(n.Lines, rail.Hang{Kind: rail.Note, Text: s.Note})
	}
	p.redraw()
}

// StageFailed marks the stage failed with its message, the command's output verbatim
// under it behind a gutter of its own and any saved part notes' path on a line of
// its own, and writes the one-line summary to err. A failure is always drawn, an
// open question's own activity's too.
func (p *RailPresenter) StageFailed(s StageFailure) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.run.stage(s.Key, s.Name).node
	p.run.ended(s.Key, false)
	n.State, n.Detail, n.Time, n.Progress = rail.Failed, s.Message, "", nil
	if s.Elapsed > 0 {
		n.Time = stageTime(s.Elapsed)
	}
	if s.Output != "" {
		n.Lines = append(n.Lines, rail.Hang{Kind: rail.Output, Text: s.Output})
	}
	if s.SavedNotes != "" {
		n.Lines = append(n.Lines, rail.Hang{Kind: rail.Note, Text: savedNotesLead}, rail.Hang{Kind: rail.Path, Text: s.SavedNotes})
	}
	p.errf("✗ %s  %s\n", s.Name, s.Message)
	p.redraw()
}

// NotesChecking runs the exclusion check as its own node, holding back the notes
// stage, which started first, until it is done.
func (p *RailPresenter) NotesChecking(c NotesCheck) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.run.running(); r != nil && r.key != c.Key {
		r.node.State, r.node.Detail = rail.ToCome, firstOf(r.plan, r.node.Detail)
		p.run.held = r
	}
	pt := p.run.stage(c.Key, c.Key)
	pt.node.State, pt.node.Detail, pt.node.Time = rail.Running, pt.plan, ""
	pt.started, pt.timed = p.now(), true
	p.redraw()
}

// NotesChecked settles the exclusion check done in its time, stating its result where
// the engine gives one and its plan where the rows after it will, and runs the stage
// it held back again. A Failed check keeps its node, timed, for the warning that
// follows to end.
func (p *RailPresenter) NotesChecked(c NotesCheck) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pt := p.run.stage(c.Key, c.Key)
	pt.timed, pt.node.Time = false, stageTime(c.Elapsed)
	if !c.Failed {
		pt.node.State, pt.node.Detail = rail.Done, firstOf(c.Detail, pt.plan)
	}
	if h := p.run.held; h != nil {
		h.node.State, h.node.Detail = rail.Running, h.doing
		p.run.held = nil
	}
	p.redraw()
}

// NotesProgress draws the running notes' bar: the parts done of the parts.
func (p *RailPresenter) NotesProgress(n NotesProgress) {
	p.progress(func(bar *rail.Progress) {
		*bar = rail.Progress{Label: fmt.Sprintf("%d of %d parts done", n.Done, n.Parts), Done: n.Done, Total: n.Parts}
	})
}

// NotesHalving names the re-cut as the bar's label.
func (p *RailPresenter) NotesHalving(h NotesHalving) {
	p.progress(func(bar *rail.Progress) {
		bar.Label = halvingText(h)
		if bar.Total == 0 {
			bar.Total = h.Parts
		}
	})
}

// NotesMerging names the merge as the bar's label, every part done.
func (p *RailPresenter) NotesMerging(m NotesMerge) {
	p.progress(func(bar *rail.Progress) {
		*bar = rail.Progress{Label: fmt.Sprintf("merging %d parts", m.Parts), Done: m.Parts, Total: m.Parts}
	})
}

// progress sets the running node's bar; with no node running it draws nothing.
func (p *RailPresenter) progress(set func(*rail.Progress)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pt := p.run.running()
	if pt == nil {
		return
	}
	var bar rail.Progress
	if pt.node.Progress != nil {
		bar = *pt.node.Progress
	}
	set(&bar)
	pt.node.Progress = &bar
	p.run.touched = pt
	p.redraw()
}

// NextMoves keeps the moves for next, under the end station once the run is over.
func (p *RailPresenter) NextMoves(m NextMoves) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, move := range m.Moves {
		p.run.keep(&p.run.moves, rail.Move{Lead: move.Lead, Command: move.Command})
	}
}

// ExclusionSuggestions draws each suggestion no pause asked about as a warning above
// the stage it came from, and keeps it, with the diff_exclude line that acts on it,
// for next after the failure's moves.
func (p *RailPresenter) ExclusionSuggestions(s ExclusionSuggestions) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, x := range s.Suggestions {
		p.run.flag(s.Stage, rail.Node{State: rail.Warning, Name: x.Glob, Detail: flaggedNotAsked + about(x)})
		p.run.keep(&p.run.suggested, rail.Move{
			Lead:    fmt.Sprintf("Leave %s out of the notes, under %s in %s:", x.Glob, s.Table, s.File),
			Command: x.Line,
		})
	}
	p.redraw()
}

// about is what a suggestion is about: the AI's reason, then the files it matches
// and their size.
func about(x ExclusionSuggestion) string {
	return x.Reason + " · " + x.Matches
}

// ExclusionAnswersRecorded hangs where the answers went under the exclusion check's
// node, below the answers.
func (p *RailPresenter) ExclusionAnswersRecorded(a ExclusionAnswers) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.hang(a.Key, rail.Hang{Kind: rail.Note, Text: exclusionAnswersText(a)})
	p.redraw()
}

// Warn draws a warning as a node, a Failed one as its key's failed node, and writes
// its summary to err. A Skipped one draws nothing: its stage stays to come.
func (p *RailPresenter) Warn(w Warning) {
	if w.Skipped {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var n *rail.Node
	if w.Failed {
		n = p.run.stage(w.Key, w.Key).node
		n.State = rail.Failed
	} else {
		n = p.warned(w.Key, w.Label)
		n.State = rail.Warning
	}
	p.run.ended(w.Key, false)
	n.Detail, n.Progress = firstOf(w.Row, w.Message), nil
	if w.Output != "" {
		n.Lines = append(n.Lines, rail.Hang{Kind: rail.Output, Text: w.Output})
	}
	p.errf("⚠ %s\n", warnText(w))
	p.redraw()
}

// warned is the node a warning about the stage keyed key draws as: the stage's own
// where it is running or still to come, which the warning ends; else a node of its
// own, labelled, under the stage's node where that is done, which it leaves as it
// was, and at the run's edge where it has settled or there is none.
func (p *RailPresenter) warned(key, label string) *rail.Node {
	pt := p.run.find(key)
	switch {
	case pt != nil && pt.node.State == rail.Done:
		return p.run.aside(pt, rail.Node{Name: label}).node
	case pt == nil && p.run.gone[key]:
		return p.run.add(rail.Node{Name: label}).node
	}
	return p.run.stage(key, label).node
}

// Unwound draws the unwind's node: what it undid, and what it kept under it. An
// unwind that couldn't finish is already drawn, as the failed nodes of its steps
// keyed as it is, so it hangs what it undid and what it kept under the last. Where it
// left the repo is a claim about the tree the rail doesn't make; a step left to do by
// hand reaches next.
func (p *RailPresenter) Unwound(u Unwind) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var kept []rail.Hang
	if u.Kept != "" {
		kept = append(kept, rail.Hang{Kind: rail.Note, Text: u.Kept})
	}
	if failed := p.run.find(u.Key); failed != nil && failed.node.State == rail.Failed {
		if u.Undone != "" {
			failed.node.Lines = append(failed.node.Lines, rail.Hang{Kind: rail.Note, Text: u.Undone})
		}
		failed.node.Lines = append(failed.node.Lines, kept...)
		p.run.touched = failed
	} else {
		p.run.add(rail.Node{State: rail.Unwind, Name: unwindName, Detail: u.Undone, Lines: kept})
	}
	p.redraw()
}

// ShowNotes shows the notes for review, hanging off the rail above the question that
// follows.
func (p *RailPresenter) ShowNotes(notes Notes) { p.show(notes.Body) }

// ShowMessage shows the message for review, as ShowNotes does the notes.
func (p *RailPresenter) ShowMessage(m Message) { p.show(m.Body) }

func (p *RailPresenter) show(body string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.show(body)
	p.redraw()
}

// ShowVersion draws the version line.
func (p *RailPresenter) ShowVersion(v Version) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.settleItem(rail.Version{Value: v.Value})
	p.redraw()
}

// InitResult draws a row for the file: done where it was created, dim where it was
// left alone, with why.
func (p *RailPresenter) InitResult(r InitOutcome) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := rail.Node{Name: r.Target, Detail: r.Action.String()}
	if r.Action == InitSkipped {
		n.State, n.Detail = rail.LeftAlone, r.Reason
	}
	p.run.add(n)
	p.run.touched = nil
	p.redraw()
}

// SuspendSpinner takes the live region off the terminal, and stops the spinner, for
// the editor to have it.
func (p *RailPresenter) SuspendSpinner() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.suspended = true
	p.region.Erase()
	p.syncTicker()
}

// ResumeSpinner draws the live region again once the editor is done with the
// terminal, its spinner running again.
func (p *RailPresenter) ResumeSpinner() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.suspended = false
	p.redraw()
}

// RunFinished closes the run at its end station, with its time and any URL under
// it; next follows once the drawing ends, under whatever arrives after the close.
// What it never reached leaves the drawing with the live region, but for a dry run's
// stages to come.
func (p *RailPresenter) RunFinished(r RunResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	end := rail.End{Landed: landed(r.Outcome), Name: r.End.Name, Detail: r.End.Detail, Time: stageTime(r.Elapsed), URL: r.URL}
	p.run.finish(end, r.Outcome == OutcomeDryRun)
	p.redraw()
}

// landed is whether a run that ended as o reached the station it set out for.
func landed(o Outcome) bool {
	switch o {
	case OutcomeDryRun, OutcomeDeclined, OutcomeUnwound, OutcomeFailed:
		return false
	}
	return true
}

// Close ends the presenter's drawing: a run still open, one no RunFinished closed,
// settles as a run that never landed, next settles last, and the spinner stops for
// good. Whatever presents after it settles straight away.
func (p *RailPresenter) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run.open {
		p.run.finish(rail.End{}, false)
	}
	p.run.closed = true
	p.run.settleNext()
	p.redraw()
}

// minRegionRows is the fewest rows a terminal must have, where it gives its height,
// to hold a live region: a shorter one can't keep the run's edge and a question
// under it on screen while the question waits.
const minRegionRows = 12

// redraw draws the run at the terminal's size now.
func (p *RailPresenter) redraw() { p.draw(p.measure()) }

// draw draws the run, the terminal rows tall as last measured: what has settled
// since the last draw, then the live region. With no room for a region, a terminal
// too narrow or too short, the run prints as it happens instead. It starts or stops
// the spinner to match.
func (p *RailPresenter) draw(rows int) {
	p.run.pass()
	var run rail.Run
	var region []string
	if p.fits {
		p.run.settleFinished()
		run, region = p.fit(rows)
	} else {
		p.run.log()
		run = p.drawing()
	}
	p.region.Draw(p.layout.Settled(run, p.run.printed), region, rows)
	p.run.printed = len(run.Settled)
	p.syncTicker()
}

// measure reads the terminal's size and lays the run out for it, settling whether it
// has room for a region, and returns its rows.
func (p *RailPresenter) measure() int {
	cols, rows := p.size()
	p.layout = rail.NewLayout(cols, p.level)
	p.fits = p.layout.RegionFits() && (rows <= 0 || rows >= minRegionRows)
	return rows
}

// fit is the run and its region, a region too tall for the terminal giving way: the
// plan still to come at its foot first, which Draw cuts, then its top, settling what
// can settle early, until what the run has reached fits.
func (p *RailPresenter) fit(rows int) (rail.Run, []string) {
	for {
		run := p.drawing()
		reached := p.layout.Region(p.run.reached(p.now()))
		if len(reached) <= regionRows(rows) || !p.run.settleFirst() {
			return run, p.layout.Region(run)
		}
	}
}

func (p *RailPresenter) drawing() rail.Run { return p.run.drawing(p.now()) }

// ticker is the spinner's ticker running: stop ends its ticks, done its goroutine.
type ticker struct {
	stop func()
	done chan struct{}
}

// syncTicker runs the spinner's ticker while a stage runs in a region drawn, the
// run open and nothing else holding the terminal, and stops it otherwise.
func (p *RailPresenter) syncTicker() {
	want := p.fits && !p.run.closed && !p.suspended && !p.asking && p.run.running() != nil
	switch {
	case want && p.ticking == nil:
		ticks, stop := p.tick(spinnerFrame)
		p.ticking = &ticker{stop: stop, done: make(chan struct{})}
		go p.spin(p.ticking, ticks)
	case !want && p.ticking != nil:
		p.ticking.stop()
		close(p.ticking.done)
		p.ticking = nil
	}
}

// spin redraws at each tick, the spinner's frame and the running stage's time read
// from the clock, for as long as t is the ticker running. A tick it takes after t has
// stopped finds it no longer running and draws nothing.
func (p *RailPresenter) spin(t *ticker, ticks <-chan time.Time) {
	for {
		select {
		case <-t.done:
			return
		case <-ticks:
			p.mu.Lock()
			if p.ticking == t {
				p.redraw()
			}
			p.mu.Unlock()
		}
	}
}

// errf writes a summary line to err, unstyled: err is a redirect-visibility channel.
func (p *RailPresenter) errf(format string, args ...any) {
	if p.err == nil {
		return
	}
	_, _ = fmt.Fprintf(p.err, format, args...)
}

// firstOf is the first of texts that isn't empty.
func firstOf(texts ...string) string {
	for _, t := range texts {
		if t != "" {
			return t
		}
	}
	return ""
}
