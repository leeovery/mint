Look back at your usage with `switchboard requests` and `switchboard history`, read straight from the request ledger.

✨ Added
- `switchboard requests` lists the requests the router has routed — filter by `--session` or `--account`, start with `--since`, or print `--json` for scripts and agents.
- `switchboard history` shows each day by account and model with requests, tokens, sessions and what they'd have cost through the API — also with `--json`.
- Daily summaries of the request ledger, written beside its files and kept for good after the day's lines are pruned.
- `keep = "forever"` for `[history]` and `[ledger]` — never removes a day's files.
- The bundled skill now teaches agents to use `requests --json` and `history --json`.

🔧 Changed
- The readings history and request ledger are now kept for 400 days by default, up from 14 and 90.
- `keep` in `[history]` and `[ledger]` now accepts any number of days from `8d` up, no longer capped at `400d`.

🐛 Fixed
- Dashboard day and hour labels now land correctly on days where the clocks change at midnight.

🔒 Security
- Tokens pasted as an unexpected argument, flag value, `--since` value or session id are shown as `[redacted]` in error messages.
