Opening past days in the Log and session pages is faster, because finished days are now digested once and read back from saved results instead of re-reading every raw line.

✨ Added
- Finished days are now digested in the background, so session pages and the Log's past days load from saved results and fall back to the raw lines when none stand.

🔧 Changed
- The Log reads past days back through its filters, so reaching further back only reads the days that can hold matching requests.
- A session's turn details read only the days and stretches of the ledger that the turn spans.
- COMING UP on the Runway view no longer numbers accounts when the account column is collapsed — the numbers are gone from the lines and the help text.

🐛 Fixed
- The Log's find no longer applies a stale read of day summaries after it has been closed.
- The Log's Events no longer show days that were only partly read through a request filter.
