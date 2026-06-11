package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
)

// TestStagingMode_ZeroValueIsStagedOnly proves the StagingMode zero value is
// StagedOnly — the Phase 1 default, so a freshly-constructed Deps (or any unset
// field) selects the staged-only behaviour with no explicit flag. This is the
// load-bearing enum contract: neither -a nor -A leaves the mode at StagedOnly.
func TestStagingMode_ZeroValueIsStagedOnly(t *testing.T) {
	t.Parallel()

	var mode commit.StagingMode
	if mode != commit.StagedOnly {
		t.Errorf("zero-value StagingMode = %v, want StagedOnly (the Phase 1 default)", mode)
	}
}

// TestStagingMode_DistinctValues proves the three staging modes are distinct
// values, so the orchestrator (and later Phase 2 tasks) can switch on them without
// two modes colliding.
func TestStagingMode_DistinctValues(t *testing.T) {
	t.Parallel()

	modes := []commit.StagingMode{commit.StagedOnly, commit.All, commit.AddAll}
	for i := range modes {
		for j := i + 1; j < len(modes); j++ {
			if modes[i] == modes[j] {
				t.Errorf("StagingMode values at %d and %d collide (%v); the three modes must be distinct", i, j, modes[i])
			}
		}
	}
}

// TestRun_ExplicitStagedOnly_ByteIdenticalToBare proves threading an EXPLICIT
// StagedOnly mode through Deps.Staging leaves the bare (Phase 1) thread unchanged:
// the same three git calls in the same order (preflight read, L1 diff read, commit
// mutation), the commit body verbatim, and NO `git add`. StagedOnly is the seam this
// task threads; the All/AddAll deferred-staging behaviour is Phase 2 (tasks 2-2/2-3),
// not built here.
func TestRun_ExplicitStagedOnly_ByteIdenticalToBare(t *testing.T) {
	t.Parallel()

	const message = "feat: explicit staged-only"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := gitInvocations(r)
	if len(gits) != 3 {
		t.Fatalf("git invocations = %d (%v), want exactly 3 (preflight + diff + commit) — StagedOnly is unchanged from Phase 1", len(gits), gits)
	}
	for _, inv := range gits {
		if len(inv.Args) > 0 && inv.Args[0] == "add" {
			t.Errorf("explicit StagedOnly ran `git add %v`; StagedOnly stages nothing", inv.Args)
		}
	}
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}
