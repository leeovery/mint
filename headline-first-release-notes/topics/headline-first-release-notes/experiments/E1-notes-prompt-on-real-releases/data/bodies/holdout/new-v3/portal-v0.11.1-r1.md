Resume hooks now stay attached to a pane however tmux rearranges or renames it.

💥 Breaking
- Resume hooks are now keyed by a per-pane token instead of `<session>:<window>.<pane>` — existing hooks under the old keys no longer fire and are left in `hooks.json` untouched, so re-register them with `portal hook set` or remove them with `portal hook rm --pane-key <key>`.
- `portal hook rm` now exits non-zero when it removes nothing — scripts that relied on a silent success for a missing hook need to handle the error.

✨ Added
- `portal hook list` shows each hook's current pane location as a fourth column — empty when no live pane carries the hook.
- Session names starting with `$` or `-`, or containing `:`, are refused when renaming from the picker — the picker explains why and keeps the old name.
- The picker's notice band no longer collides with the theme panel — the panel lines up with the page beneath it when a message is showing.

🔧 Changed
- `portal doctor` explains why a stale-hook prune or check could not run (restore in progress, unreadable `hooks.json`, locked file, unreadable pane list) instead of reporting nothing.
- Generated session names never start with `-` or `$`, so a project named that way still gets a usable session.
- Stale-hook cleanup logs each removed hook's command at the default log level, so a reaped hook can be recovered.
- Saver daemon startup waits longer on slow machines before giving up, and bootstrap now reports a warning if it never comes up.
- Restored panes are armed with the exact `portal` binary doing the restore rather than whatever is found on `PATH`.

🐛 Fixed
- Concurrent `hook set`/`hook rm`/cleanup runs no longer overwrite each other's changes in `hooks.json`.
- Hooks registered while a stale-hook sweep is running are no longer deleted by it.
- Sessions with a period in their name, or a name that prefixes another live session, are no longer mistaken for missing sessions, killed, or dropped from saved state.
- Hooks survive session renames and tmux renumbering, and are restored after a reboot together with their pane.
- The filter footer keeps showing its keys on narrow terminals instead of vanishing.
- The `hook set` command asks the daemon to capture the new pane token right away.
- A corrupt `prefs.json` now reports which file is invalid.
