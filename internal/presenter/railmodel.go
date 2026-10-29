package presenter

import (
	"fmt"
	"slices"
	"time"

	"mint/internal/presenter/rail"
)

// partKind is what a part of the live region is.
type partKind int

const (
	// nodePart is a node: a stage, a warning, the unwind.
	nodePart partKind = iota
	// rulePart is the point of no return.
	rulePart
	// endPart is the end station waiting at the foot of the plan.
	endPart
	// bodyPart is a body shown for review.
	bodyPart
	// questionPart is a question waiting for its answer.
	questionPart
	// previewPart is a question a dry run shows without asking it.
	previewPart
)

// part is one piece of the live region as the presenter keeps it, the rail item it
// draws as made from it at each redraw.
type part struct {
	kind partKind
	// key is the plan stage a node fills, "" for none.
	key  string
	node *rail.Node
	// plan is what a planned node will do, its detail while it is to come.
	plan string
	// doing is a running node's text, kept while the exclusion check holds it back.
	doing string
	// started is when the node began running: its spinner's frame counts from it,
	// and, where timed, its time.
	started time.Time
	timed   bool
	// hidden is a node to come that a question asks about, drawn as the question.
	hidden bool
	// passed is a stage a dry run went past without running it, hollow for good.
	passed bool
	// awaiting is a node done but for the result it waits to state: the exclusion
	// check, while the pauses or report on what it flagged run.
	awaiting bool
	// waits is a node finished, so final, but kept from settling until what goes
	// under it comes: a failed recovery step of the unwind, until the Unwound keyed as
	// it is hangs what the unwind undid under the last, or the run closes.
	waits bool
	// inside is a planned stage that runs inside the next one, which starts first.
	inside bool
	// out is a body shown once one before it, for the same question, has settled: it
	// settles too, as it is shown.
	out bool
	// shown is a part printed as it was when it showed while it stays live: a stage
	// running where there is no region to hold it, a question printed to read a
	// typed line at.
	shown bool
	// reached, crossed and ended are the point of no return's: its stage under way,
	// done, and over, done or not.
	reached, crossed, ended bool
	station                 rail.Station
	text                    string
	question                rail.Question
	// about is the node an exclusion pause hangs its subject under while it asks.
	about *part
}

// live is the part as the rail draws it now: a running node at its spinner's frame,
// one every spinnerFrame since it began, with its time so far where it takes time.
func (pt *part) live(now time.Time) rail.Live {
	switch pt.kind {
	case rulePart:
		return rail.Rule{Reached: pt.reached, Crossed: pt.crossed}
	case endPart:
		return pt.station
	case bodyPart:
		return rail.Body{Text: pt.text}
	case questionPart, previewPart:
		return pt.question
	}
	n := *pt.node
	if n.State == rail.Running {
		n.Frame = int(now.Sub(pt.started) / spinnerFrame)
		if pt.timed {
			n.Time = liveTime(now.Sub(pt.started))
		}
	}
	return n
}

// still is the part as it prints the moment it shows where there is no region: a
// running node at its spinner's first frame, with no time and no bar, as it never
// moves; a question as it is asked.
func (pt *part) still() rail.Item {
	if pt.kind == questionPart {
		return pt.question
	}
	n := *pt.node
	n.Time, n.Progress = "", nil
	return n
}

// hangsUnder is whether the part hangs under the node above it: a body, or a node
// of no stage's that has finished, a warning.
func (pt *part) hangsUnder() bool {
	return pt.kind == bodyPart || pt.kind == nodePart && pt.key == "" && pt.finished()
}

// running is whether the part is a node running.
func (pt *part) running() bool {
	return pt.kind == nodePart && !pt.hidden && pt.node.State == rail.Running
}

// finished is whether nothing more will change the part: a node past running or
// passed by and awaiting nothing, a rule whose stage is over, a previewed question. A
// body and a question go when the question is answered, the end station when the run
// ends.
func (pt *part) finished() bool {
	switch pt.kind {
	case nodePart:
		return !pt.hidden && !pt.awaiting && (pt.passed || pt.node.State != rail.Running && pt.node.State != rail.ToCome)
	case rulePart:
		return pt.ended
	case previewPart:
		return true
	case bodyPart:
		return pt.out
	}
	return false
}

// settleable is whether the part can settle before the run moves past it, when the
// region is too tall for it or there is no region: anything finished that waits on
// nothing, a body, and a point of no return the run has reached, so it stays above
// its stage. A question open stays on the terminal's last rows until it is answered.
func (pt *part) settleable() bool {
	return pt.settles() || pt.kind == bodyPart || pt.kind == rulePart && pt.reached
}

// settles is whether the part is finished and waits on nothing more under it.
func (pt *part) settles() bool {
	return pt.finished() && !pt.waits
}

// toCome is whether the part is still ahead of the run.
func (pt *part) toCome() bool {
	switch pt.kind {
	case nodePart:
		return pt.hidden || pt.node.State == rail.ToCome
	case rulePart:
		return !pt.reached
	}
	return pt.kind == endPart
}

// runModel is the run as the rail draws it: the items settled, printed once and never
// changed, and the parts of the live region under them.
//
// The region starts at the part the last event touched, the node an open question's
// loop asks about, or the first part not yet finished, whichever is highest:
// everything above it settles. So a node stays live while the run is still at it (the exclusion
// check gathering its answers, the notes waiting for the review's), and settles once
// the run moves on.
type runModel struct {
	settled []rail.Item
	// printed is how many of settled are on the terminal.
	printed int
	parts   []*part
	touched *part
	// gone are the stage keys whose nodes settled: an event for one draws a new node.
	gone map[string]bool
	// held is the running node the exclusion check holds back while it runs.
	held *part
	// looping is a question's loop open: from a review's e until the next answer
	// to any gate. asked is the stage that question asks about.
	looping bool
	asked   string
	// planned are the plan's stage keys, each with the name it gave the stage.
	planned map[string]string
	// bodyOut is a body of the question under review settled, too tall for the region
	// or ahead of a typed read: every later one settles as it is shown, and the answer
	// records on a row under the last, so the scrollback ends with the body that
	// ships.
	bodyOut bool
	// lastBody is the text of the last body settled or shown so, under bodyOut: shown
	// again unchanged, it isn't settled twice.
	lastBody        string
	pointOfNoReturn string
	// endName is the name of the end station the plan waits on, "" for a run with no
	// start station.
	endName string
	// moves are the ways forward from a failure, and suggested the exclusion
	// check's suggestions, next's items in that order.
	moves, suggested []rail.Move
	// next is next's items once the run has closed, kept to settle when the drawing
	// ends or another run starts, so what arrives after the close (the autostash's
	// warning) still draws under the end station and next stays last.
	next   []rail.Move
	dryRun bool
	open   bool
	closed bool
}

func newRunModel() *runModel { return &runModel{gone: map[string]bool{}, planned: map[string]string{}} }

// drawing is the run as the rail draws it now.
func (m *runModel) drawing(now time.Time) rail.Run {
	return rail.Run{Settled: m.settled, Region: lives(m.parts, now)}
}

// reached is the run as drawn now down to the run's edge, without the plan still to
// come: the region a too-tall one must keep, its foot cut first.
func (m *runModel) reached(now time.Time) rail.Run {
	return rail.Run{Settled: m.settled, Region: lives(m.parts[:m.insertionPoint()], now)}
}

func lives(parts []*part, now time.Time) []rail.Live {
	region := make([]rail.Live, 0, len(parts))
	for _, pt := range parts {
		if !pt.hidden {
			region = append(region, pt.live(now))
		}
	}
	return region
}

// start opens a run under info's header at its start station, closing any run open:
// a regenerate --all opens one per version.
func (m *runModel) start(info RunInfo) {
	if m.open {
		m.finish(rail.End{}, false)
	}
	m.settleNext()
	m.settleItem(rail.Header{Command: info.Header.Command, Where: info.Header.Where, Version: info.Header.Version})
	m.endName = ""
	if info.Start.Name != "" {
		m.settleItem(rail.Station{Kind: rail.Start, Name: info.Start.Name, Detail: info.Start.Detail})
		m.endName = info.Start.Name
		if info.Version != "" {
			m.endName = "v" + info.Version
		}
	}
	m.dryRun = info.DryRun
	m.open, m.closed, m.touched = true, false, nil
}

// plan lays the plan's stages out as nodes to come, below the stages already drawn,
// with the point of no return across the rail above its stage and the end station
// waiting at the foot.
func (m *runModel) plan(p Plan) {
	m.pointOfNoReturn = p.PointOfNoReturn
	at := m.insertionPoint()
	for _, s := range p.Stages {
		m.planned[s.Key] = s.Name
		if pt := m.find(s.Key); pt != nil {
			pt.node.Name, pt.plan, pt.inside = s.Name, s.Detail, s.Inside
			at = slices.Index(m.parts, pt) + 1
			continue
		}
		if m.gone[s.Key] {
			continue
		}
		if s.Key == p.PointOfNoReturn {
			m.insert(&part{kind: rulePart}, at)
			at++
		}
		m.insert(&part{key: s.Key, plan: s.Detail, inside: s.Inside, node: &rail.Node{State: rail.ToCome, Name: s.Name, Detail: s.Detail}}, at)
		at++
	}
	if m.endName != "" && m.partOf(endPart) == nil {
		m.parts = append(m.parts, &part{kind: endPart, station: rail.Station{Kind: rail.Pending, Name: m.endName}})
	}
}

// stage is the node an event for the stage keyed key fills, touched: its live node,
// unless that is gone or finished, else a new node named as the plan named the stage,
// or name where the plan has none. A finished node is final, settled or still in the
// region alike: a stage that runs again after it finished, as the notes do for the
// review's r, draws a node of its own below it.
func (m *runModel) stage(key, name string) *part {
	pt := m.find(key)
	if pt == nil || pt.finished() {
		if planned, ok := m.planned[key]; ok {
			name = planned
		}
		pt = m.add(rail.Node{Name: name})
		pt.key = key
	}
	if r := m.rule(key); r != nil {
		r.reached = true
	}
	m.touched = pt
	return pt
}

// ended marks the stage keyed key over, done where crossed says so: where it is the
// point of no return's stage, the run has crossed that point once the stage is done,
// not as it starts, nor should it fail.
func (m *runModel) ended(key string, crossed bool) {
	if r := m.rule(key); r != nil {
		r.reached, r.ended, r.crossed = true, true, r.crossed || crossed
	}
}

// rule is the point of no return where key is its stage, nil otherwise.
func (m *runModel) rule(key string) *part {
	if key == "" || key != m.pointOfNoReturn {
		return nil
	}
	return m.partOf(rulePart)
}

// isPlanned is whether the plan laid out the stage keyed key.
func (m *runModel) isPlanned(key string) bool {
	_, ok := m.planned[key]
	return ok
}

// activity is whether a stage keyed key is an open question's own doing, as the
// review's edit is, rather than a stage of the run's: a key that names no stage
// planned and no node drawn, inside the question's loop. Its start and its success
// draw nothing; its failure is drawn.
func (m *runModel) activity(key string) bool {
	return m.looping && !m.isPlanned(key) && m.find(key) == nil && !m.gone[key]
}

// reach marks, in a dry run, the planned stages still to come above pt, a planned
// stage that has started, passed, so they settle above it in plan order: all but one
// that runs inside it, as the exclusion check runs inside the notes, which start
// first.
func (m *runModel) reach(pt *part) {
	if !m.dryRun || !m.isPlanned(pt.key) {
		return
	}
	at := slices.Index(m.parts, pt)
	for i, q := range m.parts[:at] {
		if q.kind != nodePart || q.hidden || q.node.State != rail.ToCome || !m.isPlanned(q.key) {
			continue
		}
		if !q.inside || m.nextPlanned(i) != pt {
			q.passed = true
		}
	}
}

// nextPlanned is the first planned stage's node below index i, nil for none.
func (m *runModel) nextPlanned(i int) *part {
	for _, pt := range m.parts[i+1:] {
		if pt.kind == nodePart && m.isPlanned(pt.key) {
			return pt
		}
	}
	return nil
}

// add puts n at the run's edge, touched.
func (m *runModel) add(n rail.Node) *part {
	pt := &part{node: &n}
	m.insert(pt, m.insertionPoint())
	m.touched = pt
	return pt
}

// aside puts n, a warning about the node pt that leaves it as it was, under it and
// what hangs under it, touched.
func (m *runModel) aside(pt *part, n rail.Node) *part {
	w := &part{node: &n}
	m.insert(w, m.under(pt))
	m.touched = w
	return w
}

// under is where a part goes under the node pt: past the body shown off it and the
// nodes of no stage's drawn since, its warnings.
func (m *runModel) under(pt *part) int {
	at := slices.Index(m.parts, pt) + 1
	for at < len(m.parts) && m.parts[at].hangsUnder() {
		at++
	}
	return at
}

// flag puts n above the live node of the stage keyed stage, the stage it came from,
// or at the run's edge where that has none.
func (m *runModel) flag(stage string, n rail.Node) {
	at := m.insertionPoint()
	if pt := m.find(stage); pt != nil {
		at = slices.Index(m.parts, pt)
	}
	m.insert(&part{node: &n}, at)
}

// find is the live node of the stage keyed key, nil for none.
func (m *runModel) find(key string) *part {
	if key == "" {
		return nil
	}
	for i := len(m.parts) - 1; i >= 0; i-- {
		if pt := m.parts[i]; pt.kind == nodePart && pt.key == key {
			return pt
		}
	}
	return nil
}

// running is the node running, nil for none.
func (m *runModel) running() *part {
	for _, pt := range m.parts {
		if pt.running() {
			return pt
		}
	}
	return nil
}

func (m *runModel) partOf(kind partKind) *part {
	for _, pt := range m.parts {
		if pt.kind == kind {
			return pt
		}
	}
	return nil
}

// insertionPoint is where a new part goes: at the run's edge, under the last part the
// run has reached, so above the plan still to come but below a stage it passed by,
// as a dry run passes its hooks.
func (m *runModel) insertionPoint() int {
	for i := len(m.parts) - 1; i >= 0; i-- {
		if !m.parts[i].toCome() {
			return i + 1
		}
	}
	return 0
}

func (m *runModel) insert(pt *part, at int) {
	m.parts = slices.Insert(m.parts, at, pt)
}

func (m *runModel) remove(pt *part) {
	m.parts = slices.DeleteFunc(m.parts, func(p *part) bool { return p == pt })
	if m.touched == pt {
		m.touched = nil
	}
}

// hang hangs a line under the stage keyed key, or, its node gone, draws it as a node
// of its own.
func (m *runModel) hang(key string, h rail.Hang) {
	pt := m.find(key)
	if pt == nil {
		m.add(rail.Node{Name: key, Detail: h.Text})
		return
	}
	pt.node.Lines = append(pt.node.Lines, h)
	m.touched = pt
}

// show shows text for review under the run's edge, in place of any body shown.
func (m *runModel) show(text string) {
	if m.bodyOut {
		if text != m.lastBody {
			m.insert(&part{kind: bodyPart, text: text, out: true}, m.insertionPoint())
			m.lastBody = text
		}
		return
	}
	if b := m.partOf(bodyPart); b != nil {
		b.text = text
		return
	}
	m.insert(&part{kind: bodyPart, text: text}, m.insertionPoint())
}

// keepBody keeps the body a review's regenerate was answered over: body, shown live,
// settles where it is, and from there, as when a body settled before, out says, every
// body shown settles as it is shown, last being the last of those.
func (m *runModel) keepBody(body *part, out bool, last string) {
	switch {
	case body != nil && !body.out:
		body.out = true
		m.bodyOut, m.lastBody = true, body.text
	case out:
		m.bodyOut, m.lastBody = true, last
	}
}

// ask puts the question q about the stage keyed stage on the rail: in place of its
// node where that is still to come, under it and what hangs under it where the node
// is drawn, or at the run's edge where it has none.
func (m *runModel) ask(q *part, stage string) {
	asked := m.find(stage)
	switch {
	case asked != nil && asked.node.State == rail.ToCome:
		asked.hidden = q.kind == questionPart
		m.insert(q, slices.Index(m.parts, asked))
	case asked != nil:
		m.insert(q, m.under(asked))
	default:
		m.insert(q, m.insertionPoint())
	}
}

// pass marks the stages a dry run went past without running them: a stage to come
// with one finished below it. A dry run never comes back for one, so it settles,
// hollow, as the run moves on.
func (m *runModel) pass() {
	if !m.dryRun {
		return
	}
	past := false
	for i := len(m.parts) - 1; i >= 0; i-- {
		switch pt := m.parts[i]; {
		case pt.finished():
			past = true
		case past && pt.kind == nodePart && !pt.hidden && pt.node.State == rail.ToCome:
			pt.passed = true
		}
	}
}

// settleFinished settles every part above the region's start: the part last
// touched, the node an open question's loop asks about, or the first part not
// finished or still waiting, whichever is highest. Once the run is closed, nothing is live, so all of
// it settles.
func (m *runModel) settleFinished() {
	n := len(m.parts)
	if !m.closed {
		var asked *part
		if m.looping {
			asked = m.find(m.asked)
		}
		for _, i := range []int{
			slices.Index(m.parts, m.touched),
			slices.Index(m.parts, asked),
			slices.IndexFunc(m.parts, func(pt *part) bool { return !pt.settles() }),
		} {
			if i >= 0 {
				n = min(n, i)
			}
		}
	}
	m.settle(n)
}

// settleFirst settles the region's first part where it can settle early, and
// reports whether it did.
func (m *runModel) settleFirst() bool {
	if len(m.parts) == 0 || !m.parts[0].settleable() {
		return false
	}
	m.settle(1)
	return true
}

// log is the run where there is no region to hold it, printed as it happens:
// whatever is finished settles, wherever it is, and a body as it is shown; a stage
// running and a question print the moment they show, still, and stay in the model
// for what comes of them. Nothing waits to be shown.
func (m *runModel) log() {
	parts := m.parts
	m.parts = nil
	for _, pt := range parts {
		if pt.settleable() {
			m.settlePart(pt)
			continue
		}
		if !pt.shown && (pt.running() || pt.kind == questionPart) {
			m.settleItem(pt.still())
			pt.shown = true
		}
		m.parts = append(m.parts, pt)
	}
}

// settleAbove settles what can settle early above q, finished parts and bodies, down
// to the first part that can't: a stage still running stays live.
func (m *runModel) settleAbove(q *part) {
	at := slices.Index(m.parts, q)
	n := 0
	for n < at && m.parts[n].settleable() {
		n++
	}
	m.settle(n)
}

// settle settles the first n parts.
func (m *runModel) settle(n int) {
	for _, pt := range m.parts[:n] {
		m.settlePart(pt)
	}
	m.parts = slices.Delete(m.parts, 0, n)
}

// settlePart settles pt as it is now, which the caller takes out of the region: a
// stage still running as it shows still, no spinner frame nor time read from a
// clock it has none of.
func (m *runModel) settlePart(pt *part) {
	if pt.running() {
		m.settleItem(pt.still())
	} else {
		m.settleItem(pt.live(time.Time{}))
	}
	if pt.kind == bodyPart {
		m.bodyOut, m.lastBody = true, pt.text
	}
	if pt.key != "" {
		m.gone[pt.key] = true
	}
	if m.touched == pt {
		m.touched = nil
	}
	if m.held == pt {
		m.held = nil
	}
}

func (m *runModel) settleItem(it rail.Item) {
	m.settled = append(m.settled, it)
}

// finish closes the run at end, an End with no name drawing none, keeping any moves
// the run left for next, which settleNext puts under it. What the run never reached
// leaves the drawing, but for a dry run's stages to come, which stay hollow above
// its end.
func (m *runModel) finish(end rail.End, dryRun bool) {
	m.drop(dryRun)
	m.settle(len(m.parts))
	if end.Name != "" {
		m.settleItem(end)
	}
	m.next = append(m.next, append(m.moves, m.suggested...)...)
	m.moves, m.suggested, m.looping, m.bodyOut, m.lastBody = nil, nil, false, false, ""
	m.open, m.closed, m.touched, m.held, m.planned = false, true, nil, nil, map[string]string{}
}

// keep keeps move for next in list, the run's moves or its suggestions, while the run
// is open, each list in its place at the close; once the run has closed, in next
// itself after what it holds, so nothing that arrives after the close is lost.
func (m *runModel) keep(list *[]rail.Move, move rail.Move) {
	if m.closed {
		list = &m.next
	}
	*list = append(*list, move)
}

// settleNext settles next, with the moves kept since the run closed, if any.
func (m *runModel) settleNext() {
	if len(m.next) > 0 {
		m.settleItem(rail.Next{Moves: m.next})
		m.next = nil
	}
}

// drop takes what the run never reached out of the region: the end station waiting,
// a question, and, but for a dry run's, the stages to come and a point of no return
// not reached. A stage still running never finished, so it goes too.
func (m *runModel) drop(dryRun bool) {
	m.parts = slices.DeleteFunc(m.parts, func(pt *part) bool {
		switch {
		case pt.kind == endPart || pt.kind == questionPart || pt.running():
			return true
		}
		return pt.toCome() && !dryRun
	})
	for _, pt := range m.parts {
		pt.hidden = false
	}
}

// liveTime is a running stage's time so far, in whole seconds, as it ticks.
func liveTime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return minutes(d)
}

// stageTime is a stage's or a run's time, as it settles: to a tenth of a second
// under a minute, then minutes and seconds.
func stageTime(d time.Duration) string {
	d = d.Round(100 * time.Millisecond)
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return minutes(d)
}

func minutes(d time.Duration) string {
	return fmt.Sprintf("%dm %02ds", int(d/time.Minute), int(d%time.Minute/time.Second))
}
