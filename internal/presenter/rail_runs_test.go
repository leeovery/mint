package presenter_test

import (
	"time"

	"mint/internal/presenter"
)

// The golden runs: whole runs as the engine's events carry them, pigeon going
// v1.3.2 → v1.4.0, with the wording of the design doc's screens (the rail package's
// own model frames) wherever a payload carries free text, so the frames show what
// the presenter makes of the payloads.

const (
	pigeonWhere     = "~/Code/pigeon"
	mintVersion     = "0.1.2"
	preflightDetail = "tree clean · on main · v1.4.0 free · in sync with origin"
	releaseURL      = "https://github.com/acme/pigeon/releases/tag/v1.4.0"
	savedPath       = "~/Library/Caches/mint/chunk-notes/Users-lee-Code-pigeon/v1.4.0.md"
	pigeonNotes     = "Webhooks retry on their own now, so a flaky endpoint no longer drops events.\n" +
		"\n" +
		"✨ Added\n" +
		"- Failed webhook deliveries retry with backoff for up to an hour.\n" +
		"- `pigeon replay <id>` resends any delivery from the last 30 days.\n" +
		"\n" +
		"🐛 Fixed\n" +
		"- The dashboard no longer shows a delivery twice after a reconnect.\n" +
		"- Signing secrets with a trailing newline are accepted again.\n"
)

func secs(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

func releaseStarted(dryRun bool) presenter.RunInfo {
	return presenter.RunInfo{
		Project: "pigeon", Version: "1.4.0", Action: "releasing", DryRun: dryRun,
		Header: presenter.Header{Command: "release", Where: pigeonWhere, Version: mintVersion},
		Start:  presenter.Station{Name: "v1.3.2", Detail: "main @ 7c1e0b4"},
	}
}

func versionGate() presenter.Gate {
	return presenter.Gate{
		Question: "Release v1.3.2 → v1.4.0 (minor)?", Subject: "version", AcceptEcho: "v1.4.0",
		Choices: []presenter.GateChoice{{Key: presenter.ChoiceYes, Action: "yes"}, {Key: presenter.ChoiceNo, Action: "no"}},
		Default: presenter.ChoiceYes, Stage: "version", Detail: "v1.4.0 (minor bump)",
	}
}

func releasePlan() presenter.Plan {
	return presenter.Plan{Quiet: true, PointOfNoReturn: "push", Stages: []presenter.PlanStage{
		{Key: "version", Name: "version", Detail: "v1.4.0 (minor bump)"},
		{Key: "preflight", Name: "preflight", Detail: preflightDetail},
		{Key: "pre_tag", Name: "pre_tag", Detail: "npm ci && npm run build"},
		{Key: "check", Name: "check", Detail: "files that don't belong in the notes", Inside: true},
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
		{Key: "record", Name: "record", Detail: "CHANGELOG.md · package.json"},
		{Key: "push", Name: "push", Detail: "main + v1.4.0 → origin, atomic"},
		{Key: "publish", Name: "publish", Detail: "GitHub release"},
		{Key: "post_release", Name: "post_release", Detail: "./scripts/notify.sh"},
	}}
}

func specsSuggestion() presenter.ExclusionSuggestion {
	return presenter.ExclusionSuggestion{Glob: "docs/specs/**", Reason: "design documents", Matches: "6 files, 48 kB"}
}

func pauseGate() presenter.Gate {
	s := specsSuggestion()
	return presenter.Gate{
		Question: "Exclude docs/specs/** from the notes? (design documents; 6 files, 48 kB)",
		Subject:  "docs/specs/**", AcceptEcho: "kept",
		Choices: []presenter.GateChoice{{Key: presenter.ChoiceExclude, Action: "exclude"}, {Key: presenter.ChoiceKeep, Action: "keep"}},
		Default: presenter.ChoiceKeep, Stage: "check", Suggestion: &s,
	}
}

func reviewGate() presenter.Gate {
	g := presenter.ReleaseReviewGate()
	g.Stage = "notes"
	return g
}

// cacheGate is the cached-notes question a release asks while its notes stage runs,
// a dry run before it having written notes for the same diff.
func cacheGate() presenter.Gate {
	return presenter.Gate{
		Question: "Cached notes found for this diff — use them?", Subject: "notes", AcceptEcho: "reused",
		Choices: []presenter.GateChoice{{Key: presenter.ChoiceYes, Action: "use cached"}, {Key: presenter.ChoiceRegen, Action: "regenerate"}},
		Default: presenter.ChoiceYes, Stage: "notes",
	}
}

func notesStarted() presenter.StageStart {
	return presenter.StageStart{Name: "notes", Key: "notes", Blocking: true, Text: "writing the notes…"}
}

// preparedRelease drives a release to its notes: the version asked, preflight, the
// plan, the pre_tag hook and its commit, and the exclusion check, done in 6.2s,
// having found checked.
func preparedRelease(p presenter.Presenter, clock *fakeClock, checked string) {
	p.RunStarted(releaseStarted(false))
	_, _ = p.Prompt(versionGate())
	p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Key: "preflight", Detail: preflightDetail})
	p.ShowPlan(releasePlan())
	p.StageStarted(presenter.StageStart{Name: "pre_tag", Key: "pre_tag", Blocking: true, Text: "running the pre_tag hook…"})
	clock.Advance(secs(4.1))
	p.StageSucceeded(presenter.StageSuccess{
		Name: "pre_tag", Key: "pre_tag", Blocking: true, Elapsed: secs(4.1),
		Detail: "npm ci && npm run build", Commit: "chore(release): pre-tag artifacts",
	})
	p.StageStarted(notesStarted())
	p.NotesChecking(presenter.NotesCheck{Key: "check"})
	clock.Advance(secs(6.2))
	p.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2), Detail: checked})
}

// paused answers the exclusion pause, its spinner suspended around it as the
// engine's is, and records the answers.
func paused(p presenter.Presenter) {
	p.SuspendSpinner()
	_, _ = p.Prompt(pauseGate())
	p.ResumeSpinner()
	p.SuspendSpinner()
	p.ExclusionAnswersRecorded(presenter.ExclusionAnswers{Stage: "notes", Key: "check", File: ".mint.toml", Table: "[release]", Exclude: []string{"docs/specs/**"}})
	p.ResumeSpinner()
}

// tour is a release through the exclusion pause and the review to its end station,
// its answers e at the pause and y at each other question. running is called with
// the notes written in parts, three of four done, 38.56s after they started.
func tour(p presenter.Presenter, clock *fakeClock, running func()) {
	preparedRelease(p, clock, "1 flagged")
	paused(p)
	for done := range 4 {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 412_000, Parts: 4, Done: done})
	}
	clock.Advance(secs(38.56) - secs(6.2))
	running()
	p.NotesProgress(presenter.NotesProgress{PromptBytes: 412_000, Parts: 4, Done: 4})
	p.NotesMerging(presenter.NotesMerge{Parts: 4})
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(52.3), Detail: "written in 4 parts and merged"})
	p.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{{Verb: "commit", Target: "bookkeeping"}, {Verb: "tag", Target: "v1.4.0"}}})
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: pigeonNotes})
	_, _ = p.Prompt(reviewGate())
	shipped(p)
}

// cachedRelease is a release whose notes a dry run before it wrote: the exclusion
// pause answered, the cached notes taken, asked while the notes stage runs with the
// spinner suspended around the question as the engine's is, and the review.
func cachedRelease(p presenter.Presenter, clock *fakeClock) {
	preparedRelease(p, clock, "1 flagged")
	paused(p)
	p.SuspendSpinner()
	_, _ = p.Prompt(cacheGate())
	p.ResumeSpinner()
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(6.6), Detail: "the dry run's, from the cache"})
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: pigeonNotes})
	_, _ = p.Prompt(reviewGate())
	shipped(p)
}

// shipped is a release from its notes accepted to its end station: recorded,
// pushed, published and its post_release hook run.
func shipped(p presenter.Presenter) {
	p.StageSucceeded(presenter.StageSuccess{Name: "record", Key: "record", Detail: "CHANGELOG.md · package.json → 🌿 Release v1.4.0"})
	p.StageStarted(presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing to origin…"})
	p.StageSucceeded(presenter.StageSuccess{Name: "push", Key: "push", Blocking: true, Elapsed: secs(1.9), Detail: "main + v1.4.0 → origin, atomic"})
	p.StageSucceeded(presenter.StageSuccess{Name: "publish", Key: "publish", Blocking: true, Elapsed: secs(1.3), Detail: "GitHub release"})
	p.StageStarted(presenter.StageStart{Name: "post_release", Key: "post_release", Blocking: true, Text: "running the post_release hook…"})
	p.StageSucceeded(presenter.StageSuccess{Name: "post_release", Key: "post_release", Blocking: true, Elapsed: secs(0.8), Detail: "./scripts/notify.sh"})
	p.RunFinished(presenter.RunResult{
		Project: "pigeon", Version: "1.4.0", URL: releaseURL, Outcome: presenter.OutcomeReleased,
		End: presenter.Station{Name: "v1.4.0", Detail: "released"}, Elapsed: secs(74),
	})
}

// failedRelease is release -y stopping on the notes, part 3 of 4 failing with the
// usage limit, and unwinding the pre_tag commit, a suggestion of the check's not
// asked about.
func failedRelease(p presenter.Presenter, clock *fakeClock) {
	preparedRelease(p, clock, "1 flagged, not asked")
	for done := range 3 {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 412_000, Parts: 4, Done: done})
	}
	p.StageFailed(presenter.StageFailure{
		Name: "notes", Key: "notes", Message: "the AI call for part 3 of 4 failed", Elapsed: secs(41),
		Output:     "Claude usage limit reached. Your limit will reset at 4pm (Europe/London).\n",
		SavedNotes: savedPath,
	})
	p.NextMoves(presenter.NextMoves{Stage: "notes", Moves: []presenter.NextMove{
		{Lead: "Tidy the saved part notes, then release with them:", Command: "mint release -m --notes-file " + savedPath},
		{Lead: "Release with the commit-subject list:", Command: "mint release -m --no-ai"},
		{Lead: "Ship that list whenever the AI fails, under [release] in .mint.toml:", Command: `on_notes_failure = "fallback"`},
	}})
	s := specsSuggestion()
	s.Line = `diff_exclude = ["docs/specs/**"]`
	p.ExclusionSuggestions(presenter.ExclusionSuggestions{Stage: "notes", Table: "[release]", File: ".mint.toml", Suggestions: []presenter.ExclusionSuggestion{s}})
	p.Unwound(presenter.Unwind{Undone: "pre-tag artifacts commit removed", Repo: "repo clean"})
	p.RunFinished(presenter.RunResult{
		Project: "pigeon", Version: "1.4.0", Outcome: presenter.OutcomeUnwound,
		End: presenter.Station{Name: "v1.3.2", Detail: "back where it started · nothing pushed"}, Elapsed: secs(47),
	})
}

// dryRun is release --dry-run: the version stated, not asked, the pre_tag hook
// skipped, the exclusion pause previewed, the notes shown, and nothing changed.
func dryRun(p presenter.Presenter, clock *fakeClock) {
	p.RunStarted(releaseStarted(true))
	p.StageSucceeded(presenter.StageSuccess{Name: "version", Key: "version", Detail: "v1.4.0 (minor bump)"})
	p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Key: "preflight", Detail: preflightDetail})
	p.ShowPlan(releasePlan())
	p.Warn(presenter.Warning{Label: "dry-run", Message: "skipping pre_tag hook"})
	p.StageStarted(notesStarted())
	p.NotesChecking(presenter.NotesCheck{Key: "check"})
	clock.Advance(secs(6.2))
	p.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2), Detail: "1 flagged, not asked"})
	p.SuspendSpinner()
	p.PreviewGate(pauseGate())
	p.ResumeSpinner()
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(52.3), Detail: "written in 4 parts and merged"})
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: pigeonNotes})
	p.RunFinished(presenter.RunResult{
		Project: "pigeon", Version: "1.4.0", Outcome: presenter.OutcomeDryRun,
		End: presenter.Station{Name: "v1.4.0", Detail: "dry run · nothing changed"}, Elapsed: secs(61),
	})
}

// committed is commit -Ap: everything staged once the message is accepted, then
// committed and pushed.
func committed(p presenter.Presenter, _ *fakeClock) {
	p.RunStarted(presenter.RunInfo{
		Project: "pigeon", Action: "committing",
		Header: presenter.Header{Command: "commit", Where: pigeonWhere, Version: mintVersion},
		Start:  presenter.Station{Name: "main", Detail: "7c1e0b4"},
	})
	p.ShowPlan(presenter.Plan{Quiet: true, Stages: []presenter.PlanStage{
		{Key: "diff", Name: "diff", Detail: "every change"},
		{Key: "message", Name: "message", Detail: "written by the AI"},
		{Key: "stage", Name: "stage", Detail: "everything"},
		{Key: "commit", Name: "commit", Detail: "on main"},
		{Key: "push", Name: "push", Detail: "to its upstream"},
	}})
	p.StageSucceeded(presenter.Settled("diff", "4 files · +86 −12 · everything, staged when you accept"))
	p.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing the commit message…"})
	p.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Blocking: true, Elapsed: secs(2.1), Detail: "written"})
	p.ShowMessage(presenter.Message{Title: "commit message", Body: "feat(webhooks): retry failed deliveries with backoff\n\nDeliveries that fail retry for up to an hour.\n"})
	g := presenter.NotesReviewGate()
	g.Question, g.Subject, g.Stage = "Commit this message?", "message", "message"
	_, _ = p.Prompt(g)
	p.StageSucceeded(presenter.Settled("stage", "4 files"))
	p.StageSucceeded(presenter.Settled("commit", "a3f9c21"))
	p.StageStarted(presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing…"})
	p.StageSucceeded(presenter.StageSuccess{Name: "push", Key: "push", Blocking: true, Elapsed: secs(1.2), Detail: "main → origin"})
	p.RunFinished(presenter.RunResult{
		Project: "pigeon", Outcome: presenter.OutcomeCommitted,
		End: presenter.Station{Name: "main", Detail: "a3f9c21 · pushed"}, Elapsed: secs(4.8),
	})
}

// initialised is init: a row per file, created or left alone.
func initialised(p presenter.Presenter, _ *fakeClock) {
	p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".mint.toml"})
	p.InitResult(presenter.InitOutcome{Action: presenter.InitSkipped, Target: "release", Reason: "already there · --force overwrites it"})
	p.InitResult(presenter.InitOutcome{Action: presenter.InitCreated, Target: ".github/workflows/release.yml"})
}

func versioned(p presenter.Presenter, _ *fakeClock) {
	p.ShowVersion(presenter.Version{Value: mintVersion})
}
