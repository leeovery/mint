Past days load faster in the Log and session pages, because finished days are now digested and their stored results are reused instead of re-read from the raw request lines.

✨ Added
- Finished days of the request ledger are digested in the background — the Log's past days, its session find, and session pages and turns read from stored results and fall back to the raw lines when none stand.

🔧 Changed
- The Log reads past days through its filter and reads days again as the filter changes — rows shown while a wider filter loads stay as they were instead of switching to a partial table.
- A session's page, its turns and its totals are priced when shown, so a page opened on a later day shows what its requests cost at that day's prices.
- The Log's Events no longer show days that haven't yet been read in full under a narrowed Requests filter.
- Closing the Log's find now drops what it read, so reopening it reads the days' sessions afresh.

🗑️ Removed
- The account numbers in COMING UP on Runway are gone, along with the note that tied them to the collapsed lanes.

🐛 Fixed
- Request ledger lines for a session are read back from the end of a day's file, so opening a session no longer reads the whole of a long day.
