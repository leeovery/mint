✨ Added
- Pack support folders — folders in a pack's `skills/` that hold no skill (shared references, gates) now install whole to `.claude/skills/<name>/` alongside its skills.
- Update safety check — an update that would remove a folder the repo still has now stops before deleting anything, names the folders and prints the `remove` and `add` that update it anyway.

🔧 Changed
- Pack counts in menus, summaries, `--list` and the `list` detail view now include support folders, e.g. `2 skills, 3 folders, 1 agent`.
- A skill taken from a pack with `--skill` installs without the pack's support folders, and its note names them.
- A pack's support folder landing on a loose skill from the same source now counts as files in the way rather than being absorbed.
- A pack whose own folders would install to one place is refused as it stands, with no misleading "choose one" prompt, on both `add` and `update`.
- Failed update checks and version fetches print one readable line, such as `couldn't check for updates (git@github.com: Permission denied (publickey).) — left untouched`.
- Update failure lines no longer repeat the member name, and copy-failure hints now say why the copy failed.
- Remote git reads (tags, default branch, update probes) now retry up to three times, and SSH "Permission denied" is retried rather than failing at once.
- Timed-out git commands are reported as timeouts and aren't retried.

🐛 Fixed
- Clone retries no longer fail on the partial checkout left by an earlier attempt, which hid the real error.
- `list` detail view counts each skill folder, hook and agent once instead of once per recorded file path.
- `list` detail view now reports a failed update check instead of staying silent.
