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
	// aside is a warning about a node it leaves as it was, drawn by it.
	aside bool
	// shown is a stage running or a question printed as it was when it showed, where
	// there is no region to hold it.
	shown    bool
	crossed  bool
	station  rail.Station
	text     string
	question rail.Question
	// about is the node an exclusion pause hangs its subject under while it asks.
	about *part
}

// live is the part as the rail draws it now: a running node at its spinner's frame,
// one every spinnerFrame since it began, with its time so far where it takes time.
func (pt *part) live(now time.Time) rail.Live {
	switch pt.kind {
	case rulePart:
		return rail.Rule{Crossed: pt.crossed}
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

// running is whether the part is a node running.
func (pt *part) running() bool {
	return pt.kind == nodePart && !pt.hidden && pt.node.State == rail.Running
}

// finished is whether nothing more will change the part: a node past running or
// passed by, a crossed rule, a previewed question. A body and a question go when the
// question is answered, the end station when the run ends.
func (pt *part) finished() bool {
	switch pt.kind {
	case nodePart:
		return !pt.hidden && (pt.passed || pt.node.State != rail.Running && pt.node.State != rail.ToCome)
	case rulePart:
		return pt.crossed
	case previewPart:
		return true
	}
	return false
}

// settleable is whether the part can settle before the run moves past it, when the
// region is too tall for it or there is no region: anything finished, and a body. A
// question open stays on the terminal's last rows until it is answered.
func (pt *part) settleable() bool {
	return pt.finished() || pt.kind == bodyPart
}

// toCome is whether the part is still ahead of the run.
func (pt *part) toCome() bool {
	switch pt.kind {
	case nodePart:
		return pt.hidden || pt.node.State == rail.ToCome
	case rulePart:
		return !pt.crossed
	}
	return pt.kind == endPart
}

// final is whether a node's state is for good: a failure, a warning or the unwind. A
// later event for its stage draws a node of its own under it.
func (pt *part) final() bool {
	switch pt.node.State {
	case rail.Failed, rail.Warning, rail.Unwind:
		return true
	}
	return false
}

// runModel is the run as the rail draws it: the items settled, printed once and never
// changed, and the parts of the live region under them.
//
// The region starts at the part the last event touched, the node a question's loop
// is about, or the first part not yet finished, whichever is highest: everything
// above it settles. So a node stays live while the run is still at it (the exclusion
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
	// loop is the stage a question answered e or r asks about until it is answered
	// otherwise: what runs meanwhile, as the edit does, is the question's, not a node.
	loop            string
	pointOfNoReturn string
	// endName is the name of the end station the plan waits on, "" for a run with no
	// start station.
	endName string
	// moves are the ways forward from a failure, and suggested the exclusion
	// check's suggestions, next's items in that order.
	moves, suggested []rail.Move
	dryRun           bool
	open             bool
	closed           bool
}

func newRunModel() *runModel { return &runModel{gone: map[string]bool{}} }

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
		if pt := m.find(s.Key); pt != nil {
			pt.node.Name, pt.plan = s.Name, s.Detail
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
		m.insert(&part{key: s.Key, plan: s.Detail, node: &rail.Node{State: rail.ToCome, Name: s.Name, Detail: s.Detail}}, at)
		at++
	}
	if m.endName != "" && m.partOf(endPart) == nil {
		m.parts = append(m.parts, &part{kind: endPart, station: rail.Station{Kind: rail.Pending, Name: m.endName}})
	}
}

// stage is the node an event for the stage keyed key fills, touched: its live node,
// unless that is gone or final, else a new node named name.
func (m *runModel) stage(key, name string) *part {
	pt := m.find(key)
	if pt == nil || pt.final() {
		pt = m.add(rail.Node{Name: name})
		pt.key = key
	}
	if key != "" && key == m.pointOfNoReturn {
		m.crossRule()
	}
	m.touched = pt
	return pt
}

// looping is whether an event for the stage keyed key happens inside a question's
// loop, the question's own doing rather than a stage of the run's.
func (m *runModel) looping(key string) bool {
	return m.loop != "" && key != m.loop
}

// add puts n at the run's edge, touched.
func (m *runModel) add(n rail.Node) *part {
	pt := &part{node: &n}
	m.insert(pt, m.insertionPoint())
	m.touched = pt
	return pt
}

// aside puts n, a warning about the node pt that leaves it as it was, by it,
// touched: under it and what hangs under it, or above it where it is still to come,
// where a question about it asks.
func (m *runModel) aside(pt *part, n rail.Node) *part {
	w := &part{node: &n, aside: true}
	at := m.under(pt)
	if pt.node.State == rail.ToCome {
		at = slices.Index(m.parts, pt)
	}
	m.insert(w, at)
	m.touched = w
	return w
}

// under is where a part goes under the node pt: past the body shown off it and the
// warnings drawn about it.
func (m *runModel) under(pt *part) int {
	at := slices.Index(m.parts, pt) + 1
	for at < len(m.parts) && (m.parts[at].kind == bodyPart || m.parts[at].aside) {
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

func (m *runModel) crossRule() {
	if r := m.partOf(rulePart); r != nil {
		r.crossed = true
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
	if b := m.partOf(bodyPart); b != nil {
		b.text = text
		return
	}
	m.insert(&part{kind: bodyPart, text: text}, m.insertionPoint())
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
// touched, the node a question's loop is about, or the first part not finished,
// whichever is highest. Once the run is closed, nothing is live, so all of it
// settles.
func (m *runModel) settleFinished() {
	n := len(m.parts)
	if !m.closed {
		for _, i := range []int{
			slices.Index(m.parts, m.touched),
			slices.Index(m.parts, m.find(m.loop)),
			slices.IndexFunc(m.parts, func(pt *part) bool { return !pt.finished() }),
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

// settle settles the first n parts.
func (m *runModel) settle(n int) {
	for _, pt := range m.parts[:n] {
		m.settlePart(pt)
	}
	m.parts = slices.Delete(m.parts, 0, n)
}

// settlePart settles pt as it is now, which the caller takes out of the region.
func (m *runModel) settlePart(pt *part) {
	m.settleItem(pt.live(time.Time{}))
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

// finish closes the run at end, an End with no name drawing none, with next under it
// where the run left any moves. What the run never reached leaves the drawing, but
// for a dry run's stages to come, which stay hollow above its end.
func (m *runModel) finish(end rail.End, dryRun bool) {
	m.drop(dryRun)
	m.settle(len(m.parts))
	if end.Name != "" {
		m.settleItem(end)
	}
	if moves := append(m.moves, m.suggested...); len(moves) > 0 {
		m.settleItem(rail.Next{Moves: moves})
	}
	m.moves, m.suggested, m.loop = nil, nil, ""
	m.open, m.closed, m.touched, m.held = false, true, nil, nil
}

// drop takes what the run never reached out of the region: the end station waiting,
// a question, and, but for a dry run's, the stages to come and a point of no return
// not crossed. A stage still running never finished, so it goes too.
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
