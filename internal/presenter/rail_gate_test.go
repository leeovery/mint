package presenter_test

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
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
// commitKeys is the commit review's keys, read as a typed line.
const commitKeys = " ┆  Y accept   n abort   e edit   r regenerate ›"

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
			" ?  Commit this message?", commitKeys), commitAhead...)
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
		want := append(append(append(commitHead[:4:4], shownBody...), " ?  Commit this message?", commitKeys), commitAhead...)
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
	at := slices.Index(screen, " ?  Add one-time context: ›")
	if at < 0 {
		t.Fatalf("the question isn't up:\n%s", strings.Join(screen, "\n"))
	}
	want := utf8.RuneCountInString(" ?  Add one-time context: › ")
	if row, col := r.term.Cursor(); row != at || col != want {
		t.Errorf("the cursor is at row %d, column %d, want row %d, column %d: a space after the input marker", row, col, at, want)
	}
	typed := "the export is the headline, " + strings.Repeat("and more ", 6)
	if _, err := r.term.Write([]byte(typed + "\n")); err != nil {
		t.Fatal(err)
	}
	in.lines <- typed + "\n"
	if a := <-got; a.err != nil || a.line != typed {
		t.Fatalf("AskLine = %q, %v, want the line typed, verbatim", a.line, a.err)
	}
	record := wrapped(" ?  Add one-time context: › "+typed, 80)
	wantScreen(t, r, slices.Concat(commitHead, []string{" ●  message       written", " │", " │    feat: retry deliveries", " │"}, record, commitAhead)...)
}

// wrapped is line as a terminal cols wide shows it, a row every cols cells, its
// trailing blanks trimmed; every rune of line is one cell.
func wrapped(line string, cols int) []string {
	var rows []string
	rest := []rune(line)
	for len(rest) > cols {
		rows, rest = append(rows, strings.TrimRight(string(rest[:cols]), " ")), rest[cols:]
	}
	return append(rows, strings.TrimRight(string(rest), " "))
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
		if got := r.screen(t)[4:11]; !slices.Contains(got, " ?  Use these notes?") || !slices.Contains(got, " ┆  Y accept   n abort   e edit   r regenerate ›") || slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, "○  review") }) {
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

func TestRailGate_TheExclusionPauseAsksInsideTheCheckAndItsAnswerTakesTheQuestionsPlace(t *testing.T) {
	t.Parallel()
	in := answers("y\n", "k\n")
	r := newRailTerm(t, 100, 30, in)
	preparedRelease(r, r.clock, "1 flagged")
	in.asked = func(n int) {
		if n == 1 {
			return
		}
		got := r.screen(t)[8:14]
		want := []string{
			pane(" ●  check         1 flagged                                                                     6.2s"),
			" ├┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄ ? Exclude docs/specs/** from the notes?",
			" │                  design documents · 6 files, 48 kB",
			" │                  e exclude   K keep ›",
			" ┆",
			" ○  notes         written by the AI",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("while asked the rail reads\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	if c, _ := r.Prompt(pauseGate()); c != presenter.ChoiceKeep {
		t.Fatalf("Prompt = %q, want k", c)
	}
	if got := r.screen(t)[9]; got != " │                – docs/specs/** kept · design documents" {
		t.Errorf("the answer reads %q, want it under the check", got)
	}
}

// TestRailGate_AnExclusionAnswerIsMarkedForWhatItDoes pins the mark each answer
// takes in the question's place: an exclude, which the run acts on, a ✓ in the
// accent, its text in the normal colour; a keep, which changes nothing, a – and all
// of it dim, -y's keep included. It does so however the check stands when the answer
// comes: still in the region, or settled already, as on a terminal too narrow for a
// region or once the question was printed to read a typed line at, where the answer
// hangs under the check's block as it would have under its node, and no node of its
// own draws for it.
func TestRailGate_AnExclusionAnswerIsMarkedForWhatItDoes(t *testing.T) {
	t.Parallel()
	modes := []struct {
		name       string
		cols, rows int
		// echo is a terminal that echoes each line typed at it where its cursor
		// stands, so a question reads its answer as a typed line printed last.
		echo bool
		// col is the detail column, where the mark goes.
		col int
	}{
		{name: "in the region", cols: 100, rows: 30, col: 18},
		{name: "printed as it happens, under sixty columns", cols: 50, rows: 40, col: 4},
		{name: "after a typed line the terminal echoes", cols: 100, rows: 30, echo: true, col: 18},
	}
	tests := []struct {
		name  string
		typed []string
		yes   bool
		row   string
		kept  bool
	}{
		{name: "an exclude", typed: []string{"y\n", "e\n"}, row: "✓ docs/specs/** excluded · design documents"},
		{name: "a keep", typed: []string{"y\n", "k\n"}, row: "– docs/specs/** kept · design documents", kept: true},
		{name: "a keep -y took", yes: true, row: "– docs/specs/** kept (-y) · design documents", kept: true},
	}
	for _, mode := range modes {
		for _, tt := range tests {
			t.Run(mode.name+", "+tt.name, func(t *testing.T) {
				t.Parallel()
				in := answers(tt.typed...)
				r := newRailTermIn(t, termenv.TrueColor, mode.cols, mode.rows, in, presenter.WithRailEcho(mode.echo))
				if mode.echo {
					in.asked = func(int) { _, _ = io.WriteString(r.term, strings.ReplaceAll(in.answers[0], "\n", "\r\n")) }
				}
				r.WithYes(tt.yes)
				preparedRelease(r, r.clock, "1 flagged")
				_, _ = r.Prompt(pauseGate())
				g, err := frametest.Parse(r.frame(t))
				if err != nil {
					t.Fatal(err)
				}
				want := " │" + strings.Repeat(" ", mode.col-2) + tt.row
				at := slices.Index(g.Text(), want)
				if at < 0 {
					t.Fatalf("no answer reads %q on\n%s", want, strings.Join(g.Text(), "\n"))
				}
				if mark := g[at][mode.col]; !tt.kept && mark.FG != accent || tt.kept && !mark.Faint {
					t.Errorf("the mark draws %v, want a ✓ in the accent or a – dim", mark)
				}
				if glob := g[at][mode.col+2]; glob.Faint != tt.kept {
					t.Errorf("the glob draws %v, want faint %v", glob, tt.kept)
				}
				nodes := slices.DeleteFunc(g.Text(), func(line string) bool { return !strings.HasPrefix(line, " ●  check") })
				if len(nodes) != 1 {
					t.Errorf("the check draws as %d done nodes, want one, the answer hung under its block", len(nodes))
				}
			})
		}
	}
}

// TestRailGate_OnAShortTerminalAnswersHangUnderTheSettledCheck pins that once the
// check has settled out of a short terminal's region, each answer after it still
// hangs as its line, marked for what it does, and draws no node of its own.
func TestRailGate_OnAShortTerminalAnswersHangUnderTheSettledCheck(t *testing.T) {
	t.Parallel()
	const pauses = 14
	typed := []string{"y\n"}
	for i := range pauses {
		typed = append(typed, []string{"e\n", "k\n"}[i%2])
	}
	r := newRailTerm(t, 100, 14, answers(typed...))
	preparedRelease(r, r.clock, fmt.Sprintf("%d flagged", pauses))
	for i := range pauses {
		x := presenter.ExclusionSuggestion{Glob: fmt.Sprintf("gen/part%02d/", i), Reason: "generated", Matches: "1 file, 1 kB"}
		g := pauseGate()
		g.Suggestion, g.Subject, g.Question = &x, x.Glob, "Exclude "+x.Glob+" from the notes? (generated; 1 file, 1 kB)"
		if _, err := r.Prompt(g); err != nil {
			t.Fatal(err)
		}
	}
	r.ExclusionAnswersRecorded(presenter.ExclusionAnswers{Stage: "notes", Key: "check", File: ".mint.toml", Table: "[release]"})
	screen := r.screen(t)
	if !slices.Contains(screen, " │                written to .mint.toml's [release] table and committed") {
		t.Errorf("the screen shows\n%s\nwant where the answers went hung under them", strings.Join(screen, "\n"))
	}
	for i := pauses - 4; i < pauses; i++ {
		want := fmt.Sprintf(" │                ✓ gen/part%02d/ excluded · generated", i)
		if i%2 == 1 {
			want = fmt.Sprintf(" │                – gen/part%02d/ kept · generated", i)
		}
		if !slices.Contains(screen, want) {
			t.Errorf("the screen shows\n%s\nwant %q", strings.Join(screen, "\n"), want)
		}
	}
	if nodes := slices.DeleteFunc(screen, func(line string) bool { return !strings.HasPrefix(line, " ●  check") }); len(nodes) != 1 {
		t.Errorf("the check draws as %d done nodes, want one, each answer hung under its block", len(nodes))
	}
}

// TestRailGate_OnAShortTerminalEveryPauseIsOnScreenWhileItWaits pins that the
// answers hung under the check never push the open pause off a short terminal: what
// can settle settles, the check's node with it, and each pause waits on the screen.
func TestRailGate_OnAShortTerminalEveryPauseIsOnScreenWhileItWaits(t *testing.T) {
	t.Parallel()
	const pauses = 14
	typed := []string{"y\n"}
	for range pauses {
		typed = append(typed, "e\n")
	}
	in := answers(typed...)
	r := newRailTerm(t, 100, 14, in)
	var asking string
	in.asked = func(n int) {
		if n == 1 {
			return
		}
		if screen := r.term.Screen(); !slices.ContainsFunc(screen, func(line string) bool { return strings.Contains(line, asking) }) {
			t.Errorf("while %s is asked the screen shows\n%s\nwant it on screen", asking, strings.Join(screen, "\n"))
		}
	}
	preparedRelease(r, r.clock, fmt.Sprintf("%d flagged", pauses))
	for i := range pauses {
		x := presenter.ExclusionSuggestion{Glob: fmt.Sprintf("gen/part%02d/", i), Reason: "generated", Matches: "1 file, 1 kB"}
		g := pauseGate()
		g.Suggestion, g.Subject, g.Question = &x, x.Glob, "Exclude "+x.Glob+" from the notes? (generated; 1 file, 1 kB)"
		asking = x.Glob
		if _, err := r.Prompt(g); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRailGate_TheKeysMarkTheGatesDefault pins that the default marked on the keys
// line is the gate's own, whichever key that is, and that Enter takes it.
func TestRailGate_TheKeysMarkTheGatesDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		gate presenter.Gate
		keys string
	}{
		{
			name: "a key, a capital",
			gate: presenter.Gate{Question: "Go on?", Default: presenter.ChoiceNo, Choices: []presenter.GateChoice{{Key: presenter.ChoiceYes, Action: "yes"}, {Key: presenter.ChoiceNo, Action: "no"}}},
			keys: " ┆  y yes   N no ›",
		},
		{
			name: "a word's letter, a capital",
			gate: presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "tag"),
			keys: " ┆  f fresh (re-diff + AI)   T tag (tag annotation body) ›",
		},
		{
			name: "a word alike in its first letter, marked after its action",
			gate: presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "file", Action: "a notes file"}}, "file"),
			keys: " ┆  fresh re-diff + AI   file a notes file (default) ›",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := answers("\n")
			r := newRailTerm(t, 80, 24, in)
			r.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})
			in.asked = func(int) {
				if screen := r.screen(t); !slices.Contains(screen, tt.keys) {
					t.Errorf("asked, the terminal shows\n%s\nwant the keys %q", strings.Join(screen, "\n"), tt.keys)
				}
			}
			if c, err := r.Prompt(tt.gate); err != nil || c != tt.gate.Default {
				t.Fatalf("Prompt = %q, %v, want Enter to take the default %q", c, err, tt.gate.Default)
			}
		})
	}
}

// TestRailGate_AWordAnswersToItsFirstLetter pins that a question whose keys are
// words, regenerate's Source and Target, is asked as every other is: each word keyed
// by its first letter, the default's a capital, the word, then what it means, dim in
// parentheses, unless that only repeats the word. Its letter answers it, as does the
// word typed in full, and Enter takes the default.
func TestRailGate_AWordAnswersToItsFirstLetter(t *testing.T) {
	t.Parallel()
	source := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}, {Key: "release", Action: "published release body"}}, "fresh")
	target := presenter.TargetGate([]presenter.GateChoice{{Key: "release", Action: "provider release"}, {Key: "changelog", Action: "CHANGELOG.md"}, {Key: "both", Action: "both"}}, "release")
	const sourceKeys = " ┆  F fresh (re-diff + AI)   t tag (tag annotation body)   r release (published release body) ›"
	const targetKeys = " ┆  R release (provider release)   c changelog (CHANGELOG.md)   b both ›"
	tests := []struct {
		name  string
		gate  presenter.Gate
		keys  string
		typed string
		want  presenter.Choice
	}{
		{name: "its letter", gate: source, keys: sourceKeys, typed: "t", want: "tag"},
		{name: "its letter as a capital", gate: source, keys: sourceKeys, typed: "R", want: "release"},
		{name: "the word in full", gate: source, keys: sourceKeys, typed: "tag", want: "tag"},
		{name: "Enter, the default", gate: source, keys: sourceKeys, typed: "", want: "fresh"},
		{name: "a letter whose word only repeats its meaning", gate: target, keys: targetKeys, typed: "b", want: "both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := answers(tt.typed + "\n")
			r := newRailTerm(t, 100, 24, in)
			r.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})
			in.asked = func(int) {
				if screen := r.screen(t); !slices.Contains(screen, tt.keys) {
					t.Errorf("asked, the terminal shows\n%s\nwant the keys %q", strings.Join(screen, "\n"), tt.keys)
				}
			}
			if c, err := r.Prompt(tt.gate); err != nil || c != tt.want {
				t.Fatalf("Prompt = %q, %v, want %q", c, err, tt.want)
			}
		})
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
	if got := r.screen(t)[4]; got != " ●  source        tag" {
		t.Errorf("the answer reads %q, want a row of its own naming the value chosen, the question asked before any stage", got)
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
// engine runs it: what an e runs, the edit's stage around the editor, draws no node;
// the body is shown again in place, the question comes back under it, and the accept
// settles the notes with its tail. A warning drawn while the loop is open, an editor
// that wouldn't launch, came before the answer, so the answer records on a row of its
// own under it. An r runs a stage of the run's again, its own node
// (TestRailGate_ARegenerateSettlesTheAskedNodeAndRunsItsStageAgainBelow).
func TestRailGate_WhatTheReviewsLoopRunsIsTheQuestions(t *testing.T) {
	t.Parallel()
	question := []string{" ?  Release with these notes?", " ┆  Y release   n abort   e edit   r regenerate ›"}
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
				want = append(want, question...)
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
				want = []string{" ●  notes         written by the AI", warned, " ●  review        accepted"}
			}
			if at := slices.Index(screen, want[0]); at < 0 || !slices.Equal(screen[at:min(at+len(want), len(screen))], want) {
				t.Errorf("once accepted the terminal shows\n%s\nwant the answer drawn where it came\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
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
	const question = " ?  Commit this message?"
	in.asked = func(int) {
		if screen := r.screen(t); !slices.Contains(screen, question) || !slices.Contains(screen, commitKeys) {
			t.Errorf("the question isn't up:\n%s", strings.Join(screen, "\n"))
		}
	}
	if _, err := r.Prompt(commitGate()); err != nil {
		t.Fatal(err)
	}
	screen := r.screen(t)
	if !slices.Contains(screen, " ●  message       written · accepted") || slices.Contains(screen, question) || slices.Contains(screen, commitKeys) {
		t.Errorf("once answered the terminal shows\n%s\nwant the question gone and the message settled with the answer", strings.Join(screen, "\n"))
	}
}

// cursorUp matches a move of the cursor up.
var cursorUp = regexp.MustCompile(`\x1b\[\d*A`)

// TestRailGate_ATypedLineIsReadAtTheQuestionPrintedLast pins the bound on every
// typed line the terminal echoes: the question is the last thing on the terminal,
// the cursor at its end and nothing drawn below it, so the echo, however it wraps,
// lands only on blank rows. The question and the line stay as typed, each row of
// them holding nothing else, and the region is drawn afresh under them, nothing
// moved up, with a stage still running above the question live again in it.
func TestRailGate_ATypedLineIsReadAtTheQuestionPrintedLast(t *testing.T) {
	t.Parallel()
	// question takes 28 columns, leaving 52 on its row.
	const question = " ?  Add one-time context: › "
	x := func(n int) string { return strings.Repeat("x", n) }
	reviewed := func(r *railTerm) { messageWritten(r, "feat: retry deliveries") }
	running := func(r *railTerm) {
		commitRun(r)
		r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing…"})
	}
	tests := []struct {
		name   string
		before func(r *railTerm)
		typed  string
		// closed is the input closing after the line, with no Enter typed.
		closed bool
		// rows are the question's row on: the question and the line as typed, then the
		// region's first row.
		rows []string
	}{
		{name: "a line and its Enter", before: reviewed, typed: "the export", rows: []string{question + "the export", " ┆"}},
		{name: "a line the input closed on", before: reviewed, typed: "abc", closed: true, rows: []string{question + "abc", " ┆"}},
		{name: "a wrap that leaves one character", before: reviewed, typed: x(53), rows: []string{question + x(52), "x", " ┆"}},
		{name: "a wrap at exactly the width", before: reviewed, typed: x(52), rows: []string{question + x(52), " ┆"}},
		{name: "two rows filled exactly", before: reviewed, typed: x(132), rows: []string{question + x(52), x(80), " ┆"}},
		{name: "under a stage still running", before: running, typed: x(53), rows: []string{question + x(52), "x", row(" ⠋  message       writing…", "0s")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line := tt.typed
			if !tt.closed {
				line += "\n"
			}
			in := answers(line)
			r := newRailTerm(t, 80, 30, in)
			r.WithEcho(true)
			tt.before(r)
			var before int
			in.asked = func(int) {
				if screen := r.screen(t); screen[len(screen)-1] != strings.TrimRight(question, " ") {
					t.Errorf("while the line is typed the terminal shows\n%s\nwant the question last", strings.Join(screen, "\n"))
				}
				if _, err := r.term.Write([]byte(line)); err != nil {
					t.Fatal(err)
				}
				before = len(r.writes.written())
			}
			if _, err := r.AskLine("Add one-time context:"); err != nil {
				t.Fatal(err)
			}
			if after := r.writes.written()[before:]; cursorUp.MatchString(after) {
				t.Errorf("once the line was read the writer wrote %q, want nothing moved up", after)
			}
			screen := r.screen(t)
			at := slices.IndexFunc(screen, func(s string) bool { return strings.HasPrefix(s, question) })
			if at < 0 || !slices.Equal(screen[at:min(at+len(tt.rows), len(screen))], tt.rows) {
				t.Errorf("the terminal shows\n%s\nwant from the question on\n%s", strings.Join(screen, "\n"), strings.Join(tt.rows, "\n"))
			}
		})
	}
}

// TestRailGate_ATypedAnswerAskedAgainPrintsTheQuestionAgainUnderTheLine pins that a
// re-ask reads its line the same way: the question printed again under the line
// that answered nothing, last on the terminal. A question asked before the run's
// stages has no node to settle, so the question and the line typed at it are its
// answer's record, and no row repeats it.
func TestRailGate_ATypedAnswerAskedAgainPrintsTheQuestionAgainUnderTheLine(t *testing.T) {
	t.Parallel()
	const question, keys = " ?  Source?", " ┆  F fresh (re-diff + AI)   t tag (tag annotation body) › "
	in := answers("nope\n", "tag\n")
	r := newRailTerm(t, 80, 24, in)
	r.WithEcho(true)
	r.RunStarted(presenter.RunInfo{Header: presenter.Header{Command: "release regenerate", Where: pigeonWhere}, Start: presenter.Station{Name: "v1.4.0"}})
	in.asked = func(n int) {
		if screen := r.screen(t); !slices.Equal(screen[len(screen)-2:], []string{question, strings.TrimRight(keys, " ")}) {
			t.Errorf("asked %d, the terminal shows\n%s\nwant the question last", n, strings.Join(screen, "\n"))
		}
		if _, err := r.term.Write([]byte([]string{"nope\n", "tag\n"}[n-1])); err != nil {
			t.Fatal(err)
		}
	}
	g := presenter.SourceGate([]presenter.GateChoice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}, "fresh")
	if c, err := r.Prompt(g); err != nil || c != "tag" {
		t.Fatalf("Prompt = %q, %v, want tag", c, err)
	}
	screen := r.screen(t)
	want := []string{question, keys + "nope", question, keys + "tag"}
	if at := slices.Index(screen, want[0]); at < 0 || !slices.Equal(screen[at:], want) {
		t.Errorf("the terminal shows\n%s\nwant\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
	}
}

// TestRailGate_WithNoRegionATypedLineWritesNoEscape pins that on a terminal with no
// region, too narrow or too short, a typed line's end writes only the newline a line
// with no Enter needs: no move, no erase.
func TestRailGate_WithNoRegionATypedLineWritesNoEscape(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ cols, rows int }{{50, 40}, {80, 11}} {
		t.Run(fmt.Sprintf("%dx%d", size.cols, size.rows), func(t *testing.T) {
			t.Parallel()
			in := answers("abc")
			r := newRailTerm(t, size.cols, size.rows, in)
			r.WithEcho(true)
			messageWritten(r, "feat: retry deliveries")
			in.asked = func(int) {
				if _, err := r.term.Write([]byte("abc")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.AskLine("Add context:"); err != nil {
				t.Fatal(err)
			}
			r.StageSucceeded(presenter.Settled("commit", "a3f9c21"))
			if all := r.writes.written(); strings.Contains(all, "\x1b[") || strings.Contains(all, "\r") {
				t.Errorf("wrote %q, want no escape and no carriage return", all)
			}
			screen := r.screen(t)
			if at := slices.Index(screen, " ?  Add context: › abc"); at < 0 || at+1 >= len(screen) || !strings.HasPrefix(screen[at+1], " ●  commit") {
				t.Errorf("the terminal shows\n%s\nwant the line as typed, then the commit on a row of its own", strings.Join(screen, "\n"))
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

// TestRailGate_ATypedLineUnderAStageStillRunningIsReadUnderTheSettledLines pins that
// settling ahead of a typed read never settles a stage still running: the region
// comes off the terminal, the question prints under the settled lines and the line
// is typed under it, then the region comes back below, the stage live, its spinner
// ticking and its bar drawn again.
func TestRailGate_ATypedLineUnderAStageStillRunningIsReadUnderTheSettledLines(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 80, 24, in)
	r.WithEcho(true)
	r.RunStarted(releaseStarted(false))
	r.ShowPlan(releasePlan())
	r.StageStarted(notesStarted())
	const question, marker = " ?  Cached notes found for this diff — use them?", " ┆  Y use cached   r regenerate ›"
	in.asked = func(int) {
		screen := r.screen(t)
		if !slices.Equal(screen[len(screen)-2:], []string{question, marker}) || slices.ContainsFunc(screen, func(s string) bool { return strings.Contains(s, "notes   ") }) {
			t.Errorf("while the line is read the terminal shows\n%s\nwant the question last, the region off it", strings.Join(screen, "\n"))
		}
		if _, err := r.term.Write([]byte("y\n")); err != nil {
			t.Fatal(err)
		}
	}
	r.SuspendSpinner()
	if _, err := r.Prompt(cacheGate()); err != nil {
		t.Fatal(err)
	}
	r.ResumeSpinner()
	if !r.ticks.running() {
		t.Error("the spinner doesn't tick again for the stage still running")
	}
	r.NotesProgress(presenter.NotesProgress{Parts: 4, Done: 1})
	screen := r.screen(t)
	running := slices.IndexFunc(screen, func(s string) bool { return strings.HasPrefix(s, " ⠋  notes") })
	answered := slices.Index(screen, marker+" y")
	if running < 0 || answered < 1 || screen[answered-1] != question || running < answered || !strings.Contains(screen[running], "1 of 4 parts done") {
		t.Errorf("the terminal shows\n%s\nwant the notes running, with their bar, under the question as answered", strings.Join(screen, "\n"))
	}
	if n := len(slices.DeleteFunc(slices.Clone(screen), func(s string) bool { return !strings.Contains(s, "  notes   ") })); n != 1 {
		t.Errorf("the notes are on the terminal %d times, want once, live:\n%s", n, strings.Join(screen, "\n"))
	}
}

// TestRailGate_ARegenerateSettlesTheAskedNodeAndRunsItsStageAgainBelow pins that a
// finished node is final: an r settles the node it asked about with its answer, the
// body under review staying under it, in view as the one-time context is typed under
// it, and the stage runs again as a node of its own below, its body settling as it
// is shown and the next answer recorded on a row under it, for the release's r and
// the commit's.
func TestRailGate_ARegenerateSettlesTheAskedNodeAndRunsItsStageAgainBelow(t *testing.T) {
	t.Parallel()
	const (
		context    = "shorter please"
		firstDraft = " │    first draft"
	)
	tests := []struct {
		name string
		// start is the run to its review with its first body shown, gate the question
		// it asks and regenerate the stage its r runs again.
		start      func(r *railTerm)
		gate       presenter.Gate
		regenerate func(r *railTerm)
		// asked is the asked node as the r settles it, again the node it runs again.
		asked, again string
	}{
		{
			name:  "the release's notes",
			start: func(r *railTerm) { notesUnderReview(r, "first draft") },
			gate:  reviewGate(),
			regenerate: func(r *railTerm) {
				r.StageStarted(presenter.StageStart{Name: "notes", Key: "notes", Blocking: true, Text: "generating release notes…", Quiet: true})
				r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(3), Detail: "generated", Quiet: true})
				r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
			},
			asked: " ●  notes         written by the AI · regenerate",
			again: row(" ●  notes         generated", "3.0s"),
		},
		{
			name:  "the commit's message",
			start: func(r *railTerm) { messageWritten(r, "first draft") },
			gate:  commitGate(),
			regenerate: func(r *railTerm) {
				r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "regenerating commit message…"})
				r.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Blocking: true, Elapsed: secs(3), Detail: "regenerated"})
				r.ShowMessage(presenter.Message{Body: "second draft"})
			},
			asked: " ●  message       written · regenerate",
			again: row(" ●  message       regenerated", "3.0s"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := answers("r\n", context+"\n", "y\n")
			r := newRailTerm(t, 80, 30, in)
			tt.start(r)
			in.asked = func(n int) {
				if n == 2 {
					if !slices.Contains(r.screen(t), firstDraft) {
						t.Errorf("as the context is asked the terminal shows\n%s\nwant the draft %q in view", strings.Join(r.screen(t), "\n"), firstDraft)
					}
					if _, err := r.term.Write([]byte(context + "\n")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if c, err := r.Prompt(tt.gate); err != nil || c != presenter.ChoiceRegen {
				t.Fatalf("Prompt = %q, %v, want r", c, err)
			}
			r.WithEcho(true)
			if _, err := r.AskLine("Add one-time context for regeneration (optional):"); err != nil {
				t.Fatal(err)
			}
			r.WithEcho(false)
			tt.regenerate(r)
			if _, err := r.Prompt(tt.gate); err != nil {
				t.Fatal(err)
			}
			screen := r.screen(t)
			order := []string{
				tt.asked,
				firstDraft,
				" ?  Add one-time context for regeneration (optional): › " + context,
				tt.again,
				" │    second draft",
				" ●  review        accepted",
			}
			at := -1
			for _, want := range order {
				i := slices.Index(screen, want)
				if i <= at {
					t.Fatalf("the terminal shows\n%s\nwant, in this order\n%s", strings.Join(screen, "\n"), strings.Join(order, "\n"))
				}
				at = i
			}
		})
	}
}

// TestRailGate_TheReviewsRecordRowDrawsItsAnswerAsADetail pins the record row's
// style: the answer a review records on a row of its own, after an r, is the row's
// detail, inked as a done node's detail is, not a dim tail.
func TestRailGate_TheReviewsRecordRowDrawsItsAnswerAsADetail(t *testing.T) {
	t.Parallel()
	r := newRailTermIn(t, termenv.TrueColor, 80, 30, answers("r\n", "y\n"))
	notesUnderReview(r, "first draft")
	if _, err := r.Prompt(reviewGate()); err != nil {
		t.Fatal(err)
	}
	r.StageStarted(presenter.StageStart{Name: "notes", Key: "notes", Blocking: true, Text: "generating release notes…", Quiet: true})
	r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(3), Detail: "generated", Quiet: true})
	r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "second draft"})
	if _, err := r.Prompt(reviewGate()); err != nil {
		t.Fatal(err)
	}

	g, err := frametest.Parse(r.frame(t))
	if err != nil {
		t.Fatal(err)
	}
	text := g.Text()
	recorded := slices.IndexFunc(text, func(s string) bool { return strings.HasPrefix(s, " ●  review        accepted") })
	again := slices.IndexFunc(text, func(s string) bool { return strings.HasPrefix(s, " ●  notes         generated") })
	if recorded < 0 || again < 0 {
		t.Fatalf("the terminal shows\n%s\nwant the notes again and the review's record row", strings.Join(text, "\n"))
	}
	const detailAt = 18
	if got, want := g[recorded][detailAt], g[again][detailAt]; got.Pen != want.Pen {
		t.Errorf("the record row's answer is inked %v, want a done node's detail, %v", got.Pen, want.Pen)
	}
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

// TestRailGate_AReAskTheInputClosedOn_WhatFollowsIsInEventOrder pins that
// when the input closes at a re-ask, what follows still draws in event order:
// the unwind and the autostash's warning after it land at the run's edge, placed by key.
func TestRailGate_AReAskTheInputClosedOn_WhatFollowsIsInEventOrder(t *testing.T) {
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

// TestRailGate_ABodyShownAgainUnchangedSettlesOnce pins that once a body of the
// question has settled, too tall for the region, the same body shown again, after an
// e whose editor won't launch, isn't settled a second time, and the answer records on
// a row under it.
func TestRailGate_ABodyShownAgainUnchangedSettlesOnce(t *testing.T) {
	t.Parallel()
	in := answers("e\n", "y\n")
	r := newRailTerm(t, 80, 14, in)
	lines := []string{"feat: retry deliveries", ""}
	for i := range 16 {
		lines = append(lines, fmt.Sprintf("- delivery change %d", i+1))
	}
	tall := strings.Join(lines, "\n")
	messageWritten(r, tall)
	if c, err := r.Prompt(commitGate()); err != nil || c != presenter.ChoiceEdit {
		t.Fatalf("Prompt = %q, %v, want e", c, err)
	}
	r.SuspendSpinner()
	r.ResumeSpinner()
	r.Warn(presenter.Warning{Key: "message", Label: "editor", Message: "could not launch editor, keeping the message"})
	r.ShowMessage(presenter.Message{Body: tall})
	if _, err := r.Prompt(commitGate()); err != nil {
		t.Fatal(err)
	}
	screen := r.screen(t)
	if n := count(screen, " │    feat: retry deliveries"); n != 1 {
		t.Errorf("the message is on the terminal %d times, want once:\n%s", n, strings.Join(screen, "\n"))
	}
	if body, answer := slices.Index(screen, " │    feat: retry deliveries"), slices.Index(screen, " ●  review        accepted"); answer < body {
		t.Errorf("the terminal shows\n%s\nwant the answer on a row under the body", strings.Join(screen, "\n"))
	}
}
