Resume hooks now stay attached to a pane however tmux rearranges or renames it.

💥 Breaking
- Resume hooks are now keyed by a per-pane token instead of `<session>:<window>.<pane>` — existing hooks registered under the old key no longer fire, so re-register them with `hook set --on-resume` (the old entries are kept, and can be removed with `hook rm --pane-key`).
- `hook rm` now exits non-zero when it removes nothing — scripts that treated it as always succeeding must handle the error.

✨ Added
- `hook list` shows each hook's current pane location in a fourth column — empty when no live pane carries the hook.
- Session names starting with `$` or `-`, or containing `:`, are refused on rename with a clear message in the picker — tmux can't address them reliably.

🔧 Changed
- Generated session names never start with `$` or `-`, and a project name that sanitises to nothing yields a name from the ID alone.
- `hook set` nudges the daemon to capture the new hook's pane on its next tick.
- Concurrent hook changes are serialised through a `hooks.json.lock` file beside `hooks.json`, so simultaneous writers no longer overwrite each other.
- `portal doctor` reports why a stale-hook check or prune couldn't run (restore in progress, unreadable `hooks.json`, locked file, pane list unavailable).
- The theme panel header stays aligned with the page when a notice band is showing.
- The saver daemon readiness wait tolerates slow, loaded machines, and a daemon that never comes up now surfaces a warning.

🐛 Fixed
- Sessions with a period in their name are no longer treated as vanished and dropped from saved state, and kill, rename and switch no longer hit a similarly-named sibling session.
- Restored panes now launch the same `portal` binary that performed the restore rather than whichever one is first on PATH.
- Hook command strings and shell paths containing quotes or spaces are quoted correctly when run.
- The stale-hook sweep no longer deletes hooks registered while it was running, and never deletes old-format keys it can't judge.
- The filter footer shows as much of its keymap as fits on narrow terminals instead of vanishing.
- A failed restore-marker read no longer lets the daemon capture a half-restored state.
- An invalid `prefs.json` error now names the file.
