Packs can now ship shared folders alongside their skills, and updates no longer silently delete folders the repo still has.

✨ Added
- Pack support folders — folders in a pack's `skills/` with no skill in them (shared references, gates) now install whole to `.claude/skills/<name>/` beside its skills.
- Pack counts include support folders in the add menu, summaries and `list` detail, e.g. `2 skills, 3 folders, 1 agent`.
- `add --list` JSON output now reports each pack's support folders.
- Notes for a skill taken out of a pack now name the support folders left behind.

🔧 Changed
- A pack that holds two folders installing to the same place is now refused with "can't be installed as it stands" instead of offering a choice, and `update` aborts with no remedy.
- Update check failures print one line with the cause — `<repo>: couldn't check for updates (<cause>) — left untouched` — and `list` and version-change report failures the same way.
- Failed update lines no longer repeat the entry key, and copy-failure hints now name the cause.
- `list` detail counts each installed skill folder, hook and agent once, however many paths are recorded.
- A support folder landing on a loose skill from the same source is now reported as files in the way instead of being absorbed.

🐛 Fixed
- `update` now stops, naming the folders, instead of silently removing installed skill folders the repo still holds but would no longer install, and prints the `remove` + `add` that updates it anyway.
- Remote git reads (tag lists, update checks) now retry on transient failures, and a hung command times out with a clear message instead of failing on the first error.
- Cloning retries into a clean folder, so a partial clone no longer hides the real error, and an SSH "Permission denied" is retried rather than failing at once.
- Failed version fetches in `list` no longer crash the flow, and instead report why and leave the install untouched.
