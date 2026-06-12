package commit

// This file holds commit's SHARED editor RESOLVER — the single git-faithful way the
// three "no AI message" fallback cases (--no-ai, AI-generation failure, oversized
// diff) and the gate's `e` action find which editor to open. It resolves ONLY; it
// never opens the editor, stages, or commits. Launching (and the missing-editor
// return-to-gate handling) is a downstream concern (tasks 3-2/3-3/3-4 and Phase 4).
//
// mint must open whatever `git commit` would open, honouring git's FULL precedence
// (GIT_EDITOR → core.editor → $VISUAL → $EDITOR → git's built-in default). Rather
// than hand-roll that chain — engine.ResolveEditor's $VISUAL → $EDITOR → vi order
// OMITS GIT_EDITOR/core.editor and is wrong for this contract — the resolver asks
// git itself: `git var GIT_EDITOR` returns exactly the editor git commit would
// launch, so mint INHERITS git's precedence instead of duplicating it.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/runner"
)

// ErrNoEditor is the distinguished signal ResolveEditor returns when git itself
// cannot resolve a usable editor — `git var GIT_EDITOR` fails or yields nothing
// (e.g. a dumb terminal with nothing configured). It means "no candidate in the
// chain is launchable", NOT "$EDITOR is unset" (an unset $EDITOR still resolves to
// git's built-in default). Downstream consumers — the fallback drop (3-2/3-5) and
// Phase 4's `e` action — branch on it via errors.Is to fail loud or graceful-degrade
// rather than attempt a launch.
var ErrNoEditor = errors.New("commit: no launchable editor resolved")

// ResolveEditor resolves which editor mint should open, following git's OWN
// resolution order, by interrogating `git var GIT_EDITOR` through the read-only
// CommandRunner. git var GIT_EDITOR returns exactly the editor `git commit` would
// launch, honouring the full precedence (GIT_EDITOR → core.editor → $VISUAL →
// $EDITOR → git's built-in default), so mint inherits git's chain rather than
// re-deriving it.
//
// The returned command line is git's value TRIMMED; it may carry arguments (e.g.
// "code --wait"). Splitting it into program + args is the launcher's concern
// downstream — the resolver returns the value as-is.
//
// An unset $EDITOR is NOT an error: git supplies its built-in default, so the call
// still succeeds. ErrNoEditor is returned ONLY when git cannot resolve a usable
// editor at all — `git var GIT_EDITOR` exits non-zero or returns a blank value (a
// dumb terminal with nothing configured). On that path mint returns the sentinel
// (matched via errors.Is), NOT a launch attempt and NOT a panic.
//
// The resolver is interrogation ONLY: it runs the single read-only `git var
// GIT_EDITOR` and interprets the result. It NEVER opens the editor, stages, or
// commits — all git access goes through the consumed CommandRunner.
func ResolveEditor(ctx context.Context, r runner.CommandRunner) (string, error) {
	res, err := r.Run(ctx, "git", "var", "GIT_EDITOR")
	if err != nil {
		// git could not resolve a usable editor (e.g. a dumb terminal with nothing
		// configured): a non-zero exit means no candidate in the chain is launchable.
		// Surface the distinguished signal with the cause preserved for diagnostics.
		return "", fmt.Errorf("%w: %v", ErrNoEditor, err)
	}

	editor := strings.TrimSpace(res.Stdout)
	if editor == "" {
		// git exited clean but named no editor — equivalently no launchable candidate.
		return "", ErrNoEditor
	}

	return editor, nil
}
