Packs can now install the shared folders in their `skills/` directory, so skills that load shared references work after install.

✨ Added
- Packs install support folders — a folder in a pack's `skills/` that holds no skill installs whole as `.claude/skills/<name>/`, so shared references, gates and similar travel with the pack.
- Pack counts in the add menu, `--list`, install summaries and `list` detail include support folders, for example `2 skills, 3 folders, 1 agent`.
- `--list` JSON output carries each pack's `supportFolders`.
- A skill taken from a pack with `--skill` has a note naming the support folders left behind.

🔧 Changed
- `update` stops, naming the folders, when it would remove a `.claude/skills/` folder that the source still has but would no longer install — run `remove` then `add` to update anyway.
- `update` re-walks packs so support folders added or removed upstream arrive or go.
- A pack whose own folders would install to the same place is refused with "can't be installed as it stands", with no choice offered, on both `add` and `update`.
- A pack's support folder landing on a loose skill from the same source is treated as files in the way rather than absorbing the skill.
- Failed update checks print a single line, `<label>: couldn't check for updates (<cause>) — left untouched`, and the `list` detail view shows the cause; machine output keeps git's full error.
- Failed copies during `update` name the cause in the recovery hint.
- Failure lines in `update` no longer print the key twice.
- `list` detail counts each skill folder and hook once, however many of its paths are recorded.
- Changing a version in `list` reports why versions couldn't be fetched instead of failing.

🐛 Fixed
- Remote git reads (version checks, tag lists, default branch) now retry on transient failures and time out, instead of failing on the first error.
- A retried clone starts from an emptied folder, so a partial first attempt no longer hides the real error.
- An SSH "Permission denied" is retried rather than failing immediately.
- Timed-out git commands now say they timed out and after how long.
