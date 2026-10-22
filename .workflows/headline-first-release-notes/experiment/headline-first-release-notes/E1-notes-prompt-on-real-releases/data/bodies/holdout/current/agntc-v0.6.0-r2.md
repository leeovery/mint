✨ Added
- A custom terminal renderer for every command — a header, rows that show each result with its contents as a tree, spinners for work in progress, and a closing tally with elapsed time.
- A "Next" section at the end of a run listing the exact commands that mend or advance what couldn't go ahead, such as the `remove` and `add` for a blocked update or the `add` that reaches a newer version.
- Questions now appear directly under the row they're about and disappear once answered, in a new select, multi-select and confirm prompt set with scrolling, key hints and narrow-terminal handling.
- The `list` dashboard groups installs by repo, showing each one's kind, version, constraint and update status, with a footer counting what needs attention.
- The `list` detail view shows an install's version, constraint, commit, install date, contents and status, with Update, Change version and Remove actions.
- `list` ends a session with a tally of what was updated, version-changed or removed.
- `init` shows the files it will create as a tree, marks those already present, and skips the question when nothing would be written.
- `add --list` and the `add` menu show each pack's kind and contents and each skill's description, with a folder shown beside items that share a name.
- Colour follows `NO_COLOR` and `FORCE_COLOR`, and the cursor row picks up a highlight band matched to your terminal's background.
- Ctrl-C and Ctrl-Z are handled cleanly mid-run, clearing spinners and questions, restoring the terminal and still printing what the run had left to say.

🔧 Changed
- `update` now shows one row per repo group or local install, with the version move and each member beneath it, replacing the streamed header and trailing summary.
- Files in the way of an install are asked about under that install's row, showing the files involved, and the result lists what was removed or replaced.
- `remove` asks one question across all selected entries that counts what will go, such as "Remove them and their 14 skills?", and then shows what was removed under each asset folder.
- Entries gone upstream are asked about under their row, with what they installed listed beneath it.
- Failure and recovery messages now separate what happened from the commands that fix it, and the commands print on their own lines so they can be copied cleanly.
- Local installs show their folder with `~` for your home directory.
- The README was rewritten around packs, with screenshots, a "Why agntc?" section and updated command docs.

🗑️ Removed
- The `@clack/prompts` and `picocolors` dependencies, replaced by agntc's own renderer.
