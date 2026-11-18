✨ Added
- Daily digests of the request ledger — the router now builds a per-session digest, parts file and day head for each finished day in the background.
- Faster Log and session pages — past days and session pages restore from stored digests and rows instead of re-reading every raw ledger line.
- Log filters read back through past days smartly — days are re-read only as far as needed when you change a filter, and days that can't match are skipped.

🔧 Changed
- COMING UP on the Runway no longer numbers accounts when the account column is collapsed — the lane numbers are dropped from the list and its help text.
- Session turns and totals are now priced when the page is built — costs reflect the prices of the day you view them.
- Reading a session's recent lines is quicker — the ledger reads a day's file backwards from its end and stops once it passes the turn or span you need.
- The Log's find closes cleanly — it drops its cached day summaries and ignores late reads once closed.

🐛 Fixed
- The Log no longer shows stale or misplaced rows after a filter change — tables built through an older filter are refilled before they show.
- Lines the ledger holds twice are now counted once across pages, turns and the Log.
