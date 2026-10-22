Search live sessions straight from the command line with `x /term`, which attaches the one match or opens the picker pre-filtered.

💥 Breaking
- A single-segment absolute directory such as `x /tmp` now runs a session search instead of minting a session there — use `x -p /tmp` to mint at that path.

✨ Added
- Session search with `x /term` — matches the term case-insensitively against each live session's name and directory, attaches directly when exactly one matches, and otherwise opens the picker pre-filtered.
- `x /` opens the picker with the filter focused and ready to type.
- Search-opened pickers show each session's directory beside its name, so rows matched by directory make sense.
- Tab completion for `x /po<TAB>` completes session names after the slash, and `x <TAB>` no longer falls back to filename completion (re-run `portal init` output in your shell to pick this up).
- Picking a project while startup is still finishing now waits and mints the session once startup completes, with a banner explaining the wait.

🔧 Changed
- Startup warnings are now written to the terminal when the picker exits, rather than only after a loading page dismisses.
- A `/term` search combined with another target, a command, `-f`, a domain pin or `--ack` is refused as a usage error before any tmux server is started.
- Invalid `-f` and command-scope combinations are now rejected before tmux bootstrap runs.

🐛 Fixed
- A malformed `hooks.json` no longer breaks hook lookups, listings or diagnosis — reads treat it as empty, while edits refuse to overwrite it.
- Grouped session views (By Project / By Tag) no longer make a session findable by a directory guessed from its pane.
- `portal doctor` no longer reports the stale-hook check as unevaluable when there are no hooks, and reports an unreadable hook store when pruning.
- A pending-command picker now exits with an error on a bootstrap failure instead of getting stuck.
- `hook list` shows a single location for panes shared across grouped or linked windows.
