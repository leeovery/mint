package notes_test

import (
	"context"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

const (
	answersV130 = "chore(release): release-notes path answers for v1.3.0"
	answersV120 = "chore(release): release-notes path answers for v1.2.0"
)

// forwardSubjectLists are the routes that list the commit subjects since lastTag.
var forwardSubjectLists = []struct {
	name string
	list func(context.Context, runner.CommandRunner, string) (string, error)
}{
	{name: "--no-ai", list: notes.NoAIBody},
	{name: "on_notes_failure fallback", list: func(ctx context.Context, r runner.CommandRunner, lastTag string) (string, error) {
		return notes.ResolveFailure(ctx, r, ai.ErrGenerationFailed, lastTag, config.Release{OnNotesFailure: "fallback"})
	}},
}

func TestCommitSubjectList_LeavesOutTheAnswersCommits(t *testing.T) {
	t.Parallel()

	for _, route := range forwardSubjectLists {
		for _, tc := range []struct {
			name string
			log  string
			want string
		}{
			{
				name: "one answers commit at the tip",
				log:  answersV130 + "\nfix: handle empty input\nfeat: add export\n",
				want: "fix: handle empty input\nfeat: add export\n",
			},
			{
				name: "answers commits for different tags",
				log:  answersV130 + "\nfeat: add export\n" + answersV120 + "\nfix: handle empty input\n",
				want: "feat: add export\nfix: handle empty input\n",
			},
			{
				name: "the last subject unterminated",
				log:  "feat: add export\n" + answersV130,
				want: "feat: add export\n",
			},
			{
				name: "the prefix not at the start of the subject",
				log:  "revert: " + answersV130 + "\nchore(release): pre-tag artifacts for v1.3.0\n",
				want: "revert: " + answersV130 + "\nchore(release): pre-tag artifacts for v1.3.0\n",
			},
		} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				body, err := route.list(t.Context(), seedCommitSubjects(t, tc.log), "v1.2.0")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if body != tc.want {
					t.Errorf("body = %q, want %q", body, tc.want)
				}
			})
		}
	}
}

func TestCommitSubjectList_OnlyAnswersCommitsGiveTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	for _, route := range forwardSubjectLists {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()

			body, err := route.list(t.Context(), seedCommitSubjects(t, answersV130+"\n"+answersV130+"\n"), "v1.2.0")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body != notes.StubBody() {
				t.Errorf("body = %q, want the maintenance-release body %q", body, notes.StubBody())
			}
		})
	}
}

func TestVersionNoAIBody_LeavesOutTheAnswersCommitAndKeepsThePreTagArtifactCommit(t *testing.T) {
	t.Parallel()

	const log = "🌿 Release v1.3.0\n" + answersV130 + "\nchore(release): pre-tag artifacts for v1.3.0\nfeat: add export\n"
	body, err := notes.VersionNoAIBody(t.Context(), seedCommitSubjects(t, log), "v1.2.0..v1.3.0", "v1.3.0")
	if err != nil {
		t.Fatalf("VersionNoAIBody returned unexpected error: %v", err)
	}
	if want := "chore(release): pre-tag artifacts for v1.3.0\nfeat: add export\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestVersionNoAIBody_TagOnTheAnswersCommitLeavesTheSubjectBeneathListed(t *testing.T) {
	t.Parallel()

	const log = answersV130 + "\ndocs: notes ahead of Release v1.3.0\nfeat: add export\n"
	body, err := notes.VersionNoAIBody(t.Context(), seedCommitSubjects(t, log), "v1.2.0..v1.3.0", "v1.3.0")
	if err != nil {
		t.Fatalf("VersionNoAIBody returned unexpected error: %v", err)
	}
	if want := "docs: notes ahead of Release v1.3.0\nfeat: add export\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestVersionNoAIBody_OnlyTheBookkeepingAndAnswersCommitsGiveTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	const log = "🌿 Release v1.3.0\n" + answersV130 + "\n"
	body, err := notes.VersionNoAIBody(t.Context(), seedCommitSubjects(t, log), "v1.2.0..v1.3.0", "v1.3.0")
	if err != nil {
		t.Fatalf("VersionNoAIBody returned unexpected error: %v", err)
	}
	if body != notes.StubBody() {
		t.Errorf("body = %q, want the maintenance-release body %q", body, notes.StubBody())
	}
}
