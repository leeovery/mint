✨ Added
- `switchboard requests` lists the routed requests from the request ledger — filter by session or account, start from a day, time or "3h ago", or print JSON; works without the router.
- `switchboard history` shows each day by account and model with requests, tokens, sessions and what they'd have cost through the API — also available as `--json`, and works without the router.
- Daily ledger summaries (`day-<date>.json`) are written an hour after each day ends and kept for good, even after the day's request lines are pruned.
- `keep = "forever"` for `[history]` and `[ledger]` — never removes a day's files.

🔧 Changed
- The readings history and request ledger are now kept for 400 days by default, up from 14 and 90.
- The `keep` setting now accepts any number of days from `8d` up, with no 400-day cap.
- The dashboard's day chart places day labels and hour ticks correctly around daylight-saving changes.
- The bundled Claude Code skill documents the new `requests` and `history` commands.

🐛 Fixed
- Reading a day's files no longer misses lines when the day is compressed mid-read.
- Request lines are now returned in the order they arrived, even when a long request finished after later ones.

🔒 Security
- Tokens pasted as an unexpected argument, flag value or `--since` value are shown as `[redacted]` in error messages instead of being echoed back.
