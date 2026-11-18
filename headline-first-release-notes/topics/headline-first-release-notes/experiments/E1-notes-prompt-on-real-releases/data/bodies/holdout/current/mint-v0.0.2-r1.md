✨ Added
- Single-keypress review gates — press y/n/e/r to decide with no Enter; Esc declines and Ctrl-C aborts cleanly.
- Version confirmation before a release starts — confirm the bump up front, so declining leaves nothing to undo.
- Cached-notes prompt on release — when notes for the same diff already exist, choose to reuse them or regenerate.
- Spinner and progress narration while commit messages are generated or regenerated.
- A `release` shim script at the repo root that runs `mint release` and points to the install command if mint is missing.

🔧 Changed
- `--dry-run` now previews the version, plan, and notes without prompting, then closes with a "no changes made" summary.
- Terminal output redesigned — flush-left layout, a gutter-style notes panel, a compact hotkey bar, and a verb-first header like `mint › releasing acme v1.4.0`.
- Review gates use clearer wording, such as "Release with these notes?" and "Commit this message?", with short action labels.
- Progress lines now read as plain sentences such as "Pushed branch + v1.4.0 atomically" and name what was recorded.
- Release notes are now one concise line per change, with no TL;DR.
- The note cache moved to your user cache directory, keeping project repos clean and pruning stale entries.
- AI prompts now end with a reminder to output only the message, curbing preamble and narration from the model.
- Commit now explains why the editor opens when AI generation fails.

🐛 Fixed
- A second Ctrl-C now force-quits instead of being swallowed by a stuck prompt.
- Failures no longer print twice when stdout and stderr share a terminal.
- Plain releases no longer show a misleading "diff changed since dry-run preview" notice when no preview existed.
