package notes_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/notes"
)

// recutLog records a run's progress reports as lines. It holds no lock: the
// generator serialises its reports, so the race detector fails a run that does not.
type recutLog struct {
	reports []string
}

func (rl *recutLog) PartsDone(promptBytes, parts, done int) {
	rl.reports = append(rl.reports, fmt.Sprintf("%d bytes, %d of %d done", promptBytes, done, parts))
}

func (rl *recutLog) Halved(part, parts int) {
	rl.reports = append(rl.reports, fmt.Sprintf("part %d re-cut into %d parts", part, parts))
}

func (rl *recutLog) Merging(parts int) {
	rl.reports = append(rl.reports, fmt.Sprintf("merging %d parts", parts))
}

func (rl *recutLog) Merged(parts int) {
	rl.reports = append(rl.reports, fmt.Sprintf("merged %d parts", parts))
}

func (rl *recutLog) CallSucceeded(int) {}

// halvings returns the re-cut reports in order.
func (rl *recutLog) halvings() []string {
	var out []string
	for _, r := range rl.reports {
		if strings.HasSuffix(r, " parts") && strings.HasPrefix(r, "part ") {
			out = append(out, r)
		}
	}
	return out
}

// recutTransport writes a chunked run in which chosen chunk calls fail. A chunk call
// whose slice is in failures fails, the first time it is sent, with that output; each
// classify call takes the next of answers. A chunk call that succeeds is held until
// release closes, then answered with notes. The merge is answered "merged".
type recutTransport struct {
	failures map[string]string
	answers  []string
	release  chan struct{}

	mu       sync.Mutex
	failed   map[string]bool
	sent     []string
	inFlight int
}

func (rt *recutTransport) Generate(_ context.Context, prompt string) (string, error) {
	rt.mu.Lock()
	switch {
	case isClassifyPrompt(prompt):
		defer rt.mu.Unlock()
		answer := rt.answers[0]
		rt.answers = rt.answers[1:]
		return answer, nil
	case !isChunkPrompt(prompt):
		rt.mu.Unlock()
		return "merged", nil
	}
	slice := sliceOf(prompt)
	rt.sent = append(rt.sent, prompt)
	if output, ok := rt.failures[slice]; ok && !rt.failed[slice] {
		if rt.failed == nil {
			rt.failed = map[string]bool{}
		}
		rt.failed[slice] = true
		rt.mu.Unlock()
		return "", &ai.GenerationError{Stdout: output + "\n", ExitCode: 1}
	}
	rt.inFlight++
	rt.mu.Unlock()

	select {
	case <-rt.release:
	case <-time.After(waitLimit):
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.inFlight--
	return fmt.Sprintf("- notes for %s", firstPath(slice)), nil
}

// waitHeld waits until n successful chunk calls are held in flight together.
func (rt *recutTransport) waitHeld(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		rt.mu.Lock()
		inFlight := rt.inFlight
		rt.mu.Unlock()
		if inFlight == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("chunk calls held in flight = %d, never reached %d", inFlight, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// recutsOf counts the chunk calls sent carrying only diff that slice carries, other
// than slice itself: the calls its re-cut yielded.
func (rt *recutTransport) recutsOf(slice string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	n := 0
	for _, p := range rt.sent {
		if s := sliceOf(p); s != slice && sliceWithin(slice, s) {
			n++
		}
	}
	return n
}

func (s chunkScenario) startWithProgress(t *testing.T, transport notes.Transport, progress notes.Progress) <-chan generation {
	t.Helper()
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, s.root).WithProgress(progress)
	done := make(chan generation, 1)
	go func() {
		body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
		done <- generation{body: body, err: err}
	}()
	return done
}

// assertCountsOutOf fails unless every parts-done report in reports counts out of
// total and the last reaches it.
func assertCountsOutOf(t *testing.T, reports []string, total int) {
	t.Helper()
	var last string
	for _, r := range reports {
		if strings.HasSuffix(r, " done") {
			if !strings.HasSuffix(r, fmt.Sprintf(" of %d done", total)) {
				t.Errorf("report %q does not count out of %d parts", r, total)
			}
			last = r
		}
	}
	if !strings.HasSuffix(last, fmt.Sprintf(", %d of %d done", total, total)) {
		t.Errorf("last parts-done report = %q, want %d of %d done", last, total, total)
	}
}

func TestGenerator_SinglePassClassifiedSize_ReportsTheRecutThenEachFinishedPartThenTheMerge(t *testing.T) {
	t.Parallel()

	s := underCapScenario(t, threeTenKBFiles())
	size := len(s.singlePass(t))
	ot := &overflowTransport{classifyAnswer: "SIZE"}
	log := &recutLog{}
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), ot, s.root).WithProgress(log)

	if _, err := gen.Generate(t.Context(), "v1.0.0", s.cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	parts := len(ot.chunks)
	if parts < 2 {
		t.Fatalf("chunk calls = %d, want the single-pass call re-cut into parts", parts)
	}
	want := []string{fmt.Sprintf("part 0 re-cut into %d parts", parts)}
	for done := 1; done <= parts; done++ {
		want = append(want, fmt.Sprintf("%d bytes, %d of %d done", size, done, parts))
	}
	want = append(want, fmt.Sprintf("merging %d parts", parts), fmt.Sprintf("merged %d parts", parts))
	if !slices.Equal(log.reports, want) {
		t.Errorf("progress reports =\n%s\nwant\n%s", strings.Join(log.reports, "\n"), strings.Join(want, "\n"))
	}
}

func TestGenerator_PartClassifiedSize_ReportsThePartAndCountsOutOfTheGrownTotal(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	rt := &recutTransport{
		failures: map[string]string{areaDiff(4): overflowOutput},
		answers:  []string{"SIZE"},
		release:  make(chan struct{}),
	}
	log := &recutLog{}

	done := s.startWithProgress(t, rt, log)
	// Parts 1-3 and the part first packed as 5 are held once part 4 has re-cut.
	rt.waitHeld(t, 4)
	close(rt.release)
	if _, err := awaitResult(t, done); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	added := rt.recutsOf(areaDiff(4))
	if added < 2 {
		t.Fatalf("calls re-cut from part 4 = %d, want it re-cut into parts", added)
	}
	total := 6 + added - 1
	if got, want := log.halvings(), []string{fmt.Sprintf("part 4 re-cut into %d parts", added)}; !slices.Equal(got, want) {
		t.Fatalf("re-cut reports = %q, want %q", got, want)
	}
	at := slices.Index(log.reports, log.halvings()[0])
	for _, r := range log.reports[:at] {
		if !strings.HasSuffix(r, " of 6 done") {
			t.Errorf("report %q before the re-cut does not count out of 6 parts", r)
		}
	}
	assertCountsOutOf(t, log.reports[at+1:], total)
	if got, want := log.reports[len(log.reports)-2:], []string{fmt.Sprintf("merging %d parts", total), fmt.Sprintf("merged %d parts", total)}; !slices.Equal(got, want) {
		t.Errorf("last reports = %q, want %q", got, want)
	}
}

func TestGenerator_LaterPartClassifiedSize_NamedByItsPlaceAfterAnEarlierRecut(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	rt := &recutTransport{
		failures: map[string]string{areaDiff(4): overflowOutput, areaDiff(5): overflowOutput},
		answers:  []string{"SIZE", "SIZE"},
		release:  make(chan struct{}),
	}
	log := &recutLog{}

	done := s.startWithProgress(t, rt, log)
	// Parts 1-3 and the part first packed as 6 are held once parts 4 and 5 have re-cut.
	rt.waitHeld(t, 4)
	close(rt.release)
	if _, err := awaitResult(t, done); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	fourth, fifth := rt.recutsOf(areaDiff(4)), rt.recutsOf(areaDiff(5))
	if fourth < 2 || fifth < 2 {
		t.Fatalf("calls re-cut from parts 4 and 5 = %d and %d, want each re-cut into parts", fourth, fifth)
	}
	want := []string{
		fmt.Sprintf("part 4 re-cut into %d parts", fourth),
		fmt.Sprintf("part %d re-cut into %d parts", 4+fourth, fifth),
	}
	if got := log.halvings(); !slices.Equal(got, want) {
		t.Fatalf("re-cut reports = %q, want %q", got, want)
	}
	total := 6 + fourth - 1 + fifth - 1
	assertCountsOutOf(t, log.reports[slices.Index(log.reports, want[1])+1:], total)
	if got, want := log.reports[len(log.reports)-2], fmt.Sprintf("merging %d parts", total); got != want {
		t.Errorf("merge report = %q, want %q", got, want)
	}
}

func TestGenerator_LaterPartFailsOtherAfterARecut_CausePhraseNamesItsPlaceInTheGrownTotal(t *testing.T) {
	t.Parallel()

	s := capOneAreaPerChunk(t, areaFiles(6))
	rt := &recutTransport{
		failures: map[string]string{areaDiff(4): overflowOutput, areaDiff(6): "Not logged in · Please run /login"},
		answers:  []string{"SIZE", "OTHER"},
		release:  make(chan struct{}),
	}

	log := &recutLog{}

	done := s.startWithProgress(t, rt, log)
	// Parts 1-3 and the part first packed as 5 are held once part 4 has re-cut.
	rt.waitHeld(t, 4)
	close(rt.release)
	_, err := awaitResult(t, done)
	if err == nil {
		t.Fatal("Generate returned nil error, want the failed part's")
	}

	// The wave stops at part 6's failure, so not every call part 4 was re-cut into is
	// sent; the re-cut's report counts them.
	var added int
	if halvings := log.halvings(); len(halvings) != 1 {
		t.Fatalf("re-cut reports = %q, want part 4's alone", halvings)
	} else if _, scanErr := fmt.Sscanf(halvings[0], "part 4 re-cut into %d parts", &added); scanErr != nil || added < 2 {
		t.Fatalf("re-cut report = %q, want part 4 re-cut into parts", halvings[0])
	}
	total := 6 + added - 1
	got, known := notes.CauseText(err)
	if want := fmt.Sprintf("AI call failed (part %d of %d)", total, total); !known || got != want {
		t.Errorf("CauseText = %q (known %v), want %q", got, known, want)
	}
}
