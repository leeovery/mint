// Package rail draws a run as the Rail: a line from the station the run starts at,
// through a node per stage, to the station it lands at. It is pure: a model of the
// drawing in, styled terminal lines out, at a width and a colour level. It knows
// nothing of the presenter's events; every word it draws but its own frame (the
// point of no return, next) comes in the model.
//
// A Run is two lists of items: the settled ones, which never change once printed,
// and the live region below them, the live node down to the end station, which is
// redrawn in place. The rail between items is drawn from the items themselves.
package rail

// Run is a run's drawing: its Settled items, then its Region.
type Run struct {
	Settled []Item
	Region  []Item
}

// Item is one piece of the drawing: a Header, a Station, a Node, a Question, the
// Rule, a Body or Next.
type Item interface {
	draw(l Layout) []line
	// reached is whether the run has come this far: the rail into an item not yet
	// reached is dim.
	reached() bool
}

// Header is the run's opening line: "mint ─ {Command}", and on the right
// "{Where} · v{Version}". An empty Version, a dev build's, shows none.
type Header struct {
	Command string
	Where   string
	Version string
}

// StationKind is where a station sits on the run.
type StationKind int

const (
	// Start is where the run begins.
	Start StationKind = iota
	// Pending is the end station waiting, dim, at the foot of the plan.
	Pending
	// Landed is the end station of a run that landed: ◆.
	Landed
	// Stopped is the end station of a run that didn't: back at the start, nothing
	// changed, or a dry run.
	Stopped
)

// Station is a point the rail starts from or ends at. Time is the run's, on the
// end station; URL, under the end station, is never cut or wrapped.
type Station struct {
	Kind   StationKind
	Name   string
	Detail string
	Time   string
	URL    string
}

// State is what a node's mark says.
type State int

const (
	// Done is a stage done.
	Done State = iota
	// Running is the stage running: a spinner in its mark's place, its line cut to
	// the width rather than wrapped.
	Running
	// ToCome is a stage still to come; the whole row is dim.
	ToCome
	// Failed is a stage failed; its name is red and the rail down its lines too.
	Failed
	// Warning needs a look: a warning, a skipped version, a suggestion not asked
	// about.
	Warning
	// Unwind is the unwind, naming what it undid.
	Unwind
)

// Node is a stage on the rail: its mark, Name, Detail, then Tail (the answer, dim,
// after " · "), with Time dim on the right. A Running node shows Frame, the spinner's
// frame, as its mark and its Detail dim, or Progress where it is set. Lines hang
// off the node under it.
type Node struct {
	State    State
	Name     string
	Detail   string
	Tail     string
	Time     string
	Frame    int
	Progress *Progress
	Lines    []Hang
}

// Progress is a running stage's bar: Label, then Done of Total filled.
type Progress struct {
	Label string
	Done  int
	Total int
}

// HangKind is what a line hanging off a node holds.
type HangKind int

const (
	// Note is mint's own words, dim.
	Note HangKind = iota
	// Output is a command's own output, verbatim behind a gutter of its own.
	Output
	// Path is a path, on a line of its own.
	Path
)

// Hang is text hanging off a node, at the detail column, wrapped to the width with
// the rail on every line. A newline in Text starts a line.
type Hang struct {
	Kind HangKind
	Text string
}

// Question is a question node: Text bold after its Name (where it has one, as the
// exclusion check's glob), then each choice's key bold in the accent with its
// action. A dry run's preview has no Choices.
type Question struct {
	Name    string
	Text    string
	Choices []Choice
}

// Choice is a key that answers a question and what it does.
type Choice struct {
	Key    string
	Action string
}

// Rule is the point of no return drawn across the rail, its ┿ in the accent once
// the run has Crossed it.
type Rule struct {
	Crossed bool
}

// Body is a body shown for review (release notes, a commit message), four in behind
// the rail, verbatim but for the wrap.
type Body struct {
	Text string
}

// Next is what the reader can do once the run is over, after the end station.
type Next struct {
	Moves []Move
}

// Move is one way forward: a Lead, then its Command, alone on its line in the
// accent, never cut or wrapped, so it copies clean. A move with no Command is its
// Lead alone.
type Move struct {
	Lead    string
	Command string
}
