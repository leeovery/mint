✨ Added
- Single-keypress review gates — press y/n/e/r to decide with no Enter needed, Esc declines, and Ctrl-C aborts cleanly.
- Version confirmation before a release starts — mint asks "Release v1.2.0 → v1.3.0 (minor)?" before touching anything.
- Cached release notes can be reused — when a cached note matches the current diff, a real run offers to use it or regenerate.
- A `release` shim script at the repo root that runs `mint release` and tells you how to install mint if it's missing.
- Spinners now show what is happening (e.g. "generating commit message…") during AI calls and other slow steps.

🔧 Changed
- Dry runs ask nothing and change nothing — they preview the version, plan, and notes, then close with a "no changes made" summary marked "dry run".
- Release notes are now concise one-line bullets with no TL;DR, and the prompt ends with a reminder to output only the notes.
- Commit message prompts end with a reminder to output only the message, preventing models from narrating before it.
- Terminal output has a new look — flush-left layout, a hotkey bar for gates, and a guttered panel for notes and commit messages.
- Review gates use clearer wording, such as "Release with these notes?" and "Commit this message?" with short action labels.
- The note cache moved to your user cache directory instead of a `.mint` folder inside each repo, and old entries are pruned automatically.
- Completion lines read as plain sentences (e.g. "Pushed branch + v1.4.0 atomically"), and preflight and record steps say what was checked or committed.
- A cache miss is now silent instead of warning that the diff changed since a dry run.

🐛 Fixed
- A second Ctrl-C now force-quits, so a stuck prompt can no longer be unkillable.
- Failures no longer print twice when stdout and stderr share a terminal.
- If the AI fails during `mint commit`, a notice explains why the editor is opening.
