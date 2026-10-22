package notes_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// waitLimit bounds every wait in the wave tests, so a generator that never reaches
// the awaited state fails the test instead of hanging it.
const waitLimit = 5 * time.Second

// waveTransport is a concurrency-recording fake: it counts the chunk calls in flight,
// logs when each call starts and ends (or ends on a cancelled context), and holds each chunk call in gate until the
// test lets it finish, calling ended once its end is logged. Chunk calls answer with chunkNotesFor their slice's first file;
// the merge call answers mergeBody, and a classify call OTHER.
type waveTransport struct {
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	events      []string
	merges      []string
	mergeBody   string
	gate        func(part int) error
	ended       func(part int)
}

func (wt *waveTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if isClassifyPrompt(prompt) {
		return "OTHER", nil
	}
	if !isChunkPrompt(prompt) {
		wt.record(func() {
			wt.events = append(wt.events, "merge")
			wt.merges = append(wt.merges, prompt)
		})
		return wt.mergeBody, nil
	}
	part := partNumber(prompt)
	wt.record(func() {
		wt.inFlight++
		wt.maxInFlight = max(wt.maxInFlight, wt.inFlight)
		wt.events = append(wt.events, fmt.Sprintf("start %d", part))
	})
	err := wt.gate(part)
	wt.record(func() {
		wt.inFlight--
		if ctx.Err() != nil {
			wt.events = append(wt.events, fmt.Sprintf("cancelled %d", part))
		} else {
			wt.events = append(wt.events, fmt.Sprintf("end %d", part))
		}
	})
	if wt.ended != nil {
		wt.ended(part)
	}
	if err != nil {
		return "", err
	}
	return chunkNotesFor(firstPath(sliceOf(prompt))), nil
}

func (wt *waveTransport) record(change func()) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	change()
}

func (wt *waveTransport) snapshot() (inFlight, maxInFlight int, events, merges []string) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	return wt.inFlight, wt.maxInFlight, append([]string(nil), wt.events...), append([]string(nil), wt.merges...)
}

// waitInFlight waits until n chunk calls are in flight together.
func (wt *waveTransport) waitInFlight(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		inFlight, _, _, _ := wt.snapshot()
		if inFlight == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("chunk calls in flight = %d, never reached %d together", inFlight, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func partNumber(prompt string) int {
	m := partFraming.FindStringSubmatch(prompt)
	if m == nil {
		return 0
	}
	part, _ := strconv.Atoi(m[1])
	return part
}

// heldGate holds every chunk call until release is closed.
func heldGate(release <-chan struct{}) func(int) error {
	return func(int) error {
		select {
		case <-release:
			return nil
		case <-time.After(waitLimit):
			return errors.New("chunk call was never released")
		}
	}
}

// tenKBFiles is n 10KB files in distinct areas, each packing into a chunk of its own
// under capOneFilePerChunk.
func tenKBFiles(n int) []chunkFile {
	files := make([]chunkFile, n)
	for i := range files {
		files[i] = chunkFile{path: fmt.Sprintf("area%d/file.go", i+1), size: 10_000}
	}
	return files
}

func capOneFilePerChunk(t *testing.T, files []chunkFile) chunkScenario {
	t.Helper()
	s := chunkScenario{files: files, root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s = s.withShortPrompt(t)
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000
	return s
}

type generation struct {
	body string
	err  error
}

// startGenerate runs the scenario's generation in the background over transport.
func (s chunkScenario) startGenerate(t *testing.T, transport notes.Transport) <-chan generation {
	t.Helper()
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, s.root)
	done := make(chan generation, 1)
	go func() {
		body, err := gen.Generate(t.Context(), "v1.0.0", s.cfg)
		done <- generation{body: body, err: err}
	}()
	return done
}

func awaitGeneration(t *testing.T, done <-chan generation) string {
	t.Helper()
	select {
	case g := <-done:
		if g.err != nil {
			t.Fatalf("Generate returned unexpected error: %v", g.err)
		}
		return g.body
	case <-time.After(waitLimit):
		t.Fatal("Generate did not finish")
		return ""
	}
}

func TestGenerator_OverCap_SixChunkCalls_FourInFlightAndNeverMore(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	wt := &waveTransport{mergeBody: "merged", gate: heldGate(release)}

	done := s.startGenerate(t, wt)
	wt.waitInFlight(t, 4)
	close(release)
	body := awaitGeneration(t, done)

	_, maxInFlight, events, merges := wt.snapshot()
	if maxInFlight != 4 {
		t.Errorf("most chunk calls in flight at once = %d, want 4", maxInFlight)
	}
	if starts := countPrefix(events, "start "); starts != 6 {
		t.Errorf("chunk calls = %d, want 6", starts)
	}
	if len(merges) != 1 || body != "merged" {
		t.Errorf("merge calls = %d, body = %q; want one merge whose output is the body", len(merges), body)
	}
}

func TestGenerator_OverCap_ThreeChunkCalls_AllInFlightTogether(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	release := make(chan struct{})
	wt := &waveTransport{mergeBody: "merged", gate: heldGate(release)}

	done := s.startGenerate(t, wt)
	wt.waitInFlight(t, 3)
	close(release)
	awaitGeneration(t, done)

	if _, maxInFlight, _, _ := wt.snapshot(); maxInFlight != 3 {
		t.Errorf("most chunk calls in flight at once = %d, want 3", maxInFlight)
	}
}

func TestGenerator_ChunkBound_NoConfigKeyControlsIt(t *testing.T) {
	t.Parallel()

	concurrency := regexp.MustCompile(`(?i)parallel|concurren|in flight|at once|fan.?out|workers`)
	for _, row := range config.MetadataRows() {
		if concurrency.MatchString(row.Key) || concurrency.MatchString(row.Description) {
			t.Errorf("config key %q (level %v) reads as a concurrency setting: %q", row.Key, row.Level, row.Description)
		}
	}
}

func TestGenerator_OverCap_ChunksFinishingOutOfOrder_MergeCarriesNotesInChunkOrder(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	finished := map[int]chan struct{}{1: make(chan struct{}), 2: make(chan struct{}), 3: make(chan struct{})}
	wt := &waveTransport{mergeBody: "merged"}
	// Each part finishes only after the part after it has, so they end 3, 2, 1.
	wt.ended = func(part int) { close(finished[part]) }
	wt.gate = func(part int) error {
		next, ok := finished[part+1]
		if !ok {
			return nil
		}
		select {
		case <-next:
			return nil
		case <-time.After(waitLimit):
			return fmt.Errorf("part %d waited for part %d, which never finished", part, part+1)
		}
	}

	awaitGeneration(t, s.startGenerate(t, wt))

	_, _, events, merges := wt.snapshot()
	var ends []string
	for _, e := range events {
		if strings.HasPrefix(e, "end ") {
			ends = append(ends, e)
		}
	}
	if strings.Join(ends, ",") != "end 3,end 2,end 1" {
		t.Fatalf("chunk calls ended %v, want end 3, end 2, end 1", ends)
	}
	if len(merges) != 1 {
		t.Fatalf("merge calls = %d, want one", len(merges))
	}
	last := -1
	for _, f := range s.files {
		at := strings.Index(merges[0], chunkNotesFor(f.path))
		if at < 0 {
			t.Fatalf("merge is missing the notes of the part opening with %s", f.path)
		}
		if at < last {
			t.Errorf("merge carries the notes of %s out of chunk order", f.path)
		}
		last = at
	}
}

func TestGenerator_OverCap_MergeStartsOnlyAfterEveryChunkCallEnds(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	release := make(chan struct{})
	wt := &waveTransport{mergeBody: "merged", gate: heldGate(release)}

	done := s.startGenerate(t, wt)
	wt.waitInFlight(t, 4)
	close(release)
	awaitGeneration(t, done)

	_, _, events, _ := wt.snapshot()
	if len(events) != 13 || events[len(events)-1] != "merge" {
		t.Fatalf("events = %v, want six chunk calls started and ended, then the merge", events)
	}
	if countPrefix(events[:12], "end ") != 6 {
		t.Errorf("events = %v, want every chunk call ended before the merge", events)
	}
}

func countPrefix(events []string, prefix string) int {
	n := 0
	for _, e := range events {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

// pacedAIRunner stands in for the AI command behind a real ai.Transport: each call
// takes callTime. It is safe for concurrent calls.
//
// It ignores its context on purpose: it never returns a context error, so no
// scheduler stall can time a call out (ai.Transport.attempt hands back a successful
// RunWith result without re-checking the context).
type pacedAIRunner struct {
	callTime time.Duration

	mu    sync.Mutex
	calls []pacedCall
}

// pacedCall is one RunWith call as pacedAIRunner saw it on entry.
type pacedCall struct {
	entered     time.Time
	deadline    time.Time
	hasDeadline bool
	chunk       bool
}

func (r *pacedAIRunner) RunWith(ctx context.Context, stdin io.Reader, _ string, _ ...string) (runner.Result, error) {
	entered := time.Now()
	prompt, err := io.ReadAll(stdin)
	if err != nil {
		return runner.Result{}, err
	}
	deadline, hasDeadline := ctx.Deadline()
	chunk := isChunkPrompt(string(prompt))
	r.mu.Lock()
	r.calls = append(r.calls, pacedCall{entered: entered, deadline: deadline, hasDeadline: hasDeadline, chunk: chunk})
	r.mu.Unlock()

	time.Sleep(r.callTime)
	if chunk {
		return runner.Result{Stdout: chunkNotesFor(firstPath(sliceOf(string(prompt))))}, nil
	}
	return runner.Result{Stdout: "merged within every deadline"}, nil
}

func (r *pacedAIRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected Run")
}

func (r *pacedAIRunner) RunInteractive(context.Context, string, ...string) error {
	return errors.New("unexpected RunInteractive")
}

func (r *pacedAIRunner) RunInDir(context.Context, string, []string, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("unexpected RunInDir")
}

// Every check here holds by construction, never by a timing margin: no call can time
// out, and no check bounds from below the time a call has left at runner entry. What
// tells a fresh per-attempt deadline from one spanning the wave (or the chunk calls)
// is the ordering of the deadlines: the merge starts only after every chunk call has
// slept callTime, and with at most four chunk calls in flight a second-round chunk
// call starts only after a first-round one has — so each later call's own deadline
// lies at least callTime past the earlier one's, where a shared deadline is equal.
func TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten(t *testing.T) {
	t.Parallel()

	const timeout = 500 * time.Millisecond
	const callTime = 200 * time.Millisecond
	aiRunner := &pacedAIRunner{callTime: callTime}
	perAttempt := timeout
	transport := ai.NewTransport(aiRunner, ai.Config{AICommand: "claude -p", Timeout: &perAttempt})
	s := capOneFilePerChunk(t, tenKBFiles(6))

	started := time.Now()
	body := awaitGeneration(t, s.startGenerate(t, transport))
	elapsed := time.Since(started)

	if body != "merged within every deadline" {
		t.Errorf("body = %q, want the merge output", body)
	}
	if elapsed <= timeout {
		t.Fatalf("wave took %v, want longer than the %v timeout for the scenario to hold", elapsed, timeout)
	}
	aiRunner.mu.Lock()
	defer aiRunner.mu.Unlock()
	calls := aiRunner.calls
	var chunks []pacedCall
	var merges []pacedCall
	for i, c := range calls {
		if !c.hasDeadline {
			t.Fatalf("call %d carried no deadline, want every call with one", i+1)
		}
		if allotted := c.deadline.Sub(c.entered); allotted > timeout {
			t.Errorf("call %d was given %v by its deadline, want at most the per-attempt %v", i+1, allotted, timeout)
		}
		if c.chunk {
			chunks = append(chunks, c)
		} else {
			merges = append(merges, c)
		}
	}
	if len(chunks) != 6 || len(merges) != 1 {
		t.Fatalf("chunk calls = %d, merges = %d; want 6 chunk calls then 1 merge", len(chunks), len(merges))
	}

	merge := merges[0]
	for i, c := range chunks {
		if gap := merge.deadline.Sub(c.deadline); gap < callTime {
			t.Errorf("merge deadline lies %v past chunk call %d's, want at least %v (a deadline spanning the wave?)", gap, i+1, callTime)
		}
	}
	// The two latest deadlines are checked, not the last two calls to record
	// themselves: a first-round call stalled before recording itself would pass for a
	// second-round one. Both second-round calls' deadlines lie callTime past the
	// earliest, so the two latest do too, however the deadlines interleave.
	slices.SortFunc(chunks, func(a, b pacedCall) int { return a.deadline.Compare(b.deadline) })
	earliestFirstRound := chunks[0].deadline
	for i, c := range chunks[4:] {
		if gap := c.deadline.Sub(earliestFirstRound); gap < callTime {
			t.Errorf("second-round chunk call %d's deadline lies %v past the earliest first-round one, want at least %v (a deadline spanning the chunk calls?)", i+1, gap, callTime)
		}
	}
}
