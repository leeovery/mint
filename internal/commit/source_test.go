package commit

// White-box tests for the structurally single-sourced per-mode git source COMMAND
// selection (source.go) shared by the preflight probes (preflight.go) and the L1 diff
// sources (generate.go). The two consumers share the source command but differ by ONE
// deliberate tail: the L1 diff applies diff_exclude, the preflight does NOT (it is
// diff_exclude-blind — it answers "would `git commit` create a commit?"). These assert
// that single-sourcing directly against the SINGLE shared builders — the preflight probe
// argv is provably the L1 source COMMAND minus the diff_exclude tail, plus `--numstat`
// (diff cases) / the shared `ls-files` prefix verbatim (untracked case) — and that the
// emptiness verdict and the L1 source dispatch agree per StagingMode through the shared
// sourcesForMode descriptor (AddAll reading both its sources).

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

// TestProbeArgv_IsL1SourceCommandMinusExcludesPlusNumstat proves each diff-mode preflight
// probe argv is EXACTLY the corresponding L1 source COMMAND with `--numstat` inserted
// after the diff verb/refspec and with NO diff_exclude tail, and the untracked probe is the
// shared ls-files prefix VERBATIM (no `--numstat`, no excludes) — all derived from the
// SINGLE shared base builders, so the preflight and the AI's L1 diff share the command and
// differ only by the exclusion tail (L1-only) that diff_exclude adds.
func TestProbeArgv_IsL1SourceCommandMinusExcludesPlusNumstat(t *testing.T) {
	t.Parallel()

	exclude := []string{"*.min.js"}

	// Staged (diff source): probe = command + `--numstat`, NO excludes; L1 = command +
	// `:(exclude)…`.
	if got, want := stagedProbeArgs(), []string{"diff", "--cached", "--numstat", "--", "."}; !argsEqual(got, want) {
		t.Errorf("stagedProbeArgs = %v, want the command + --numstat with NO excludes %v", got, want)
	}
	if got, want := sourceArgs(stagedBaseArgs(), exclude), []string{"diff", "--cached", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("staged L1 sourceArgs = %v, want %v", got, want)
	}

	// Tracked (diff source): probe = command + `--numstat`, NO excludes; L1 = command +
	// `:(exclude)…`.
	if got, want := trackedProbeArgs(), []string{"diff", "HEAD", "--numstat", "--", "."}; !argsEqual(got, want) {
		t.Errorf("trackedProbeArgs = %v, want the command + --numstat with NO excludes %v", got, want)
	}
	if got, want := sourceArgs(trackedBaseArgs(), exclude), []string{"diff", "HEAD", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("tracked L1 sourceArgs = %v, want %v", got, want)
	}

	// Untracked (ls-files source): the probe is the shared prefix VERBATIM (no
	// `--numstat`, no excludes); the L1 enumeration is that same prefix + `:(exclude)…`.
	if got, want := untrackedProbeArgs(), []string{"ls-files", "--others", "--exclude-standard", "-z", "--", "."}; !argsEqual(got, want) {
		t.Errorf("untrackedProbeArgs = %v, want the ls-files prefix verbatim with NO excludes %v", got, want)
	}
	if got, want := sourceArgs(untrackedBaseArgs(), exclude), []string{"ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude)*.min.js"}; !argsEqual(got, want) {
		t.Errorf("untracked L1 sourceArgs = %v, want the prefix + excludes %v", got, want)
	}
}

// TestProbeArgv_DerivesFromSharedBase proves the probe builders are derived from the SAME
// per-mode base prefix the L1 sourceArgs uses — the diff probe is the base with
// `--numstat` spliced after the refspec, and the untracked probe is the base verbatim.
// Asserting against the shared base (not a re-spelled literal) is what makes the
// single-sourcing structural.
func TestProbeArgv_DerivesFromSharedBase(t *testing.T) {
	t.Parallel()

	// The probe for a diff source is the base + `--numstat` and NO excludes (the
	// `--numstat` lands before the `-- .` selector, which the base carries as its tail).
	for _, tc := range []struct {
		name string
		base []string
		got  []string
	}{
		{"staged", stagedBaseArgs(), stagedProbeArgs()},
		{"tracked", trackedBaseArgs(), trackedProbeArgs()},
	} {
		// The diff probe must be the base with exactly one extra element, `--numstat`,
		// and the verb/refspec/selector must be the SAME elements (no re-spelling).
		want := append([]string{tc.base[0], tc.base[1], "--numstat"}, tc.base[2:]...)
		if !argsEqual(tc.got, want) {
			t.Errorf("%s probe = %v, want base+--numstat %v", tc.name, tc.got, want)
		}
	}

	// The untracked probe is the untracked base VERBATIM (no `--numstat`, no excludes).
	if got := untrackedProbeArgs(); !argsEqual(got, untrackedBaseArgs()) {
		t.Errorf("untracked probe = %v, want the untracked base verbatim %v", got, untrackedBaseArgs())
	}
}

// TestEmptinessVerdictAgreesWithL1Source_PerMode proves, per StagingMode, that the
// emptiness verdict (readDiffStat counting no file) and the L1 source (sourceDiff) DISPATCH the same
// per-mode command through the SHARED sourcesForMode descriptor — so for identical
// underlying source output (here with no excludes on either side) they agree, both empty
// or both non-empty, AddAll reading its tracked changes and then its untracked files. (Their
// ONE intended divergence — diff_exclude on the L1 side only — is not exercised here; it
// is covered by the all-excluded editor-fallback tests in staging_excluded_test.go.)
func TestEmptinessVerdictAgreesWithL1Source_PerMode(t *testing.T) {
	t.Parallel()

	const nonEmptyDiff = "diff --git a/x b/x\n+work\n"
	// A non-empty numstat probe output (one path) for the diff-source emptiness probes.
	const nonEmptyNames = "1\t0\tx\n"
	// A non-empty untracked enumeration (one NUL-terminated path — the -z format), then
	// its addition diff for the L1 AddAll path.
	const untrackedName = "new.go\x00"
	const untrackedAddition = "diff --git a/new.go b/new.go\n+content\n"

	for _, tc := range []struct {
		name      string
		mode      StagingMode
		wantEmpty bool
		// probeOut scripts the counting probes in order: one per spec, and a numstat per
		// untracked file the AddAll enumeration lists.
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
			// tracked probe empty, untracked probe empty (both are read).
			probeOut: []string{"", ""},
			// L1: tracked diff empty, untracked enumeration empty (no addition diffs).
			l1Out: []string{"", ""},
		},
		{
			name:      "addall tracked only",
			mode:      AddAll,
			wantEmpty: false,
			// tracked probe NON-empty, untracked probe empty.
			probeOut: []string{nonEmptyNames, ""},
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
			_, found, err := readDiffStat(ctx, rp, "", tc.mode)
			if err != nil {
				t.Fatalf("readDiffStat returned error: %v", err)
			}
			empty := !found
			if empty != tc.wantEmpty {
				t.Errorf("readDiffStat found %v, want empty %v", found, tc.wantEmpty)
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

			// The shared-dispatch agreement: for identical underlying source output (no
			// excludes on either side here), the emptiness verdict and the L1 source must
			// agree (both empty / both non-empty) for every mode through the shared
			// descriptor.
			if empty != l1Empty {
				t.Errorf("emptiness verdict (%v) and L1 source emptiness (%v) disagree for mode %v; the shared descriptor must keep them in lock-step (L1 diff = %q)", empty, l1Empty, tc.mode, diff)
			}
		})
	}
}

// TestProbeArgs_MatchesNamedBuilders closes the test-vs-production argv gap: production
// preflight routes through probeArgs(spec) while the named per-mode builders
// (stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs) are test-facing — this pins the
// two derivations equal for every spec, so an edit to one path that misses the other
// fails here rather than silently diverging the documented probe argv. Both are
// diff_exclude-blind, so the equality also guards that production preflight never picks up
// an exclusion tail.
func TestProbeArgs_MatchesNamedBuilders(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		spec sourceSpec
		want []string
	}{
		{"staged", sourceSpec{base: stagedBaseArgs(), kind: diffSource}, stagedProbeArgs()},
		{"tracked", sourceSpec{base: trackedBaseArgs(), kind: diffSource}, trackedProbeArgs()},
		{"untracked", sourceSpec{base: untrackedBaseArgs(), kind: untrackedListSource}, untrackedProbeArgs()},
	} {
		if got := probeArgs(tc.spec); !argsEqual(got, tc.want) {
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
