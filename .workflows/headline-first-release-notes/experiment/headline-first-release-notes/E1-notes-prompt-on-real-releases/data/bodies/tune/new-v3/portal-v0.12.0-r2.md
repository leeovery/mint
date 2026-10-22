Search live sessions straight from the command line with `x /term`, attaching outright when only one matches.

💥 Breaking
- A single-segment absolute directory such as `x /tmp` now searches live sessions instead of minting a session there — mint with `x -p /tmp`.

✨ Added
- `x /term` searches live session names and directories (case-folded, contiguous match) — one match attaches directly, otherwise the picker opens pre-filtered by the term, and no match is never an error.
- `x /` opens the picker with the filter focused and ready to type.
- Search-opened pickers show each session's directory beside its name, abbreviated to `~/…` and shortened from the left when space is tight.
- Tab completion for the session-opening function: `x <TAB>` offers live session names and `x /po<TAB>` completes search terms (re-evaluate `portal init` output to pick it up).
- Picking a project while startup is still finishing now waits and mints the session once startup completes, with a banner explaining the wait.

🔧 Changed
- A `/term` search combined with another target, a command, `-f`, a domain pin or a second search is rejected as a usage error before any tmux server starts.
- `-f` and other invalid argument combinations are now refused before tmux bootstraps, so refused command lines start nothing.
- Startup warnings are now written to the terminal when the picker exits if no screen showed them, rather than being lost.
- A `portal open` picker with a pending command that hits a fatal startup error now exits with the error instead of sitting on a half-bootstrapped picker.

🐛 Fixed
- A malformed `hooks.json` no longer breaks hook lookups, listings or diagnosis, and no longer lets a hook write overwrite the unreadable registrations.
- `portal doctor` no longer reports stale hooks as unevaluable when there are no hooks to judge, and reports a skipped prune when the hooks store couldn't be opened.
- Grouped session views (by project / by tag) no longer make sessions findable by a guessed directory they weren't findable by before.
- Panes shown by multiple grouped or linked sessions no longer cause inconsistent hook locations.
