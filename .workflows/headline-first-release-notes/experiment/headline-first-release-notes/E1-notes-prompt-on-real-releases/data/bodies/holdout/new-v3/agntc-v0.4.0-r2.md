Update-all now checks and clones each repo once instead of once per plugin, and streams progress as it goes.

✨ Added
- Streamed `update` progress — each repo shows an `Updating <repo> <old> -> <new> (N members)` header with a per-plugin line beneath it as it finishes.
- Out-of-constraint notices now give an exact re-add command, such as `npx agntc add owner/repo` for caret users or `npx agntc add owner/repo@<newest>` for exact pins.

🔧 Changed
- `update` with no argument checks and clones each repo once per version intent, so a 10-plugin collection clones once instead of ten times.
- The closing `update` summary collapses to one line per repo for up-to-date counts, newer-tag notices, check failures and unmatched constraints.
- Version moves display as semver tags (`v1.2.3 -> v1.3.0`) when both refs are genuine tags and the ref changed, and as short commit hashes otherwise, in both single-plugin and update-all output.
- The out-of-constraint footer names the version you landed on this run as the current version, not the pre-update one.
- A repo whose clone fails shows a single line naming the affected plugins instead of one line per plugin.
- Failed updates now display as errors rather than warnings in all update-all layouts.
