package commit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// emptyTree is the empty tree's id, which `git hash-object -t tree /dev/null` prints.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// unbornHead scripts the start station's reads on a branch with no commits yet: `git
// rev-parse HEAD --abbrev-ref HEAD` fails, and `git symbolic-ref --short -q HEAD`
// names the branch.
func unbornHead() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		{Result: runner.Result{ExitCode: 128, Stdout: "HEAD\n"}, Err: errors.New("exit status 128")},
		{Result: runner.Result{Stdout: "main\n"}},
	}
}

// firstCommitReads is what a first commit under mode reads before its review, each
// read's argv and the output scripted for it: the branch, then, where the mode has a
// tracked source, the empty tree it compares against, then the preflight's counts.
func firstCommitReads(mode commit.StagingMode) ([][]string, []runner.ScriptedCall) {
	branch := []string{"symbolic-ref", "--short", "-q", "HEAD"}
	counted := runner.ScriptedCall{Result: runner.Result{Stdout: "3\t0\tREADME.md\n"}}
	untracked := []string{"ls-files", "--others", "--exclude-standard", "-z", "--", "."}
	switch mode {
	case commit.All:
		return [][]string{branch, {"hash-object", "-t", "tree", "/dev/null"}, {"diff", emptyTree, "--numstat", "--", "."}},
			[]runner.ScriptedCall{{Result: runner.Result{Stdout: emptyTree + "\n"}}, counted}
	case commit.AddAll:
		return [][]string{branch, {"hash-object", "-t", "tree", "/dev/null"}, {"diff", emptyTree, "--numstat", "--", "."}, untracked},
			[]runner.ScriptedCall{{Result: runner.Result{Stdout: emptyTree + "\n"}}, counted, {}}
	default:
		return [][]string{branch, {"diff", "--cached", "--numstat", "--", "."}},
			[]runner.ScriptedCall{counted}
	}
}

// stagingArgs is the deferred `git add` mode runs on accept, nil for none.
func stagingArgs(mode commit.StagingMode) []string {
	switch mode {
	case commit.All:
		return []string{"add", "-u"}
	case commit.AddAll:
		return []string{"add", "-A"}
	default:
		return nil
	}
}

func modeName(mode commit.StagingMode) string {
	switch mode {
	case commit.All:
		return "-a"
	case commit.AddAll:
		return "-A"
	default:
		return "staged-only"
	}
}

func argvOf(invs []runner.Invocation) [][]string {
	argv := make([][]string, len(invs))
	for i, inv := range invs {
		argv[i] = inv.Args
	}
	return argv
}

// A first commit is proposed "Initial commit" with no AI call, in every staging
// mode: accepted, or under -y, it stages as the mode does and commits that message;
// declined, it runs no git add and no git commit.
func TestRun_FirstCommit_ProposesInitialCommitWithoutTheAI(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name    string
		choice  presenter.Choice
		yes     bool
		commits bool
	}{
		{name: "accepted", choice: presenter.ChoiceYes, commits: true},
		{name: "declined", choice: presenter.ChoiceNo},
		{name: "under -y", yes: true, commits: true},
	}
	for _, mode := range []commit.StagingMode{commit.StagedOnly, commit.All, commit.AddAll} {
		for _, outcome := range outcomes {
			t.Run(modeName(mode)+" "+outcome.name, func(t *testing.T) {
				t.Parallel()

				want, reads := firstCommitReads(mode)
				calls := append(unbornHead(), reads...)
				if outcome.commits {
					if add := stagingArgs(mode); add != nil {
						want = append(want, add)
						calls = append(calls, runner.ScriptedCall{})
					}
					want = append(want, []string{"commit", "-F", "-"})
					calls = append(calls, runner.ScriptedCall{}, scriptedShortHead())
				}
				r := runner.NewFakeRunner()
				r.SeedSequence("git", calls...)
				rec := &presentertest.RecordingPresenter{}
				if !outcome.yes {
					rec.NextChoices = []presenter.Choice{outcome.choice}
				}
				transport := scriptedTransport("feat: never asked for")
				deps := newCommitDeps(rec, r, transport, t.TempDir())
				deps.Staging, deps.Yes = mode, outcome.yes

				err := commit.Run(context.Background(), deps)

				if outcome.commits && err != nil {
					t.Fatalf("Run returned unexpected error: %v", err)
				}
				if !outcome.commits && err == nil {
					t.Fatal("Run returned nil for a declined first commit, want the abort")
				}
				gits := gitInvocations(r)
				if got := argvOf(gits); !slices.EqualFunc(got, want, slices.Equal) {
					t.Errorf("git argv =\n%q\nwant\n%q", got, want)
				}
				if outcome.commits {
					if got := gits[len(gits)-1].Stdin; got != "Initial commit\n" {
						t.Errorf("commit message = %q, want %q", got, "Initial commit\n")
					}
				}
				if transport.calls() != 0 {
					t.Errorf("AI calls = %d, want none for a first commit", transport.calls())
				}
			})
		}
	}
}

// The message node names the first commit's default as its source, in the plan and
// once settled, and the review shows the message it proposes.
func TestRun_FirstCommit_TheMessageNodeNamesTheFirstCommitsDefault(t *testing.T) {
	t.Parallel()

	_, reads := firstCommitReads(commit.StagedOnly)
	r := runner.NewFakeRunner()
	r.SeedSequence("git", append(append(unbornHead(), reads...), runner.ScriptedCall{}, scriptedShortHead())...)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), newCommitDeps(rec, r, scriptedTransport("unused"), t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	plan := railEvent(t, rec, presentertest.KindShowPlan).ShowPlan
	if got, want := plan.Stages[1], (presenter.PlanStage{Key: "message", Name: "message", Detail: "the first commit's default"}); got != want {
		t.Errorf("plan's message stage = %+v, want %+v", got, want)
	}
	wantSuccess := presenter.StageSuccess{Name: "commit message", Key: "message", Detail: "the first commit's default"}
	var settled []presenter.StageSuccess
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Key == "message" {
			settled = append(settled, ev.StageSucceeded)
		}
	}
	if len(settled) != 1 || settled[0] != wantSuccess {
		t.Errorf("message successes = %+v, want only %+v", settled, wantSuccess)
	}
	if got := railEvent(t, rec, presentertest.KindShowMessage).ShowMessage.Body; got != "Initial commit\n" {
		t.Errorf("reviewed message = %q, want %q", got, "Initial commit\n")
	}
}

// `r` on a first commit asks the AI, its diff comparing the working tree against the
// empty tree, and the regenerated message is what commits.
func TestRun_FirstCommit_RegenerateAsksTheAIAgainstTheEmptyTree(t *testing.T) {
	t.Parallel()

	const message = "feat: scaffold the pigeon service\n"
	const diff = "diff --git a/README.md b/README.md\nnew file mode 100644\n+# pigeon\n"
	_, reads := firstCommitReads(commit.All)
	r := runner.NewFakeRunner()
	r.SeedSequence("git", append(append(unbornHead(), reads...),
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}}, // git diff <empty tree> -- . (the AI's diff)
		runner.ScriptedCall{}, // git add -u
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),
	)...)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes}}
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, t.TempDir())
	deps.Staging = commit.All

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := findGitArgs0(t, r, "diff"); !slices.Equal(got.Args, []string{"diff", emptyTree, "--numstat", "--", "."}) {
		t.Errorf("preflight count argv = %q, want it against the empty tree", got.Args)
	}
	diffs := gitVerbInvocations(r.Invocations(), "diff")
	if got, want := diffs[len(diffs)-1].Args, []string{"diff", emptyTree, "--", "."}; !slices.Equal(got, want) {
		t.Errorf("AI diff argv = %q, want %q", got, want)
	}
	if transport.calls() != 1 || !strings.Contains(transport.lastPrompt(t), "+# pigeon") {
		t.Errorf("AI calls = %d, want one carrying the first commit's diff", transport.calls())
	}
	commits := gitVerbInvocations(r.Invocations(), "commit")
	if len(commits) != 1 || commits[0].Stdin != message {
		t.Errorf("commits = %+v, want one carrying the regenerated message", commits)
	}
}

// --no-ai on a first commit opens the editor as it does on any other.
func TestRun_FirstCommit_NoAIOpensTheEditor(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	_, reads := firstCommitReads(commit.StagedOnly)
	f.SeedSequence("git", append(append(unbornHead(), reads...),
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),
	)...)
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: f, saved: "chore: start pigeon\n"}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if plan := railEvent(t, rec, presentertest.KindShowPlan).ShowPlan; plan.Stages[1].Detail != "written in your editor" {
		t.Errorf("plan's message stage = %+v, want it written in your editor", plan.Stages[1])
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != "chore: start pigeon\n" {
		t.Errorf("commits = %+v, want one carrying the editor's message", commits)
	}
}
