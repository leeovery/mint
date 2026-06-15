# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.5] - 2026-06-15

✨ Added

- New `--source release` option for `mint release regenerate` — reads the existing published GitHub release body verbatim, making it easy to backfill a `CHANGELOG.md` from releases you already have.

🔧 Changed

- `--reuse` and `--fresh` flags replaced by a single `--source <fresh|tag|release>` flag — `--source tag` is the equivalent of the old `--reuse`, and `fresh` remains the default.
- Source and target axes are now fully independent — any source (`fresh`, `tag`, `release`) can write any target (`release`, `changelog`, `both`); the old constraint that `--reuse` implied `--target release` is removed.
- `--target` is now required with `-y` for every source, not just `fresh`; no source has a safe default surface to guess unattended.
- A skipped version with no recorded changelog section is silently omitted from the rebuilt file rather than aborting the entire batch — fixes a bug where one body-less version in a `--all` run discarded every other regenerated section.
- Post-push publish failure heal message updated to `regenerate --source tag`.

## [0.0.4] - 2026-06-15

✨ Added

- `mint regenerate` now displays the generated notes before the review gate — previously the "Use these notes?" prompt could appear over a blank preview.

🔧 Changed

- The plan block no longer adds a double blank line when it follows the brand header directly — the extra gap is collapsed to a single blank.

🗑️ Removed

- Deleted the internal design handoff document and skills lock file, which were development artifacts no longer needed in the repository.

## [0.0.3] - 2026-06-14

✨ Added

- Per-verb `ai_command` and `timeout` overrides — set `[release].ai_command`, `[release].timeout`, `[commit].ai_command`, or `[commit].timeout` to repoint either key for one verb without affecting the other.
- `timeout` config key (integer seconds) — sets the per-attempt AI deadline at the shared top level or per verb; `0` disables the deadline entirely for operators running slow or local models.

🔧 Changed

- Default `ai_command` is now `claude -p --model sonnet` — the model is pinned so zero-config behaviour is predictable regardless of which model your Claude CLI defaults to.
- Per-attempt AI deadline defaults to 60 seconds and is now configurable; a timeout is fatal and never retried (the single retry covers bad content only).
- Setting `timeout = 0` disables the per-attempt deadline entirely — the AI call runs unbounded, which is a deliberate operator choice that overrides mint's "fail loud, never hang" posture.
- `diff_exclude` default in the scaffold is now shown as `[]` with a descriptive comment; mint's own workflow artifact directories are excluded in the project's own `.mint.toml`.

## [0.0.2] - 2026-06-13

✨ Added
- Confirm the version and bump before any work begins — a real release now opens with a "Release v1.3.2 → v1.4.0 (minor)?" gate that aborts cleanly with nothing to unwind.
- Single-keypress review gates — press y/n/e/r to decide with no Enter, Esc to decline, and Ctrl-C to abort cleanly.
- The release-notes cache is shared across runs and offers to reuse a matching note from a prior dry run, so an unchanged diff skips the AI on the next real run.

🔧 Changed
- The release notes prompt now asks for concise one-line bullets per change with no TL;DR, so changelogs read at a glance.
- Dry runs are pure previews — they show the version, plan, and notes, prompt for nothing, and end with a clear "no changes made" line.
- Review gates now spell out their consequence — accepting the final release gate reads "[y] release", not a vague "accept & proceed".
- Pretty output was redesigned flush-left with a dim gutter for notes, a one-line hotkey bar, animated activity spinners, and per-stage narration of what each step did.
- The note cache now lives under your user cache directory instead of a `.mint/cache` folder inside the repo, so no project is polluted with an in-repo cache.
- A spinner now animates while the AI writes a commit message, with a clear note when it falls back to your editor.

🐛 Fixed
- Failures no longer print twice when stdout and stderr share a terminal.
- A first-time release with an empty cache no longer prints a misleading "diff changed since dry-run preview" notice.
- The AI prompt now repeats the output contract after the diff, stopping the model from prefacing notes or commit messages with stray narration.

