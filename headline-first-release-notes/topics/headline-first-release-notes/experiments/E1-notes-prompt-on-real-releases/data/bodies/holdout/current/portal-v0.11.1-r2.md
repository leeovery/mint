✨ Added
- Resume hooks now follow a pane's own durable token, so they keep firing after a session rename or any window/pane rearrangement.
- `hook list` shows each hook's current pane location as a fourth column — empty when no live pane carries the hook's token.
- `hook set` asks the daemon to capture the new pane token on its next tick, narrowing the window where a crash could lose it.
- `portal doctor` and `--fix` now say why a stale-hook check or prune was skipped (restore in progress, locked file, unreadable panes) instead of looking like it found nothing.
- Renaming a session to a name tmux can't address is refused with a clear message in the picker — names containing `:` or starting with `$` or `-`.
- Concurrent writers can no longer overwrite each other's hook changes — `hooks.json` is now protected by a `hooks.json.lock` file created beside it on first change.

🔧 Changed
- `hook rm` exits non-zero whenever it removes nothing, so scripts can tell a real removal from a no-op.
- Stale-hook cleanup now logs the command of every hook it removes at INFO level, so a reaped hook can be recovered from the log.
- Old-format `<session>:<window>.<pane>` hook keys are kept rather than guessed at — remove them by hand with `hook rm --pane-key`.
- Generated session names never start with `$` or `-` and fall back to the bare id when the project name sanitises to nothing.
- Restored panes now run the same portal binary that performed the restore, rather than whatever `portal` is first on PATH.
- The daemon's readiness wait now tolerates slow starts on loaded machines, and bootstrap surfaces a warning if the daemon never comes up.
- Homebrew formula updates run as their own release job, so a failure there can be retried without re-running the whole release.
- Theme panel layout stays aligned with the page when a notice band is showing.
- The filter footer now trims its key hints to fit narrow terminals instead of disappearing entirely.
- Corrected contrast values in the `tokyo-night-day` theme.

🗑️ Removed
- The internal `state migrate-rename` command and session-ID (`@portal-id`) based hook keys, now unnecessary since hooks follow the pane's token.

🐛 Fixed
- Sessions with a period in their name are no longer mistaken for a vanished session and dropped from saved state with their scrollback.
- Session operations (kill, attach, switch, rename) now target the exact session name, so they can no longer hit a live session whose name merely starts with the same text.
- A stale-hook sweep can no longer delete a hook registered while it was running.
- Daemon capture and shutdown flush now consistently stand down when the restore marker can't be read, rather than proceeding.
- Config migration failures now log clear write-failure causes.
- Invalid `prefs.json` errors now name the offending file.
