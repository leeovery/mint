# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

