✨ Added
- Grouped, streamed `update` (no key) — plugins sharing a repo and version intent are checked and cloned once per group, so a 10-member collection clones once instead of ten times.
- Per-group progress output during update-all — an `Updating <repo> <old> -> <new> (N members)` header with a line per member, then a trailing summary.

🔧 Changed
- Update messages show semver tags (`v1.2.3 -> v1.3.0`) when both refs are real tags and the ref moved, and short commit hashes otherwise — in both single-key and update-all.
- The trailing update summary collapses to one line per group for up-to-date counts, newer-tag notices, check failures and unmatched constraints, instead of one line per plugin.
- The "newer versions outside constraints" footer is now actionable — it shows the current and newest versions and the exact `npx agntc add` command to upgrade.
- A clone failure in update-all prints one line naming the affected members rather than a line per member.
- Newer-tag notices for tag-pinned plugins now include the `npx agntc add <repo>@<newest>` upgrade command.

🐛 Fixed
- The out-of-constraint footer now reports the version you actually landed on after a same-run update, instead of the stale pre-update version.
