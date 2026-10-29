package commit

import "mint/internal/presenter"

// commitCommand is the command words of the rail's header for a commit run.
const commitCommand = "commit"

// commitPlan lays out a commit run's stages, all known from its flags: the diff, the
// message, the deferred staging of -a/-A, the commit, and the -p push. A commit never
// unwinds, so the plan has no point of no return; it is Quiet, as plain has never
// printed a plan for a commit.
func commitPlan(deps Deps) presenter.Plan {
	message := "written by the AI"
	if deps.NoAI {
		message = "written in your editor"
	}
	stages := []presenter.PlanStage{
		{Key: stageDiff, Name: stageDiff, Detail: stagingDetail(deps.Staging)},
		{Key: stageMessage, Name: stageMessage, Detail: message},
	}
	if deps.Staging != StagedOnly {
		stages = append(stages, presenter.PlanStage{Key: stageStage, Name: stageStage, Detail: stagingCommand(deps.Staging)})
	}
	stages = append(stages, presenter.PlanStage{Key: stageCommit, Name: stageCommit, Detail: "on " + deps.branch})
	if deps.Push {
		stages = append(stages, presenter.PlanStage{Key: stagePush, Name: stagePush, Detail: "to its upstream"})
	}
	return presenter.Plan{Stages: stages, Quiet: true}
}

// stagingDetail is what the diff stage covers under mode, and when it is staged.
func stagingDetail(mode StagingMode) string {
	switch mode {
	case All:
		return "tracked changes, staged when you accept"
	case AddAll:
		return "everything, staged when you accept"
	default:
		return "staged changes"
	}
}

// stagingCommand is the git add the staging stage runs under mode.
func stagingCommand(mode StagingMode) string {
	if mode == AddAll {
		return "git add -A"
	}
	return "git add -u"
}
