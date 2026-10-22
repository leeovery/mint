Look back at your past use with the new `requests` and `history` commands, which read the request ledger with or without the router running.

✨ Added
- `switchboard requests` lists the requests the router has routed, with `--session`, `--account`, `--since` and `--json` to narrow or script it.
- `switchboard history` shows each day by account and model with requests, tokens, sessions, moves, limits reached and what it would have cost through the API.
- The request ledger now keeps a summary of each day once it ends, and keeps it for good even after the day's lines are pruned.
- `keep` for the readings history and request ledger accepts `forever`, which never removes a day's files.

🔧 Changed
- The readings history and request ledger are now kept 400 days by default, up from 14 and 90.
- `keep` now accepts any number of days from `8d` up, not just to `400d`.
- The dashboard's day view marks each day's start at the right time where the clocks change at midnight.

🐛 Fixed
- Pasted tokens are no longer echoed back in errors for bad arguments, flag values, `--since` values, unknown commands or the config path.
