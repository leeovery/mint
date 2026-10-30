package presenter

import (
	"errors"
	"fmt"
	"strings"

	"mint/internal/presenter/rail"
)

const (
	// pauseQuestion is the exclusion pause's question about its glob, its reason and
	// size moved under it.
	pauseQuestion = "Exclude %s from the notes?"
	// previewTail ends a question a dry run shows where a real run would ask it.
	previewTail = "skipped (dry run)"
	// reviewName names the row that records a review's answer about a node already
	// settled above the question, as a body too tall for the region leaves it, but
	// for a node that goes on past the question, which the row names.
	reviewName = "review"
)

// Prompt asks g as a question node on the rail and returns its answer. Under -y
// nothing is asked: the default is taken and the node it settles records it, the
// auto-accept echo. Without an interactive stdin it fails loud, rendered, with
// ErrNotInteractive. Otherwise a single keypress answers where the input is a
// terminal and every key one character, or every key a word answering to its first
// letter, and a typed line answers where not, what was typed before the question was
// asked discarded first. Input closed before an
// answer, or Ctrl-C, returns ErrInputClosed, not rendered, and once Ctrl-C has ended
// a typed line every question after it does.
func (p *RailPresenter) Prompt(g Gate) (Choice, error) {
	if c, err, done := p.unasked(g); done {
		return c, err
	}
	if p.input.abandoned {
		return "", ErrInputClosed
	}
	p.input.discardTypedAhead()
	l := letter(g)
	f, keys := rawKeyFile(p.input.in)
	keys = keys && singleKeyGate(l.keyed)
	q := p.ask(g, !keys)
	if keys {
		c, err := readKeypress(f, l.keyed)
		if !errors.Is(err, errNoRawMode) {
			c = l.word(c)
			return c, p.answered(g, q, c, err)
		}
		p.retype(q)
	}
	return p.typedChoice(l, q)
}

// unasked answers g without asking, under -y, or fails loud without an interactive
// stdin; done is false where g is to be asked.
func (p *RailPresenter) unasked(g Gate) (Choice, error, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.yes:
		p.answer(g, g.Default, true, nil)
		p.redraw()
		return g.Default, nil, true
	case !p.stdinInteractive:
		p.failNotInteractive(gateFailLabel)
		return "", ErrNotInteractive, true
	}
	return "", nil, false
}

// typedChoice reads the answer to l's gate, the open question q, as a typed line,
// asking again after a line that names none of its keys.
func (p *RailPresenter) typedChoice(l lettering, q *part) (Choice, error) {
	g := l.gate
	for {
		line, ended, err := p.input.line()
		p.released(ended)
		if err != nil {
			return "", p.answered(g, q, "", err)
		}
		if c, ok := l.parse(line); ok {
			q.typed = line
			return c, p.answered(g, q, c, nil)
		}
		p.reask(q)
	}
}

// AskLine asks prompt as a question node reading a typed line, the cursor on it, and
// returns the line verbatim. What is typed is the terminal's echo, so nothing redraws
// while the line is read, and the question and the line stay as typed; what was
// typed before it was asked is discarded first. Without an interactive stdin it fails
// loud, rendered, with ErrNotInteractive, -y or not; input closed, or Ctrl-C, returns
// ErrInputClosed, not rendered, and once Ctrl-C has ended a typed line it returns it
// without asking.
func (p *RailPresenter) AskLine(prompt string) (string, error) {
	p.mu.Lock()
	if !p.stdinInteractive {
		p.failNotInteractive(inputFailLabel)
		p.mu.Unlock()
		return "", ErrNotInteractive
	}
	if p.input.abandoned {
		p.mu.Unlock()
		return "", ErrInputClosed
	}
	p.input.discardTypedAhead()
	q := p.open(&part{kind: questionPart, question: rail.Question{Text: prompt, Typing: true}}, "")
	p.mu.Unlock()
	line, ended, err := p.input.line()
	p.released(ended)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dismiss(q)
	p.redraw()
	return line, err
}

// PreviewGate shows g as a dry run's preview, where and as a real run would stop to
// ask it, but with no keys and a dim tail saying it was skipped. It stays on the
// rail.
func (p *RailPresenter) PreviewGate(g Gate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.run.ask(p.question(previewPart, g, false), g.Stage)
	p.redraw()
}

// ask puts g's question on the rail and draws it, reading a typed line where typing
// is set.
func (p *RailPresenter) ask(g Gate, typing bool) *part {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.open(p.question(questionPart, g, typing), g.Stage)
}

// open puts q on the rail about the stage keyed stage, and draws it with the spinner
// stopped, as nothing may write while it waits, and the clock of any stage running
// stood still, as no time spent waiting on an answer is a stage's.
func (p *RailPresenter) open(q *part, stage string) *part {
	p.run.ask(q, stage)
	p.run.stop(p.now())
	p.asking = true
	p.present(q)
	return q
}

// present draws the open question q: on the rail, or, for a typed line the terminal
// echoes, printed last to be read at its end.
func (p *RailPresenter) present(q *part) {
	rows := p.measure()
	if q.question.Typing && p.echo {
		p.readAt(q, rows)
		return
	}
	p.draw(rows)
}

// readAt prints q to read a typed line at, the one way every line the terminal
// echoes is read: what can settle above the question settles, the region comes off
// the terminal, and the question prints under the settled lines with no newline,
// the cursor at its end. Nothing is drawn below it while the line is typed, so the
// echo lands only on blank rows, however long the line or however it was edited;
// it stays as typed, the answer's record, and the region is drawn afresh under it
// once the line is read, nothing gone back over. A stage still running above the
// question comes back live with the region.
func (p *RailPresenter) readAt(q *part, rows int) {
	q.shown, q.read = true, true
	if p.fits {
		p.run.settleAbove(q)
	} else {
		p.run.log()
	}
	p.run.settleItem(q.question)
	run := p.drawing()
	lines := p.layout.Settled(run, p.run.printed)
	// A question reading a typed line ends on a blank cell for its cursor, which the
	// echo takes.
	lines[len(lines)-1] = strings.TrimSuffix(lines[len(lines)-1], " ")
	p.region.Ask(lines, rows)
	p.run.printed = len(run.Settled)
	p.syncTicker()
}

// released ends the read of a typed line the terminal echoed, ended where it ended in
// a newline.
func (p *RailPresenter) released(ended bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.echo {
		p.region.Released(ended)
	}
}

// retype turns the open question q, asked for a keypress the terminal wouldn't go
// into raw mode for, into one reading a typed line: the same question, not asked
// again. Where it has printed already, with no region to hold it, it doesn't print
// twice, and the line is typed on the row under it.
func (p *RailPresenter) retype(q *part) {
	p.mu.Lock()
	defer p.mu.Unlock()
	q.question.Typing = true
	if !q.shown {
		p.present(q)
	}
}

// reask asks the question q again after a typed line that answered nothing, for
// another to be typed under it.
func (p *RailPresenter) reask(q *part) {
	p.mu.Lock()
	defer p.mu.Unlock()
	q.shown = false
	p.present(q)
}

// answered takes q off the rail and, unless err ended the read, settles c as g's
// answer; it returns err.
func (p *RailPresenter) answered(g Gate, q *part, c Choice, err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dismiss(q)
	if err == nil {
		p.answer(g, c, false, q)
	} else {
		// A node that went on past the question goes on no more, so the failure
		// that says why draws under what the question showed, as one after an
		// answer does, not in place above it.
		p.run.rerun(g.Stage)
	}
	p.redraw()
	return err
}

// dismiss takes the question q off the rail: the node it stood in for goes back as
// it was, and the clock of a stage running goes on from where it stood.
func (p *RailPresenter) dismiss(q *part) {
	p.asking = false
	p.run.restart(p.now())
	p.run.remove(q)
	for _, pt := range p.run.parts {
		pt.hidden = false
	}
}

// question is g as a question part of kind: a dry run's preview, in the place and
// the shape the question takes when asked, has no keys, but a tail saying it was
// skipped. A pause sits inside the exclusion check, the check's sub-matter, asking
// about its glob with what the AI said of it under the question.
func (p *RailPresenter) question(kind partKind, g Gate, typing bool) *part {
	q := &part{kind: kind, question: rail.Question{Text: g.Question, Typing: typing}}
	if kind == questionPart {
		q.question.Choices = letter(g).choices()
	} else {
		q.question.Tail = previewTail
	}
	if x := g.Suggestion; x != nil {
		q.question.Under, q.question.Text, q.question.About = true, fmt.Sprintf(pauseQuestion, x.Glob), about(*x)
	}
	return q
}

// answer settles c as g's answer to the question q, nil where -y took it: the
// exclusion pause's as a line under the check's node; any other's as the tail of the
// node it asked about, what hung above it gone, or, that node settled above or a body
// of the question settled, on a row of its own under the last body; a gate asked
// before the run's stages as a row naming what was chosen, unless the question and
// the line typed at it stay as its record. A review's regenerate settles as any answer
// does, but its body stays, settled under the node it settles, so the draft is in
// view while what to change is typed: its stage runs again as a node of its own
// below, under which the body and the question come back. A review's edit settles
// nothing: it opens the question's loop, until the next answer to any gate, the body
// is shown again, in place, and the question comes back. A warning drawn while the
// loop was open, under the node, came before the answer, which records on a row of
// its own under it.
func (p *RailPresenter) answer(g Gate, c Choice, auto bool, q *part) {
	p.run.looping = false
	if x := g.Suggestion; x != nil {
		p.run.hang(g.Stage, rail.Hang{Kind: rail.Note, Text: x.Glob + " " + pauseAnswer(g, c, auto) + " · " + x.Reason})
		return
	}
	if c == ChoiceEdit && g.Review {
		p.run.looping, p.run.asked = true, g.Stage
		return
	}
	out, last, warned := p.run.bodyOut, p.run.lastBody, p.run.loopWarned
	p.run.bodyOut, p.run.lastBody, p.run.loopWarned = false, "", false
	switch body := p.run.partOf(bodyPart); {
	case c == ChoiceRegen && g.Review:
		p.run.keepBody(body, out, last)
		p.run.rerun(g.Stage)
	case body != nil && !body.out:
		p.run.remove(body)
	}
	tail := answerTail(g, c, auto)
	switch asked := p.run.find(g.Stage); {
	case asked != nil && (!out && !warned || asked.node.State == rail.ToCome):
		if asked.node.State == rail.ToCome {
			asked.node.State = rail.Done
			asked.node.Detail, tail = settled(g, c, asked.plan, tail)
		}
		asked.node.Tail = tail
		p.run.touched = asked
	case g.Stage == "" && q != nil && q.read && strings.TrimSpace(q.typed) != "":
	case g.Stage == "":
		p.run.add(rail.Node{Name: g.Subject, Detail: chosen(g, c), Tail: autoTail(auto)})
	case out || warned || p.run.gone[g.Stage]:
		p.run.add(recordRow(g, c, q, tail))
	default:
		n := p.run.stage(g.Stage, g.Stage).node
		n.Detail, n.Tail = g.Detail, tail
	}
}

// recordRow is the row recording c, g's answer to the question q, nil where -y took
// it, tail being how the answer reads after a node, where what it asked about has
// settled: named for that node where it goes on past the question, else the review.
// A gate with no Detail of its own records the answer as the row's detail.
func recordRow(g Gate, c Choice, q *part, tail string) rail.Node {
	name := reviewName
	if q != nil && q.record != "" {
		name = q.record
	}
	if g.Detail == "" {
		return rail.Node{Name: name, Detail: tail}
	}
	detail, tail := settled(g, c, "", tail)
	return rail.Node{Name: name, Detail: detail, Tail: tail}
}

// settled is the detail and the tail a node waiting for g's answer settles to once c
// answers it, tail being how that answer reads after it: plan, the node's plan
// detail, for a gate with no Detail of its own; else the gate's Detail where its
// default answered, which is what that detail says, or what c chose, the tail alone
// where it only restates that, as a review's "accepted (-y)" does its "accepted".
func settled(g Gate, c Choice, plan, tail string) (string, string) {
	if g.Detail == "" {
		return plan, tail
	}
	detail := g.Detail
	if c != g.Default {
		detail = chosen(g, c)
	}
	if strings.HasPrefix(tail, detail) {
		return tail, ""
	}
	return detail, tail
}

// autoAccepted is the tail of every answer -y took, the auto-accept echo.
const autoAccepted = "accepted (-y)"

// answerTail is how an answer reads after the node it settles: under -y, the
// auto-accept echo; else the gate's own word for its default (none for a gate whose
// node's detail already says what was accepted, the version's) and the action of any
// other key.
func answerTail(g Gate, c Choice, auto bool) string {
	switch {
	case auto:
		return autoAccepted
	case c == g.Default && g.Detail != "":
		return ""
	}
	return chosen(g, c)
}

func autoTail(auto bool) string {
	if auto {
		return autoAccepted
	}
	return ""
}

// chosen is what answering g with c chose: the gate's own word for its default, how
// any other key reads once it answered.
func chosen(g Gate, c Choice) string {
	if c == g.Default {
		return g.AcceptEcho
	}
	return answered(g, c)
}

// pauseAnswer is what became of the exclusion pause's glob, as chosen says it, marked
// (-y) where -y took it.
func pauseAnswer(g Gate, c Choice, auto bool) string {
	word := chosen(g, c)
	if auto {
		word += " (-y)"
	}
	return word
}

// answered is how g's key c reads once it is the answer: its own word for that, or
// the action it names.
func answered(g Gate, c Choice) string {
	for _, choice := range g.Choices {
		if choice.Key == c {
			return firstOf(choice.Answered, choice.Action)
		}
	}
	return string(c)
}

// failNotInteractive renders the forbidden combination, a question with no TTY to
// answer it and no -y, as a failed node labelled for the mechanism failing, and its
// summary to err.
func (p *RailPresenter) failNotInteractive(label string) {
	p.run.add(rail.Node{State: rail.Failed, Name: label, Detail: gateNotTTYMessagePretty})
	p.errf("✗ %s  %s\n", label, gateNotTTYMessagePretty)
	p.redraw()
}
