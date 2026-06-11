package engine_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins the nil-Publisher guard on the regenerate provider-write paths
// (task 10-2). A downgraded run — one whose provider could not be resolved (a
// non-github / no-remote origin) — carries a NIL Publisher through the regenerate
// write/dispatch. Before this guard, RegenerateWrite (single) and processOneVersion
// (batch) called ReleaseExists on that nil interface and PANICKED with a
// nil-pointer dereference. The guard skips the provider write (a warned downgrade,
// mirroring the forward engine.Release behaviour) instead of dereferencing nil.

// TestRegenerateWrite_NilPublisher_Release_SkipsProviderNoPanic proves a single-version
// --target release run with a NIL publisher (a downgraded provider) skips the provider
// dispatch with a warn rather than dereferencing nil — no panic, no abort.
func TestRegenerateWrite_NilPublisher_Release_SkipsProviderNoPanic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	// A nil Publisher models the downgraded (provider-unresolved) run.
	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, reuseWriteReq(engine.RegenerateTargetRelease))
	if err != nil {
		t.Fatalf("RegenerateWrite with a nil publisher returned %v, want nil (warned downgrade, provider skipped)", err)
	}

	// The downgrade is announced.
	if !hasWarn(rec) {
		t.Errorf("no Warn emitted for the provider-skipped downgrade; kinds = %v", rec.Kinds())
	}
}

// TestRegenerateWrite_NilPublisher_Both_StillWritesChangelogSkipsProvider proves a
// --target both run with a NIL publisher still commits + pushes the changelog (its
// resolvable surface) and SKIPS the provider write with a warn — never dereferencing nil
// post-push.
func TestRegenerateWrite_NilPublisher_Both_StillWritesChangelogSkipsProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // add
		ScriptedOut(""),                   // commit
		ScriptedOut(""),                   // push origin HEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetBoth))
	if err != nil {
		t.Fatalf("RegenerateWrite (both, nil publisher) returned %v, want nil", err)
	}

	// The changelog surface still wrote + pushed.
	if !invokedWith(f, "git", "push", "origin", "HEAD") {
		t.Errorf("changelog was not pushed; --target both must still write its resolvable surface; got %v", commandLines(f.Invocations()))
	}
	// The provider was skipped with a warn (no panic on the nil interface).
	if !hasWarn(rec) {
		t.Errorf("no Warn emitted for the provider-skipped downgrade; kinds = %v", rec.Kinds())
	}
}

// TestRegenerateAll_NilPublisher_SkipsProviderNoPanic proves the batch loop with a NIL
// publisher and a provider-writing target skips every per-version provider dispatch with
// a warn rather than dereferencing nil — no panic, and every version is still counted
// (none is skipped as a per-version FAILURE).
func TestRegenerateAll_NilPublisher_SkipsProviderNoPanic(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedReuseGit(f)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}}

	collected, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), nil,
		batchReq(engine.RegenerateSourceReuse, threeVersions(), false))
	if err != nil {
		t.Fatalf("RegenerateAll with a nil publisher returned %v, want nil (warned downgrade, provider skipped)", err)
	}

	// All three versions are processed (the provider skip is a downgrade, not a
	// per-version failure that drops the version).
	if len(collected) != 3 {
		t.Errorf("collected %d versions, want 3 (provider skip downgrades, never drops a version)", len(collected))
	}
	if !hasWarn(rec) {
		t.Errorf("no Warn emitted for the provider-skipped downgrade; kinds = %v", rec.Kinds())
	}
}

// TestResolvePublisher_Downgrade_WarnsReturnsNilNoError proves the shared
// engine.ResolvePublisher helper mirrors the forward engine.Release Stage-6 handling for
// an UNRESOLVED provider: it WARNS (the loud downgrade signal) and returns a nil
// Publisher with NO error — the caller proceeds downgraded rather than aborting.
func TestResolvePublisher_Downgrade_WarnsReturnsNilNoError(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	// `git remote get-url origin` fails → empty remote URL → ErrProviderUnresolved.
	f.Seed("git", runner.Result{ExitCode: 1}, errors.New("no origin remote"))
	rec := &presentertest.RecordingPresenter{}

	publisher, err := engine.ResolvePublisher(t.Context(), regenWriteDeps(rec, f), noProviderConfig())
	if err != nil {
		t.Fatalf("ResolvePublisher returned %v, want nil (an unresolved provider is a warned downgrade, not an abort)", err)
	}
	if publisher != nil {
		t.Errorf("ResolvePublisher returned a non-nil publisher %v on an unresolved provider; want nil (downgrade)", publisher)
	}
	if !hasWarn(rec) {
		t.Errorf("no downgrade Warn emitted; kinds = %v", rec.Kinds())
	}
	// The warn names the downgrade, not a failure.
	if got := downgradeWarnLabels(rec); !slices.Contains(got, "publish skipped") {
		t.Errorf("downgrade warn labels = %v, want one to be %q", got, "publish skipped")
	}
}

// downgradeWarnLabels returns the Label of every Warn the recorder captured.
func downgradeWarnLabels(rec *presentertest.RecordingPresenter) []string {
	var labels []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			labels = append(labels, ev.Warn.Label)
		}
	}
	return labels
}

// noProviderConfig returns a config with no provider override and the changelog enabled —
// the minimal config the publisher resolver consumes.
func noProviderConfig() config.Config {
	return config.Config{}
}
