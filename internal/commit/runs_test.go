package commit_test

// This file holds whole commit runs, the real orchestrator with its git, editor and AI
// scripted, each drawn two ways: on the rail (rail_frames_test.go holds those frames)
// and in plain, whose transcript, stdout then stderr, is held byte for byte to
// testdata/plain. Plain's transcripts were written before the rail switch and pin that
// it changed nothing plain prints.
// `go test ./internal/commit -run PlainTranscripts -update-plain` writes them again.

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/runner"
)

var updatePlain = flag.Bool("update-plain", false, "write the plain transcripts of the commit runs again")

const (
	runMessage = "feat(webhooks): retry failed deliveries with backoff\n" +
		"\n" +
		"Deliveries that fail are retried with exponential backoff for up to an\n" +
		"hour before they are marked dead.\n"
	runRegenMessage = "feat(webhooks): back off between delivery retries\n" +
		"\n" +
		"A failed delivery waits longer before each retry, for up to an hour.\n"
	runEditedMessage = "feat(webhooks): retry failed deliveries for up to an hour\n"
	runTrackedDiff   = "diff --git a/src/deliver.go b/src/deliver.go\n+\tretry(d)\n"
	runStagedDiff    = "diff --git a/src/deliver.go b/src/deliver.go\n+\tretry(d)\n"
	runUsageLimit    = "Claude AI usage limit reached|1761742800\n"
	runPushRejected  = " ! [rejected]        main -> main (fetch first)\n" +
		"error: failed to push some refs to 'https://github.com/acme/pigeon'\n" +
		"hint: Updates were rejected because the remote contains work that you do not\n" +
		"hint: have locally. Integrate the remote changes (e.g., 'git pull ...')\n" +
		"hint: before pushing again.\n"
)

// runEpoch is when every whole run starts.
var runEpoch = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// steppingClock is the commit's clock in the whole runs: each read is step past the
// one before, so every time a run shows is the same in every run.
type steppingClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.step)
	return c.now
}

// scriptedAI answers each message call with the next of its answers.
type scriptedAI struct {
	mu      sync.Mutex
	answers []aiAnswer
}

type aiAnswer struct {
	body string
	err  error
}

func (s *scriptedAI) Generate(context.Context, string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.answers) == 0 {
		return "", errors.New("scriptedAI: no answer left")
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a.body, a.err
}

// commitWorld is pigeon committing its retry change: its root at ~/Code/pigeon under
// a home of its own, git and the editor scripted, the AI answering from a script, and
// the deps of a run.
type commitWorld struct {
	f    *runner.FakeRunner
	er   *editorRunner
	ai   *scriptedAI
	deps commit.Deps
}

// commitRun is one whole commit run, drawn on the rail and in plain: its answers, a
// line each, whether it ends in an error, and the run itself.
type commitRun struct {
	answers string
	aborted bool
	run     func(t *testing.T, w *commitWorld) error
}

// newCommitWorld is pigeon with its commands scripted, its run drawing on p.
func newCommitWorld(t *testing.T, p presenter.Presenter) *commitWorld {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "Code", "pigeon")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f := runner.NewFakeRunner()
	er := &editorRunner{fake: f}
	ai := &scriptedAI{}
	return &commitWorld{
		f:  f,
		er: er,
		ai: ai,
		deps: commit.Deps{
			Presenter:        p,
			Runner:           er,
			Mutator:          git.NewMutator(er),
			Transport:        ai,
			Root:             root,
			StdinInteractive: true,
			MintVersion:      "0.1.2",
			Home:             home,
			Clock:            (&steppingClock{now: runEpoch, step: 700 * time.Millisecond}).Now,
		},
	}
}

func out(stdout string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{Stdout: stdout}}
}

// failing is a git call that exits non-zero, writing stderr.
func failing(stderr string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{Stderr: stderr, ExitCode: 1}, Err: errors.New("exit status 1")}
}

// addition is `git diff --no-index -- /dev/null <file>` rendering an untracked file,
// which exits 1 as its inputs differ.
func addition(file string) runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{Stdout: "diff --git a/" + file + " b/" + file + "\nnew file mode 100644\n+package src\n", ExitCode: 1},
		Err:    errors.New("exit status 1"),
	}
}

// started scripts where HEAD stands as the run opens: main at 7c1e0b4.
func (w *commitWorld) started() {
	w.f.SeedSequence("git", out("7c1e0b4d2a9f8e6c5b3a1f0e9d8c7b6a5f4e3d2c\nmain\n")) // rev-parse HEAD --abbrev-ref HEAD
}

// stagedDiff scripts what is staged: its probe finding something, then the diff the
// AI reads.
func (w *commitWorld) stagedDiff() {
	w.f.SeedSequence("git",
		out("src/deliver.go\nsrc/queue.go\n"), // diff --cached --name-only -- .
		out(runStagedDiff),                    // diff --cached -- .
	)
}

// trackedDiff scripts the tracked changes -a stages: its probe finding something, then
// the diff the AI reads.
func (w *commitWorld) trackedDiff() {
	w.f.SeedSequence("git",
		out("src/deliver.go\nsrc/queue.go\n"), // diff HEAD --name-only -- .
		out(runTrackedDiff),                   // diff HEAD -- .
	)
}

// everything scripts every change -A stages: the tracked changes, then the untracked
// files, each rendered as an addition.
func (w *commitWorld) everything() {
	w.f.SeedSequence("git",
		out("src/deliver.go\nsrc/queue.go\n"),            // diff HEAD --name-only -- .
		out(runTrackedDiff),                              // diff HEAD -- .
		out("src/backoff.go\x00src/backoff_test.go\x00"), // ls-files --others --exclude-standard -z -- .
		addition("src/backoff.go"),                       // diff --no-index -- /dev/null src/backoff.go
		addition("src/backoff_test.go"),                  // diff --no-index -- /dev/null src/backoff_test.go
	)
}

// editor scripts git naming the editor, which saves saved.
func (w *commitWorld) editor(saved string) {
	w.er.saves = append(w.er.saves, saved)
	w.f.SeedSequence("git", out("vim\n")) // var GIT_EDITOR
}

// committed scripts the commit going in, and the new commit's short sha read.
func (w *commitWorld) committed() {
	w.f.SeedSequence("git",
		out(""),          // commit -F -
		out("a3f9c21\n"), // rev-parse --short HEAD
	)
}

// pushed scripts the push answering with push.
func (w *commitWorld) pushed(push runner.ScriptedCall) {
	w.f.SeedSequence("git", push) // push
}

// messages scripts the AI's answers, a message call each.
func (w *commitWorld) messages(answers ...aiAnswer) {
	w.ai.answers = append(w.ai.answers, answers...)
}

// commit runs the commit under the flags set.
func (w *commitWorld) commit(t *testing.T, set func(*commit.Deps)) error {
	t.Helper()
	deps := w.deps
	if set != nil {
		set(&deps)
	}
	return commit.Run(t.Context(), deps)
}

func addAllPush(d *commit.Deps) { d.Staging, d.Push = commit.AddAll, true }

var commitRuns = map[string]commitRun{
	// commit -Ap: the message accepted, then everything staged, committed and pushed.
	"commit-pushed-100": {answers: "y\n", run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.everything()
		w.messages(aiAnswer{body: runMessage})
		w.f.SeedSequence("git", out("")) // add -A
		w.committed()
		w.pushed(out(""))
		return w.commit(t, addAllPush)
	}},
	// The message answered r, written again with no extra context, then y.
	"commit-regenerated-100": {answers: "r\n\ny\n", run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.stagedDiff()
		w.f.SeedSequence("git", out(runStagedDiff)) // diff --cached -- . (the r)
		w.messages(aiAnswer{body: runMessage}, aiAnswer{body: runRegenMessage})
		w.committed()
		return w.commit(t, nil)
	}},
	// The message answered e, edited, then y.
	"commit-edited-100": {answers: "e\ny\n", run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.stagedDiff()
		w.messages(aiAnswer{body: runMessage})
		w.editor(runEditedMessage)
		w.committed()
		return w.commit(t, nil)
	}},
	// commit -a with the AI failing on a usage limit: the editor opens, and what is
	// written there is staged and committed.
	"commit-ai-failed-100": {run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.trackedDiff()
		w.messages(aiAnswer{err: &ai.GenerationError{Stdout: runUsageLimit, ExitCode: 1}})
		w.editor(runEditedMessage)
		w.f.SeedSequence("git", out("")) // add -u
		w.committed()
		return w.commit(t, func(d *commit.Deps) { d.Staging = commit.All })
	}},
	// commit -p whose push is rejected: the commit stays, and the run exits non-zero.
	"commit-push-failed-100": {answers: "y\n", aborted: true, run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.stagedDiff()
		w.messages(aiAnswer{body: runMessage})
		w.committed()
		w.pushed(failing(runPushRejected))
		return w.commit(t, func(d *commit.Deps) { d.Push = true })
	}},
	// commit -Ap with the message declined: nothing staged, nothing committed.
	"commit-declined-100": {answers: "n\n", aborted: true, run: func(t *testing.T, w *commitWorld) error {
		w.started()
		w.everything()
		w.messages(aiAnswer{body: runMessage})
		return w.commit(t, addAllPush)
	}},
}

// plainCommit runs r in plain, and returns its stdout, then its stderr under a rule of
// its own.
func plainCommit(t *testing.T, r commitRun) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	p := presenter.NewPlainPresenterWithInput(&stdout, &stderr, strings.NewReader(r.answers))
	err := r.run(t, newCommitWorld(t, p))
	return stdout.String() + "--- stderr ---\n" + stderr.String(), err
}

func TestCommit_PlainTranscripts(t *testing.T) {
	t.Parallel()

	for name, r := range commitRuns {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			transcript, err := plainCommit(t, r)
			if aborted := err != nil; aborted != r.aborted {
				t.Errorf("the run returned %v, want aborted %v", err, r.aborted)
			}
			path := filepath.Join("testdata", "plain", name+".txt")
			if *updatePlain {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(transcript), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v: write the transcripts with -update-plain", err)
			}
			if transcript != string(want) {
				t.Errorf("plain printed\n%s\nwant, as before the rail\n%s", transcript, want)
			}
		})
	}
}
