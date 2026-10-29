package presenter_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"mint/internal/presenter"
)

// This file pins COMPOSITION: it drives the spec's full worked-example event
// sequence end-to-end through the plain presenter and asserts the complete composed
// transcript. Every individual event's rendering is unit-tested in isolation
// elsewhere (plain_test.go); this golden pins how those per-event renderings
// ASSEMBLE — the notes block sitting between the plan and the gate, the gate echo
// placement, and the trailing footer landing after the last stage. The Rail's whole
// runs are pinned by its golden frames (rail_frames_test.go).
//
// CRITICAL reconciliation with the spec: the spec's worked-example transcript
// (specification/cli-presentation/specification.md) is an illustrative SNAPSHOT.
// Plain emits NO decorative blank lines — each method writes only its own line(s),
// so the golden is tightly packed — and the sequence drives ONLY StageSucceeded for
// the blocking prep/notes stages (NOT StageStarted), matching the snapshot's shown
// lines, which omit the transient "{name}: running..." start lines. The notes body
// is rendered VERBATIM / flush (writeNotesBody) between the sliceable delimiters.

// The engine-supplied details that produce the spec's wording.
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

// transcriptPlan is the engine-supplied plan steps, verbatim, which the plain
// one-liner renders.
func transcriptPlan() presenter.Plan {
	return presenter.Plan{Steps: []presenter.PlanStep{
		{Verb: "commit", Target: "CHANGELOG.md + bin/acme"},
		{Verb: "tag", Target: "v1.4.0 (annotated)"},
		{Verb: "push", Target: "--atomic → origin"},
		{Verb: "publish", Target: "GitHub release"},
	}}
}

// driveWorkedExample drives the FULL spec worked-example event sequence through a
// presenter, capturing the composed narration. The gate is driven via
// Prompt(NotesReviewGate()) at the notes-review point — under -y it renders the
// auto-accept echo.
func driveWorkedExample(p presenter.Presenter) {
	p.RunStarted(presenter.RunInfo{Action: transcriptAction, Project: transcriptProject, Version: transcriptVersion})
	p.StageSucceeded(presenter.StageSuccess{Name: "version", Detail: transcriptVersionDetail})
	p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: transcriptPreflightDetail})
	p.ShowPlan(transcriptPlan())
	p.StageSucceeded(presenter.StageSuccess{Name: "prep", Detail: transcriptPrepDetail, Elapsed: 2300 * time.Millisecond, Blocking: true})
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: transcriptNotesDetail, Elapsed: 1100 * time.Millisecond, Blocking: true})
	p.ShowNotes(presenter.Notes{Version: transcriptVersion, Body: notesBody})
	_, _ = p.Prompt(presenter.NotesReviewGate())
	p.StageSucceeded(presenter.StageSuccess{Name: "record", Detail: "CHANGELOG.md + bin/acme"})
	p.StageSucceeded(presenter.StageSuccess{Name: "tag/push", Detail: "v1.4.0 pushed (atomic)"})
	p.StageSucceeded(presenter.StageSuccess{Name: "publish", Detail: "github release created"})
	p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: transcriptProject, Version: transcriptVersion, URL: transcriptURL})
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

	driveWorkedExample(p)

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
