✨ Added
- Pack support folders — folders in a pack's `skills/` that hold no skill now install whole to `.claude/skills/<name>/`, so skills can share references.
- Update safety check that stops and names any installed `.claude/skills/` folder an update would remove while the repo still has it, with a `remove` + `add` remedy to update anyway.
- Pack support folders appear in `add` menus, `--list` output (JSON included) and install summaries, e.g. `2 skills, 3 folders, 1 agent`.

🔧 Changed
- `list` detail view counts what an entry installs the way `add` does, telling skills from support folders.
- Skills taken from a pack with `--skill` now note the support folders left behind.
- Packs only absorb loose skills through their skills; a support folder landing on a loose skill is reported as files in the way.
- A pack whose own folders would install to the same place is refused with a clear message and no misleading "choose one" prompt.
- Failed update checks now print one readable line, e.g. `couldn't check for updates (<cause>) — left untouched`, and copy-failure hints name the cause.
- Update failure lines no longer repeat the member name.
- `list` detail view and version change report why a check or version fetch failed instead of failing silently.

🐛 Fixed
- Clones retry into a freshly emptied folder, so a partial checkout no longer hides the real error on retry.
- Remote reads (tags, default branch, update checks) now retry on transient failures, with per-attempt timeouts and clear "timed out" messages.
- SSH "Permission denied" errors are retried rather than treated as final, while HTTPS credential failures and timeouts fail immediately.
