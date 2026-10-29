package presenter_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter"
)

// unread is input a test expects never to be read.
type unread struct{ t *testing.T }

func (u unread) Read([]byte) (int, error) {
	u.t.Error("the input was read, want nothing asked")
	return 0, io.EOF
}

// commitGate is the commit's review of its message.
func commitGate() presenter.Gate {
	g := presenter.NotesReviewGate()
	g.Question, g.Subject, g.Stage = "Commit this message?", "message", "message"
	return g
}

// messageWritten takes a commit run to its review: the message written and shown.
func messageWritten(p presenter.Presenter, body string) {
	commitRun(p)
	p.StageSucceeded(presenter.Settled("message", "written"))
	p.ShowMessage(presenter.Message{Title: "commit message", Body: body})
}

// commitAhead is the commit's plan past its message, waiting.
var commitAhead = []string{
	" ┆",
	" ○  commit        on main",
	" ┆",
	" ┿━━ point of no return " + strings.Repeat("━", 56),
	" ○  push          to its upstream",
	" ┆",
	" ◇  main",
}

func TestRailGate_UnderYesNothingIsAsked_TheNodeRecordsTheAutoAccept(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, unread{t})
	r.WithYes(true)
	messageWritten(r, "feat: retry deliveries")
	c, err := r.Prompt(commitGate())
	if err != nil || c != presenter.ChoiceYes {
		t.Fatalf("Prompt = %q, %v, want the default y", c, err)
	}
	wantScreen(t, r, append(append(commitHead[:4:4], " ●  message       written · accepted (-y)"), commitAhead...)...)
}

func TestRailGate_WithoutAnInteractiveStdinTheQuestionFailsLoud(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		ask   func(p *presenter.RailPresenter) error
		label string
	}{
		{name: "a question", label: "gate", ask: func(p *presenter.RailPresenter) error {
			_, err := p.Prompt(commitGate())
			return err
		}},
		{name: "a typed line, -y or not", label: "input", ask: func(p *presenter.RailPresenter) error {
			_, err := p.WithYes(true).AskLine("Add one-time context:")
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 80, 24, unread{t})
			r.WithInteractiveStdin(false)
			commitRun(r)
			if err := tt.ask(r.RailPresenter); !errors.Is(err, presenter.ErrNotInteractive) {
				t.Fatalf("err = %v, want ErrNotInteractive", err)
			}
			want := " ✗  " + tt.label + strings.Repeat(" ", 14-len(tt.label)) + "not a TTY — pass -y to run unattended"
			if got := r.screen(t)[4]; got != want {
				t.Errorf("the rail reads %q, want the failure rendered %q", got, want)
			}
			if got, want := r.err.String(), "✗ "+tt.label+"  not a TTY — pass -y to run unattended\n"; got != want {
				t.Errorf("err = %q, want %q", got, want)
			}
		})
	}
}

func TestRailGate_InputClosedIsNotRendered_TheQuestionGoes(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	messageWritten(r, "feat: retry deliveries")
	before := r.screen(t)
	if _, err := r.Prompt(commitGate()); !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("err = %v, want ErrInputClosed", err)
	}
	wantScreen(t, r, before...)
	if r.err.Len() != 0 {
		t.Errorf("err = %q, want nothing: the engine surfaces a closed input", r.err.String())
	}
}

func TestRailGate_AnEditRedrawsTheBodyInPlaceBeforeTheQuestionComesBack(t *testing.T) {
	t.Parallel()
	in := answers("e\n", "y\n")
	r := newRailTerm(t, 80, 24, in)
	messageWritten(r, "feat: retry deliveries")
	body := func(text string) []string {
		return append(append(commitHead[:4:4], " ●  message       written", " │", " │    "+text), commitAhead...)
	}
	asked := func(text string) []string {
		return append(append(commitHead[:4:4], " ●  message       written", " │", " │    "+text, " │",
			" ?  Commit this message?   y accept   n abort   e edit   r regenerate"), commitAhead...)
	}
	in.asked = func(n int) {
		if n == 2 {
			wantScreen(t, r, asked("feat(webhooks): retry deliveries")...)
		}
	}

	if c, _ := r.Prompt(commitGate()); c != presenter.ChoiceEdit {
		t.Fatalf("Prompt = %q, want e", c)
	}
	r.SuspendSpinner()
	wantScreen(t, r, commitHead[:3]...)
	r.ResumeSpinner()
	wantScreen(t, r, body("feat: retry deliveries")...)
	r.ShowMessage(presenter.Message{Body: "feat(webhooks): retry deliveries"})
	wantScreen(t, r, body("feat(webhooks): retry deliveries")...)
	if c, _ := r.Prompt(commitGate()); c != presenter.ChoiceYes {
		t.Fatalf("Prompt = %q, want y", c)
	}
	wantScreen(t, r, append(append(commitHead[:4:4], " ●  message       written · accepted"), commitAhead...)...)
}

func TestRailGate_ABodyTooTallForTheRegionSettlesAboveTheQuestion(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 80, 12, in)
	body := lines(12, "line ")
	messageWritten(r, strings.Join(body, "\n"))
	shownBody := []string{" ●  message       written", " │"}
	for _, ln := range body {
		shownBody = append(shownBody, " │    "+ln)
	}
	shownBody = append(shownBody, " │")
	in.asked = func(int) {
		want := append(append(append(commitHead[:4:4], shownBody...), " ?  Commit this message?   y accept   n abort   e edit   r regenerate"), commitAhead...)
		wantScreen(t, r, want...)
	}
	if _, err := r.Prompt(commitGate()); err != nil {
		t.Fatal(err)
	}
	want := append(append(append(commitHead[:4:4], shownBody...), " ●  review        accepted"), commitAhead...)
	wantScreen(t, r, want...)
}

func TestRailGate_ATypedLineIsReadWithTheCursorOnTheQuestion(t *testing.T) {
	t.Parallel()
	in := newBlockingInput()
	r := newRailTerm(t, 80, 24, in)
	r.WithEcho(true)
	messageWritten(r, "feat: retry deliveries")
	r.ShowMessage(presenter.Message{Body: "feat: retry deliveries"})

	type answer struct {
		line string
		err  error
	}
	got := make(chan answer)
	go func() {
		line, err := r.AskLine("Add one-time context:")
		got <- answer{line, err}
	}()
	<-in.reading
	screen := r.screen(t)
	at := slices.Index(screen, " ?  Add one-time context:")
	if at < 0 {
		t.Fatalf("the question isn't up:\n%s", strings.Join(screen, "\n"))
	}
	if row, col := r.term.Cursor(); row != at || col != len(" ?  Add one-time context: ") {
		t.Errorf("the cursor is at row %d, column %d, want row %d, column %d: a space after the question", row, col, at, len(" ?  Add one-time context: "))
	}
	typed := "the export is the headline, " + strings.Repeat("and more ", 6)
	if _, err := r.term.Write([]byte(typed + "\n")); err != nil {
		t.Fatal(err)
	}
	in.lines <- typed + "\n"
	if a := <-got; a.err != nil || a.line != typed {
		t.Fatalf("AskLine = %q, %v, want the line typed, verbatim", a.line, a.err)
	}
	wantScreen(t, r, append(append(commitHead[:4:4], " ●  message       written", " │", " │    feat: retry deliveries"), commitAhead...)...)
}

func TestRailGate_AQuestionAboutAStageToComeStandsInItsPlace(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 80, 24, in)
	r.RunStarted(presenter.RunInfo{Version: "1.4.0", Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0", Detail: "fresh → release"}})
	r.ShowPlan(presenter.Plan{Stages: []presenter.PlanStage{
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
		{Key: "review", Name: "review", Detail: "the notes before they are written"},
		{Key: "publish", Name: "release", Detail: "provider release"},
	}})
	r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Detail: "generated"})
	r.ShowNotes(presenter.Notes{Body: "Retries."})
	in.asked = func(int) {
		if got := r.screen(t)[4:10]; !slices.Contains(got, " ?  Use these notes?   y accept   n abort   e edit   r regenerate") || slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, "○  review") }) {
			t.Errorf("while asked the rail reads\n%s\nwant the question in the review's place", strings.Join(got, "\n"))
		}
	}
	g := presenter.NotesReviewGate()
	g.Stage = "review"
	if _, err := r.Prompt(g); err != nil {
		t.Fatal(err)
	}
	if got := r.screen(t)[5]; got != " ●  review        the notes before they are written · accepted" {
		t.Errorf("the review reads %q, want it settled with the answer", got)
	}
}

func TestRailGate_TheExclusionPauseAsksAboutItsGlobAndAnswersHangUnderTheCheck(t *testing.T) {
	t.Parallel()
	in := answers("y\n", "k\n")
	r := newRailTerm(t, 100, 30, in)
	preparedRelease(r, r.clock)
	in.asked = func(n int) {
		if n == 1 {
			return
		}
		got := r.screen(t)[8:11]
		want := []string{
			pane(" ●  check         files that don't belong in the notes                                          6.2s"),
			" │                design documents · 6 files, 48 kB",
			" ?  docs/specs/** Exclude it from the notes?   e exclude   k keep",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("while asked the rail reads\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	if c, _ := r.Prompt(pauseGate()); c != presenter.ChoiceKeep {
		t.Fatalf("Prompt = %q, want k", c)
	}
	if got := r.screen(t)[9]; got != " │                docs/specs/** kept · design documents" {
		t.Errorf("the answer reads %q, want it under the check", got)
	}
}

func TestRailGate_ATypedAnswerNamingNoKeyIsAskedAgain(t *testing.T) {
	t.Parallel()
	in := answers("nope\n", "tag\n")
	r := newRailTerm(t, 80, 24, in)
	r.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})
	g := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "fresh")
	c, err := r.Prompt(g)
	if err != nil || c != "tag" {
		t.Fatalf("Prompt = %q, %v, want tag", c, err)
	}
	if in.n != 2 {
		t.Errorf("asked %d times, want again after the line naming no key", in.n)
	}
	if got := r.screen(t)[4]; got != " ●  source        tag annotation body" {
		t.Errorf("the answer reads %q, want a row of its own, the question asked before any stage", got)
	}
}

func TestRailGate_AKeypressAnswers(t *testing.T) {
	t.Parallel()
	review := presenter.ReleaseReviewGate()
	source := presenter.SourceGate([]presenter.GateChoice{{Key: "f", Action: "fresh"}}, "f")
	tests := []struct {
		name     string
		keys     string
		gate     presenter.Gate
		want     presenter.Choice
		answered bool
		err      error
	}{
		{name: "a declared key", keys: "e", gate: review, want: presenter.ChoiceEdit, answered: true},
		{name: "a declared key in upper case", keys: "N", gate: review, want: presenter.ChoiceNo, answered: true},
		{name: "Enter, the default", keys: "\r", gate: review, want: presenter.ChoiceYes, answered: true},
		{name: "Space, the default", keys: " ", gate: review, want: presenter.ChoiceYes, answered: true},
		{name: "a lone Escape, the decline", keys: "\x1b", gate: review, want: presenter.ChoiceNo, answered: true},
		{name: "a lone Escape at a gate with no decline", keys: "\x1b", gate: source},
		{name: "an arrow key", keys: "\x1b[A", gate: review},
		{name: "a key not declared", keys: "q", gate: review},
		{name: "Ctrl-C", keys: "\x03", gate: review, err: presenter.ErrInputClosed},
		{name: "Ctrl-D", keys: "\x04", gate: review, err: presenter.ErrInputClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, answered, err := presenter.KeyChoice([]byte(tt.keys), tt.gate)
			if c != tt.want || answered != tt.answered || !errors.Is(err, tt.err) {
				t.Errorf("KeyChoice(%q) = %q, %t, %v, want %q, %t, %v", tt.keys, c, answered, err, tt.want, tt.answered, tt.err)
			}
		})
	}
}
