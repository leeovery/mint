Look back over your past requests and what they cost, without the router running.

💥 Breaking
- The readings history and request ledger now keep 400 days by default, not 14 and 90 — set `keep` under `[history]` or `[ledger]` if you want less on disk.

✨ Added
- `switchboard requests` lists the requests the router has routed, with `--session`, `--account`, `--since` and `--json` to narrow or script it.
- `switchboard history` shows each day by account and model with requests, tokens, sessions, moves, limits reached and what they'd have cost through the API.
- Each day of the request ledger is summarised in `day-<date>.json` an hour after it ends, and the summary is kept for good after the day's lines are pruned.
- `keep = "forever"` under `[history]` and `[ledger]` never removes a day's file, and counts above 400d are now allowed.

🔧 Changed
- The dashboard's day chart now marks each day's start correctly where clocks change at midnight.
- Token counts now run to billions, as "1.2B", wherever they are shown.

🐛 Fixed
- A token pasted where a command, a help topic, a flag value or an invalid `--since` or interval goes is no longer echoed back in error messages — it shows as `[redacted]`.
