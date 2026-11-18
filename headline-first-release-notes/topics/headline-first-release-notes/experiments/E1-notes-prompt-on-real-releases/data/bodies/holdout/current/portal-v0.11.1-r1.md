✨ Added
- Resume hooks now follow the pane itself — each hooked pane gets a durable token, so hooks survive pane moves and session renames.
- `portal hook list` shows a fourth column with each hook's current pane location (empty if no live pane carries it).
- Rename refusals in the picker — names containing `:` or starting with `$` or `-` are rejected with a clear message instead of leaving a session tmux can't address.
- `hooks.json` writes are now guarded by a lock file (`hooks.json.lock`) so concurrent writers can't overwrite each other.

🔧 Changed
- `portal hook rm` now exits non-zero when it removes nothing, so scripts can tell a real removal from a no-op.
- Stale-hook cleanup never deletes old-format `<session>:<window>.<pane>` keys it can't judge, and logs the command of every hook it reaps.
- `portal doctor` reports why a stale-hook check or prune couldn't run (restore in progress, unreadable `hooks.json`, locked file) instead of looking like it found nothing.
- Every per-session tmux command now uses an exact-match target, so a session whose name extends another's (or contains a period) is never confused with it.
- Generated session names no longer start with `$` or `-` or contain periods and colons from the project name.
- Restored panes are armed with the exact `portal` binary doing the restore rather than whatever is first on PATH.
- The saver-daemon readiness wait now tolerates slow starts and reports a warning if the daemon never comes up.
- Capture and shutdown flushes stand down when the restore marker can't be read, rather than risk saving a half-restored state.

🗑️ Removed
- The hidden `portal state migrate-rename` command — hooks no longer need re-keying on rename.

🐛 Fixed
- Hooks no longer get orphaned when a session is renamed or panes are rearranged.
- A hook registered while a cleanup is running can no longer be deleted by it.
- Live sessions with a period in their name are no longer treated as vanished and dropped from saved state.
- The filter footer keeps its keymap visible at narrow widths instead of disappearing.
- The theme panel stays aligned with the page when a notice band is showing.
- Config-migration failures now log the specific write-phase error class.
- Homebrew tap update runs as its own release job, so a tap failure no longer turns a published release red and can be retried alone.
