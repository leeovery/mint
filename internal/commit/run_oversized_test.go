package commit_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/config"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// aiGenerationFailed builds the wrapped transport-failure sentinel a generate step
// surfaces when the AI returns nothing usable after its one retry — used to drive the
// AI-failure (3-3) half of the distinct-from-generation-failure assertion.
func aiGenerationFailed() error {
	return fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)
}

// oversizedNote is the exact, verbatim note the oversized fallback must emit (em
// dash U+2014). It is the spec string (Commit Message Format & Prompt → The $EDITOR
// fallback, case 3) — asserted byte-for-byte, never substring-matched.
const oversizedNote = "diff too large to summarise — opening editor"

// writeMaxPromptBytes writes a .mint.toml into dir setting max_prompt_bytes plus any
// extra TOML lines, so the real config.Load picks up a cap sized to the test's own
// composed prompt. Every other key stays at its default.
func writeMaxPromptBytes(t *testing.T, dir string, max int, extra ...string) {
	t.Helper()
	body := "max_prompt_bytes = " + strconv.Itoa(max) + "\n" + strings.Join(extra, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

// diffOfLines builds a synthetic diff body of exactly n newline-terminated lines.
func diffOfLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("+line\n")
	}
	return b.String()
}

// seedOversizedFallback scripts the StagedOnly git thread for an oversized-diff run
// that falls back to the editor: the empty-index preflight read (non-empty), the L1
// staged diff read (returning the over-limit diff), the `git var GIT_EDITOR`
// resolution, then the `git commit -F -` on a non-empty save. No L2/transport call
// happens (the size guard fires before it), and no `git add` runs under StagedOnly.
func seedOversizedFallback(diff, editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},          // git diff --cached -- . (L1, over-limit)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
	)
	return f
}

// seedPassesToL2 scripts the StagedOnly git thread for a run whose diff is within the
// ceiling, so it proceeds to L2 then the gate then the commit: preflight read
// (non-empty), the L1 staged diff read (returning the at/under-limit diff), then the
// `git commit -F -` on gate-accept. No editor resolution and no launch occur.
func seedPassesToL2(diff string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff --cached -- . (L1, within ceiling)
		runner.ScriptedCall{}, // git commit -F -
	)
	return f
}

// oversizedDeps assembles production-shaped Deps for an oversized run over an
// editorRunner driving the REAL Generator: the recording presenter, the editorRunner
// as the read/interactive seam, the lock-resilient git Mutator (git_safe) as the
// staging+commit sink wrapping the SAME editorRunner, and a recordingTransport so the
// test can assert the transport is NEVER called. NoAI is FALSE — this is the AI path
// short-circuited by the size guard, not --no-ai.
func oversizedDeps(rec *presentertest.RecordingPresenter, er *editorRunner, tr commit.Transport, mode commit.StagingMode, root string) commit.Deps {
	// These tests exercise the TTY editor-fallback path (a TTY stdin, no -y), so the
	// no-message-source fail-loud guard (task 3-5) does NOT fire and the oversized diff
	// reaches the editor (StdinInteractive defaults true). The guard's own
	// preconditions live in run_failloud_test.go.
	return editorDeps(rec, er, editorDepsOptions{Transport: tr, Root: root, Staging: mode})
}

// warnEvents returns every recorded Warn payload, in order.
func warnEvents(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	return warns
}

// TestRun_Oversized_SkipsL2_RoutesToEditor proves a composed prompt over
// max_prompt_bytes is caught BEFORE any L2 call — the transport is never invoked — and
// the run routes to the editor fallback (a RunInteractive launch is recorded) rather
// than aborting. Driven through the REAL Generator over a FakeRunner so the size guard
// measures the prompt composed over the real L1 diff.
func TestRun_Oversized_SkipsL2_RoutesToEditor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

	const saved = "feat: human message for an oversized diff\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedOversizedFallback(diffOfLines(3), "myedit"), saved: saved}
	tr := scriptedTransport("must never be returned (L2 was skipped)")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor, not an abort", err)
	}

	if tr.calls() != 0 {
		t.Errorf("transport.Generate called %d times; an over-limit diff must SKIP L2 entirely (no AI call)", tr.calls())
	}
	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the oversized diff routes to the editor)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_Oversized_EmitsNote proves the oversized fallback emits the spec note
// verbatim via the Presenter — a KindWarn whose Message is exactly
// "diff too large to summarise — opening editor" (em dash U+2014).
func TestRun_Oversized_EmitsNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedOversizedFallback(diffOfLines(3), "myedit"), saved: "feat: msg\n"}
	tr := scriptedTransport("unused")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	warns := warnEvents(rec)
	if len(warns) != 1 {
		t.Fatalf("Warn count = %d (%v), want exactly 1 oversized note", len(warns), rec.Kinds())
	}
	if warns[0].Message != oversizedNote {
		t.Errorf("oversized note Message = %q, want exactly %q (em dash U+2014)", warns[0].Message, oversizedNote)
	}
}

// TestRun_Oversized_DiffExcludeAppliedBeforeMeasure proves diff_exclude is applied
// FIRST, so excluded noise alone cannot push an otherwise-fine prompt over the cap: a
// run whose prompt over the post-exclusion L1 diff fits passes to L2 (the AI is called,
// no fallback, no note), even though the configured exclude glob is present. The real
// Generator issues the :(exclude) pathspec; the FakeRunner returns git's post-exclusion
// (small) diff, over which the prompt is composed and measured.
func TestRun_Oversized_DiffExcludeAppliedBeforeMeasure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(1)), `diff_exclude = ["*.min.js"]`)

	const message = "feat: post-exclusion prompt fits the cap"
	rec := &presentertest.RecordingPresenter{}
	// The L1 read returns git's POST-exclusion diff — the noisy excluded file is
	// already gone, so the measure never sees it.
	er := &editorRunner{fake: seedPassesToL2(diffOfLines(1)), saved: "should never reach the editor\n"}
	tr := scriptedTransport(message)

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The exclude pathspec was issued at L1 (git performs the exclusion).
	l1 := editorGitInvocations(er)[1]
	if !containsArg(l1.Args, ":(exclude)*.min.js") {
		t.Errorf("L1 argv = %v, want it to carry the :(exclude)*.min.js pathspec (exclusion at L1)", l1.Args)
	}
	if tr.calls() != 1 {
		t.Errorf("transport.Generate called %d times; an excluded-noise prompt within the cap must reach L2 exactly once", tr.calls())
	}
	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s); excluded noise alone must NOT trigger the oversized fallback", len(er.launches))
	}
	if warns := warnEvents(rec); len(warns) != 0 {
		t.Errorf("Warn count = %d; excluded noise within the cap must emit NO oversized note", len(warns))
	}
}

// TestRun_Oversized_AtLimitPassesToL2 proves the inclusive boundary: a composed prompt
// EXACTLY at max_prompt_bytes passes to L2 normally — the AI is called, the gate
// accepts, the commit carries the AI body, and NO editor/fallback/note occurs.
func TestRun_Oversized_AtLimitPassesToL2(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(3)))

	const message = "feat: at-limit diff summarised by the AI"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedPassesToL2(diffOfLines(3)), saved: "should never reach the editor\n"}
	tr := scriptedTransport(message)

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls() != 1 {
		t.Errorf("transport.Generate called %d times; a prompt exactly at max_prompt_bytes must reach L2 (inclusive boundary)", tr.calls())
	}
	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s); an at-cap prompt must NOT trigger the fallback", len(er.launches))
	}
	if warns := warnEvents(rec); len(warns) != 0 {
		t.Errorf("Warn count = %d; an at-cap prompt must emit NO oversized note", len(warns))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != message {
		t.Fatalf("commit invocations = %v, want exactly one carrying the AI body %q", got, message)
	}
}

// TestRun_Oversized_OverLimitTriggersFallback proves the over-limit half of the
// boundary: a composed prompt ONE byte OVER max_prompt_bytes triggers the fallback (the AI is
// skipped and the editor is launched with the note emitted), the strict complement of
// the at-limit case above.
func TestRun_Oversized_OverLimitTriggersFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(4))-1)

	const saved = "feat: human message; prompt was one byte over the cap\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedOversizedFallback(diffOfLines(4), "myedit"), saved: saved}
	tr := scriptedTransport("must never be returned (L2 was skipped)")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls() != 0 {
		t.Errorf("transport.Generate called %d times; one-over-limit must SKIP L2", tr.calls())
	}
	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (one-over-limit triggers the fallback)", len(er.launches))
	}
	if warns := warnEvents(rec); len(warns) != 1 || warns[0].Message != oversizedNote {
		t.Errorf("Warn events = %v, want exactly one carrying %q", warns, oversizedNote)
	}
}

// TestRun_Oversized_NonEmptySaveUnderAll_AddsTrackedThenCommits proves the oversized
// fallback reuses save-as-accept UNCHANGED from --no-ai: a non-empty save under -a
// applies `git add -u` then commits the saved body, in that order.
func TestRun_Oversized_NonEmptySaveUnderAll_AddsTrackedThenCommits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

	const saved = "feat: staged tracked then committed after oversized fallback\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},          // git diff HEAD --name-only (preflight, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: diffOfLines(3)}}, // git diff HEAD -- . (L1, over-limit)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},     // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -u (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	tr := scriptedTransport("must never be returned (L2 was skipped)")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.All, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-u" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -u`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
	assertAddBeforeCommit(t, er)
}

// TestRun_Oversized_EmptySave_TrueNoOp proves an empty/aborted editor on the oversized
// fallback path is a true no-op: no `git add`, no `git commit`, a non-zero abort —
// save-as-accept reused unchanged from --no-ai.
func TestRun_Oversized_EmptySave_TrueNoOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		saved     string
		launchErr error
	}{
		{name: "WhitespaceOnlySave", saved: "  \n\t\n"},
		{name: "AbortedEditor", launchErr: errExitOne},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

			rec := &presentertest.RecordingPresenter{}
			f := runner.NewFakeRunner()
			// Only the preflight read, the L1 diff, and the editor resolution are scripted;
			// staging/commit must never be reached on an empty/aborted save.
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: diffOfLines(3)}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},
			)
			er := &editorRunner{fake: f, saved: tt.saved, launchErr: tt.launchErr}
			tr := scriptedTransport("must never be returned (L2 was skipped)")

			err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root))
			if err == nil {
				t.Fatal("Run returned nil for an empty/aborted editor; want a non-zero no-op abort")
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("empty/aborted editor ran `git add` %v; an empty save is a true no-op", adds)
			}
			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("empty/aborted editor created %d commit(s); an empty save is a true no-op", len(commits))
			}
			// The note still fires (the fallback was entered) even though the save aborted.
			if warns := warnEvents(rec); len(warns) != 1 || warns[0].Message != oversizedNote {
				t.Errorf("Warn events = %v, want exactly one carrying %q (the fallback was entered)", warns, oversizedNote)
			}
		})
	}
}

// TestRun_Oversized_EditorBufferIsEmptyTemplate proves the oversized fallback opens the
// editor with an EMPTY buffer — same save-as-accept template as --no-ai, no synthetic
// stub and no partial message.
func TestRun_Oversized_EditorBufferIsEmptyTemplate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedOversizedFallback(diffOfLines(3), "myedit"), saved: "feat: human message\n"}
	var opened string
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}
	tr := scriptedTransport("must never be returned (L2 was skipped)")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if opened != "" {
		t.Errorf("editor opened with buffer %q; the oversized buffer must be empty (same template as --no-ai)", opened)
	}
}

// TestRun_Oversized_DistinctFromGenerationFailure proves the oversized SKIP is distinct
// from a generation FAILURE: the oversized path carries the note AND skips L2 (the
// transport is never called), whereas the AI-failure path (3-3) carries NO oversized
// note. Asserts both halves of the distinction in one place.
func TestRun_Oversized_DistinctFromGenerationFailure(t *testing.T) {
	t.Parallel()

	// Oversized half: note emitted, transport never called.
	t.Run("OversizedSkipCarriesNoteAndSkipsL2", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeMaxPromptBytes(t, root, composedSize(diffOfLines(3))-1)

		rec := &presentertest.RecordingPresenter{}
		er := &editorRunner{fake: seedOversizedFallback(diffOfLines(3), "myedit"), saved: "feat: msg\n"}
		tr := scriptedTransport("must never be returned (L2 was skipped)")

		if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
		if tr.calls() != 0 {
			t.Errorf("transport.Generate called %d times; the oversized SKIP must never reach L2", tr.calls())
		}
		if warns := warnEvents(rec); len(warns) != 1 || warns[0].Message != oversizedNote {
			t.Errorf("Warn events = %v, want exactly one oversized note %q", warns, oversizedNote)
		}
	})

	// AI-failure half: a within-cap prompt whose transport fails routes to the editor
	// (3-3) but emits NO oversized note — the failure path is noteless.
	t.Run("AIFailurePathCarriesNoOversizedNote", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeMaxPromptBytes(t, root, composedSize(diffOfLines(3)))

		const saved = "feat: human message after AI failed\n"
		rec := &presentertest.RecordingPresenter{}
		er := &editorRunner{fake: seedOversizedFallback(diffOfLines(3), "myedit"), saved: saved}
		tr := &failTransport{err: aiGenerationFailed()}

		if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
		if len(er.launches) != 1 {
			t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the AI failure routes to the editor)", len(er.launches))
		}
		for _, w := range warnEvents(rec) {
			if w.Message == oversizedNote {
				t.Errorf("AI-failure path emitted the oversized note %q; the failure path must be noteless", oversizedNote)
			}
		}
	})
}

// TestRun_Oversized_ManyLinesUnderDefaultCap_ShowsAIMessage proves the line count no
// longer gates commit: with no max_prompt_bytes set, a diff of more than 50,000 lines
// whose composed prompt fits the shipped cap gets an AI call, and the generated message
// is shown at the review gate.
func TestRun_Oversized_ManyLinesUnderDefaultCap_ShowsAIMessage(t *testing.T) {
	t.Parallel()

	diff := diffOfLines(50001)
	if size := composedSize(diff); size > config.DefaultMaxPromptBytes {
		t.Fatalf("fixture prompt is %d bytes, want it within the %d-byte default cap", size, config.DefaultMaxPromptBytes)
	}

	const message = "feat: summarise a long diff"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedPassesToL2(diff), saved: "should never reach the editor\n"}
	tr := scriptedTransport(message)

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls() != 1 {
		t.Errorf("transport.Generate called %d times, want exactly 1", tr.calls())
	}
	var shown []presenter.Message
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowMessage {
			shown = append(shown, ev.ShowMessage)
		}
	}
	if len(shown) != 1 || shown[0].Body != message {
		t.Errorf("ShowMessage events = %+v, want exactly one carrying the AI body %q", shown, message)
	}
	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s); a prompt within the cap must NOT open the editor", len(er.launches))
	}
}

// TestRun_Oversized_ContextPushesPromptOverCap_OpensEmptyEditor proves the cap measures
// the composed prompt, not the diff alone: a diff that fits on its own, pushed over the
// cap by an outsized [commit].context, makes no AI call and opens the editor empty with
// the oversized note.
func TestRun_Oversized_ContextPushesPromptOverCap_OpensEmptyEditor(t *testing.T) {
	t.Parallel()

	diff := diffOfLines(3)
	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diff)+100, "[commit]", `context = "`+strings.Repeat("c", 200)+`"`)

	const saved = "feat: human message; context pushed the prompt over the cap\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedOversizedFallback(diff, "myedit"), saved: saved}
	opened := "unset"
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}
	tr := scriptedTransport("must never be returned (L2 was skipped)")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls() != 0 {
		t.Errorf("transport.Generate called %d times; an over-cap composed prompt must never be sent", tr.calls())
	}
	if warns := warnEvents(rec); len(warns) != 1 || warns[0] != (presenter.Warning{Label: "diff", Message: oversizedNote}) {
		t.Errorf("Warn events = %+v, want exactly the oversized note %q", warns, oversizedNote)
	}
	if opened != "" {
		t.Errorf("editor opened with buffer %q, want an empty buffer", opened)
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_Oversized_RegenNudgeOverCap_OpensEmptyEditor proves `r` at the review gate is
// held to the cap: when the nudge pushes the regeneration's composed prompt over it, no
// regeneration call is made, the oversized note is shown, and the editor opens EMPTY —
// not on the prior message — with the save as the accept.
func TestRun_Oversized_RegenNudgeOverCap_OpensEmptyEditor(t *testing.T) {
	t.Parallel()

	diff := diffOfLines(3)
	root := t.TempDir()
	writeMaxPromptBytes(t, root, composedSize(diff)+100)

	const saved = "feat: human message after an over-cap regeneration\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{strings.Repeat("n", 200)},
	}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},       // git diff --cached -- . (initial L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},       // git diff --cached -- . (regeneration L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	opened := "unset"
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}
	tr := scriptedTransport("feat: the message shown before r")

	if err := commit.Run(context.Background(), oversizedDeps(rec, er, tr, commit.StagedOnly, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v; an over-cap r must fall back to the editor, not abort", err)
	}

	if tr.calls() != 1 {
		t.Errorf("transport.Generate called %d times, want exactly 1 (the first generation; no regeneration call)", tr.calls())
	}
	if warns := warnEvents(rec); len(warns) != 1 || warns[0] != (presenter.Warning{Label: "diff", Message: oversizedNote}) {
		t.Errorf("Warn events = %+v, want exactly the oversized note %q", warns, oversizedNote)
	}
	if containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("over-cap r emitted a StageFailed; it must route to the editor fallback, not abort: %v", rec.Kinds())
	}
	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1", len(er.launches))
	}
	if opened != "" {
		t.Errorf("editor opened with buffer %q, want an empty buffer (not the prior message)", opened)
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}
