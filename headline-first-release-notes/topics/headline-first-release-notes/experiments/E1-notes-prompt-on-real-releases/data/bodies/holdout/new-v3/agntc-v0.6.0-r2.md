The terminal output of every command is redrawn so each run reads as a header, rows showing what happened, and a closing tally.

✨ Added
- Per-run tally closing the output, counting outcomes with the time the run took.
- Next section listing the commands that mend a failed or blocked update or reach a newer version.
- `list` dashboard grouped by repo, showing each install's kind, version, constraint and status, with counts of what needs attention.
- `list` detail view with the install's version, constraint, commit, install date, contents and status.
- `add` shows a pack's kind and contents (skills, folders, agents, hooks) in its menu and `--list` output.
- `add` tells you what it cleared out of an item's way, such as a replaced entry or files agntc didn't install.
- `init` shows the files it will create as a tree and marks those already there.

🔧 Changed
- Work in progress now spins on the row it will settle into, and questions are asked under the row they concern, then disappear once answered.
- `update` shows each repo's pack and skills as one row with the version move, and the members beneath it.
- `remove` shows what each install holds, grouped by asset folder, and asks one question across all selected installs.
- Ctrl-C clears the spinner or question, prints what remained to say, and exits cleanly.
- Colour now follows `NO_COLOR` and `FORCE_COLOR`, and layout adapts to narrow terminals.
