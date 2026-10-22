✨ Added
- `switchboard sessions` lists today's sessions with their account, model, state, requests and API-equivalent worth, and `sessions <id>` shows one session's page with its accounts, moves and turns.
- `switchboard events` lists what the router decided and noticed — moves, limits, caps, pins, restarts — with `-f` to follow live.
- `switchboard history` now adds per-account totals, plan cost against worth, weekly peaks and a verdict on whether you have as many accounts as your use needs.
- `history --windows` prints each account's window readings over the days asked for.
- Optional `plan` per account, `week_starts` and `[prices]` settings to price plans and models yourself.
- `--json` and `--pretty` flags on the data commands to choose the output form wherever stdout is.
- Router "rounds" probe any account nothing has read in half an hour, and each account once in the 5 minutes before its weeks reset, so last-minute weekly use is read.
- The status document now carries each window's even pace and allowance, the pool of room for new sessions, and a list of what's coming up.

🔧 Changed
- `status`, `usage`, `sessions`, `requests`, `history` and `events` print JSON by default when output isn't a terminal — so agents need no flag.
- `history` counts only real messages as requests and shows weeks reset and notes for idle accounts only when something happened.
- Day summaries now include every account the readings history saw that day, whether or not it served a request.
- The request ledger no longer prunes a day's lines before its summary is written and stands.
- Tables no longer pad trailing empty cells.
- The Claude skill is updated to cover the new commands and JSON-by-default output.

🐛 Fixed
- A window that reset between readings no longer inflates a day's climb or double-counts limits when an older reading arrives late.
- An account's probe backoff and error reset when it gets a new or first usable token.
- A listing that fails midway through a prune no longer leaves ledger or readings days silently mis-summarised.
