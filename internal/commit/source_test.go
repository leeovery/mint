package commit

// White-box tests for the structurally single-sourced per-mode git source selection
// (source.go) shared by the preflight probes (preflight.go) and the L1 diff sources
// (generate.go). These assert the "one source, cannot drift" invariant directly against
// the SINGLE shared builders — the preflight probe argv is provably the L1 source argv
// plus `--name-only` (diff cases) / the shared `ls-files` prefix (untracked case), and
// the emptiness verdict and the L1 source agree per StagingMode through the shared
// sourcesForMode descriptor (including the AddAll short-circuit).

import (
	"context"
	"testing"

	"mint/internal/runner"
)

// argsEqual reports whether two argv slices are element-for-element equal.
func argsEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestProbeArgv_IsL1SourceArgvPlusNameOnly proves each diff-mode preflight probe argv is
// EXACTLY the corresponding L1 source argv with `--name-only` inserted after the diff
// verb/refspec, and the untracked probe is the shared ls-files prefix VERBATIM (no
// `--name-only`) — all derived from the SINGLE shared base builders, so the preflight and
// the AI's L1 diff cannot drift.
func TestProbeArgv_IsL1SourceArgvPlusNameOnly(t *testing.T) {
	t.Parallel()

	exclude := []string{"*.min.js"}
	excludes := excludePathspecs(exclude)

	// Staged (diff source): L1 = `git diff --cached -- . :(exclude)…`; probe = same +
	// `--name-only`.
	if got, want := stagedProbeArgs(excludes), []string{"diff", "--cached", "--name-only", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("stagedProbeArgs = %v, want %v", got, want)
	}
	if got, want := sourceArgs(stagedBaseArgs(), exclude), []string{"diff", "--cached", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("staged L1 sourceArgs = %v, want %v", got, want)
	}

	// Tracked (diff source): L1 = `git diff HEAD -- . :(exclude)…`; probe = same +
	// `--name-only`.
	if got, want := trackedProbeArgs(excludes), []string{"diff", "HEAD", "--name-only", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("trackedProbeArgs = %v, want %v", got, want)
	}
	if got, want := sourceArgs(trackedBaseArgs(), exclude), []string{"diff", "HEAD", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("tracked L1 sourceArgs = %v, want %v", got, want)
	}

	// Untracked (ls-files source): the probe is the shared prefix VERBATIM (no
	// `--name-only`) — identical to the L1 enumeration argv.
	if got, want := untrackedProbeArgs(excludes), []string{"ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("untrackedProbeArgs = %v, want %v", got, want)
	}
	if got, want := sourceArgs(untrackedBaseArgs(), exclude), untrackedProbeArgs(excludes); !argsEqual(got, want) {
		t.Errorf("untracked probe %v != untracked L1 sourceArgs %v; the ls-files prefix must be shared verbatim", untrackedProbeArgs(excludes), got)
	}
}

// TestProbeArgv_DerivesFromSharedBase proves the probe builders are derived from the SAME
// per-mode base prefix the L1 sourceArgs uses — the diff probe is the base with
// `--name-only` spliced after the refspec, and the untracked probe is the base verbatim.
// Asserting against the shared base (not a re-spelled literal) is what makes the
// single-sourcing structural.
func TestProbeArgv_DerivesFromSharedBase(t *testing.T) {
	t.Parallel()

	// No excludes: the probe for a diff source is the base + `--name-only` (the
	// `--name-only` lands before the `-- .` selector, which the base carries as its tail).
	for _, tc := range []struct {
		name string
		base []string
		got  []string
	}{
		{"staged", stagedBaseArgs(), stagedProbeArgs(nil)},
		{"tracked", trackedBaseArgs(), trackedProbeArgs(nil)},
	} {
		// The diff probe must be the base with exactly one extra element, `--name-only`,
		// and the verb/refspec/selector must be the SAME elements (no re-spelling).
		want := append([]string{tc.base[0], tc.base[1], "--name-only"}, tc.base[2:]...)
		if !argsEqual(tc.got, want) {
			t.Errorf("%s probe = %v, want base+--name-only %v", tc.name, tc.got, want)
		}
	}

	// The untracked probe is the untracked base VERBATIM (no `--name-only`).
	if got := untrackedProbeArgs(nil); !argsEqual(got, untrackedBaseArgs()) {
		t.Errorf("untracked probe = %v, want the untracked base verbatim %v", got, untrackedBaseArgs())
	}
}

// TestEmptinessVerdictAgreesWithL1Source_PerMode proves, per StagingMode, that the
// emptiness verdict (wouldStageNothing) and the L1 source (sourceDiff) agree — both empty
// or both non-empty — driven through the SHARED sourcesForMode descriptor, including the
// AddAll tracked-then-untracked short-circuit.
func TestEmptinessVerdictAgreesWithL1Source_PerMode(t *testing.T) {
	t.Parallel()

	const nonEmptyDiff = "diff --git a/x b/x\n+work\n"
	// A non-empty name-only probe output (one path) for the diff-source emptiness probes.
	const nonEmptyNames = "x\n"
	// A non-empty untracked enumeration (one NUL-terminated path — the -z format), then
	// its addition diff for the L1 AddAll path.
	const untrackedName = "new.go\x00"
	const untrackedAddition = "diff --git a/new.go b/new.go\n+content\n"

	for _, tc := range []struct {
		name      string
		mode      StagingMode
		wantEmpty bool
		// probeOut scripts the emptiness probes in order (one per spec, plus the AddAll
		// short-circuit semantics handled by wouldStageNothing).
		probeOut []string
		// l1Out scripts the L1 source reads in order for sourceDiff.
		l1Out []string
	}{
		{
			name:      "staged empty",
			mode:      StagedOnly,
			wantEmpty: true,
			probeOut:  []string{""},
			l1Out:     []string{""},
		},
		{
			name:      "staged non-empty",
			mode:      StagedOnly,
			wantEmpty: false,
			probeOut:  []string{nonEmptyNames},
			l1Out:     []string{nonEmptyDiff},
		},
		{
			name:      "all empty",
			mode:      All,
			wantEmpty: true,
			probeOut:  []string{""},
			l1Out:     []string{""},
		},
		{
			name:      "all non-empty",
			mode:      All,
			wantEmpty: false,
			probeOut:  []string{nonEmptyNames},
			l1Out:     []string{nonEmptyDiff},
		},
		{
			name:      "addall both empty",
			mode:      AddAll,
			wantEmpty: true,
			// tracked probe empty, untracked probe empty (the short-circuit reads both).
			probeOut: []string{"", ""},
			// L1: tracked diff empty, untracked enumeration empty (no addition diffs).
			l1Out: []string{"", ""},
		},
		{
			name:      "addall tracked short-circuits",
			mode:      AddAll,
			wantEmpty: false,
			// tracked probe NON-empty → the untracked probe is never read (short-circuit).
			probeOut: []string{nonEmptyNames},
			// L1: tracked diff non-empty, untracked enumeration empty.
			l1Out: []string{nonEmptyDiff, ""},
		},
		{
			name:      "addall untracked only",
			mode:      AddAll,
			wantEmpty: false,
			// tracked probe empty, untracked probe NON-empty.
			probeOut: []string{"", untrackedName},
			// L1: tracked diff empty, untracked enumeration has one file, then its addition
			// diff.
			l1Out: []string{"", untrackedName, untrackedAddition},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			// Emptiness verdict via the preflight path.
			rp := runner.NewFakeRunner()
			rp.SeedSequence("git", scriptedResults(tc.probeOut)...)
			empty, err := wouldStageNothing(ctx, rp, "", tc.mode, nil)
			if err != nil {
				t.Fatalf("wouldStageNothing returned error: %v", err)
			}
			if empty != tc.wantEmpty {
				t.Errorf("wouldStageNothing empty = %v, want %v", empty, tc.wantEmpty)
			}

			// L1 source via the generate path (same shared descriptor).
			rl := runner.NewFakeRunner()
			rl.SeedSequence("git", scriptedResults(tc.l1Out)...)
			g := NewGenerator(rl, nil, "", tc.mode)
			diff, err := g.sourceDiff(ctx, nil)
			if err != nil {
				t.Fatalf("sourceDiff returned error: %v", err)
			}
			l1Empty := diff == ""

			// The load-bearing agreement: the emptiness verdict and the L1 source must
			// agree (both empty / both non-empty) for every mode through the shared
			// descriptor.
			if empty != l1Empty {
				t.Errorf("emptiness verdict (%v) and L1 source emptiness (%v) disagree for mode %v; the shared descriptor must keep them in lock-step (L1 diff = %q)", empty, l1Empty, tc.mode, diff)
			}
		})
	}
}

// TestProbeArgs_MatchesNamedBuilders closes the test-vs-production argv gap: production
// preflight routes through probeArgs(spec, …) while the named per-mode builders
// (stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs) are test-facing — this pins the
// two derivations equal for every spec, so an edit to one path that misses the other
// fails here rather than silently diverging the documented probe argv.
func TestProbeArgs_MatchesNamedBuilders(t *testing.T) {
	t.Parallel()

	exclude := []string{"*.min.js"}
	excludes := excludePathspecs(exclude)

	for _, tc := range []struct {
		name string
		spec sourceSpec
		want []string
	}{
		{"staged", sourceSpec{base: stagedBaseArgs(), kind: diffSource}, stagedProbeArgs(excludes)},
		{"tracked", sourceSpec{base: trackedBaseArgs(), kind: diffSource}, trackedProbeArgs(excludes)},
		{"untracked", sourceSpec{base: untrackedBaseArgs(), kind: untrackedListSource}, untrackedProbeArgs(excludes)},
	} {
		if got := probeArgs(tc.spec, exclude); !argsEqual(got, tc.want) {
			t.Errorf("%s: probeArgs = %v, want the named builder's %v", tc.name, got, tc.want)
		}
	}
}

// TestGitOutputEmpty_SeparatorOnlyOutputCountsAsEmpty proves the shared emptiness probe
// treats output consisting only of separators — trailing newlines AND the `-z`
// enumeration's NUL terminators — as empty, so a degenerate lone-NUL probe result can
// never be mistaken for "something to commit".
func TestGitOutputEmpty_SeparatorOnlyOutputCountsAsEmpty(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		stdout string
		want   bool
	}{
		{"empty", "", true},
		{"newline only", "\n", true},
		{"lone NUL (empty -z enumeration)", "\x00", true},
		{"NULs and whitespace", "\x00\n\x00", true},
		{"real path NUL-terminated", "new.go\x00", false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{Stdout: tc.stdout}, nil)
			empty, err := gitOutputEmpty(context.Background(), r, "", "ls-files", "--others", "-z")
			if err != nil {
				t.Fatalf("gitOutputEmpty returned error: %v", err)
			}
			if empty != tc.want {
				t.Errorf("gitOutputEmpty(%q) = %v, want %v", tc.stdout, empty, tc.want)
			}
		})
	}
}

// scriptedResults maps a slice of stdout strings to the FakeRunner ScriptedCall sequence.
func scriptedResults(outs []string) []runner.ScriptedCall {
	calls := make([]runner.ScriptedCall, 0, len(outs))
	for _, out := range outs {
		calls = append(calls, runner.ScriptedCall{Result: runner.Result{Stdout: out}})
	}
	return calls
}
