package presenter_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// This file pins COMPOSITION: it drives the spec's full worked-example event
// sequence end-to-end through each presenter and asserts the complete composed
// transcript. Every individual event's rendering is unit-tested in isolation
// elsewhere (plain_test.go / pretty_*.go); these two golden tests pin how those
// per-event renderings ASSEMBLE — inter-block spacing, the notes block sitting
// between the plan and the gate, the gate echo/menu placement, and the trailing
// footer landing after the last stage.
//
// CRITICAL reconciliation with the spec: the spec's two worked-example transcript
// blocks (specification/cli-presentation/specification.md) are illustrative
// SNAPSHOTS. The presenters legitimately differ from the snapshot in ways prior
// tasks established, and these goldens reflect the IMPLEMENTATION's real output:
//
//   - Inter-block blank lines: the spec snapshots show decorative blank lines
//     between blocks (after the brand line, around the notes block, around the
//     menu). The presenters emit NO such blank lines — each method writes only its
//     own line(s). The ONLY blank line in either transcript is the one renderGate
//     itself writes between the menu options and the question. The goldens are
//     therefore tightly packed; that packing is exactly the composition being
//     pinned.
//   - Plain blocking-start lines: the plain presenter emits "{name}: running..." on
//     a blocking StageStarted. This sequence drives ONLY StageSucceeded for the
//     blocking prep/notes stages (NOT StageStarted), matching the spec plain
//     snapshot's shown lines (which omit the transient start lines) and keeping the
//     SAME event sequence drivable across both modes — in pretty those blocking
//     StageStarted events would feed the spinner, so omitting them keeps the two
//     goldens aligned on one canonical sequence. The pretty golden drives the
//     blocking StageStarted events (so the spinner lifecycle is exercised) but uses
//     the spy/no-op spinner factory so NO live ⠋ frame is written.
//   - Notes body: rendered VERBATIM / flush (writeNotesBody), NOT indented — even
//     though the pretty snapshot shows the body indented two spaces. Both goldens'
//     body lines are flush.
//   - Pretty notes RULES: also flush-left (no two-space stage indent), even though
//     the pretty snapshot shows them indented under the stage column. ShowNotes
//     writes the titled/closing rule with no stageIndent, so the golden's rule lines
//     start at column 0 — another snapshot-vs-implementation reconciliation pinned
//     here.
//   - Pretty colour: forced to termenv.Ascii so the golden is byte-stable layout
//     with no ANSI — the composition test is about LAYOUT, not colour (colour is
//     separately tested). stripANSI is therefore unnecessary here.

// The engine-supplied details that produce the spec's wording. Centralised so the
// two goldens (and any composition-mutation experiment) drive ONE canonical
// sequence; the per-mode rendering differences come from the presenters, not from
// divergent inputs.
const (
	transcriptProject = "acme"
	transcriptVersion = "1.4.0"
	transcriptAction  = "releasing"
	transcriptURL     = "https://github.com/acme/acme/releases/tag/v1.4.0"

	transcriptVersionDetail   = "v1.3.2 → v1.4.0 (minor)"
	transcriptPreflightDetail = "clean · on main · tag free · in sync with origin"
	transcriptPrepDetail      = "pre_tag: npm ci && npm run build"
	transcriptNotesDetail     = "generated"
)

// The notes body is the package-shared notesBody const (defined in plain_test.go):
// reusing it keeps this composition golden's body driven by the SAME bytes the
// byte-identity tests assert, so the body can never drift between the two.

// transcriptPlan is the engine-supplied plan steps, verbatim. The plain one-liner
// and the pretty bulleted block both render from this SAME slice.
func transcriptPlan() presenter.Plan {
	return presenter.Plan{Steps: []presenter.PlanStep{
		{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
		{Verb: "tag", Target: "v1.4.0 (annotated)"},
		{Verb: "push", Target: "--atomic → origin"},
		{Verb: "publish", Target: "GitHub release"},
	}}
}

// driveWorkedExample drives the FULL spec worked-example event sequence through a
// presenter, capturing the composed narration. Both goldens drive the SAME ordered
// sequence so the assertion is purely about how each mode COMPOSES it. driveBlocking
// controls whether the two blocking stages (prep, notes) emit a StageStarted before
// their StageSucceeded: the pretty run drives them (feeding the spy spinner), the
// plain run does not (matching the spec plain snapshot, whose start lines are
// omitted). The gate is driven via Prompt(NotesReviewGate()) at the notes-review
// point — under -y it renders the auto-accept echo; interactively it renders the
// menu and reads one line.
func driveWorkedExample(p presenter.Presenter, driveBlocking bool) {
	p.RunStarted(presenter.RunInfo{Action: transcriptAction, Project: transcriptProject, Version: transcriptVersion})
	p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: transcriptVersionDetail})
	p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: transcriptPreflightDetail})
	p.ShowPlan(transcriptPlan())
	if driveBlocking {
		p.StageStarted(presenter.StageStart{Name: "prep", Blocking: true})
	}
	p.StageSucceeded(presenter.StageSuccess{Name: "prep", Detail: transcriptPrepDetail, Elapsed: 2300 * time.Millisecond, Blocking: true})
	if driveBlocking {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	}
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: transcriptNotesDetail, Elapsed: 1100 * time.Millisecond, Blocking: true})
	p.ShowNotes(presenter.Notes{Version: transcriptVersion, Body: notesBody})
	_, _ = p.Prompt(presenter.NotesReviewGate())
	p.StageSucceeded(presenter.StageSuccess{Name: "record", Detail: "CHANGELOG.md + bin/acme"})
	p.StageSucceeded(presenter.StageSuccess{Name: "tag/push", Detail: "v1.4.0 pushed (atomic)"})
	p.StageSucceeded(presenter.StageSuccess{Name: "publish", Detail: "github release created"})
	p.RunFinished(presenter.RunResult{Verb: presenter.VerbRelease, Project: transcriptProject, Version: transcriptVersion, URL: transcriptURL})
}

// TestPlainGoldenWorkedExampleTranscript drives the full plain -y worked-example
// sequence and asserts the complete composed transcript byte-for-byte. The gate is
// SKIPPED under -y (no menu), emitting the "notes: accepted (-y)" echo between the
// notes block and the record stage. The expected string below is built by reasoning
// about each event's known plain rendering and the sequence ordering — it is NOT a
// capture-and-compare-to-itself.
func TestPlainGoldenWorkedExampleTranscript(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, strings.NewReader("")).WithYes(true)

	driveWorkedExample(p, false)

	want := "mint: releasing acme v1.4.0\n" +
		"version: v1.3.2 → v1.4.0 (minor)\n" +
		"preflight: clean · on main · tag free · in sync with origin\n" +
		"plan: commit CHANGELOG.md + bin/acme; tag v1.4.0 (annotated); push --atomic → origin; publish GitHub release\n" +
		"prep: pre_tag: npm ci && npm run build (2.3s)\n" +
		"notes: generated (1.1s)\n" +
		"--- release notes v1.4.0 ---\n" +
		notesBody + "\n" +
		"--- end notes ---\n" +
		"notes: accepted (-y)\n" +
		"record: CHANGELOG.md + bin/acme\n" +
		"tag/push: v1.4.0 pushed (atomic)\n" +
		"publish: github release created\n" +
		"done: acme v1.4.0 https://github.com/acme/acme/releases/tag/v1.4.0\n"

	if got := out.String(); got != want {
		t.Errorf("plain worked-example transcript mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// The -y skip reads no input and writes nothing to err on a clean run.
	if errBuf.Len() != 0 {
		t.Errorf("plain -y run wrote to err, want nothing:\n%q", errBuf.String())
	}
}

// TestPrettyGoldenWorkedExampleTranscript drives the full pretty worked-example
// sequence with a SPY/no-op spinner (so no live ⠋ frame is written) and a fixed
// Ascii profile (so no ANSI — the golden is stable layout), and a scripted reader
// ("y\n") so the gate renders the vertical menu and reads y INTERACTIVELY. It
// asserts the complete composed transcript: brand line, stages, plan, notes block,
// gate menu + "Continue? › " prompt, record/tag/push/publish stages, then the
// bottom brand footer. The expected string is reasoned from each event's known
// pretty rendering, NOT captured-and-compared-to-itself.
func TestPrettyGoldenWorkedExampleTranscript(t *testing.T) {
	out := &bytes.Buffer{}
	tr := &spyTracker{}
	p := presenter.NewPrettyPresenter(
		out,
		presenter.WithProfile(termenv.Ascii),
		presenter.WithInput(strings.NewReader("y\n")),
	).WithSpinnerFactory(tr.factory())

	driveWorkedExample(p, true)

	want := "🌿 mint · acme  ›  releasing v1.4.0\n" +
		"  ✓ version    v1.3.2 → v1.4.0 (minor)\n" +
		"  ✓ preflight  clean · on main · tag free · in sync with origin\n" +
		"  Plan\n" +
		"    • commit   CHANGELOG.md + bin/acme\n" +
		"    • tag      v1.4.0 (annotated)\n" +
		"    • push     --atomic → origin\n" +
		"    • publish  GitHub release\n" +
		"  ✓ prep       pre_tag: npm ci && npm run build (2.3s)\n" +
		"  ✓ notes      generated (1.1s)\n" +
		notesTitledRule(transcriptVersion) + "\n" +
		notesBody + "\n" +
		notesClosingRule() + "\n" +
		"    y  accept & proceed [default]\n" +
		"    n  abort\n" +
		"    e  edit in $EDITOR\n" +
		"    r  regenerate\n" +
		"\n" +
		"  Continue? › " +
		"  ✓ record     CHANGELOG.md + bin/acme\n" +
		"  ✓ tag/push   v1.4.0 pushed (atomic)\n" +
		"  ✓ publish    github release created\n" +
		"🌿 released acme v1.4.0 · https://github.com/acme/acme/releases/tag/v1.4.0\n"

	if got := out.String(); got != want {
		t.Errorf("pretty worked-example transcript mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// The spy spinner factory built exactly the two blocking-stage spinners and left
	// none running — proving no live animation frame contaminated the composed golden.
	if len(tr.created) != 2 {
		t.Errorf("expected two spy spinners (prep, notes), got %d", len(tr.created))
	}
	if tr.active != 0 {
		t.Errorf("a spy spinner was left active, want 0: active=%d", tr.active)
	}
}
