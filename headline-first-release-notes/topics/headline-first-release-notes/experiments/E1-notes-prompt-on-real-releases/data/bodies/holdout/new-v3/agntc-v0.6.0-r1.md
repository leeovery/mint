Every command now draws its own terminal output: a header, a row for each thing it did with its contents as a tree, and a closing tally.

✨ Added
- Next section at the end of a run lists what you can do about it — the newer version's command, or the `remove` and `add` that mend an update that couldn't go ahead.
- Closing tally counts a run's outcomes with the time it took, shown when a run has more than one outcome or anything under Next.
- `list` dashboard groups installs by repo with kind, version, constraint and status, plus a footer counting what wants attention.
- `list` detail view shows an install's version, constraint, commit, install date, contents and update status.
- `add --list` shows each pack with its skill, folder, agent and hook counts.
- `init` shows the files it will create as a tree before asking, and leaves files that already exist alone.
- Colour follows `NO_COLOR` and `FORCE_COLOR`, and Ctrl-C clears the spinner or question and exits cleanly.

🔧 Changed
- Questions now appear under the row they are about and disappear once answered, instead of printing boxes and prompts in the scroll.
- Progress spins on the row it belongs to (`cloning…`, `updating to v1.3.0…`) and settles into the result.
- `update` shows each repo's pack and skills as one row with their version move, and groups left alone (up to date, newer tags, check failed) as one row each.
- `remove` shows each install's contents by asset folder, and its question counts what will go (`Remove it and its 2 skills?`).
- `list` actions (update, change version, remove) now run under the same rows as the commands, and the session ends with a tally of what changed.
- Files in the way during `add` are asked about under the install's row, listing the files.
- Help and version output are unchanged, but the banner is no longer printed when a command runs.

🐛 Fixed
- Failures that the previous release printed in two styles now print once, with their cause and remedy laid out under the error.
