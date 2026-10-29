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

// TestRailGate_UnderYesNothingIsAsked_TheNodeRecordsTheAutoAccept pins that every
// answer -y takes reads accepted (-y): after the node's own detail, the version's
// included, and after what a question asked before the run's stages chose.
func TestRailGate_UnderYesNothingIsAsked_TheNodeRecordsTheAutoAccept(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ask  func(r *railTerm) (presenter.Choice, error)
		want string
	}{
		{name: "the commit's message", want: " ●  message       written · accepted (-y)", ask: func(r *railTerm) (presenter.Choice, error) {
			messageWritten(r, "feat: retry deliveries")
			return r.Prompt(commitGate())
		}},
		{name: "the version, settled to the gate's detail", want: " ●  version       v1.4.0 (minor bump) · accepted (-y)", ask: func(r *railTerm) (presenter.Choice, error) {
			r.RunStarted(releaseStarted(false))
			r.ShowPlan(releasePlan())
			return r.Prompt(versionGate())
		}},
		{name: "a source, asked before the stages", want: " ●  source        fresh · accepted (-y)", ask: func(r *railTerm) (presenter.Choice, error) {
			r.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})
			return r.Prompt(presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "fresh"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 80, 24, unread{t})
			r.WithYes(true)
			c, err := tt.ask(r)
			if err != nil || c == "" {
				t.Fatalf("Prompt = %q, %v, want the default", c, err)
			}
			if got := r.screen(t)[4]; got != tt.want {
				t.Errorf("the answer reads %q, want %q", got, tt.want)
			}
		})
	}
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

// notesUnderReview takes a release to its review: the notes written and body shown,
// the record and the push still to come.
func notesUnderReview(p presenter.Presenter, body string) {
	p.RunStarted(releaseStarted(false))
	p.ShowPlan(presenter.Plan{Quiet: true, PointOfNoReturn: "push", Stages: []presenter.PlanStage{
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
		{Key: "record", Name: "record", Detail: "CHANGELOG.md"},
		{Key: "push", Name: "push", Detail: "main + v1.4.0 → origin, atomic"},
	}})
	p.StageStarted(notesStarted())
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Detail: "written by the AI"})
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})
}

// editStage is the edit's stage as the engine runs it around the editor, the
// editor's warning, where one won't launch, inside it.
func editStage(r *railTerm, warn *presenter.Warning) {
	r.StageStarted(presenter.StageStart{Name: "edit", Key: "edit", Blocking: true, Text: "applying edited notes…"})
	r.SuspendSpinner()
	if warn != nil {
		r.Warn(*warn)
	}
	r.ResumeSpinner()
	r.StageSucceeded(presenter.StageSuccess{Name: "edit", Key: "edit", Blocking: true, Elapsed: secs(9), Detail: "notes updated"})
}

// TestRailGate_WhatTheReviewsLoopRunsIsTheQuestions pins the review's loop as the
// engine runs it: what an e or an r runs, the edit's stage around the editor and the
// regenerate's one-time context, draws no node; the body is shown again in place,
// the question comes back under it, and the accept settles the notes with its tail.
func TestRailGate_WhatTheReviewsLoopRunsIsTheQuestions(t *testing.T) {
	t.Parallel()
	const question = " ?  Release with these notes?   y release   n abort   e edit   r regenerate"
	missing := &presenter.Warning{Label: "editor", Message: `could not launch editor "vim": not found`}
	const warned = ` !  editor        could not launch editor "vim": not found`
	tests := []struct {
		name    string
		answers []string
		loop    func(r *railTerm)
		// body is the body shown when the question comes back.
		body string
		// warned is whether the editor's warning is drawn, under the notes.
		warned bool
	}{
		{name: "an edit", answers: []string{"e\n", "y\n"}, body: "second draft", loop: func(r *railTerm) {
			editStage(r, nil)
			r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
		}},
		{name: "a regenerate", answers: []string{"r\n", "shorter\n", "y\n"}, body: "second draft", loop: func(r *railTerm) {
			if _, err := r.AskLine("Add one-time context for regeneration (optional):"); err != nil {
				t.Fatal(err)
			}
			r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
		}},
		{name: "an edit with no editor to launch", answers: []string{"e\n", "y\n"}, body: "first draft", warned: true, loop: func(r *railTerm) {
			editStage(r, missing)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := answers(tt.answers...)
			r := newRailTerm(t, 80, 30, in)
			notesUnderReview(r, "first draft")
			in.asked = func(n int) {
				if n != len(tt.answers) {
					return
				}
				screen := r.screen(t)
				noEditRow(t, screen)
				want := []string{" ●  notes         written by the AI", " │", " │    " + tt.body, " │"}
				if tt.warned {
					want = append(want, warned)
				}
				want = append(want, question)
				if at := slices.Index(screen, want[0]); at < 0 || !slices.Equal(screen[at:min(at+len(want), len(screen))], want) {
					t.Errorf("asked again, the terminal shows\n%s\nwant the body in place and the question under it\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
				}
			}
			for range 2 {
				c, err := r.Prompt(reviewGate())
				if err != nil {
					t.Fatal(err)
				}
				if c == presenter.ChoiceYes {
					break
				}
				tt.loop(r)
			}
			screen := r.screen(t)
			noEditRow(t, screen)
			want := []string{" ●  notes         written by the AI · accepted"}
			if tt.warned {
				want = append(want, warned)
			}
			if at := slices.Index(screen, want[0]); at < 0 || !slices.Equal(screen[at:min(at+len(want), len(screen))], want) {
				t.Errorf("once accepted the terminal shows\n%s\nwant the notes settled with the answer\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
			}
			if slices.ContainsFunc(screen, func(s string) bool { return strings.HasPrefix(s, " ●  review") }) {
				t.Errorf("the answer is on a row of its own:\n%s", strings.Join(screen, "\n"))
			}
		})
	}
}

// noEditRow fails where screen draws the edit's stage as a node.
func noEditRow(t *testing.T, screen []string) {
	t.Helper()
	for _, mark := range []string{"●", "⠋", "○"} {
		if slices.ContainsFunc(screen, func(s string) bool { return strings.HasPrefix(s, " "+mark+"  edit") }) {
			t.Errorf("the edit is drawn as a node:\n%s", strings.Join(screen, "\n"))
		}
	}
}

// TestRailGate_OnAShortTerminalAnOpenQuestionStaysLive pins that an open question
// never settles: on a terminal too short for it and the plan below it, the plan's
// foot gives way, and the answer settles the node the question asked about.
func TestRailGate_OnAShortTerminalAnOpenQuestionStaysLive(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 80, 12, in)
	messageWritten(r, "feat: retry deliveries\n\nDeliveries that fail retry for up to an hour.")
	const question = " ?  Commit this message?   y accept   n abort   e edit   r regenerate"
	in.asked = func(int) {
		if screen := r.screen(t); !slices.Contains(screen, question) {
			t.Errorf("the question isn't up:\n%s", strings.Join(screen, "\n"))
		}
	}
	if _, err := r.Prompt(commitGate()); err != nil {
		t.Fatal(err)
	}
	screen := r.screen(t)
	if !slices.Contains(screen, " ●  message       written · accepted") || slices.Contains(screen, question) {
		t.Errorf("once answered the terminal shows\n%s\nwant the question gone and the message settled with the answer", strings.Join(screen, "\n"))
	}
}

// TestRailGate_ALineTheInputClosedOnMovesTheCursorNoFurther pins that a typed line
// cut short by the input closing, as `abc` then Ctrl-D, is echoed with no Enter
// after it: the redraw goes back up as far as the echo took the cursor, and no
// further, for AskLine's line and a question's typed answer alike. What is above a
// question read as an echoed line settles first, so the typed answer is recorded on
// a row of its own.
func TestRailGate_ALineTheInputClosedOnMovesTheCursorNoFurther(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		typed string
		ask   func(r *railTerm) error
		want  []string
	}{
		{
			name: "AskLine", typed: "abc",
			ask: func(r *railTerm) error {
				_, err := r.AskLine("Add one-time context:")
				return err
			},
			want: append(append(commitHead[:4:4], " ●  message       written", " │", " │    feat: retry deliveries"), commitAhead...),
		},
		{
			name: "a typed answer", typed: "y",
			ask: func(r *railTerm) error {
				_, err := r.Prompt(commitGate())
				return err
			},
			want: append(append(commitHead[:4:4], " ●  message       written", " │", " │    feat: retry deliveries", " │", " ●  review        accepted"), commitAhead...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := newBlockingInput()
			r := newRailTerm(t, 80, 24, in)
			r.WithEcho(true)
			messageWritten(r, "feat: retry deliveries")
			done := make(chan error)
			go func() { done <- tt.ask(r) }()
			<-in.reading
			if _, err := r.term.Write([]byte(tt.typed)); err != nil {
				t.Fatal(err)
			}
			in.lines <- tt.typed
			close(in.lines)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			wantScreen(t, r, tt.want...)
		})
	}
}

// TestRailGate_ATypedLinesEchoNeverTakesTheWriterPastTheScreen pins the bound on a
// typed line's echo: everything above the question settles before the line is
// read, so the question heads the region and an echo that wraps below it, however
// near the terminal's foot, scrolls away nothing the writer goes back over. With no
// region, on a terminal too short for one, the echo stays where it was typed.
func TestRailGate_ATypedLinesEchoNeverTakesTheWriterPastTheScreen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		rows, body int
		typed      string
		echoErased bool
	}{
		{name: "near the region's foot", rows: 14, body: 8, typed: strings.Repeat("x", 150), echoErased: true},
		{name: "wrapping three times, on a terminal too short for a region", rows: 10, body: 3, typed: strings.Repeat("x", 250)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := newBlockingInput()
			r := newRailTerm(t, 80, tt.rows, in)
			r.WithEcho(true)
			body := lines(tt.body, "line ")
			messageWritten(r, strings.Join(body, "\n"))
			got := make(chan error)
			go func() {
				_, err := r.AskLine("Add one-time context:")
				got <- err
			}()
			<-in.reading
			if _, err := r.term.Write([]byte(tt.typed + "\n")); err != nil {
				t.Fatal(err)
			}
			in.lines <- tt.typed + "\n"
			if err := <-got; err != nil {
				t.Fatal(err)
			}
			screen := r.screen(t)
			for _, want := range []string{" ●  message       written", " │    " + body[0], " │    " + body[len(body)-1]} {
				if n := count(screen, want); n != 1 {
					t.Errorf("%q is on the terminal %d times, want once:\n%s", want, n, strings.Join(screen, "\n"))
				}
			}
			echoed := slices.ContainsFunc(screen, func(s string) bool { return strings.Contains(s, "xxxx") })
			if echoed == tt.echoErased {
				t.Errorf("the echo is on the terminal: %v, want %v:\n%s", echoed, !tt.echoErased, strings.Join(screen, "\n"))
			}
		})
	}
}

// count is how many of rows are row.
func count(rows []string, row string) int {
	n := 0
	for _, r := range rows {
		if r == row {
			n++
		}
	}
	return n
}

// TestRailGate_ACommitRegenerateThatFallsToTheEditorDrawsEveryStageAfterIt pins that
// a question's loop swallows only its own activity: commit's r, its AI call failing,
// goes to the editor without asking again, and the message, stage, commit and push
// that follow, each a stage planned, all draw.
func TestRailGate_ACommitRegenerateThatFallsToTheEditorDrawsEveryStageAfterIt(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 30, answers("r\n", "\n"))
	r.RunStarted(presenter.RunInfo{
		Header: presenter.Header{Command: "commit", Where: pigeonWhere, Version: mintVersion},
		Start:  presenter.Station{Name: "main", Detail: "7c1e0b4"},
	})
	r.ShowPlan(presenter.Plan{Quiet: true, Stages: []presenter.PlanStage{
		{Key: "diff", Name: "diff", Detail: "every change"},
		{Key: "message", Name: "message", Detail: "written by the AI"},
		{Key: "stage", Name: "stage", Detail: "git add -A"},
		{Key: "commit", Name: "commit", Detail: "on main"},
		{Key: "push", Name: "push", Detail: "to its upstream"},
	}})
	r.StageSucceeded(presenter.Settled("diff", "4 files · +86 −12"))
	r.StageSucceeded(presenter.Settled("message", "written"))
	r.ShowMessage(presenter.Message{Body: "feat: retry deliveries"})
	if c, err := r.Prompt(commitGate()); err != nil || c != presenter.ChoiceRegen {
		t.Fatalf("Prompt = %q, %v, want r", c, err)
	}
	if _, err := r.AskLine("Add one-time context for regeneration (optional):"); err != nil {
		t.Fatal(err)
	}
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "regenerating commit message…"})
	r.Warn(presenter.Warning{Key: "message", Label: "ai", Message: "could not generate a message, opening editor", Output: "claude: timed out"})
	r.SuspendSpinner()
	r.ResumeSpinner()
	r.StageSucceeded(presenter.Settled("message", "written in your editor"))
	r.StageSucceeded(presenter.Settled("stage", "staged"))
	r.StageSucceeded(presenter.Settled("commit", "a3f9c21"))
	r.StageStarted(presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing…"})
	r.StageSucceeded(presenter.StageSuccess{Name: "push", Key: "push", Blocking: true, Elapsed: secs(1.2), Detail: "main → origin"})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeCommitted, End: presenter.Station{Name: "main", Detail: "a3f9c21 · pushed"}, Elapsed: secs(30)})
	screen := r.screen(t)
	for _, want := range []string{
		" !  message       could not generate a message, opening editor",
		" ●  message       written in your editor",
		" ●  stage         staged",
		" ●  commit        a3f9c21",
		row(" ●  push          main → origin", "1.2s"),
	} {
		if !slices.Contains(screen, want) {
			t.Errorf("the terminal shows\n%s\nwant %q on it", strings.Join(screen, "\n"), want)
		}
	}
}

// TestRailGate_AReAskTheInputClosedOnEndsTheLoop_WhatFollowsIsInEventOrder pins that
// a question's loop ends with the gate that ends unanswered: the unwind and the
// autostash's warning after it draw at the run's edge, in the order they came.
func TestRailGate_AReAskTheInputClosedOnEndsTheLoop_WhatFollowsIsInEventOrder(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 30, answers("e\n"))
	notesUnderReview(r, "first draft")
	if c, err := r.Prompt(reviewGate()); err != nil || c != presenter.ChoiceEdit {
		t.Fatalf("Prompt = %q, %v, want e", c, err)
	}
	editStage(r, nil)
	r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
	if _, err := r.Prompt(reviewGate()); !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("Prompt = %v, want ErrInputClosed", err)
	}
	r.Unwound(presenter.Unwind{Undone: "reset 1 commit"})
	r.Warn(presenter.Warning{Label: "autostash", Message: "could not restore stashed changes cleanly"})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "v1.3.2", Detail: "back where it started · nothing pushed"}, Elapsed: secs(20)})
	screen := r.screen(t)
	noEditRow(t, screen)
	order := []string{
		" ●  notes         written by the AI",
		" │    second draft",
		" ↩  unwind        reset 1 commit",
		" !  autostash     could not restore stashed changes cleanly",
		row(" ◇  v1.3.2        back where it started · nothing pushed", "20.0s"),
	}
	at := -1
	for _, want := range order {
		i := slices.Index(screen, want)
		if i <= at {
			t.Fatalf("the terminal shows\n%s\nwant, in this order\n%s", strings.Join(screen, "\n"), strings.Join(order, "\n"))
		}
		at = i
	}
}

// TestRailGate_UnderSixtyColumnsAnEditDrawsNoNode pins that the review's loop opens
// whether or not its body is still live: with no region, the body printed, the
// edit's stage still draws no node.
func TestRailGate_UnderSixtyColumnsAnEditDrawsNoNode(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 50, 40, answers("e\n", "y\n"))
	notesUnderReview(r, "first draft")
	for range 2 {
		c, err := r.Prompt(reviewGate())
		if err != nil {
			t.Fatal(err)
		}
		if c == presenter.ChoiceYes {
			break
		}
		editStage(r, nil)
		r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
	}
	screen := r.screen(t)
	noEditRow(t, screen)
	if !slices.Contains(screen, " │    second draft") {
		t.Errorf("the edited notes aren't on the terminal:\n%s", strings.Join(screen, "\n"))
	}
}

// TestRailGate_TheEchoIsCountedAsTheTerminalDrawsIt pins the rows a typed line's echo
// moves the cursor down, on a terminal 20 columns wide: a row filled leaves the
// cursor on it, a tab goes to its tab stop, a control character takes two columns,
// as ^X, a wide character that won't fit goes to the next row, and the Enter is a
// row only where the line ended in one.
func TestRailGate_TheEchoIsCountedAsTheTerminalDrawsIt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		line  string
		at    int
		ended bool
		want  int
	}{
		{name: "a line and its Enter", line: "abc", ended: true, want: 1},
		{name: "a line the input closed on", line: "abc", want: 0},
		{name: "a row filled", line: strings.Repeat("a", 20), ended: true, want: 1},
		{name: "a row and one more", line: strings.Repeat("a", 21), ended: true, want: 2},
		{name: "from the question's column", line: "abcdef", at: 15, ended: true, want: 2},
		{name: "tabs to their stops", line: "\t\tbbbbb", ended: true, want: 2},
		{name: "control characters as ^X", line: strings.Repeat("\x01", 11), ended: true, want: 2},
		{name: "a wide character on the last column", line: "界", at: 19, ended: true, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := presenter.EchoRows(tt.line, tt.at, 20, tt.ended); got != tt.want {
				t.Errorf("EchoRows(%q, %d) = %d, want %d", tt.line, tt.at, got, tt.want)
			}
		})
	}
}
