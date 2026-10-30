// Package rail draws a run as the Rail: a line from the station the run starts at,
// through a node per stage, to the station it lands at. It is pure: a model of the
// drawing in, styled terminal lines out, at a width and a colour level. It knows
// nothing of the presenter's events; every word it draws but its own frame (the
// point of no return, next, the wordmark) comes in the model.
//
// A Run is two lists of items: the settled ones, which never change once printed,
// and the live region below them, the live node down to the end station waiting,
// which is redrawn in place. Every line the region draws is one terminal row: text
// from the model is wrapped or cut to the width, and drawn so that nothing in it can
// move the cursor or change the pen (see shown). The rail between items is drawn
// from the items themselves; a run with no station, as init's, has none.
package rail

// Run is a run's drawing: its Settled items, then its Region.
type Run struct {
	Settled []Item
	Region  []Live
}

// Item is one piece of the drawing: a Header, a Station, a Node, Hanging lines, a
// Question, the Rule, a Body, an End, Next or a Version.
type Item interface {
	draw(l Layout) []line
	// reached is whether the run has come this far: the rail into an item, or out
	// of one, not yet reached is dim.
	reached() bool
}

// Live is an item the live region may hold: anything but the header, the end a run
// landed at, next and the version, which are only ever settled. The end's URL and
// next's commands are never cut, so they may be wider than a row.
type Live interface {
	Item
	live()
}

// Header is the run's opening line: "mint ─ {Command}", and on the right
// "{Where} · v{Version}". An empty Version, a dev build's, shows none.
type Header struct {
	Command string
	Where   string
	Version string
}

// Version is the version line: "mint v{Value}".
type Version struct {
	Value string
}

// StationKind is which station a Station is.
type StationKind int

const (
	// Start is where the run begins.
	Start StationKind = iota
	// Pending is the end station waiting, dim, at the foot of the plan.
	Pending
)

// Station is the station a run starts at, or the end station waiting for it.
type Station struct {
	Kind   StationKind
	Name   string
	Detail string
}

// End is the station a run ended at: ◆ where it Landed, ◇ where it didn't (back at
// the start, nothing changed, a dry run). Time is the run's; URL, under it, is never
// cut or wrapped.
type End struct {
	Landed bool
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
	// LeftAlone is a file init left as it was: the whole row dim, as one to come is,
	// but the run is past it.
	LeftAlone
)

// Node is a stage on the rail: its mark, Name, Detail, then Tail (the answer, dim,
// after " · "), with Time dim on the right. A Detail's first line is the detail and
// any after it hang under the node. A Running node shows Frame, the spinner's frame,
// as its mark and its Detail dim, or Progress where it is set. Lines hang off the
// node under it.
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
	// Note is metadata, dim: what a stage passed over, a reason, a size, a count.
	Note HangKind = iota
	// Did is what mint did, in the normal colour: a commit it made, a file it wrote,
	// what an unwind undid.
	Did
	// Output is a command's own output, verbatim behind a gutter of its own.
	Output
	// Path is a path, on a line of its own.
	Path
	// Acted is an answer the run acts on, given to a question under the node (an
	// exclusion check's exclude), in the question's place: ✓ in the accent where its ?
	// stood, then Text, About dim.
	Acted
	// Kept is an answer that changes nothing (an exclusion check's keep), drawn as
	// Acted is but marked –, and all of it dim.
	Kept
)

// Hang is text hanging off a node, at the detail column, wrapped to the width with
// the rail on every line. A newline in Text starts a line. An answer, Acted or Kept,
// has what it is About dim after " · ", and wraps under its Text, clear of its mark.
type Hang struct {
	Kind  HangKind
	Text  string
	About string
}

// Hanging is lines hanging off a node that settled before they came, as an answer
// the exclusion check is given once its node has gone up the scrollback: the solid
// rail, and each line as the node would hang it, under what the node drew last.
type Hanging struct {
	Lines []Hang
}

// Question is a question node: Text bold, then, on a line of their own, each
// choice's key bold in the accent with its action. A dry run's preview has no
// Choices, and a Tail, dim after " · " at the end of its last line, that says so. A
// question Typing reads a line, typed on its last line, on a cell for the cursor
// after the input marker. A question Under the node above it is that node's
// sub-matter, as the exclusion check's are: no node of its own, but lines hanging off
// the node's rail, Text after a ? in the accent, then, indented, About, dim, and the
// keys. While it is asked, its Choices or Typing set, a branch joins its ? to the
// rail.
type Question struct {
	Text    string
	About   string
	Choices []Choice
	Tail    string
	Typing  bool
	Under   bool
}

// Choice is a key that answers a question and what it does, with what that is About,
// where it says more, dim in parentheses after it. The Default, which Enter takes, is
// marked as Y/n marks it: a key of one character drawn as a capital, a word with a
// dim "(default)" after its action.
type Choice struct {
	Key     string
	Action  string
	About   string
	Default bool
}

// Rule is the point of no return drawn across the rail: the rail into it solid once
// the run has Reached it, its stage under way or stopped there, and its ┿ in the
// accent once the run has Crossed it.
type Rule struct {
	Reached bool
	Crossed bool
}

// Body is a body shown for review (release notes, a commit message), four in behind
// the rail, as it ships but for the wrap, or, Markdown, as its Markdown reads (see
// markdown).
type Body struct {
	Text     string
	Markdown bool
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

func (Station) live()  {}
func (Node) live()     {}
func (Question) live() {}
func (Hanging) live()  {}
func (Rule) live()     {}
func (Body) live()     {}
