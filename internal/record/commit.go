package record

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/git"
)

// CommitDirtyTree commits whatever a pre_tag hook left dirty as its OWN commit,
// kept distinct from the release-bookkeeping commit. It first probes the tree with
// `git -C {dir} status --porcelain` (the clean-tree convention: non-empty output is
// dirty — tracked changes and/or non-ignored untracked files; gitignored entries
// are omitted by porcelain, so they never count as dirty). When the tree is dirty it
// stages everything (`git -C {dir} add -A`) and commits it with subject (e.g.
// `chore(release): pre-tag artifacts for {tag}`), returning committed=true.
//
// When the tree is clean (empty porcelain) it commits NOTHING and returns
// committed=false. This single check naturally handles every no-commit case: a hook
// that built nothing, a hook that made its OWN commit and handed back a clean tree,
// and gitignored-only outputs. "Commit only if changed" falls out for free.
//
// A non-zero `git status` or `git add` exit short-circuits before the next step (so
// a failed probe or stage can never produce a commit) and the error is surfaced for
// the orchestrator to abort/unwind on. A failure once the stage ran is an
// *UncommittedError: what it staged stays staged. A commit that landed with the index
// left behind it is an *IndexBehindError, committed true.
//
// A keepOut path is left out of the probe, the stage and the commit: its changes
// neither trigger the commit nor ride in it, and any of them already staged stay
// staged (see CommitStaged). When that path's staged edits are all the index holds,
// no commit is made and committed is false.
func CommitDirtyTree(ctx context.Context, m *git.Mutator, dir, subject string, keepOut git.Exclusion) (bool, error) {
	// The status probe is a READ, so it goes through the Mutator's plain pass-through
	// (m.Run) — no lock logic. Only the add/commit mutations carry lock resilience.
	status, err := m.Run(ctx, "git", withExclusion(keepOut, "-C", dir, "status", "--porcelain")...)
	if err != nil {
		return false, fmt.Errorf("probing tree status in %s: %w", dir, err)
	}
	if strings.TrimSpace(status.Stdout) == "" {
		return false, nil
	}

	if _, err := m.Mutate(ctx, nil, "git", withExclusion(keepOut, "-C", dir, "add", "-A")...); err != nil {
		return false, fmt.Errorf("staging dirty tree in %s: %w", dir, err)
	}
	committed, err := CommitStaged(ctx, m, dir, subject, keepOut)
	if err != nil {
		return committed, committedAnyway(err, func(err error) error {
			return &UncommittedError{err: fmt.Errorf("committing dirty tree %q: %w", subject, err)}
		})
	}
	return committed, nil
}

// CommitBookkeeping builds the single release-bookkeeping commit, FOLDING the
// changelog change and the version-file projection into ONE commit with subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). Every git invocation targets dir (the repo root) via
// `git -C {dir}` so it is independent of the process working directory.
//
// changelogChanged and versionChanged are the net-change signals from
// WriteChangelog and ProjectVersionFile. Only what actually changed is staged: the
// changelog when changelogChanged, and versionFile when versionChanged (and a
// versionFile path is configured) — mint never `git add`s an untouched or absent
// path. Whatever changed is staged in ONE `git -C {dir} add {paths…}` invocation,
// so the version file is NEVER given its own separate commit — it is always folded
// here alongside the changelog. versionFile is named as git names it (RepoPath), so
// the commit's literal pathspec matches it.
//
// COMBINED NO-OP: when neither the changelog nor the version file changed there is
// nothing to stage, so CommitBookkeeping is a no-op and creates NO commit — mint
// never makes an empty commit. A non-zero `git add` exit short-circuits before the
// commit so a failed stage can never produce a commit; the error is surfaced for
// the orchestrator to unwind. A failure once the stage ran is an *UncommittedError:
// what it staged stays staged. A commit that landed with the index left behind it is
// an *IndexBehindError.
//
// This bookkeeping commit is kept DISTINCT from the pre_tag hook-artifact commit
// (CommitDirtyTree), which precedes it with its own chore subject.
//
// The commit takes the staged paths alone (CommitPaths), so whatever else the index
// holds, a notes file's edits or the user's own, neither rides in it nor loses its
// staged state.
func CommitBookkeeping(ctx context.Context, m *git.Mutator, dir, commitPrefix, tag, versionFile string, changelogChanged, versionChanged bool) error {
	// The no-op branch consumes the SAME shared predicate the engine spine consults
	// to fold the resulting commit into its tracked state, so the two can never desync.
	if !BookkeepingWillCommit(versionFile, changelogChanged, versionChanged) {
		return nil
	}

	paths := bookkeepingPaths(versionFile, changelogChanged, versionChanged)
	addArgs := append([]string{"-C", dir, "add"}, paths...)
	if _, err := m.Mutate(ctx, nil, "git", addArgs...); err != nil {
		return fmt.Errorf("staging %v: %w", paths, err)
	}

	subject := BookkeepingSubject(commitPrefix, tag)
	if err := CommitPaths(ctx, m, dir, subject, paths); err != nil {
		return committedAnyway(err, func(err error) error {
			return &UncommittedError{err: fmt.Errorf("committing release bookkeeping %q: %w", subject, err)}
		})
	}
	return nil
}

// UncommittedError is a commit that failed once its changes were staged, which it
// leaves staged. Its message is the failure's own.
type UncommittedError struct{ err error }

func (e *UncommittedError) Error() string { return e.err.Error() }

// Unwrap exposes the commit's failure so errors.Is and errors.As match it.
func (e *UncommittedError) Unwrap() error { return e.err }

// CommitFile stages path in dir by name and commits it alone with subject
// (CommitPaths). A failed step's error carries git's stderr as a *GitError, and a
// commit that landed with the index left behind it is an *IndexBehindError.
func CommitFile(ctx context.Context, m *git.Mutator, dir, path, subject string) error {
	if res, err := m.Mutate(ctx, nil, "git", "-C", dir, "add", path); err != nil {
		return fmt.Errorf("staging %s: %w", path, &GitError{Stderr: res.Stderr, err: err})
	}
	if err := CommitPaths(ctx, m, dir, subject, []string{path}); err != nil {
		return committedAnyway(err, func(err error) error { return fmt.Errorf("committing %q: %w", subject, err) })
	}
	return nil
}

// CommitPaths commits exactly paths in dir with subject, as `git commit` given
// pathspecs does: their content in the work tree and nothing else the index holds,
// which stays staged and uncommitted, so a change the user stages while mint runs
// never rides in mint's commit. The paths reach git on stdin (PathspecInput), so no
// count of them meets the argument limit. Each must be known to git already: a new
// file is staged first. On this pathspec route git runs the commit hooks against a
// temporary index, so what a hook re-stages, its rewrite of a path or a file of its
// own, rides in the commit while the real index keeps what it held before; the index
// is matched to the commit after it (matchIndexToCommit). A failed commit's error
// carries git's stderr as a *GitError; once the commit landed, a failure to match the
// index is an *IndexBehindError.
func CommitPaths(ctx context.Context, m *git.Mutator, dir, subject string, paths []string) error {
	if res, err := m.Mutate(ctx, PathspecInput(paths), "git", "-C", dir, "commit", "-m", subject, "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return NewGitError(res.Stderr, err)
	}
	return matchIndexToCommit(ctx, m, dir)
}

// matchIndexToCommit returns the index entry of every path HEAD's commit changed, the
// files its hooks added among them, to HEAD, touching no work tree, so the index holds
// what the commit holds and nothing the commit made reads as a change, then refreshes
// the index's stat data. Every other entry, a change the user staged, stays as it is.
func matchIndexToCommit(ctx context.Context, m *git.Mutator, dir string) error {
	res, err := m.Run(ctx, "git", "-C", dir, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "HEAD")
	if err != nil {
		return &IndexBehindError{Stderr: res.Stderr, err: err}
	}
	paths := strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 })
	if len(paths) == 0 {
		return nil
	}
	if res, err := m.Mutate(ctx, PathspecInput(paths), "git", "-C", dir, "restore", "--source=HEAD", "--staged", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return &IndexBehindError{Stderr: res.Stderr, err: err}
	}
	// The restored entries carry no stat data, which `reset --keep` reads as a change
	// and refuses over, so an unwind after the commit could not take it back.
	if res, err := m.Mutate(ctx, nil, "git", "-C", dir, "update-index", "-q", "--refresh"); err != nil {
		return &IndexBehindError{Stderr: res.Stderr, err: err}
	}
	return nil
}

// IndexBehindError is a commit that landed with the index left behind it: what the
// commit hooks staged rides in the commit, and the index still holds those paths as
// they were. The commit stands. Stderr is what git wrote.
type IndexBehindError struct {
	Stderr string
	err    error
}

func (e *IndexBehindError) Error() string {
	return "matching the index to the commit: " + e.err.Error()
}

// Unwrap exposes the runner error so errors.Is matches it.
func (e *IndexBehindError) Unwrap() error { return e.err }

// committedAnyway is err as a commit's caller reports it: an *IndexBehindError as it
// is, the commit having landed, and any other failure through uncommitted.
func committedAnyway(err error, uncommitted func(error) error) error {
	var behind *IndexBehindError
	if errors.As(err, &behind) {
		return behind
	}
	return uncommitted(err)
}

// PathspecInput is paths as git reads them under `--pathspec-from-file=-
// --pathspec-file-nul`: each a root-anchored literal pathspec ended by a NUL. The magic
// goes on each entry rather than `--literal-pathspecs`, which would export
// GIT_LITERAL_PATHSPECS to the commit hooks and break their globs.
func PathspecInput(paths []string) []byte {
	var b strings.Builder
	for _, path := range paths {
		b.WriteString(git.LiteralPathspec(path))
		b.WriteByte(0)
	}
	return []byte(b.String())
}

// IsIgnored reports whether git ignores path in dir: untracked and matched by an ignore
// rule, whether or not it exists. A tracked path is never ignored. A failed probe's
// error carries git's stderr as a *GitError.
func IsIgnored(ctx context.Context, m *git.Mutator, dir, path string) (bool, error) {
	res, err := m.Run(ctx, "git", "-C", dir, "check-ignore", "-q", "--", path)
	switch {
	case err == nil:
		return true, nil
	case res.ExitCode == 1:
		return false, nil
	default:
		return false, fmt.Errorf("checking whether git ignores %s: %w", path, &GitError{Stderr: res.Stderr, err: err})
	}
}

// GitError is a failed git step, carrying the stderr git wrote.
type GitError struct {
	Stderr string
	err    error
}

// NewGitError is the git step that failed with err, having written stderr.
func NewGitError(stderr string, err error) *GitError {
	return &GitError{Stderr: stderr, err: err}
}

func (e *GitError) Error() string { return e.err.Error() }

// Unwrap exposes the runner error so errors.Is matches it.
func (e *GitError) Unwrap() error { return e.err }

// CommitStaged commits the index in dir with subject, reporting whether a commit was
// made. Only when keepOut has staged edits does it name the other staged paths on
// stdin instead: a bare exclusion pathspec on `git commit` would commit the work-tree
// content of every other tracked path, sweeping in unstaged and assume-unchanged
// edits. The named paths are still committed from the work tree, which matches the
// index only for paths staged just before the call. The list is never empty when the
// commit runs, since an empty pathspec file commits the whole index, keepOut's staged
// edits included. That route is CommitPaths', the index matched to the commit after
// it. A failed commit's error carries git's stderr as a *GitError; committed is true
// alongside an *IndexBehindError, the commit having landed.
func CommitStaged(ctx context.Context, m *git.Mutator, dir, subject string, keepOut git.Exclusion) (bool, error) {
	keepOutStaged, err := hasStagedEdits(ctx, m, dir, keepOut)
	if err != nil {
		return false, err
	}
	if !keepOutStaged {
		if res, err := m.Mutate(ctx, nil, "git", "-C", dir, "commit", "-m", subject); err != nil {
			return false, NewGitError(res.Stderr, err)
		}
		return true, nil
	}

	staged, err := m.Run(ctx, "git", withExclusion(keepOut, "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames")...)
	if err != nil {
		return false, fmt.Errorf("listing staged paths in %s: %w", dir, err)
	}
	if staged.Stdout == "" {
		return false, nil
	}
	paths := strings.Split(strings.TrimSuffix(staged.Stdout, "\x00"), "\x00")
	if err := CommitPaths(ctx, m, dir, subject, paths); err != nil {
		var behind *IndexBehindError
		return errors.As(err, &behind), err
	}
	return true, nil
}

// hasStagedEdits reports whether keepOut names a path with edits in the index.
func hasStagedEdits(ctx context.Context, m *git.Mutator, dir string, keepOut git.Exclusion) (bool, error) {
	if keepOut == "" {
		return false, nil
	}
	res, err := m.Run(ctx, "git", "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", git.LiteralPathspec(string(keepOut)))
	if err != nil {
		return false, fmt.Errorf("checking %s for staged edits: %w", keepOut, err)
	}
	return res.Stdout != "", nil
}

// BookkeepingSubject is the SINGLE source of the release-bookkeeping commit subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). The actual bookkeeping commit (CommitBookkeeping), the
// dry-run plan, and the annotated-tag subject line all obtain the subject from here,
// so a format change can never desync the plan or the tag from the real commit.
func BookkeepingSubject(commitPrefix, tag string) string {
	return fmt.Sprintf("%s Release %s", commitPrefix, tag)
}

const answersSubjectPrefix = "chore(release): release-notes path answers for "

// AnswersSubject is the subject of the commit recording the exclusion check's answers
// for tag. Its prefix is fixed, never commit_prefix: the commit records how releases
// are made rather than being the release.
func AnswersSubject(tag string) string {
	return answersSubjectPrefix + tag
}

// IsAnswersSubject reports whether subject is an answers commit's, for any tag.
func IsAnswersSubject(subject string) bool {
	return strings.HasPrefix(subject, answersSubjectPrefix)
}

// BookkeepingWillCommit is the SINGLE commit-or-not predicate for the bookkeeping
// commit: it reports whether CommitBookkeeping will make a commit for the given
// net-change signals — exactly when bookkeepingPaths yields something to stage (the
// changelog and/or a configured version file changed). CommitBookkeeping's own no-op
// branch consumes it, and the engine spine consults it to fold the resulting commit
// into its MadeState the moment it lands WITHOUT re-probing git — so the spine's
// commit tracking can never desync from the commit rule.
func BookkeepingWillCommit(versionFile string, changelogChanged, versionChanged bool) bool {
	return len(bookkeepingPaths(versionFile, changelogChanged, versionChanged)) > 0
}

// bookkeepingPaths assembles, in stage order, the paths the bookkeeping commit
// stages: the changelog when it changed, then the version file when it changed and
// a path is configured. An empty result is the combined no-op — nothing to stage,
// so no commit.
func bookkeepingPaths(versionFile string, changelogChanged, versionChanged bool) []string {
	var paths []string
	if changelogChanged {
		paths = append(paths, ChangelogFileName)
	}
	if versionChanged && versionFile != "" {
		paths = append(paths, versionFile)
	}
	return paths
}

func withExclusion(keepOut git.Exclusion, args ...string) []string {
	return append(args, keepOut.Pathspec()...)
}
