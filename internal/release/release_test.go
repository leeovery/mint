package release_test

import (
	"errors"
	"fmt"
	"testing"

	"mint/internal/release"
	"mint/internal/runner"
)

func TestReleaser_TagAndPush_CreatesAnnotatedTagWithSubjectAndBody(t *testing.T) {
	t.Parallel()

	// The annotated tag must be created via `git tag -a {tag} -F -` with the
	// message piped on stdin: subject `{commit_prefix} Release {tag}`, a blank
	// line, then the FULL notes body verbatim. -a/-F makes it an ANNOTATED tag
	// (never lightweight) — the annotation body is the single source mint reads
	// later. Asserting the recorded stdin pins the exact composed message.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	rel := release.NewReleaser(r)

	_, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err != nil {
		t.Fatalf("TagAndPush returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (tag then push)", len(invs))
	}

	tagInv := invs[0]
	if tagInv.Name != "git" {
		t.Errorf("tag command = %q, want git", tagInv.Name)
	}
	wantTagArgs := []string{"tag", "-a", "v0.0.1", "-F", "-"}
	if !equalArgs(tagInv.Args, wantTagArgs) {
		t.Errorf("tag args = %v, want %v", tagInv.Args, wantTagArgs)
	}
	wantMessage := "🌿 Release v0.0.1\n\nInitial release."
	if tagInv.Stdin != wantMessage {
		t.Errorf("tag message (stdin) = %q, want %q", tagInv.Stdin, wantMessage)
	}
}

func TestReleaser_TagAndPush_PushesAtomicHEADAndTag(t *testing.T) {
	t.Parallel()

	// The push must be the exact atomic form `git push --atomic origin HEAD {tag}`
	// so the commits and the tag go up together or not at all (the point of no
	// return). Asserting the exact argv guards the form against drift.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	rel := release.NewReleaser(r)

	if _, err := rel.TagAndPush(t.Context(), "v1.2.3", "🌿", "body"); err != nil {
		t.Fatalf("TagAndPush returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (tag then push)", len(invs))
	}

	pushInv := invs[1]
	if pushInv.Name != "git" {
		t.Errorf("push command = %q, want git", pushInv.Name)
	}
	wantPushArgs := []string{"push", "--atomic", "origin", "HEAD", "v1.2.3"}
	if !equalArgs(pushInv.Args, wantPushArgs) {
		t.Errorf("push args = %v, want %v", pushInv.Args, wantPushArgs)
	}
}

func TestReleaser_TagAndPush_PushSuccessSignalsPointOfNoReturn(t *testing.T) {
	t.Parallel()

	// A successful atomic push is the point of no return: commits + tag are public.
	// The outcome must signal PONR crossed so the orchestrator knows publish may
	// proceed and subsequent failures are warn-only.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	rel := release.NewReleaser(r)

	outcome, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err != nil {
		t.Fatalf("TagAndPush returned unexpected error: %v", err)
	}
	if !outcome.PointOfNoReturnCrossed {
		t.Error("outcome.PointOfNoReturnCrossed = false on push success, want true")
	}
}

func TestReleaser_TagAndPush_RejectedPush_SurfacesFailureAndDoesNotSignalPONR(t *testing.T) {
	t.Parallel()

	// A rejected/failed push (non-zero exit) must surface as a clear error and the
	// run stops. Because the push did NOT succeed this is still PRE-PONR, so the
	// outcome must NOT signal the point of no return — the orchestrator must not
	// proceed to publish. The unit never publishes; it only returns the failure.
	// The tag is created (first git call succeeds) and only the push (second git
	// call) is rejected, so the sequence scripts the two calls independently.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{}},
		runner.ScriptedCall{Result: runner.Result{Stderr: "! [rejected] HEAD -> main (fetch first)\n", ExitCode: 1}, Err: errors.New("exit status 1")},
	)

	rel := release.NewReleaser(r)

	outcome, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err == nil {
		t.Fatal("TagAndPush returned nil error on a rejected push, want the failure to surface")
	}
	if !errors.Is(err, release.ErrPushRejected) {
		t.Errorf("error = %v, want it to match ErrPushRejected so it is distinguishable from a pre-tag failure", err)
	}
	if outcome.PointOfNoReturnCrossed {
		t.Error("outcome.PointOfNoReturnCrossed = true on a rejected push, want false (push did not succeed, still pre-PONR)")
	}
}

func TestReleaser_TagAndPush_TagCreationFails_DistinguishableFromPushRejection(t *testing.T) {
	t.Parallel()

	// A failure creating the tag (before any push) must surface and stop, and it
	// must be distinguishable from a push rejection so the orchestrator knows the
	// push was never attempted. A pre-tag failure does NOT match ErrPushRejected.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: tag 'v0.0.1' already exists\n", ExitCode: 128}, errors.New("exit status 128"))

	rel := release.NewReleaser(r)

	outcome, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err == nil {
		t.Fatal("TagAndPush returned nil error when tag creation failed, want the failure to surface")
	}
	if errors.Is(err, release.ErrPushRejected) {
		t.Error("tag-creation failure matched ErrPushRejected; it must be distinguishable from a push rejection")
	}
	if outcome.PointOfNoReturnCrossed {
		t.Error("outcome.PointOfNoReturnCrossed = true when tag creation failed, want false")
	}

	// The push must never be attempted once tag creation fails.
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (tag attempt only, no push)", got)
	}
}

func TestReleaser_TagAndPush_GitMissing_DistinctFromPushRejection(t *testing.T) {
	t.Parallel()

	// A missing git binary (ErrCommandNotFound) is an infrastructure failure, not a
	// push rejection. It must surface, match ErrCommandNotFound, and must NOT be
	// mistaken for a ran-and-rejected push.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	rel := release.NewReleaser(r)

	outcome, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err == nil {
		t.Fatal("TagAndPush returned nil error when git is missing, want the failure to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
	if errors.Is(err, release.ErrPushRejected) {
		t.Error("a missing git binary matched ErrPushRejected; it must be distinguishable from a ran-and-rejected push")
	}
	if outcome.PointOfNoReturnCrossed {
		t.Error("outcome.PointOfNoReturnCrossed = true when git is missing, want false")
	}
}

func TestReleaser_TagAndPush_PushReportsCommandNotFound_NotTreatedAsRejection(t *testing.T) {
	t.Parallel()

	// The push step distinguishes a missing/unrunnable git (ErrCommandNotFound) from
	// a ran-and-rejected push: a not-found at the push must surface matching
	// ErrCommandNotFound and must NOT be wrapped as a rejection. The tag succeeds
	// first, then the push reports not-found.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{}},
		runner.ScriptedCall{Err: fmt.Errorf("running %q: %w", "git", runner.ErrCommandNotFound)},
	)

	rel := release.NewReleaser(r)

	outcome, err := rel.TagAndPush(t.Context(), "v0.0.1", "🌿", "Initial release.")
	if err == nil {
		t.Fatal("TagAndPush returned nil error when the push reported git missing, want the failure to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
	if errors.Is(err, release.ErrPushRejected) {
		t.Error("a not-found push matched ErrPushRejected; an infrastructure failure must not be treated as a rejection")
	}
	if outcome.PointOfNoReturnCrossed {
		t.Error("outcome.PointOfNoReturnCrossed = true when the push could not run, want false")
	}
}

// equalArgs reports whether two argument slices are element-for-element equal, so
// command-line assertions check the exact argv rather than a substring.
func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
