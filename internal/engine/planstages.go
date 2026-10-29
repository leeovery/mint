package engine

import (
	"strings"

	"mint/internal/config"
	"mint/internal/hooks"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/record"
)

// The plan stage keys: the stage events of a run carry these to fill their nodes.
const (
	stageVersion     = "version"
	stagePreflight   = "preflight"
	stagePreTag      = "pre_tag"
	stageCheck       = "check"
	stageNotes       = "notes"
	stageRecord      = "record"
	stagePush        = "push"
	stagePublish     = "publish"
	stagePostRelease = "post_release"
	// stageReview is a regenerate's notes review, a node of its own there.
	stageReview = "review"
)

// releasePlan is what a release's stages are laid out from, all known once preflight
// has passed.
type releasePlan struct {
	cfg config.Config
	tag string
	// branch is what the atomic push sends with the tag.
	branch string
	// version and preflight are the details those settled stages showed.
	version, preflight string
	// notes is the notes path the release will take.
	notes notesRoute
}

// stages lays out the release's stages, those already settled included, leaving
// out a hook stage with no hook configured, the check on a path that runs none, and
// publish when the release publishes nothing. The point of no return is the push.
func (r releasePlan) stages() presenter.Plan {
	rel := r.cfg.Release
	stages := []presenter.PlanStage{
		{Key: stageVersion, Name: stageVersion, Detail: r.version},
		{Key: stagePreflight, Name: stagePreflight, Detail: r.preflight},
	}
	if hooks.HasEntries(rel.Hooks.PreTag) {
		stages = append(stages, presenter.PlanStage{Key: stagePreTag, Name: stagePreTag, Detail: hookDetail(rel.Hooks.PreTag)})
	}
	if r.notes.checks {
		stages = append(stages, presenter.PlanStage{Key: stageCheck, Name: stageCheck, Detail: "files that don't belong in the notes"})
	}
	stages = append(stages,
		presenter.PlanStage{Key: stageNotes, Name: stageNotes, Detail: r.notes.detail},
		presenter.PlanStage{Key: stageRecord, Name: stageRecord, Detail: recordPlanDetail(rel)},
		presenter.PlanStage{Key: stagePush, Name: stagePush, Detail: r.branch + " + " + r.tag + " → origin, atomic"},
	)
	if rel.Publish {
		stages = append(stages, presenter.PlanStage{Key: stagePublish, Name: stagePublish, Detail: providerRelease(r.cfg)})
	}
	if hooks.HasEntries(rel.Hooks.PostRelease) {
		stages = append(stages, presenter.PlanStage{Key: stagePostRelease, Name: stagePostRelease, Detail: hookDetail(rel.Hooks.PostRelease)})
	}
	return presenter.Plan{Stages: stages, PointOfNoReturn: stagePush, Quiet: true}
}

// notesRoute is the notes path a release will take, as far as it is known before the
// notes stage: its plan detail, and whether the exclusion check runs on it. A diff
// that turns out empty, or a keep list covering every path, still skips the check.
type notesRoute struct {
	detail string
	checks bool
}

// releaseNotesRoute follows the notes precedence over what it is decided by before
// any diff is read; overridden is a test's injected body, which takes no path.
func releaseNotesRoute(state notes.SelectState, overridden bool) notesRoute {
	switch {
	case overridden:
		return notesRoute{}
	case state.Supplied != nil:
		return notesRoute{detail: "from the notes file"}
	case state.FirstRelease:
		return notesRoute{detail: "first release"}
	case state.NoAI:
		return notesRoute{detail: "the commit-subject list"}
	default:
		return notesRoute{detail: "written by the AI", checks: true}
	}
}

// hookDetail is a hook's commands as its node shows them, joined as they run: each
// only once the one before it succeeded.
func hookDetail(value config.HookValue) string {
	return strings.Join(hooks.Entries(value), " && ")
}

// recordPlanDetail names the files the record stage will write.
func recordPlanDetail(rel config.Release) string {
	var files []string
	if rel.Changelog {
		files = append(files, record.ChangelogFileName)
	}
	if rel.VersionFile != "" {
		files = append(files, rel.VersionFile)
	}
	if len(files) == 0 {
		return "nothing to record"
	}
	return strings.Join(files, " + ")
}

// providerRelease names the release a publish writes: GitHub's when the config
// names the provider, since the remote that otherwise decides it is read only once
// the release is recorded.
func providerRelease(cfg config.Config) string {
	if cfg.Release.Provider == "github" {
		return "GitHub release"
	}
	return aProviderRelease
}

// aProviderRelease names the release a publish writes when which provider's is not
// known.
const aProviderRelease = "provider release"
