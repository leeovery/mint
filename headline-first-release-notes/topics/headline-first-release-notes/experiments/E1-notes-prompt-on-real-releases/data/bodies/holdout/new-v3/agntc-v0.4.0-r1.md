Updating all plugins now clones each repo once and streams progress per group instead of cloning for every plugin.

✨ Added
- Grouped, streamed `update` with no key — plugins sharing a repo and version intent are checked and cloned once, with live progress under a per-repo header and one line per member.
- Out-of-constraint footer now gives the move and the exact command — e.g. `v1.3.0 -> v2.0.0 available. To upgrade: npx agntc add owner/repo`.

🔧 Changed
- Update-all trailing summary collapses to one line per repo for up-to-date counts, newer tags, failed checks and unmatched constraints instead of one line per plugin.
- Version moves show semver tags (`v1.2.3 -> v1.3.0`) when both refs are real tags and the ref moved, otherwise short commit hashes, in both single-key and update-all output.
- A clone failure in update-all prints one line naming the affected members instead of a line per member.
- The out-of-constraint footer's "current" version now reflects the version the run just landed on rather than the pre-update one.
