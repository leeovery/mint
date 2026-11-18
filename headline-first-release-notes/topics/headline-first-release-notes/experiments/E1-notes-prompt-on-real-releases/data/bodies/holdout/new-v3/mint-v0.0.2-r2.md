💥 Breaking
- The dry-run note cache moved from the repo's `.mint/cache` to your user cache directory — previews from earlier dry runs are no longer found, so run `mint release -d` again before a real release to reuse the notes.

✨ Added
- Release now confirms the version up front ("Release v1.2.3 → v1.2.4 (patch)?") before any work starts — answer no to abort with nothing touched.
- Real releases offer to reuse cached notes for the same diff or regenerate them, with Enter reusing the cache.
- Dry runs are marked "dry run" in the header and end with a "no changes made" summary instead of a release line.
- Spinners and completion lines now say what is happening in plain sentences, such as "Generated release notes (8.2s)" and "Pushed branch + v1.2.4 atomically".
- Record, publish and preflight steps now report what was actually done, such as "Committed CHANGELOG.md + version file".

🔧 Changed
- Review gates are single keypresses — press y/n/e/r with no Enter, Esc declines, and Ctrl-C aborts cleanly.
- Output is redesigned: flush-left layout, notes and commit messages shown in a gutter panel, and a compact hotkey bar for prompts.
- Gate wording is clearer: "Release with these notes?", "Commit this message?" and "Use these notes?", with short action labels.
- Dry runs no longer ask any questions or show review gates, and they reuse a cached preview silently.
- Cache misses are silent, and old cache entries for a project are pruned automatically.
- The `commit` command now shows a spinner while the AI writes the message.
- When the AI fails during an attended `commit`, a note explains why the editor is opening.
- The dry-run plan no longer lists the push step or the downgraded publish target.

🐛 Fixed
- AI replies no longer start with narration before the commit message or release notes — the prompts now end with a reminder to output only the message.
- Failures no longer print twice when stdout and stderr share a terminal.
- A second Ctrl-C now kills mint immediately instead of being swallowed by a stuck prompt.
- Release notes are now one-line bullets with no TL;DR, matching the intended format.
