✨ Added
- Per-day digests of the request ledger — the router now digests each finished day in the background, storing session states so later reads resume from them instead of replaying every line.
- Per-day heads and row files for the Log — past days load their sessions and rows from stored summaries, so the Log's find and scroll-back open faster.
- Faster scroll-back for a session's turn — turns read only the days and row chunks that can hold their lines.

🔧 Changed
- The Log reads past days through the active filter, and re-reads days when the filter widens — fewer lines loaded while the Requests table stays consistent.
- Session pages rest on stored digest states and re-read a session whole only when those can't be used.
- Session page and turn totals are now priced when the page is built, so costs follow the prices of the day shown.
- The Log's find drops its day summaries when it closes and ignores reads that land afterwards.
- The Log's Events only show days that have been read in full, so a card never reports a day held through a filter.
- COMING UP no longer numbers accounts in the collapsed Runway view.

🐛 Fixed
- Request lines that the ledger files hold twice are now counted once, consistently across pages, turns, points and the Log.
- Reading a session's lines back from the end of a day's file no longer scans the whole file.
