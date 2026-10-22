Look back over your usage with new `requests` and `history` commands, without needing the router running.

✨ Added
- `switchboard requests` lists the requests the router has routed, from its request ledger — filter by session or account, start from a day, a time or "3h ago", or print JSON.
- `switchboard history` shows each day of the ledger by account and model with requests, tokens, sessions and what they'd have cost through the API — also shows moves, limits reached and highest window use.
- The router now writes a summary of each finished day beside the ledger, kept for good even after the day's request lines are pruned.
- `keep = "forever"` in `[history]` and `[ledger]` keeps every day's file and never removes one.

🔧 Changed
- The readings history and request ledger are now kept 400 days by default, up from 14 and 90.
- The `keep` setting for both accepts any number of days from `8d` up, with no 400-day ceiling.
- The dashboard's day chart now marks day starts correctly where the clocks change at midnight.

🐛 Fixed
- Commands no longer echo a token pasted as an argument, a flag's value or a config path in their error messages — it shows as `[redacted]`.
