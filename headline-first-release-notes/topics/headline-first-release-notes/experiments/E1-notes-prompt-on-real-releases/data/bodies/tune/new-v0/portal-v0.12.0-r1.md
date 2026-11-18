Search your live sessions straight from the command line with `x /term`, attaching at once when only one matches.

💥 Breaking
- A single-segment absolute directory such as `x /tmp` now searches sessions instead of minting there — use `x -p /tmp` to mint at that path.

✨ Added
- Session search with `x /term` — matches the term case-insensitively against session names and directories, attaches when exactly one session matches, and otherwise opens the picker pre-filtered.
- `x /` opens the picker with the filter focused and ready to type.
- Picker rows opened by a search show each session's directory beside its name.
- Tab completion for `x /po<TAB>` completes session names after the slash, and `x <TAB>` no longer falls back to filename completion (re-evaluate `portal init` output to pick this up).
- Picks made in a command-pending picker while startup is still finishing are now held and run once startup completes, with a banner explaining the wait.

🔧 Changed
- A `/term` search that can't be combined with another target, command, `-f` or domain pin is refused as a usage error before any tmux server is started.
- Bootstrap warnings are now written to the terminal on exit when no loading page showed them.
- Grouped views no longer make a session searchable by a directory guessed from its pane.

🐛 Fixed
- A malformed `hooks.json` no longer breaks hook lookups and listings — reads treat it as empty, while edits refuse to overwrite it.
- `portal doctor` no longer reports stale hooks as unknowable when there are no hooks stored, and a hook store that can't be read is now reported when pruning.
- A command-pending picker now exits with an error on a fatal bootstrap failure instead of hanging.
