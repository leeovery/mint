package presenter

import (
	"errors"
	"slices"

	"mint/internal/presenter/rail"
)

const (
	// pauseQuestion is the exclusion pause's question once its glob has moved into
	// the question's name.
	pauseQuestion = "Exclude it from the notes?"
	// reviewName names the row that records an answer about a node already settled
	// above the question, as a body too tall for the region leaves it.
	reviewName = "review"
)

// Prompt asks g as a question node on the rail and returns its answer. Under -y
// nothing is asked: the default is taken and the node it settles records it, the
// auto-accept echo. Without an interactive stdin it fails loud, rendered, with
// ErrNotInteractive. Otherwise a single keypress answers, as today's presenter reads
// it, where the input is a terminal and every key one character, and a typed line
// answers where not. Input closed before an answer returns ErrInputClosed, not
// rendered.
func (p *RailPresenter) Prompt(g Gate) (Choice, error) {
	if c, err, done := p.unasked(g); done {
		return c, err
	}
	f, keys := rawKeyFile(p.in)
	keys = keys && singleKeyGate(g)
	q := p.ask(g, !keys)
	if keys {
		c, err := readKeypress(f, g)
		if !errors.Is(err, errNoRawMode) {
			return c, p.answered(g, q, c, err)
		}
		p.retype(q)
	}
	return p.typedChoice(g, q)
}

// unasked answers g without asking, under -y, or fails loud without an interactive
// stdin; done is false where g is to be asked.
func (p *RailPresenter) unasked(g Gate) (Choice, error, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.yes:
		p.answer(g, g.Default, true)
		p.redraw()
		return g.Default, nil, true
	case !p.stdinInteractive:
		p.failNotInteractive(gateFailLabel)
		return "", ErrNotInteractive, true
	}
	return "", nil, false
}

// typedChoice reads g's answer to the open question q as a typed line, asking again
// after a line that names none of its keys.
func (p *RailPresenter) typedChoice(g Gate, q *part) (Choice, error) {
	for {
		line, ended, err := readTypedLine(bufferedReader(p.in, &p.reader))
		p.released(q, ended)
		if err != nil {
			return "", p.answered(g, q, "", err)
		}
		if c, ok := parseChoice(line, g); ok {
			return c, p.answered(g, q, c, nil)
		}
		p.reask(q)
	}
}

// AskLine asks prompt as a question node reading a typed line, the cursor on it, and
// returns the line verbatim. What is typed is the terminal's echo, so nothing redraws
// while the line is read, and the question and the line stay as typed. Without an
// interactive stdin it fails loud, rendered, with ErrNotInteractive, -y or not;
// input closed returns ErrInputClosed, not rendered.
func (p *RailPresenter) AskLine(prompt string) (string, error) {
	p.mu.Lock()
	if !p.stdinInteractive {
		p.failNotInteractive(inputFailLabel)
		p.mu.Unlock()
		return "", ErrNotInteractive
	}
	q := p.open(&part{kind: questionPart, question: rail.Question{Text: prompt, Typing: true}}, "")
	p.mu.Unlock()
	line, ended, err := readTypedLine(bufferedReader(p.in, &p.reader))
	p.released(q, ended)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dismiss(q)
	p.redraw()
	return line, err
}

// PreviewGate shows g as a dry run's preview: the question node with no keys, where
// a real run would stop to ask it. It stays on the rail.
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
// stopped, as nothing may write while it waits.
func (p *RailPresenter) open(q *part, stage string) *part {
	p.run.ask(q, stage)
	p.asking = true
	p.present(q)
	return q
}

// present draws the open question q, the cursor where it reads a typed line for the
// terminal to echo it there. The question and the line typed stay on the terminal as
// typed, the region starting afresh under them once it is read (released), so for
// an echo everything above the question settles first and the question heads the
// region. Where something still running above it can't settle, the line is read
// under the settled lines instead.
func (p *RailPresenter) present(q *part) {
	rows := p.measure()
	if q.question.Typing && p.echo && p.fits && !p.run.settleAbove(q) {
		p.readBelow(q, rows)
		return
	}
	p.redraw()
	p.place(q)
}

// readBelow takes the region off the terminal and prints the question q under the
// settled lines, its line to be typed on the row under it; the region comes back
// below the line once it is read, its stages running still live.
func (p *RailPresenter) readBelow(q *part, rows int) {
	p.region.Erase()
	p.run.settleItem(q.question)
	q.shown = true
	run := p.drawing()
	p.region.Draw(p.layout.Settled(run, p.run.printed), nil, rows)
	p.run.printed = len(run.Settled)
	p.syncTicker()
}

// released ends the read of a typed line the terminal echoed, ended where it ended in
// a newline: the question and the line stay where they were typed, the question kept
// as printed, and the region starts afresh under them.
func (p *RailPresenter) released(q *part, ended bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.echo {
		return
	}
	p.region.Released(ended)
	if !q.shown {
		p.run.record(q.question)
		q.shown = true
	}
}

// retype turns the open question q, asked for a keypress the terminal wouldn't go
// into raw mode for, into one reading a typed line: the same question, not asked
// again, so where it has printed it doesn't print twice.
func (p *RailPresenter) retype(q *part) {
	p.mu.Lock()
	defer p.mu.Unlock()
	q.question.Typing = true
	p.present(q)
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
		p.answer(g, c, false)
	}
	p.redraw()
	return err
}

// dismiss takes the question q off the rail: the node it stood in for, and the
// node its pause hung its subject under, go back as they were.
func (p *RailPresenter) dismiss(q *part) {
	p.asking = false
	p.run.remove(q)
	if n := q.about; n != nil {
		n.node.Lines = slices.Clone(n.node.Lines[:len(n.node.Lines)-1])
	}
	for _, pt := range p.run.parts {
		pt.hidden = false
	}
}

// question is g as a question part of kind: a preview has no keys. Where the
// exclusion check's node is drawn, the pause's glob is its name and what the AI said
// of it hangs above it under that node, which a preview leaves there; with the node
// settled, the pause asks in the gate's own words, which say it all.
func (p *RailPresenter) question(kind partKind, g Gate, typing bool) *part {
	q := &part{kind: kind, question: rail.Question{Text: g.Question, Typing: typing}}
	if kind == questionPart {
		for _, c := range g.Choices {
			q.question.Choices = append(q.question.Choices, rail.Choice{Key: string(c.Key), Action: c.Action})
		}
	}
	x, check := g.Suggestion, p.run.find(g.Stage)
	if x == nil || check == nil {
		return q
	}
	check.node.Lines = append(slices.Clone(check.node.Lines), rail.Hang{Kind: rail.Note, Text: about(*x)})
	q.question.Name, q.question.Text = x.Glob, pauseQuestion
	if kind == questionPart {
		q.about = check
	}
	return q
}

// answer settles c as g's answer, auto where -y took it: the exclusion pause's as a
// line under the check's node; any other's as the tail of the node it asked about,
// what hung above it gone, or, that node settled above or a body of the question
// settled, on a row of its own under the last body; a gate asked before the run's
// stages as a row naming what was chosen. An edit or a regenerate settles nothing:
// it opens the question's loop, until the next answer to any gate, the body is
// shown again, in place, and the question comes back.
func (p *RailPresenter) answer(g Gate, c Choice, auto bool) {
	p.run.looping = false
	if x := g.Suggestion; x != nil {
		p.run.hang(g.Stage, rail.Hang{Kind: rail.Note, Text: x.Glob + " " + pauseAnswer(g, c, auto) + " · " + x.Reason})
		return
	}
	if c == ChoiceEdit || c == ChoiceRegen {
		p.run.looping, p.run.asked = true, g.Stage
		return
	}
	out := p.run.bodyOut
	p.run.bodyOut = false
	if body := p.run.partOf(bodyPart); body != nil && !body.out {
		p.run.remove(body)
	}
	tail := answerTail(g, c, auto)
	switch asked := p.run.find(g.Stage); {
	case asked != nil && (!out || asked.node.State == rail.ToCome):
		if asked.node.State == rail.ToCome {
			asked.node.State, asked.node.Detail = rail.Done, firstOf(g.Detail, asked.plan)
		}
		asked.node.Tail = tail
		p.run.touched = asked
	case g.Stage == "":
		p.run.add(rail.Node{Name: g.Subject, Detail: chosen(g, c), Tail: autoTail(auto)})
	case out || p.run.gone[g.Stage]:
		p.run.add(rail.Node{Name: reviewName, Detail: tail})
	default:
		n := p.run.stage(g.Stage, g.Stage).node
		n.Detail, n.Tail = g.Detail, tail
	}
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

// chosen is what answering g with c chose: the gate's own word for its default, the
// action of any other key.
func chosen(g Gate, c Choice) string {
	if c == g.Default {
		return g.AcceptEcho
	}
	return action(g, c)
}

// pauseAnswer is what became of the exclusion pause's glob: kept, the gate's own
// word for its default, or excluded, marked (-y) where -y took it.
func pauseAnswer(g Gate, c Choice, auto bool) string {
	word := chosen(g, c)
	if c == ChoiceExclude {
		word = "excluded"
	}
	if auto {
		word += " (-y)"
	}
	return word
}

// action is the action g's key c names.
func action(g Gate, c Choice) string {
	for _, choice := range g.Choices {
		if choice.Key == c {
			return choice.Action
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

// place puts the cursor where the open question q reads its typed line, at the
// question's end, for the terminal to echo it there.
func (p *RailPresenter) place(q *part) {
	if q == nil || !q.question.Typing || !p.fits {
		return
	}
	if row, col, ok := p.layout.Cursor(p.drawing()); ok {
		p.region.Place(row, col)
	}
}
