✨ Added
- Group-first `update` (no key) — entries from the same repo and version intent are checked once and cloned once, so a 10-member collection clones once instead of ten times.
- Streamed per-group progress — each repo shows an `Updating <repo> <old> -> <new> (N members)` header with a `✓ member → agents` line per member as it finishes.

🔧 Changed
- Version moves display as semver tags (`v1.2.3 -> v1.3.0`) when both refs are real tags and the ref moved, and as short commit hashes otherwise — in both single-key and update-all output.
- The trailing update summary collapses to one line per repo for up-to-date counts, newer-tag notices, check failures, and unmatched constraints instead of one line per member.
- The out-of-constraint footer is now actionable — it shows `current -> newest available` and the exact re-add command (`npx agntc add owner/repo`).
- The out-of-constraint footer names the version you actually landed on after a same-run bump, so it agrees with the `Updated` line.
- A repo clone failure during update-all prints one line naming the affected members instead of one failure line per member.
- Single-entry updates now show the bare repo with a collapsed one-line result instead of a header plus a separate line.
- Failed members render as red errors consistently across all update layouts.

🐛 Fixed
- A constrained plugin updated on its own stays grouped with its behind siblings, so the rest of the collection still updates in one clone.
- A failed update check for a repo no longer repeats per member — it warns once, with exit code 0.
