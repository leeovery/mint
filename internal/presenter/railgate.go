package presenter

import (
	"errors"
	"slices"
	"strings"

	"github.com/rivo/uniseg"

	"mint/internal/presenter/rail"
)

const (
	// pauseQuestion is the exclusion pause's question once its glob has moved into
	// the question's name.
	pauseQuestion = "Exclude it from the notes?"
	// reviewName names the row that records an answer about a node already settled
	// above the question, as a body too tall for the region leaves it.
	reviewName = "review"
	// unknownCols is the columns a terminal that gives none is taken as, for the rows
	// a typed answer's echo takes.
	unknownCols = 80
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
	if f, ok := rawKeyFile(p.in); ok && singleKeyGate(g) {
		q := p.ask(g, false)
		c, err := readKeypress(f, g)
		if !errors.Is(err, errNoRawMode) {
			return c, p.answered(g, q, c, err)
		}
		p.withdraw(q)
	}
	return p.typedChoice(g)
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

// typedChoice reads g's answer as a typed line, asking again after a line that names
// none of its keys.
func (p *RailPresenter) typedChoice(g Gate) (Choice, error) {
	q := p.ask(g, true)
	for {
		line, err := readLine(bufferedReader(p.in, &p.reader))
		if err != nil {
			return "", p.answered(g, q, "", err)
		}
		p.echoed(line)
		if c, ok := parseChoice(line, g); ok {
			return c, p.answered(g, q, c, nil)
		}
		p.reask()
	}
}

// AskLine asks prompt as a question node reading a typed line, the cursor on it, and
// returns the line verbatim. What is typed is the terminal's echo, so nothing redraws
// while the line is read. Without an interactive stdin it fails loud, rendered, with
// ErrNotInteractive, -y or not; input closed returns ErrInputClosed, not rendered.
func (p *RailPresenter) AskLine(prompt string) (string, error) {
	p.mu.Lock()
	if !p.stdinInteractive {
		p.failNotInteractive(inputFailLabel)
		p.mu.Unlock()
		return "", ErrNotInteractive
	}
	q := p.open(&part{kind: questionPart, question: rail.Question{Text: prompt, Typing: true}}, "")
	p.mu.Unlock()
	line, err := readLine(bufferedReader(p.in, &p.reader))
	if err == nil {
		p.echoed(line)
	}
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
// stopped, as nothing may write while it waits; for a typed line, the cursor goes
// where the terminal is to echo it.
func (p *RailPresenter) open(q *part, stage string) *part {
	p.run.ask(q, stage)
	p.asking = true
	p.redraw()
	p.place(q)
	return q
}

// reask draws the question again after a typed line that answered nothing, for
// another to be typed.
func (p *RailPresenter) reask() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.redraw()
	p.place(p.run.partOf(questionPart))
}

// withdraw takes q off the rail unanswered.
func (p *RailPresenter) withdraw(q *part) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dismiss(q)
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

// question is g as a question part of kind: a preview has no keys. The exclusion
// pause's glob is its name, what the AI said of it hung above it under the check's
// node, where that is drawn; a preview leaves it there.
func (p *RailPresenter) question(kind partKind, g Gate, typing bool) *part {
	q := &part{kind: kind, question: rail.Question{Text: g.Question, Typing: typing}}
	if kind == questionPart {
		for _, c := range g.Choices {
			q.question.Choices = append(q.question.Choices, rail.Choice{Key: string(c.Key), Action: c.Action})
		}
	}
	if x := g.Suggestion; x != nil {
		if check := p.run.find(g.Stage); check != nil {
			check.node.Lines = append(slices.Clone(check.node.Lines), rail.Hang{Kind: rail.Note, Text: about(*x)})
			q.question.Name, q.question.Text = x.Glob, pauseQuestion
			if kind == questionPart {
				q.about = check
			}
		}
	}
	return q
}

// answer settles c as g's answer, auto where -y took it: the exclusion pause's as a
// line under the check's node; any other's as the tail of the node it asked about,
// what hung above it gone, or, that node settled above, on a row of its own. An edit
// or a regenerate settles nothing: its body is shown again, in place, and the
// question comes back.
func (p *RailPresenter) answer(g Gate, c Choice, auto bool) {
	if x := g.Suggestion; x != nil {
		p.run.hang(g.Stage, rail.Hang{Kind: rail.Note, Text: x.Glob + " " + pauseAnswer(g, c, auto) + " · " + x.Reason})
		return
	}
	if c == ChoiceEdit || c == ChoiceRegen {
		return
	}
	if body := p.run.partOf(bodyPart); body != nil {
		p.run.remove(body)
	}
	tail := answerTail(g, c, auto)
	switch asked := p.run.find(g.Stage); {
	case asked != nil:
		if asked.node.State == rail.ToCome {
			asked.node.State, asked.node.Detail = rail.Done, firstOf(g.Detail, asked.plan)
		}
		asked.node.Tail = tail
		p.run.touched = asked
	case g.Stage == "":
		p.run.add(rail.Node{Name: g.Subject, Detail: tail})
	case p.run.gone[g.Stage]:
		p.run.add(rail.Node{Name: reviewName, Detail: tail})
	default:
		n := p.run.stage(g.Stage, g.Stage).node
		n.Detail, n.Tail = g.Detail, tail
	}
}

// answerTail is how an answer reads after the node it settles: the gate's own word
// for its default (none for a gate whose node's detail already says what was
// accepted, the version's), the action of any other key; under -y, marked (-y), the
// auto-accept echo.
func answerTail(g Gate, c Choice, auto bool) string {
	tail := action(g, c)
	switch {
	case c == g.Default && g.Detail != "":
		tail = ""
	case c == g.Default:
		tail = g.AcceptEcho
	}
	return autoMarked(tail, auto)
}

// pauseAnswer is what became of the exclusion pause's glob: kept, the gate's own
// word for its default, or excluded.
func pauseAnswer(g Gate, c Choice, auto bool) string {
	word := action(g, c)
	switch c {
	case g.Default:
		word = g.AcceptEcho
	case ChoiceExclude:
		word = "excluded"
	}
	return autoMarked(word, auto)
}

// autoMarked is answer marked (-y) where -y gave it.
func autoMarked(answer string, auto bool) string {
	if !auto {
		return answer
	}
	return strings.TrimSpace(answer + " (-y)")
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

// place puts the cursor where the open question q reads its typed line, for the
// terminal to echo it there.
func (p *RailPresenter) place(q *part) {
	p.typedAt = 0
	if q == nil || !q.question.Typing || !p.layout.RegionFits() {
		return
	}
	if row, col, ok := p.layout.Cursor(p.drawing()); ok {
		p.region.Place(row, col)
		p.typedAt = col
	}
}

// echoed records the rows the terminal moved the cursor down echoing line, typed at
// the question, and the Enter after it.
func (p *RailPresenter) echoed(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.echo {
		return
	}
	cols := p.cols
	if cols <= 0 {
		cols = unknownCols
	}
	rows := 1
	if w := uniseg.StringWidth(line); w > 0 {
		rows += (p.typedAt + w - 1) / cols
	}
	p.region.Moved(rows)
}
