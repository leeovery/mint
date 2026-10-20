package notes_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/notes"
)

// awaitFailure waits for the generation and returns its error, failing the test on a
// body or a hang.
func awaitFailure(t *testing.T, done <-chan generation) error {
	t.Helper()
	select {
	case g := <-done:
		if g.err == nil {
			t.Fatalf("Generate returned body %q and nil error, want a failure", g.body)
		}
		return g.err
	case <-time.After(waitLimit):
		t.Fatal("Generate did not finish")
		return nil
	}
}

// startGenerateIn is startGenerate on a context the test controls.
func (s chunkScenario) startGenerateIn(t *testing.T, ctx context.Context, transport notes.Transport) <-chan generation {
	t.Helper()
	r := seedNormalPathGit(t, s.diff(), s.nameStatus(), s.numstat())
	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, s.root)
	done := make(chan generation, 1)
	go func() {
		body, err := gen.Generate(ctx, "v1.0.0", s.cfg)
		done <- generation{body: body, err: err}
	}()
	return done
}

func waitClosed(ch <-chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-time.After(waitLimit):
		return errors.New("never signalled")
	}
}

func TestGenerator_OverCap_OneChunkFails_NoMergeNoNewCallsAndRunningCallsFinish(t *testing.T) {
	t.Parallel()

	causes := []struct {
		name  string
		cause error
	}{
		{name: "after the transport's retry", cause: &ai.GenerationError{Stdout: "API Error: 529 overloaded", ExitCode: 1}},
		{name: "by timeout", cause: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)},
		{name: "by a missing binary", cause: fmt.Errorf("%w: claude not found", ai.ErrCommandMissing)},
	}
	for _, tc := range causes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := capOneFilePerChunk(t, tenKBFiles(6))
			fail, release, twoEnded := make(chan struct{}), make(chan struct{}), make(chan struct{})
			wt := &waveTransport{mergeBody: "merged"}
			wt.ended = func(part int) {
				if part == 2 {
					close(twoEnded)
				}
			}
			wt.gate = func(part int) error {
				if part == 2 {
					if err := waitClosed(fail); err != nil {
						return err
					}
					return tc.cause
				}
				return heldGate(release)(part)
			}

			done := s.startGenerate(t, wt)
			wt.waitInFlight(t, 4)
			close(fail)
			if err := waitClosed(twoEnded); err != nil {
				t.Fatalf("part 2 did not end: %v", err)
			}
			// Part 2 frees its slot just after it ends; the grace lets the wave see that
			// free slot before any held call frees another.
			time.Sleep(50 * time.Millisecond)
			close(release)
			err := awaitFailure(t, done)

			if !errors.Is(err, tc.cause) {
				t.Errorf("error = %v, want it to carry the failed chunk's cause %v", err, tc.cause)
			}
			_, _, events, merges := wt.snapshot()
			if len(merges) != 0 {
				t.Errorf("merge calls = %d, want none after a failed chunk", len(merges))
			}
			if starts := countPrefix(events, "start "); starts != 4 {
				t.Errorf("chunk calls started = %d (events %v), want only the 4 running when part 2 failed", starts, events)
			}
			for _, part := range []int{1, 3, 4} {
				if !slices.Contains(events, fmt.Sprintf("end %d", part)) {
					t.Errorf("part %d, running when part 2 failed, did not finish; events %v", part, events)
				}
			}
		})
	}
}

func TestGenerator_OverCap_TwoChunksFailInFlight_ErrorCarriesTheFirstToFail(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	threeEnded := make(chan struct{})
	wt := &waveTransport{mergeBody: "merged"}
	wt.ended = func(part int) {
		if part == 3 {
			close(threeEnded)
		}
	}
	wt.gate = func(part int) error {
		switch part {
		case 3:
			return &ai.GenerationError{Stdout: "part 3: usage limit reached", ExitCode: 1}
		case 1:
			if err := waitClosed(threeEnded); err != nil {
				return err
			}
			// Part 3's failure is recorded by the wave just after its call ends.
			time.Sleep(50 * time.Millisecond)
			return &ai.GenerationError{Stdout: "part 1: login expired", ExitCode: 1}
		default:
			return nil
		}
	}

	done := s.startGenerate(t, wt)
	err := awaitFailure(t, done)

	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) {
		t.Fatalf("error = %v, want an *ai.GenerationError", err)
	}
	if got := genErr.Output(); got != "part 3: usage limit reached" {
		t.Errorf("failure output = %q, want the first failure's %q", got, "part 3: usage limit reached")
	}
}

// failingMergeTransport answers chunk calls with notes and fails the merge with cause.
type failingMergeTransport struct {
	cause error

	mu     sync.Mutex
	merges int
}

func (ft *failingMergeTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isChunkPrompt(prompt) {
		return chunkNotesFor(firstPath(sliceOf(prompt))), nil
	}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.merges++
	return "", ft.cause
}

func TestGenerator_OverCap_MergeFails_ErrorCarriesTheMergeOutput(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	ft := &failingMergeTransport{cause: &ai.GenerationError{Stdout: "Prompt is too long", ExitCode: 1}}

	err := s.generateErr(t, ft)

	var genErr *ai.GenerationError
	if !errors.As(err, &genErr) || genErr.Output() != "Prompt is too long" {
		t.Errorf("error = %v, want the merge call's failure with its output %q", err, "Prompt is too long")
	}
	if ft.merges != 1 {
		t.Errorf("merge calls = %d, want one", ft.merges)
	}
}

// cancellableTransport holds every chunk call until its context is done, recording
// how many started.
type cancellableTransport struct {
	mu       sync.Mutex
	inFlight int
	starts   int
	merges   int
}

func (ct *cancellableTransport) Generate(ctx context.Context, prompt string) (string, error) {
	ct.mu.Lock()
	if !isChunkPrompt(prompt) {
		ct.merges++
		ct.mu.Unlock()
		return "merged", nil
	}
	ct.starts++
	ct.inFlight++
	ct.mu.Unlock()
	defer func() {
		ct.mu.Lock()
		ct.inFlight--
		ct.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(waitLimit):
		return "", errors.New("chunk call was never cancelled")
	}
}

func (ct *cancellableTransport) counts() (inFlight, starts, merges int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.inFlight, ct.starts, ct.merges
}

func TestGenerator_OverCap_CancelledMidWave_CancelsCallsInFlightAndStartsNoMore(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	ct := &cancellableTransport{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := s.startGenerateIn(t, ctx, ct)
	deadline := time.Now().Add(waitLimit)
	for inFlight, _, _ := ct.counts(); inFlight != 4; inFlight, _, _ = ct.counts() {
		if time.Now().After(deadline) {
			t.Fatalf("chunk calls in flight = %d, never reached 4", inFlight)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	err := awaitFailure(t, done)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	inFlight, starts, merges := ct.counts()
	if inFlight != 0 || starts != 4 || merges != 0 {
		t.Errorf("after cancel: in flight = %d, started = %d, merges = %d; want 0, the 4 cancelled, no merge", inFlight, starts, merges)
	}
}

// failThenHoldTransport fails part 2 once fail is closed and holds every other chunk
// call until its context is done.
type failThenHoldTransport struct {
	fail      chan struct{}
	twoFailed chan struct{}

	mu     sync.Mutex
	starts int
	merges int
}

func (ft *failThenHoldTransport) Generate(ctx context.Context, prompt string) (string, error) {
	ft.mu.Lock()
	if !isChunkPrompt(prompt) {
		ft.merges++
		ft.mu.Unlock()
		return "merged", nil
	}
	ft.starts++
	ft.mu.Unlock()
	if partNumber(prompt) == 2 {
		defer close(ft.twoFailed)
		if err := waitClosed(ft.fail); err != nil {
			return "", err
		}
		return "", &ai.GenerationError{Stdout: "part 2: usage limit reached", ExitCode: 1}
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(waitLimit):
		return "", errors.New("chunk call was never cancelled")
	}
}

func (ft *failThenHoldTransport) waitStarts(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		ft.mu.Lock()
		starts := ft.starts
		ft.mu.Unlock()
		if starts == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("chunk calls started = %d, never reached %d", starts, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestGenerator_OverCap_CancelledAfterAChunkFailed_ReturnsTheCancel(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(6))
	ft := &failThenHoldTransport{fail: make(chan struct{}), twoFailed: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := s.startGenerateIn(t, ctx, ft)
	ft.waitStarts(t, 4)
	close(ft.fail)
	if err := waitClosed(ft.twoFailed); err != nil {
		t.Fatalf("part 2 did not fail: %v", err)
	}
	// Part 2's failure is recorded by the wave just after its call returns.
	time.Sleep(50 * time.Millisecond)
	cancel()
	err := awaitFailure(t, done)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled over the earlier chunk failure", err)
	}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if ft.starts != 4 || ft.merges != 0 {
		t.Errorf("chunk calls started = %d, merges = %d; want 4 and none", ft.starts, ft.merges)
	}
}
