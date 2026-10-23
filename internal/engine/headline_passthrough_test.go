package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/publish"
	"mint/internal/runner"
	"mint/internal/version"
)

// headlineBody is an AI body in the default format with a headline: one plain
// sentence, one blank line, then the sections.
const headlineBody = "Sign-in sessions now survive a token refresh, with a new auth package behind them.\n\n" +
	"✨ Added\n" +
	"- An auth package with login and session handling.\n\n" +
	"🐛 Fixed\n" +
	"- Token refresh no longer drops the session.\n"

const (
	operatorHeadlineBody   = "Hand-written: the export command lands.\n\n✨ Added\n- Export to CSV.\n"
	operatorNoHeadlineBody = "🐛 Fixed\n- Export no longer drops the last row.\n"
)

// providerWrite is one `gh release create|edit` call: its command line and the
// body piped on its stdin.
type providerWrite struct {
	argv string
	body string
}

func providerWrites(f *runner.FakeRunner) []providerWrite {
	var writes []providerWrite
	for _, inv := range invocationsNamed(f, "gh") {
		if len(inv.Args) > 1 && inv.Args[0] == "release" && (inv.Args[1] == "create" || inv.Args[1] == "edit") {
			writes = append(writes, providerWrite{argv: commandLine(inv), body: inv.Stdin})
		}
	}
	return writes
}

// bareTagWrite is the provider write that publishes body under tag with the bare
// tag as its title.
func bareTagWrite(verb, tag, body string) providerWrite {
	return providerWrite{argv: "gh release " + verb + " " + tag + " --title " + tag + " --notes-file - --verify-tag", body: body}
}

func assertProviderWrites(t *testing.T, f *runner.FakeRunner, want ...providerWrite) {
	t.Helper()
	if got := providerWrites(f); !slices.Equal(got, want) {
		t.Errorf("provider writes = %q, want %q", got, want)
	}
}

// assertShippedEverywhere checks body reached the tag annotation, the CHANGELOG
// section and the provider release byte for byte, the release titled with the bare tag.
func assertShippedEverywhere(t *testing.T, f *runner.FakeRunner, root, tag, versionKey, body string) {
	t.Helper()
	if got := tagAnnotationBody(t, f, tag); got != terminatedAnnotationBody(body) {
		t.Errorf("tag annotation body = %q, want %q", got, terminatedAnnotationBody(body))
	}
	if got := changelogSectionBody(t, root, versionKey); got != body {
		t.Errorf("CHANGELOG body = %q, want %q", got, body)
	}
	assertProviderWrites(t, f, bareTagWrite("create", tag, body))
}

func TestRelease_HeadlineLedAIBody_ShipsByteIdenticalOnEverySurfaceUnderTheBareTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: headlineBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertShippedEverywhere(t, f, root, nextTag, "1.2.4", headlineBody)
}

// shippedRelease is a forward release that ran to the end, with what its surfaces
// are read back from.
type shippedRelease struct {
	f          *runner.FakeRunner
	root       string
	tag        string
	versionKey string
}

func releaseOrFail(t *testing.T, deps engine.ReleaseDeps, opts engine.ReleaseOptions) {
	t.Helper()
	if err := engine.Release(t.Context(), deps, opts); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
}

// priorTagFailedNotesRelease is a prior-tag release whose one notes call fails,
// scripted through to the push; tail scripts what comes between the failure and the
// bookkeeping commit.
func priorTagFailedNotesRelease(t *testing.T, config string, tail func(f *runner.FakeRunner, rec *presentertest.RecordingPresenter, deps *engine.ReleaseDeps)) shippedRelease {
	t.Helper()
	root := t.TempDir()
	if config != "" {
		writeConfig(t, root, config)
	}
	f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	tail(f, rec, &deps)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	releaseOrFail(t, deps, priorTagNormalAIOptions())
	return shippedRelease{f: f, root: root, tag: nextTag, versionKey: "1.2.4"}
}

func TestRelease_BodiesThatSkipTheAI_ShipExactlyAsBuiltWithNothingAdded(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		run  func(t *testing.T) shippedRelease
		want string
	}{
		{
			name: "first release",
			want: "Initial release.",
			run: func(t *testing.T) shippedRelease {
				root := t.TempDir()
				f := runner.NewFakeRunner()
				seedHappyGit(f, root, "main", "v0.0.1")
				f.Seed("gh", runner.Result{}, nil)
				releaseOrFail(t, newDeps(&presentertest.RecordingPresenter{}, f), patchOptions())
				return shippedRelease{f: f, root: root, tag: "v0.0.1", versionKey: "0.0.1"}
			},
		},
		{
			name: "empty post-exclusion diff",
			want: "Maintenance release — no notable source changes",
			run: func(t *testing.T) shippedRelease {
				root := t.TempDir()
				f := runner.NewFakeRunner()
				seedPriorTagReadGates(f, root, "main")
				f.SeedSequence("git", ScriptedOut("")) // diff v1.2.3..HEAD
				seedRecordTagPush(f, root)
				f.Seed("gh", runner.Result{}, nil)
				releaseOrFail(t, newDeps(&presentertest.RecordingPresenter{}, f), priorTagNormalAIOptions())
				return shippedRelease{f: f, root: root, tag: nextTag, versionKey: "1.2.4"}
			},
		},
		{
			name: "--no-ai",
			want: commitSubjects,
			run: func(t *testing.T) shippedRelease {
				root := t.TempDir()
				f := runner.NewFakeRunner()
				seedPriorTagReadGates(f, root, "main")
				f.SeedSequence("git",
					ScriptedOut(priorTagDiff),   // diff v1.2.3..HEAD
					ScriptedOut(commitSubjects), // log --format=%s v1.2.3..HEAD
				)
				seedRecordTagPush(f, root)
				f.Seed("gh", runner.Result{}, nil)
				opts := priorTagNormalAIOptions()
				opts.NoAI = true
				releaseOrFail(t, newDeps(&presentertest.RecordingPresenter{}, f), opts)
				return shippedRelease{f: f, root: root, tag: nextTag, versionKey: "1.2.4"}
			},
		},
		{
			name: "AI failure under on_notes_failure = fallback",
			want: commitSubjects,
			run: func(t *testing.T) shippedRelease {
				return priorTagFailedNotesRelease(t, fallbackConfig, func(f *runner.FakeRunner, _ *presentertest.RecordingPresenter, deps *engine.ReleaseDeps) {
					f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s v1.2.3..HEAD
					deps.Yes = true
				})
			},
		},
		{
			name: "commit subjects chosen at the notes floor",
			want: commitSubjects,
			run: func(t *testing.T) shippedRelease {
				return priorTagFailedNotesRelease(t, "", func(f *runner.FakeRunner, rec *presentertest.RecordingPresenter, _ *engine.ReleaseDeps) {
					f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s v1.2.3..HEAD
					rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceCommitSubjects, presenter.ChoiceYes}
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			shipped := tc.run(t)

			assertShippedEverywhere(t, shipped.f, shipped.root, shipped.tag, shipped.versionKey, tc.want)
		})
	}
}

func TestRelease_OperatorWrittenBodies_ShipByteForByteWithAHeadlineOnlyWhereWritten(t *testing.T) {
	t.Parallel()

	for _, body := range []struct {
		name string
		text string
	}{
		{name: "with a headline", text: operatorHeadlineBody},
		{name: "without a headline", text: operatorNoHeadlineBody},
	} {
		t.Run("--notes-file "+body.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeNotesFile(t, t.TempDir(), "notes.md", body.text)
			f := runner.NewFakeRunner()
			seedPriorTagNotesFileGit(f, root)

			releaseOrFail(t, newDeps(&presentertest.RecordingPresenter{}, f), notesFileOptions(path))

			assertShippedEverywhere(t, f, root, nextTag, "1.2.4", body.text)
		})
		t.Run("written in the editor at the notes floor "+body.name, func(t *testing.T) {
			t.Parallel()

			shipped := priorTagFailedNotesRelease(t, "", func(_ *runner.FakeRunner, rec *presentertest.RecordingPresenter, deps *engine.ReleaseDeps) {
				deps.Editor = &fakeEditor{edited: body.text}
				rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceYes}
			})

			assertShippedEverywhere(t, shipped.f, shipped.root, shipped.tag, shipped.versionKey, body.text)
		})
	}
}

// providerRelease is a provider whose release at every tag either exists already
// (regenerate edits it) or is absent (regenerate creates it).
type providerRelease struct {
	name   string
	verb   string
	runner func() *runner.FakeRunner
}

func providerReleases() []providerRelease {
	return []providerRelease{
		{
			name: "existing release",
			verb: "edit",
			runner: func() *runner.FakeRunner {
				f := runner.NewFakeRunner()
				f.Seed("gh", runner.Result{}, nil) // release view finds it; release edit succeeds
				return f
			},
		},
		{
			name: "no release yet",
			verb: "create",
			runner: func() *runner.FakeRunner {
				f := runner.NewFakeRunner()
				f.SeedSequence("gh",
					runner.ScriptedCall{Result: runner.Result{Stderr: "release not found", ExitCode: 1}, Err: errors.New("exit status 1")},
					ScriptedOut(""), // release create
				)
				return f
			},
		},
	}
}

// regenerateRunDeps is the deps a regenerate to the release target runs on, its gh
// auth gate passing.
func regenerateRunDeps() engine.ReleaseDeps {
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	return regenWriteDeps(&presentertest.RecordingPresenter{}, f)
}

func TestRegenerateRun_FreshHeadlineLedAIBody_PublishesByteIdenticalUnderTheBareTag(t *testing.T) {
	t.Parallel()

	for _, provider := range providerReleases() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()

			notesGit := seedFreshGit(priorTagDiff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
			tr := &freshTransport{body: headlineBody}
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
			freshRunOf(&req, notesGit, tr, freshCfg(), version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
			pub := provider.runner()

			if err := engine.RegenerateRun(t.Context(), regenerateRunDeps(), publish.NewGitHubPublisher(pub), t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateRun returned unexpected error: %v", err)
			}

			if tr.calls() != 1 {
				t.Fatalf("notes calls = %d, want 1", tr.calls())
			}
			assertProviderWrites(t, pub, bareTagWrite(provider.verb, regenRunTag, headlineBody))
		})
	}
}

func TestRegenerateRun_TagSourceHeadlineLedAnnotation_PublishesItByteIdenticalUnderTheBareTag(t *testing.T) {
	t.Parallel()

	for _, provider := range providerReleases() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()

			tagGit := runner.NewFakeRunner()
			seedForEachRef(tagGit, headlineBody)
			req := runReq(engine.SourceOf(engine.RegenerateSourceTag), engine.TargetOf(engine.RegenerateTargetRelease))
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
				return engine.RequireTagBody(ctx, tagGit, regenRunTag)
			}
			pub := provider.runner()

			if err := engine.RegenerateRun(t.Context(), regenerateRunDeps(), publish.NewGitHubPublisher(pub), t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateRun returned unexpected error: %v", err)
			}

			if !invokedWith(tagGit, "git", forEachRefArgs(regenRunTag)...) {
				t.Errorf("the annotation was not read; got %v", commandLines(tagGit.Invocations()))
			}
			assertProviderWrites(t, pub, bareTagWrite(provider.verb, regenRunTag, headlineBody))
		})
	}
}

// batchHeadlineBodies are the batch's versions with their headline-led bodies, oldest first.
var batchHeadlineBodies = []struct {
	res  version.Resolution
	body string
}{
	{res: version.Resolution{Tag: batchV2Tag, PreviousTag: batchV1Tag}, body: headlineBody},
	{res: version.Resolution{Tag: batchV3Tag, PreviousTag: batchV2Tag}, body: operatorHeadlineBody},
}

func batchHeadlineVersions() []version.Resolution {
	versions := make([]version.Resolution, len(batchHeadlineBodies))
	for i, v := range batchHeadlineBodies {
		versions[i] = v.res
	}
	return versions
}

func batchHeadlineWrites(verb string) []providerWrite {
	writes := make([]providerWrite, len(batchHeadlineBodies))
	for i, v := range batchHeadlineBodies {
		writes[i] = bareTagWrite(verb, v.res.Tag, v.body)
	}
	return writes
}

// regenerateAllToRelease runs an unattended `--all` batch to the release target over
// deps' runner and publishes through pub.
func regenerateAllToRelease(t *testing.T, deps engine.ReleaseDeps, pub *runner.FakeRunner, source engine.RegenerateSource, produce func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error)) {
	t.Helper()
	req := engine.BatchRegenerateRequest{
		Source:      source,
		Versions:    batchHeadlineVersions(),
		Project:     batchProject,
		TagPrefix:   "v",
		Yes:         true,
		Target:      engine.RegenerateTargetRelease,
		ProduceBody: produce,
	}
	collected, err := engine.RegenerateAll(t.Context(), deps, publish.NewGitHubPublisher(pub), t.TempDir(), req)
	if err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}
	if len(collected) != len(batchHeadlineBodies) {
		t.Fatalf("regenerated %d versions, want %d", len(collected), len(batchHeadlineBodies))
	}
}

// existingReleases is a provider that already has a release at every tag.
func existingReleases() *runner.FakeRunner {
	return providerReleases()[0].runner()
}

func TestRegenerateAll_FreshHeadlineLedAIBodies_PublishEachByteIdenticalUnderItsBareTag(t *testing.T) {
	t.Parallel()

	transports := map[string]*freshTransport{}
	for _, v := range batchHeadlineBodies {
		transports[v.res.Tag] = &freshTransport{body: v.body}
	}
	produce := func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		notesGit := seedFreshGit(priorTagDiff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
		return engine.RegenerateFreshBody(ctx, notesGit, transports[res.Tag], "", freshCfg(), res, sinks)
	}
	pub := existingReleases()

	regenerateAllToRelease(t, regenerateRunDeps(), pub, engine.RegenerateSourceFresh, produce)

	for tag, tr := range transports {
		if tr.calls() != 1 {
			t.Errorf("%s notes calls = %d, want 1", tag, tr.calls())
		}
	}
	assertProviderWrites(t, pub, batchHeadlineWrites("edit")...)
}

func TestRegenerateAll_TagSourceHeadlineLedAnnotations_PublishEachByteIdenticalUnderItsBareTag(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	var annotations []runner.ScriptedCall
	for _, v := range batchHeadlineBodies {
		annotations = append(annotations, ScriptedOut(v.body)) // for-each-ref refs/tags/<tag>
	}
	f.SeedSequence("git", annotations...)
	deps := regenWriteDeps(&presentertest.RecordingPresenter{}, f)
	produce := func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, preReadBody string, _ engine.NotesSinks) (string, error) {
		if preReadBody != "" {
			return preReadBody, nil
		}
		return engine.RequireTagBody(ctx, f, res.Tag)
	}
	pub := existingReleases()

	regenerateAllToRelease(t, deps, pub, engine.RegenerateSourceTag, produce)

	for _, v := range batchHeadlineBodies {
		if !invokedWith(f, "git", forEachRefArgs(v.res.Tag)...) {
			t.Errorf("the %s annotation was not read; got %v", v.res.Tag, commandLines(f.Invocations()))
		}
	}
	assertProviderWrites(t, pub, batchHeadlineWrites("edit")...)
}
