package engine_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// The six-area geometry scales with the default prompt so its cut holds as the
// prompt's wording changes. sixAreaSizes sizes the files of a diff that chunks into
// six parts under sixAreaCap, one area to a part, the second sized so that half its
// failed call re-cuts it into three parts.
var (
	// sixAreaFixed bounds a chunk call's bytes besides its diff.
	sixAreaFixed  = len(notes.DefaultPrompt) + 2_300
	sixAreaSecond = 3*sixAreaFixed + 1_000
	sixAreaOther  = sixAreaSecond * 3 / 4
	sixAreaSizes  = []int{sixAreaOther, sixAreaSecond, sixAreaOther, sixAreaOther, sixAreaOther, sixAreaOther}
	sixAreaCap    = sixAreaFixed + sixAreaSecond + 500
)

func areaPath(n int) string { return fmt.Sprintf("area%d/f.go", n) }

// sixAreaGit is the diff, name-status and numstat of one file per area, sized by
// sixAreaSizes.
func sixAreaGit() (diff, nameStatus, numstat string) {
	var d, ns, nu strings.Builder
	for i, size := range sixAreaSizes {
		path := areaPath(i + 1)
		d.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		d.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", size/64))
		ns.WriteString("M\t" + path + "\n")
		fmt.Fprintf(&nu, "%d\t0\t%s\n", size/64, path)
	}
	return d.String(), ns.String(), nu.String()
}

// fifthPartFailure is what the call first packed as part 5 printed.
const fifthPartFailure = "API Error: usage limit reached for this account"

var chunkFirstPath = regexp.MustCompile(`(?m)^diff --git a/(\S+) b/`)

// waveEndTransport writes a six-part chunked run whose second part overflows the
// model and is classified SIZE, then whose fifth part, sent in the place in flight
// the second frees once it is re-cut, fails with fifthPartFailure and is classified
// OTHER. The other
// first-wave parts are held until the fifth has failed, so it fails numbered after
// the re-cut. Every other call gets notes.
type waveEndTransport struct {
	fiveFailed chan struct{}

	mu        sync.Mutex
	overflown bool
	fifthPart string
}

func newWaveEndTransport() *waveEndTransport {
	return &waveEndTransport{fiveFailed: make(chan struct{})}
}

func (wt *waveEndTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	if isClassifyPrompt(prompt) {
		if strings.Contains(prompt, modelRejectsSize) {
			return "SIZE", nil
		}
		return "OTHER", nil
	}
	if !isChunkCallPrompt(prompt) {
		return "merged notes", nil
	}
	m := chunkFirstPath.FindStringSubmatch(prompt)
	switch m[1] {
	case areaPath(2):
		wt.mu.Lock()
		first := !wt.overflown
		wt.overflown = true
		wt.mu.Unlock()
		if first {
			return "", &ai.GenerationError{Stdout: modelRejectsSize + "\n", ExitCode: 1}
		}
	case areaPath(5):
		defer close(wt.fiveFailed)
		wt.mu.Lock()
		wt.fifthPart = firstPartFraming.FindString(prompt)
		wt.mu.Unlock()
		return "", &ai.GenerationError{Stdout: fifthPartFailure + "\n", ExitCode: 1}
	case areaPath(1), areaPath(3), areaPath(4):
		<-wt.fiveFailed
	}
	return "- notes for " + m[1], nil
}

// assertFifthRenumbered fails the test unless the fifth part was sent as part 7 of 8.
func (wt *waveEndTransport) assertFifthRenumbered(t *testing.T) {
	t.Helper()
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.fifthPart != "What follows is part 7 of 8" {
		t.Fatalf("fixture: the part first packed as 5 was sent as %q, want part 7 of 8", wt.fifthPart)
	}
}

func seedSixAreaRelease(t *testing.T, root string, f *runner.FakeRunner, onNotesFailure string) {
	t.Helper()
	writeConfig(t, root, fmt.Sprintf("max_prompt_bytes = %d\n\n[release]\non_notes_failure = %q\n", sixAreaCap, onNotesFailure))
	seedPriorTagReadGates(f, root, "main")
	diff, nameStatus, numstat := sixAreaGit()
	f.SeedSequence("git", ScriptedOut(diff), ScriptedOut(nameStatus), ScriptedOut(numstat))
}

func TestRelease_WaveEndsAfterAHalving_TheLineNamesTheFailureThatEndedIt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedSixAreaRelease(t, root, f, "abort")
	rec := &presentertest.RecordingPresenter{}
	tr := newWaveEndTransport()
	deps := newDeps(rec, f)
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	tr.assertFifthRenumbered(t)
	assertNoMutation(t, f)
	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != "AI call failed (part 7 of 8)" {
		t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, "AI call failed (part 7 of 8)")
	}
	if sf.Output != fifthPartFailure {
		t.Errorf("StageFailure.Output = %q, want the failure that ended the wave %q", sf.Output, fifthPartFailure)
	}
}

func TestRelease_WaveEnds_Fallback_ShipsTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedSixAreaRelease(t, root, f, "fallback")
	f.SeedSequence("git", ScriptedOut(subjects)) // log --format=%s
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := newWaveEndTransport()
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	tr.assertFifthRenumbered(t)
	if body := tagAnnotationBody(t, f, nextTag); body != subjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", body, subjects)
	}
}

func produceSixAreaWave(ctx context.Context, tr notes.Transport, root string, res version.Resolution) (string, error) {
	cfg := freshCfg()
	cfg.MaxPromptBytes = sixAreaCap
	notesGit := seedFreshGit(sixAreaGit())
	return engine.RegenerateFreshBody(ctx, notesGit, tr, root, cfg, res, engine.NotesSinks{})
}

func TestRegenerateRun_Unattended_WaveEnds_StopsNamingTheFailureThatEndedIt(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}
	tr := newWaveEndTransport()
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
		return produceSixAreaWave(ctx, tr, t.TempDir(), version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, t.TempDir(), req); err == nil {
		t.Fatal("RegenerateRun returned nil error, want it to stop")
	}

	tr.assertFifthRenumbered(t)
	sf := onlyStageFailureEvent(t, rec)
	if sf.Message != "AI call failed (part 7 of 8)" || sf.Output != fifthPartFailure {
		t.Errorf("StageFailure = %q with output %q, want %q with %q", sf.Message, sf.Output, "AI call failed (part 7 of 8)", fifthPartFailure)
	}
	if len(pub.dispatched) != 0 {
		t.Errorf("provider writes = %d, want none for a stopped regenerate", len(pub.dispatched))
	}
}

func TestRegenerateAll_WaveEnds_SkipsThatVersionAndCarriesOn(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	tr := newWaveEndTransport()
	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
		if res.Tag == batchV2Tag {
			return produceSixAreaWave(ctx, tr, t.TempDir(), res)
		}
		return "## fresh " + res.Tag + "\n", nil
	}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	tr.assertFifthRenumbered(t)
	if got := dispatchedTags(pub); len(got) != 2 || got[0] != batchV1Tag || got[1] != batchV3Tag {
		t.Errorf("provider writes = %v, want %s and %s with %s skipped", got, batchV1Tag, batchV3Tag, batchV2Tag)
	}
}
