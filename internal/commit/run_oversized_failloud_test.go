package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
)

// TestRun_Oversized_Unattended_NoNote_FailsLoud proves the UX fix for the unattended
// oversized path: an over-limit diff under -y or non-TTY stdin must NOT emit the
// "diff too large to summarise — opening editor" note (the editor will never open), and
// must still fail loud with the exact spec no-message-source message while mutating
// nothing (no editor launch, no `git add`, no `git commit`). The note may only fire when
// the fallback will genuinely open an editor.
func TestRun_Oversized_Unattended_NoNote_FailsLoud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		yes              bool
		stdinInteractive bool
	}{
		{name: "UnderYes", yes: true, stdinInteractive: true},
		{name: "NonTTYStdin", yes: false, stdinInteractive: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: seedAIPreflightOnly(), saved: "feat: should never be saved\n"}
			tr := scriptedTransport("must never be returned (L2 was skipped)")

			deps := editorDeps(rec, er, editorDepsOptions{
				Transport:           tr,
				Root:                oversizedRoot(t),
				Yes:                 tt.yes,
				NonInteractiveStdin: !tt.stdinInteractive,
			})
			err := commit.Run(context.Background(), deps)

			// Standard fail-loud contract: exact message, surfaced once, nothing mutated.
			assertFailLoudNoMutation(t, rec, er, err)

			// The UX fix: NO oversized "opening editor" note precedes the fail-loud, because
			// the editor will never open on an unattended run.
			for _, w := range warnEvents(rec) {
				if w.Message == oversizedNote {
					t.Errorf("unattended oversized run emitted the note %q; the editor never opens, so the note must be suppressed", oversizedNote)
				}
			}
		})
	}
}
