package notes_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
)

// recordGrace is how long a test lets the wave record a call's outcome after the
// transport has answered it.
const recordGrace = 50 * time.Millisecond

// endingTransport scripts a chunked run by the first file each chunk call carries:
// call decides its outcome, holding it as long as the test needs, nil for notes. A
// classify call is answered by classify over its prompt; the merge is answered
// "merged". It logs "start PATH" and "end PATH" for each chunk call, "classify PATH"
// for each classify call, naming the file whose failure it reads, and "merge".
type endingTransport struct {
	call     func(path string) error
	classify func(prompt string) (string, error)

	mu       sync.Mutex
	events   []string
	failures map[string]string
	// framed is the part each file's latest chunk call was framed as.
	framed map[string]string
}

func (et *endingTransport) Generate(_ context.Context, prompt string) (string, error) {
	switch {
	case isClassifyPrompt(prompt):
		et.record("classify " + et.failedPath(prompt))
		return et.classify(prompt)
	case isChunkPrompt(prompt):
		path := firstPath(sliceOf(prompt))
		et.mu.Lock()
		if et.framed == nil {
			et.framed = map[string]string{}
		}
		et.framed[path] = partFraming.FindString(prompt)
		et.mu.Unlock()
		et.record("start " + path)
		err := et.call(path)
		et.record("end " + path)
		if err != nil {
			et.noteFailure(path, err)
			return "", err
		}
		return chunkNotesFor(path), nil
	default:
		et.record("merge")
		return "merged", nil
	}
}

func (et *endingTransport) record(event string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.events = append(et.events, event)
}

func (et *endingTransport) noteFailure(path string, err error) {
	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() == "" {
		return
	}
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.failures == nil {
		et.failures = map[string]string{}
	}
	et.failures[genErr.Output()] = path
}

func (et *endingTransport) failedPath(prompt string) string {
	et.mu.Lock()
	defer et.mu.Unlock()
	for output, path := range et.failures {
		if strings.Contains(prompt, output) {
			return path
		}
	}
	return "?"
}

func (et *endingTransport) snapshot() []string {
	et.mu.Lock()
	defer et.mu.Unlock()
	return slices.Clone(et.events)
}

// waitFor waits until event has been logged.
func (et *endingTransport) waitFor(t *testing.T, event string) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for !slices.Contains(et.snapshot(), event) {
		if time.Now().After(deadline) {
			t.Fatalf("%q never logged; events %v", event, et.snapshot())
		}
		time.Sleep(time.Millisecond)
	}
}

func (et *endingTransport) framedAs(path string) string {
	et.mu.Lock()
	defer et.mu.Unlock()
	return et.framed[path]
}

func (et *endingTransport) count(event string) int {
	n := 0
	for _, e := range et.snapshot() {
		if e == event {
			n++
		}
	}
	return n
}

func area(n int) string { return fmt.Sprintf("area%d/file.go", n) }

func failedWith(output string) error {
	return &ai.GenerationError{Stdout: output, ExitCode: 1}
}

// answerBy answers SIZE for a classify prompt carrying any of sizeOutputs, OTHER for
// any other.
func answerBy(sizeOutputs ...string) func(string) (string, error) {
	return func(prompt string) (string, error) {
		for _, output := range sizeOutputs {
			if strings.Contains(prompt, output) {
				return "SIZE", nil
			}
		}
		return "OTHER", nil
	}
}

// heldUntil holds a call until release is closed, then fails it with err, nil for
// notes.
func heldUntil(release <-chan struct{}, err error) error {
	if waitErr := waitClosed(release); waitErr != nil {
		return waitErr
	}
	return err
}

func TestGenerator_Wave_PartClassifiedOther_StartsNoNewPartAndClassifiesNoMore(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	et := &endingTransport{classify: answerBy()}
	et.call = func(path string) error {
		switch path {
		case area(2):
			return failedWith("area2: usage limit reached")
		case area(3):
			return heldUntil(release, failedWith("area3: usage limit reached"))
		default:
			return heldUntil(release, nil)
		}
	}

	done := s.startGenerate(t, et)
	et.waitFor(t, "classify "+area(2))
	time.Sleep(recordGrace)
	close(release)
	err := awaitFailure(t, done)

	if got := causeOf(err); got != "AI call failed (part 2 of 6)" {
		t.Errorf("CauseText = %q, want part 2's failure", got)
	}
	events := et.snapshot()
	for _, n := range []int{5, 6} {
		if slices.Contains(events, "start "+area(n)) {
			t.Errorf("part %d started after part 2 ended the wave; events %v", n, events)
		}
	}
	for _, n := range []int{1, 3, 4} {
		if !slices.Contains(events, "end "+area(n)) {
			t.Errorf("part %d, in flight when the wave ended, did not finish; events %v", n, events)
		}
	}
	if got := countPrefix(events, "classify "); got != 1 {
		t.Errorf("classify calls = %d (events %v), want only part 2's", got, events)
	}
	if slices.Contains(events, "merge") {
		t.Errorf("a merge call was made after the wave ended")
	}
}

func TestGenerator_Wave_PartEndsForGood_EndsTheWaveTheSameWay(t *testing.T) {
	t.Parallel()

	const overflow = "area2: Prompt is too long"
	cases := []struct {
		name     string
		scenario func(t *testing.T) chunkScenario
		failure  error
		classify func(string) (string, error)
		// wantClassify is how many classify calls part 2's failure gets.
		wantClassify int
		wantCause    string
	}{
		{
			name:         "its classify call fails",
			failure:      failedWith("area2: Not logged in"),
			classify:     func(string) (string, error) { return "", failedWith("classify call broke") },
			wantClassify: 1,
			wantCause:    "AI call failed (part 2 of 6)",
		},
		{
			name:      "it fails with no output",
			failure:   &ai.GenerationError{Stderr: " \n"},
			wantCause: "AI returned empty notes (part 2 of 6)",
		},
		{
			name:      "it times out",
			failure:   fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout),
			wantCause: "AI timed out (part 2 of 6)",
		},
		{
			name:      "its binary is missing",
			failure:   fmt.Errorf("%w: claude not found", ai.ErrCommandMissing),
			wantCause: "AI tool not installed (part 2 of 6)",
		},
		{
			name: "its halving round has no room",
			scenario: func(t *testing.T) chunkScenario {
				t.Helper()
				s := outsizedFixedPart(t, tenKBFiles(6), 0)
				s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000
				return s
			},
			failure:      failedWith(overflow),
			classify:     answerBy(overflow),
			wantClassify: 1,
			wantCause:    tooLargePhrase + " (part 2 of 6)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(6))
			if tc.scenario != nil {
				s = tc.scenario(t)
			}
			classify := tc.classify
			if classify == nil {
				classify = answerBy()
			}
			release, twoEnded := make(chan struct{}), make(chan struct{})
			et := &endingTransport{classify: classify}
			et.call = func(path string) error {
				if path == area(2) {
					defer close(twoEnded)
					return tc.failure
				}
				return heldUntil(release, failedWith(path+": usage limit reached"))
			}

			done := s.startGenerate(t, et)
			if err := waitClosed(twoEnded); err != nil {
				t.Fatalf("part 2 did not end: %v", err)
			}
			if tc.wantClassify > 0 {
				et.waitFor(t, "classify "+area(2))
			}
			time.Sleep(recordGrace)
			close(release)
			err := awaitFailure(t, done)

			if got := causeOf(err); got != tc.wantCause {
				t.Errorf("CauseText = %q, want %q", got, tc.wantCause)
			}
			events := et.snapshot()
			if got := countPrefix(events, "start "); got != 4 {
				t.Errorf("chunk calls started = %d (events %v), want only the 4 in flight when part 2 ended", got, events)
			}
			if got := countPrefix(events, "classify "); got != tc.wantClassify {
				t.Errorf("classify calls = %d (events %v), want %d, none for the calls in flight", got, events, tc.wantClassify)
			}
			if !errors.Is(err, tc.failure) {
				t.Errorf("error = %v, want part 2's failure %v", err, tc.failure)
			}
		})
	}
}

func TestGenerator_Wave_SizeWithNoHalvingLeft_ClassifiesNoCallStillInFlight(t *testing.T) {
	t.Parallel()

	files := make([]chunkFile, 6)
	for i := range files {
		files[i] = chunkFile{path: fmt.Sprintf("area%d/file.go", i+1), size: 200_000}
	}
	s := chunkScenario{files: files, root: t.TempDir()}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 205_000
	release := make(chan struct{})
	ht := &halvingTransport{overflows: opensWith(pieceHeader("area3/file.go") + "@@"), mergeBody: "merged"}
	et := &endingTransport{}
	et.call = func(path string) error {
		if path == area(1) {
			return heldUntil(release, failedWith("area1: usage limit reached"))
		}
		return nil
	}
	// Part 1 is held through part 3's halvings; every other call goes to ht.
	tr := transportFunc(func(ctx context.Context, prompt string) (string, error) {
		if isChunkPrompt(prompt) && firstPath(sliceOf(prompt)) == area(1) {
			return et.Generate(ctx, prompt)
		}
		return ht.Generate(ctx, prompt)
	})

	done := s.startGenerate(t, tr)
	deadline := time.Now().Add(waitLimit)
	for classified := 0; classified < 4; ht.record(func() { classified = ht.classify }) {
		if time.Now().After(deadline) {
			t.Fatalf("classify calls = %d, never reached the fourth SIZE", classified)
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(recordGrace)
	close(release)
	err := awaitFailure(t, done)

	if want := tooLargePhrase + " (part 3 of 12)"; causeOf(err) != want {
		t.Errorf("CauseText = %q, want %q", causeOf(err), want)
	}
	ht.record(func() {
		if ht.classify != 4 {
			t.Errorf("classify calls = %d, want part 3's four, none for part 1 failing after the wave ended", ht.classify)
		}
		if len(ht.merges) != 0 {
			t.Errorf("merge calls = %d, want none", len(ht.merges))
		}
	})
	if !slices.Contains(et.snapshot(), "end "+area(1)) {
		t.Errorf("part 1, in flight when the wave ended, did not finish")
	}
}

type transportFunc func(ctx context.Context, prompt string) (string, error)

func (f transportFunc) Generate(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

func TestGenerator_Wave_PartClassifiedSizeWithHalvingLeft_CarriesOnAndSendsItsHalvedParts(t *testing.T) {
	t.Parallel()

	const overflow = "area2: Prompt is too long"
	s := capOneFilePerChunk(t, tenKBFiles(6))
	var once sync.Once
	et := &endingTransport{classify: answerBy(overflow)}
	et.call = func(path string) error {
		var err error
		if path == area(2) {
			once.Do(func() { err = failedWith(overflow) })
		}
		return err
	}

	body, err := s.generateWith(t, et)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if body != "merged" {
		t.Errorf("body = %q, want the merge output", body)
	}
	events := et.snapshot()
	if got := et.count("start " + area(2)); got < 3 {
		t.Errorf("calls carrying part 2's file = %d (events %v), want it and its halved parts", got, events)
	}
	for _, n := range []int{5, 6} {
		if !slices.Contains(events, "end "+area(n)) {
			t.Errorf("part %d was not sent after part 2 halved; events %v", n, events)
		}
	}
}

func TestGenerator_Wave_CallInFlightFailsAfterTheWaveEnded_IsNotClassified(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	held, releaseFive := make(chan struct{}), make(chan struct{})
	et := &endingTransport{classify: answerBy()}
	et.call = func(path string) error {
		switch path {
		case area(1):
			return nil
		case area(2):
			et.waitFor(t, "start "+area(5))
			return failedWith("area2: usage limit reached")
		case area(5):
			return heldUntil(releaseFive, failedWith("area5: usage limit reached"))
		default:
			return heldUntil(held, nil)
		}
	}

	done := s.startGenerate(t, et)
	et.waitFor(t, "classify "+area(2))
	time.Sleep(recordGrace)
	close(releaseFive)
	et.waitFor(t, "end "+area(5))
	time.Sleep(recordGrace)
	close(held)
	err := awaitFailure(t, done)

	if got := causeOf(err); got != "AI call failed (part 2 of 6)" {
		t.Errorf("CauseText = %q, want part 2's failure", got)
	}
	events := et.snapshot()
	if slices.Contains(events, "classify "+area(5)) {
		t.Errorf("part 5's failure, after the wave ended, was classified; events %v", events)
	}
	if slices.Contains(events, "start "+area(6)) {
		t.Errorf("part 6 started after the wave ended; events %v", events)
	}
}

func TestGenerator_Wave_EndsBeforeAHalvedPartStarts_NoHalvedPartIsSent(t *testing.T) {
	t.Parallel()

	const overflow = "area3: Prompt is too long"
	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	et := &endingTransport{classify: answerBy(overflow)}
	et.call = func(path string) error {
		switch path {
		case area(3):
			return failedWith(overflow)
		case area(5):
			return failedWith("area5: usage limit reached")
		default:
			return heldUntil(release, nil)
		}
	}

	done := s.startGenerate(t, et)
	et.waitFor(t, "classify "+area(5))
	time.Sleep(recordGrace)
	close(release)
	err := awaitFailure(t, done)

	five := et.framedAs(area(5))
	if five == "part 5 of 6" {
		t.Fatalf("fixture: part 5 was framed before part 3 was re-cut")
	}
	if got := causeOf(err); got != "AI call failed ("+five+")" {
		t.Errorf("CauseText = %q, want part 5's failure, framed as %s", got, five)
	}
	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() != "area5: usage limit reached" {
		t.Errorf("error = %v, want part 5's failure", err)
	}
	events := et.snapshot()
	if got := et.count("start " + area(3)); got != 1 {
		t.Errorf("calls carrying part 3's file = %d (events %v), want none of its halved parts sent", got, events)
	}
	if slices.Contains(events, "start "+area(6)) {
		t.Errorf("part 6 started after the wave ended; events %v", events)
	}
}

func TestGenerator_Wave_EndsWhileAClassifyCallRuns_ItsSizeAnswerSendsNoHalvedPart(t *testing.T) {
	t.Parallel()

	const overflow = "area3: Prompt is too long"
	s := capOneFilePerChunk(t, tenKBFiles(6))
	release, fiveClassified := make(chan struct{}), make(chan struct{})
	progress := &recutLog{}
	et := &endingTransport{}
	et.classify = func(prompt string) (string, error) {
		if strings.Contains(prompt, overflow) {
			if err := waitClosed(fiveClassified); err != nil {
				return "", err
			}
			time.Sleep(recordGrace)
			return "SIZE", nil
		}
		defer close(fiveClassified)
		return "OTHER", nil
	}
	et.call = func(path string) error {
		switch path {
		case area(1):
			return nil
		case area(3):
			et.waitFor(t, "start "+area(5))
			return failedWith(overflow)
		case area(5):
			et.waitFor(t, "classify "+area(3))
			return failedWith("area5: usage limit reached")
		default:
			return heldUntil(release, nil)
		}
	}

	done := s.startWithProgress(t, et, progress)
	et.waitFor(t, "classify "+area(3))
	if err := waitClosed(fiveClassified); err != nil {
		t.Fatalf("part 5's failure was never classified: %v", err)
	}
	time.Sleep(3 * recordGrace)
	close(release)
	err := awaitFailure(t, done)

	if got := causeOf(err); got != "AI call failed (part 5 of 6)" {
		t.Errorf("CauseText = %q, want part 5's failure", got)
	}
	events := et.snapshot()
	if got := et.count("start " + area(3)); got != 1 {
		t.Errorf("calls carrying part 3's file = %d (events %v), want no halved part sent for its late SIZE", got, events)
	}
	if got := progress.halvings(); len(got) != 0 {
		t.Errorf("re-cuts reported = %v, want none after the wave ended", got)
	}
	if slices.Contains(events, "start "+area(6)) {
		t.Errorf("part 6 started after the wave ended; events %v", events)
	}
}
