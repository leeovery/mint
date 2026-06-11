package engine

// This file is the single-version regenerate CHANGELOG write+commit (task 5-8): for a
// `--target changelog`/`both` regenerate of ONE version, write the regenerated body
// into CHANGELOG.md and stage at most ONE CHANGELOG commit.
//
// It REUSES the forward Phase 1/3 changelog writer (record.WriteChangelog) unchanged —
// the SAME idempotent in-place section-replace by version key (KaC format,
// newest-on-top, created-if-absent). There is NO second changelog mutator. The write
// replaces the target version's `## [x.y.z] - date` block in place; the surrounding
// sections keep their order, so a regenerate leaves exactly one section per version.
//
// COMMIT SUBJECT is regenerate-specific — `docs(changelog): regenerate notes for {tag}`
// (the canonical tag string) — NOT the forward `{commit_prefix} Release {tag}` subject,
// because nothing is being released. NO tag is ever cut and NO hook-artifact commit is
// made (hooks don't run on regenerate), so at most ONE commit lands.
//
// NO-OP SAFETY: when the in-place replace produces no net change (the regenerated body,
// under the same injected date, is byte-identical to what is already recorded),
// WriteChangelog reports Changed=false and this function makes NO commit — mint never
// makes an empty commit. The no-net-change signal is the SAME one the forward
// bookkeeping commit uses.
//
// This task owns only WRITING the changelog + STAGING/CREATING the single commit; it
// returns whether it committed so the single-version executor (task 5-9) can sequence
// the push, recovery, and provider write around it. The push and recovery are NOT done
// here.

import (
	"context"
	"fmt"
	"time"

	"mint/internal/git"
	"mint/internal/record"
)

// RegenerateChangelog writes the regenerated body for ONE version into CHANGELOG.md via
// the reused forward in-place section-replace and stages at most one CHANGELOG commit,
// returning whether a commit was made.
//
// root is the repo root (the changelog lives at root/CHANGELOG.md); versionKey is the
// bare x.y.z section key used in the `## [x.y.z] - date` header; tag is the canonical
// tag string used in the commit subject; date is the INJECTED section-header date
// (matching the forward writer's injected-date semantics, so re-recording an unchanged
// body+date is a true byte-for-byte no-op); body is the full regenerated notes body.
//
// The changelog write always runs with the writer enabled (the caller only invokes this
// for a changelog/both target, having already validated changelog != false up front).
// When the write nets a change, the changelog is staged in ONE `git -C {root} add
// CHANGELOG.md` and committed with subject `docs(changelog): regenerate notes for {tag}`
// through the lock-resilient Mutator; committed is true. When the write nets NO change,
// nothing is staged or committed and committed is false.
//
// A failed write or a failed `git add` is surfaced (so 5-9 can reset the local write);
// a failed stage short-circuits before the commit so it can never produce a commit.
func RegenerateChangelog(ctx context.Context, m *git.Mutator, root, versionKey, tag string, date time.Time, body string) (committed bool, err error) {
	writeResult, err := record.WriteChangelog(root, versionKey, date, body, true)
	if err != nil {
		return false, fmt.Errorf("regenerating changelog for %s: %w", tag, err)
	}
	if !writeResult.Changed {
		// No net change — no empty commit. The reused writer already left the file
		// byte-for-byte as it was.
		return false, nil
	}

	subject := fmt.Sprintf("docs(changelog): regenerate notes for %s", tag)
	if err := stageAndCommitChangelog(ctx, m, root, subject); err != nil {
		return false, fmt.Errorf("regenerating changelog for %s: %w", tag, err)
	}
	return true, nil
}
