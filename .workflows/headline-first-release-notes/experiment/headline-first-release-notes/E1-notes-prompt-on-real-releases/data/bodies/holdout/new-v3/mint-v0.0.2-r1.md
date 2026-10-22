💥 Breaking
- The dry-run note cache moved from `.mint/cache` inside the repo to your user cache directory — previews cached by an earlier release are no longer found, so re-run `mint release -d` before relying on reuse.

✨ Added
- `mint release` asks you to confirm the version bump before any work begins — decline to abort with nothing touched.
- A real release that finds cached notes for the same diff offers to use them or regenerate.
- Dry runs are marked "dry run" from the first line and end with a "no changes made" summary instead of a success footer.
- Pressing Escape at a review gate declines it, and a second Ctrl-C force-quits if a prompt ever hangs.

🔧 Changed
- Review gates are single-keypress hotkey bars — press y/n/e/r with no Enter, and Ctrl-C aborts cleanly.
- Gate questions now say what accepting does, such as "Release with these notes?" and "Commit this message?".
- A dry run asks nothing: it previews the version, plan and notes, then stops without any review gates.
- Pretty output is redesigned flush-left, with notes and commit messages shown in a gutter panel and stages narrated as plain sentences.
- Slow steps show what they are doing, such as "generating commit message…", with a spinner for commit generation and regeneration.
- Release notes are now concise one-line bullets with no TL;DR summary.
- A cache miss is silent, with no "diff changed since dry-run preview" notice.
- The dry-run plan no longer lists the push step or checks the publish provider.

🐛 Fixed
- AI no longer narrates before commit messages and release notes — a closing reminder now ends each prompt.
- Failures no longer print twice when stdout and stderr share a terminal.
- When the AI fails during `mint commit`, a note explains why the editor is opening.
