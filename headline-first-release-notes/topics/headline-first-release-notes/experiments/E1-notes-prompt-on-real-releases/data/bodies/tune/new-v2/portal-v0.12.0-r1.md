Search your live sessions straight from the command line with `x /term`, attaching at once when only one session matches.

💥 Breaking
- A single-segment absolute directory such as `x /tmp` now searches sessions instead of minting there — use `x -p /tmp` to mint at that path.

✨ Added
- `x /term` searches live sessions by name and directory — one match attaches directly, otherwise the picker opens pre-filtered.
- `x /` opens the picker with the filter focused and ready to type.
- Search-opened pickers show each session's directory beside its name.
- Tab completion for the session-opening function offers live session names and completes `/term` searches (re-evaluate `portal init` output to enable it).
- A project picked while startup is still finishing is staged and started once startup completes, with a notice shown meanwhile.

🔧 Changed
- Bootstrap warnings are now written to the terminal on exit when the picker never showed them, instead of being lost.
- Fish completion for the session-opening function no longer falls back to filename completion.
- A command-pending picker now exits with the error if startup fails fatally.

🐛 Fixed
- A malformed `hooks.json` no longer breaks hook lookups and listings — reads treat it as empty while writes refuse to overwrite it.
- `portal doctor` no longer reports stale hooks as unevaluable when there are no hooks, and reports an unreadable hooks file when pruning.
- Sessions grouped by a directory guessed from their pane no longer have that guess recorded as their directory.
