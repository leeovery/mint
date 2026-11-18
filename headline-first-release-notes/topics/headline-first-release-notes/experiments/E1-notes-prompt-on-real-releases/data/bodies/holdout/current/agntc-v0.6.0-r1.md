✨ Added
- A new terminal interface for every command — each run opens with a header, draws its work as rows with trees of what was installed, and closes with a tally and elapsed time.
- A "Next" section after a run listing the exact commands that mend or advance things, such as a newer version outside your constraint or the `remove` and `add` that fix a blocked update.
- `list` is now a dashboard grouped by repo, showing each install's kind, version, constraint and status, with a footer counting updates, newer tags and failed checks.
- `list` detail view shows an install's version, constraint, commit, install date, contents and update status, with Update, Change version and Remove actions.
- `init` shows the files it will create as a tree and asks before writing, marking files that already exist.
- Colour and layout adapt to the terminal — respects `NO_COLOR` and `FORCE_COLOR`, wraps to the terminal width, and fits long lists to the screen height.
- Ctrl-C and Ctrl-Z are handled cleanly — spinners and questions are cleared and the terminal is restored.

🔧 Changed
- Questions now appear under the row they are about and are erased once answered, instead of scrolling through boxed prompts.
- `update` shows one row per repo group with its version move and members, and reports failures and skipped installs in the same row.
- `remove` asks one combined question counting what will be deleted, such as "Remove it and its 2 skills and 1 agent?".
- `add` shows the source as a single row with each item's outcome beneath it, including files cleared out of the way.
- README rewritten with a clearer explanation of packs, commands and scripting.

🗑️ Removed
- The boxed file lists, spinner messages and outro lines of the previous interface, replaced by the new rows and tally.
