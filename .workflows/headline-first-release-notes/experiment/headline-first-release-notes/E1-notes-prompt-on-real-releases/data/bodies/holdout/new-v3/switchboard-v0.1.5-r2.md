Look back over your sessions, requests and days from the command line, with the router's own events alongside.

✨ Added
- `switchboard sessions` lists today's sessions with their account, model, state, requests and worth, and `sessions <id>` shows one session's accounts, moves and turns.
- `switchboard events` lists what the router decided and noticed, and `-f` follows new events as they're filed.
- `history` now totals each account's use against its plan's price, shows each week's peaks, and says whether you have as many accounts as your use needs.
- `history --windows` prints each account's window readings over the days asked for.
- Set an account's `plan` and a `[prices]` table in the config to price `history` and `sessions` against your own plans and model prices.
- `week_starts` in the config sets the day calendar weeks start on for `history`.
- `status` and `usage` JSON now include each window's even pace and allowance, the pool of room for new sessions, and what's coming up soonest first.
- The router reads accounts itself every half hour, and again just before each week resets, so usage stays current without a dashboard open.
- `--pretty` forces the human-readable form of `status`, `usage`, `requests`, `history`, `sessions` and `events`, even when output isn't a terminal.
- The Claude skill now covers sessions, requests, history and events.

🔧 Changed
- `status`, `usage`, `requests`, `history`, `sessions` and `events` print JSON by default when output isn't a terminal, so agents and scripts get data without `--json`.
- `history` no longer counts Claude Code's quota checks and token counts as requests.
- `history` shows an account's day with no requests only when it hit a limit or a week reset, and lists week resets in each day's notes.
- Request-ledger files are kept past their retention period until the day's summary is written.
- `status --session <id>` still prints the bare account id off a terminal.

🐛 Fixed
- `history` now includes days with no requests when accounts were read or a window reset, so a week that ended on a quiet day keeps its peak.
- Windows read late no longer inflate a day's rise or count a limit twice.
- An account given a new token or its first token no longer waits on an old probe failure or backoff.
