Look back at your past requests and what they cost, without the router running.

💥 Breaking
- Readings history and request ledger files are now kept 400 days by default instead of 14 and 90 — set `keep` under `[history]` or `[ledger]` if you want less on disk.

✨ Added
- `switchboard requests` lists the requests the router has routed, filterable with `--session`, `--account` and `--since`, with `--json` for scripts and agents.
- `switchboard history` shows each day by account and model with requests, tokens, sessions and what they'd have cost through the API, plus moves, limits reached and windows' highest use.
- A daily summary of the request ledger is written an hour after each day ends and kept for good, even after the day's request lines are pruned.
- `keep = "forever"` under `[history]` and `[ledger]` stops the files ever being removed.
- The bundled skill now tells agents how to read `switchboard requests --json` and `switchboard history --json`.

🔧 Changed
- Error messages no longer repeat anything shaped like a token pasted as a command, argument or flag value, showing `[redacted]` instead.
- The dashboard's day axis places day marks correctly where the clocks change over midnight, and no longer crowds hour labels beside a day name.
