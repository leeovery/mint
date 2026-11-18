✨ Added
- Session search with `x /term` — attaches the session outright when exactly one live session matches by name or directory, otherwise opens the picker pre-filtered.
- `x /` opens the picker with the filter focused and empty, ready to type.
- Search-opened pickers show each session's directory beside its name, abbreviated to `~/…` and truncated to fit.
- Tab completion for the session-opening function — `x <TAB>` offers live session names and `x /po<TAB>` completes search terms, in bash, zsh and fish.

🔧 Changed
- A single-segment absolute path such as `x /tmp` is now read as a search instead of a path — mint there with `x -p /tmp`.
- A search combined with another target, a command, `-f`, or a domain pin is refused as a usage error before tmux is started.
- Picking a project while startup is still finishing is now held and minted once startup completes, with a banner announcing the wait.
- Bootstrap warnings no longer wait for a loading page; they are written at teardown when the picker never showed one.
- A command-pending picker now exits with the error if bootstrap fails fatally.
- Malformed `hooks.json` now reads as empty for lookups and listings, while any command that would modify it refuses rather than overwrite it.
- Grouped session views no longer alter a session's recorded directory when guessing its location, so sessions are not findable by paths they were not found by before.

🐛 Fixed
- `portal doctor` no longer reports stale hooks as unknowable when there are no hooks and a pane read fails.
- `portal doctor --fix` now reports why a hook prune was skipped when the hook store could not be read.
- Hook locations are listed correctly for panes shared between grouped or linked sessions.
