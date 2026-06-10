package notes_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/notes"
	"mint/internal/runner"
)

// wantNameStatusArgs is the exact argv of the name-status git call BuildChangeMap
// must issue: `git diff --name-status {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'`.
// The :(exclude)CHANGELOG.md pathspec is load-bearing — the map is computed AFTER
// the built-in CHANGELOG.md exclusion, so excluded churn can never appear.
func wantNameStatusArgs(lastTag string) []string {
	return []string{"diff", "--name-status", lastTag + "..HEAD", "--", ".", ":(exclude)CHANGELOG.md"}
}

// wantNumstatArgs is the exact argv of the numstat git call: same range and same
// exclusion as name-status, differing only in the --numstat selector.
func wantNumstatArgs(lastTag string) []string {
	return []string{"diff", "--numstat", lastTag + "..HEAD", "--", ".", ":(exclude)CHANGELOG.md"}
}

// seedChangeMapGit scripts the two ordered git calls BuildChangeMap makes:
// name-status first, then numstat. The FakeRunner matches on command name only, so
// a SeedSequence is the seam for two distinct `git` calls returning DIFFERENT
// outputs in a fixed order.
func seedChangeMapGit(t *testing.T, nameStatus, numstat string) *runner.FakeRunner {
	t.Helper()
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: nameStatus}},
		runner.ScriptedCall{Result: runner.Result{Stdout: numstat}},
	)
	return r
}

// withExcludes appends one :(exclude)<glob> entry per configured diff_exclude glob,
// in config order, to a base argv that already ends with :(exclude)CHANGELOG.md —
// the same on-top-of-CHANGELOG.md layering BuildChangeMap applies to both git calls.
func withExcludes(base []string, globs ...string) []string {
	args := append([]string(nil), base...)
	for _, g := range globs {
		args = append(args, ":(exclude)"+g)
	}
	return args
}

// assertChangeMapInvocations fails unless exactly the two git calls were recorded,
// in order (name-status then numstat), each with its exact expected argv — the
// load-bearing assertion that both calls carry the :(exclude)CHANGELOG.md pathspec.
func assertChangeMapInvocations(t *testing.T, r *runner.FakeRunner, lastTag string) {
	t.Helper()
	assertChangeMapInvocationsWithExcludes(t, r, lastTag)
}

// assertChangeMapInvocationsWithExcludes is assertChangeMapInvocations parameterised
// by extra diff_exclude globs: BOTH git calls must carry :(exclude)CHANGELOG.md
// FOLLOWED BY one :(exclude)<glob> per configured glob, in order. With no globs it is
// exactly the Phase-2 assertion.
func assertChangeMapInvocationsWithExcludes(t *testing.T, r *runner.FakeRunner, lastTag string, globs ...string) {
	t.Helper()

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (name-status then numstat)", len(invs))
	}
	assertGitArgv(t, invs[0], withExcludes(wantNameStatusArgs(lastTag), globs...))
	assertGitArgv(t, invs[1], withExcludes(wantNumstatArgs(lastTag), globs...))
}

// assertGitArgv fails unless the invocation was a `git` call with the exact argv.
func assertGitArgv(t *testing.T, got runner.Invocation, want []string) {
	t.Helper()

	if got.Name != "git" {
		t.Errorf("command = %q, want %q", got.Name, "git")
	}
	if len(got.Args) != len(want) {
		t.Fatalf("args = %v, want %v", got.Args, want)
	}
	for i := range want {
		if got.Args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q (full argv %v)", i, got.Args[i], want[i], got.Args)
		}
	}
}

func TestAssembler_BuildChangeMap_NewPackageHeadlinesAboveLargerExistingArea(t *testing.T) {
	t.Parallel()

	// Structural novelty is PRIMARY: a brand-new auth/ package (every path added)
	// is the strongest headline signal and must lead the map — ABOVE a larger-
	// magnitude change in an existing area (api/, only modified, 400 churned lines
	// vs auth/'s 30). Novelty over magnitude in both ordering and emphasis.
	nameStatus := strings.Join([]string{
		"A\tauth/login.go",
		"A\tauth/session.go",
		"M\tapi/handler.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"20\t0\tauth/login.go",
		"10\t0\tauth/session.go",
		"300\t100\tapi/handler.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	novIdx := strings.Index(got, "auth/")
	if novIdx < 0 {
		t.Fatalf("change map does not headline the new auth/ package:\n%s", got)
	}
	// The novelty headline for auth/ must appear before the api/ magnitude line.
	apiIdx := strings.Index(got, "api/")
	if apiIdx < 0 {
		t.Fatalf("change map does not mention the api/ magnitude area:\n%s", got)
	}
	if novIdx > apiIdx {
		t.Errorf("new package auth/ (idx %d) must headline ABOVE the larger api/ area (idx %d):\n%s", novIdx, apiIdx, got)
	}
	assertChangeMapInvocations(t, r, "v1.0.0")
}

func TestAssembler_BuildChangeMap_ReportsRenamedAndRemovedPaths(t *testing.T) {
	t.Parallel()

	// Renames (R{score}\told\tnew) and removals (D) are structural changes and must
	// be reported in the novelty section. A rename renders old -> new; a removal
	// names the dropped path.
	nameStatus := strings.Join([]string{
		"R096\tinternal/old.go\tinternal/new.go",
		"D\tlegacy/deprecated.go",
		"M\tcore/run.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"0\t0\tinternal/old.go\tinternal/new.go",
		"0\t40\tlegacy/deprecated.go",
		"5\t5\tcore/run.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v2.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	if !strings.Contains(got, "internal/old.go -> internal/new.go") {
		t.Errorf("change map does not report the rename old -> new:\n%s", got)
	}
	if !strings.Contains(got, "legacy/deprecated.go") {
		t.Errorf("change map does not report the removed path:\n%s", got)
	}
	assertChangeMapInvocations(t, r, "v2.0.0")
}

func TestAssembler_BuildChangeMap_RanksPerAreaChurnAsSupportingMagnitude(t *testing.T) {
	t.Parallel()

	// Magnitude is SECONDARY supporting context: per-file churn rolls up to
	// directory/area granularity, ranked by total churn descending. A flat per-file
	// list is mush — rollup is the salience-preserving form. Here api/ (410) must
	// rank above util/ (12), which ranks above cmd/ (3).
	nameStatus := strings.Join([]string{
		"M\tapi/a.go",
		"M\tapi/b.go",
		"M\tutil/u.go",
		"M\tcmd/c.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"300\t10\tapi/a.go",
		"80\t20\tapi/b.go",
		"6\t6\tutil/u.go",
		"2\t1\tcmd/c.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v3.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	apiIdx := strings.Index(got, "api/")
	utilIdx := strings.Index(got, "util/")
	cmdIdx := strings.Index(got, "cmd/")
	if apiIdx < 0 || utilIdx < 0 || cmdIdx < 0 {
		t.Fatalf("change map missing one of the magnitude areas (api=%d util=%d cmd=%d):\n%s", apiIdx, utilIdx, cmdIdx, got)
	}
	if apiIdx >= utilIdx || utilIdx >= cmdIdx {
		t.Errorf("per-area churn not ranked descending (api=%d util=%d cmd=%d):\n%s", apiIdx, utilIdx, cmdIdx, got)
	}
	assertChangeMapInvocations(t, r, "v3.0.0")
}

func TestAssembler_BuildChangeMap_CallsOutSingleLargestFile(t *testing.T) {
	t.Parallel()

	// The SINGLE largest file by churn (added+removed) is called out individually as
	// a notable file. Here api/handler.go (410) is the largest and must be named.
	nameStatus := strings.Join([]string{
		"M\tapi/handler.go",
		"M\tutil/small.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"400\t10\tapi/handler.go",
		"1\t1\tutil/small.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v4.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	if !strings.Contains(got, "api/handler.go") {
		t.Errorf("change map does not call out the single largest file api/handler.go:\n%s", got)
	}
	// The largest-file callout must name the actual largest, not the small file.
	largestSection := got[strings.Index(got, "Largest"):]
	if strings.Contains(largestSection, "util/small.go") {
		t.Errorf("largest-file callout wrongly names the small file:\n%s", got)
	}
	assertChangeMapInvocations(t, r, "v4.0.0")
}

func TestAssembler_BuildChangeMap_CallsOutNewTopLevelEntries(t *testing.T) {
	t.Parallel()

	// New TOP-LEVEL entries (added paths with no directory segment — a new root file
	// or a new root directory) are called out individually as notable. Here README2.md
	// is a new top-level file and must be named.
	nameStatus := strings.Join([]string{
		"A\tREADME2.md",
		"M\tinternal/run.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"12\t0\tREADME2.md",
		"3\t1\tinternal/run.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v5.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	if !strings.Contains(got, "README2.md") {
		t.Errorf("change map does not call out the new top-level entry README2.md:\n%s", got)
	}
	assertChangeMapInvocations(t, r, "v5.0.0")
}

func TestAssembler_BuildChangeMap_AllInOneExistingArea_RollsUpNoFalseHeadline(t *testing.T) {
	t.Parallel()

	// When EVERY change is a modification within ONE existing area (no A/D/R, the
	// area is merely touched), there must be NO false novelty headline — only the
	// magnitude rollup for that area. The novelty heuristic (a dir is novel only when
	// all its paths are added) must NOT fire here.
	nameStatus := strings.Join([]string{
		"M\tapi/a.go",
		"M\tapi/b.go",
		"M\tapi/c.go",
	}, "\n") + "\n"
	numstat := strings.Join([]string{
		"40\t10\tapi/a.go",
		"20\t5\tapi/b.go",
		"3\t3\tapi/c.go",
	}, "\n") + "\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v6.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	if strings.Contains(got, "New package") || strings.Contains(got, "New top-level") {
		t.Errorf("all-modifications-in-one-area produced a FALSE novelty headline:\n%s", got)
	}
	if !strings.Contains(got, "api/") {
		t.Errorf("change map does not roll up the api/ area magnitude:\n%s", got)
	}
	assertChangeMapInvocations(t, r, "v6.0.0")
}

func TestAssembler_BuildChangeMap_ComputedAfterChangelogExclusion(t *testing.T) {
	t.Parallel()

	// The map is computed AFTER the CHANGELOG.md exclusion: BOTH git calls must carry
	// the exact :(exclude)CHANGELOG.md pathspec so excluded churn never appears. This
	// is the load-bearing argv assertion (git, not Go, performs the exclusion).
	nameStatus := "M\tapi/a.go\n"
	numstat := "5\t5\tapi/a.go\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	if _, err := a.BuildChangeMap(t.Context(), "v7.0.0"); err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	assertChangeMapInvocations(t, r, "v7.0.0")
}

func TestAssembler_BuildChangeMap_ConfiguredGlob_AppliedToBothCallsOnTopOfChangelog(t *testing.T) {
	t.Parallel()

	// A configured diff_exclude glob rides on BOTH the name-status and numstat calls,
	// as a :(exclude)<glob> entry AFTER the built-in :(exclude)CHANGELOG.md — so the
	// map is computed over the SAME post-exclusion view the diff uses.
	nameStatus := "M\tapi/a.go\n"
	numstat := "5\t5\tapi/a.go\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{Globs: []string{"skills/**/knowledge.cjs"}})
	if _, err := a.BuildChangeMap(t.Context(), "v7.0.0"); err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	assertChangeMapInvocationsWithExcludes(t, r, "v7.0.0", "skills/**/knowledge.cjs")
}

func TestAssembler_BuildChangeMap_MultipleGlobs_AllAppliedToBothCallsInOrder(t *testing.T) {
	t.Parallel()

	// Multiple diff_exclude globs ALL apply to BOTH git calls, in config order, after
	// the built-in CHANGELOG.md exclusion — the map matches the diff's exclusion set.
	nameStatus := "M\tapi/a.go\n"
	numstat := "5\t5\tapi/a.go\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	globs := []string{"skills/**/knowledge.cjs", "*.min.js"}
	a := notes.NewAssembler(r, notes.ExcludeConfig{Globs: globs})
	if _, err := a.BuildChangeMap(t.Context(), "v8.0.0"); err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	assertChangeMapInvocationsWithExcludes(t, r, "v8.0.0", globs...)
}

func TestAssembler_BuildChangeMap_AbsentDiffExclude_ExcludesOnlyChangelog(t *testing.T) {
	t.Parallel()

	// With no configured globs (nil), BOTH calls reproduce EXACTLY the Phase-2 argv:
	// the only exclude is :(exclude)CHANGELOG.md, with no extra entries.
	nameStatus := "M\tapi/a.go\n"
	numstat := "5\t5\tapi/a.go\n"
	r := seedChangeMapGit(t, nameStatus, numstat)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	if _, err := a.BuildChangeMap(t.Context(), "v9.0.0"); err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}

	assertChangeMapInvocations(t, r, "v9.0.0")
}

func TestAssembler_BuildChangeMap_NameStatusGitFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A git that runs and exits non-zero on the name-status call (e.g. a bad range)
	// is a real error and must be surfaced, not swallowed into a degenerate map.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{
			Result: runner.Result{Stderr: "fatal: bad revision 'v9.9.9..HEAD'\n", ExitCode: 128},
			Err:    errors.New("exit status 128"),
		},
	)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v9.9.9")
	if err == nil {
		t.Fatalf("BuildChangeMap returned nil error, want the git failure surfaced")
	}
	if got != "" {
		t.Errorf("change map = %q, want empty on failure", got)
	}
}

func TestAssembler_BuildChangeMap_CommandNotFound_SurfacesDistinguishableError(t *testing.T) {
	t.Parallel()

	// A missing git binary is reported as a distinguishable condition matched with
	// errors.Is(runner.ErrCommandNotFound), mirroring AssembleDiff and the sibling
	// packages.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.BuildChangeMap(t.Context(), "v1.0.0")
	if err == nil {
		t.Fatalf("BuildChangeMap returned nil error, want a command-not-found condition")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match runner.ErrCommandNotFound", err)
	}
	if got != "" {
		t.Errorf("change map = %q, want empty on failure", got)
	}
}
