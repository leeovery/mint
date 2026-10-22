Search your live sessions straight from the command line with `x /term`, which attaches a lone match or opens the picker filtered to the matches.

💥 Breaking
- A single-segment absolute directory such as `x /tmp` or `x /opt` now searches live sessions instead of minting a session there — use `x -p /tmp` to mint.

✨ Added
- `x /term` searches live sessions by name and directory — one match attaches outright, several or none open the picker pre-filtered by the term.
- `x /` opens the picker with the filter focused and ready to type.
- Search-opened picker rows show each session's directory beside its name, shortened to fit.
- Tab completion for `x` offers live session names, and `x /po<TAB>` completes search terms; re-evaluate `portal init` output in your shell to get it.

🔧 Changed
- A search combined with another target, a command, `-f` or a domain pin is rejected immediately as a usage error, before tmux is started.
- Picks made in a command-pending picker while startup is still finishing now wait and run once startup completes, with a banner saying so.
- Startup warnings no longer wait for a loading page and are written to the terminal when the picker exits.

🐛 Fixed
- A malformed `hooks.json` no longer breaks hook lookups and listings — reads treat it as empty, and changes refuse to overwrite it.
- Grouped session views no longer make a session findable by a directory guessed from its pane.
- `portal doctor` no longer reports stale hooks as not evaluable when no hooks are registered, and `--fix` now reports when the hook store could not be read.
- A command-pending picker now exits with the error instead of hanging when startup fails fatally.
