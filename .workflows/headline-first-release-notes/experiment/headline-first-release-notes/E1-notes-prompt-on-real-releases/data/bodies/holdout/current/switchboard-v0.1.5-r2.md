✨ Added
- `switchboard sessions` lists today's sessions — where each runs, what it's doing, its requests and what they'd have cost through the API — and `sessions <id>` shows one session's accounts, moves and turns.
- `switchboard events` shows what the router decided and noticed (sessions moved, limits, caps, pins, health), with `-f` to follow new events live.
- `switchboard history` now adds per-account totals, plan cost against worth, weekly peaks, and a verdict on whether you have as many accounts as your use needs.
- `history --windows` prints each account's window readings over the days asked for.
- Set `plan` on an account, `week_starts` at the top level, and `[prices]` overrides in the config to price plans and models for history and sessions.
- The router now reads accounts on its own rounds — any account unread for half an hour, and each one 5 minutes before its week resets — so usage stays current and a week's last use isn't missed.
- `status` now includes each window's even pace and allowance, the pool of room new sessions can use, and a soonest-first list of what's coming up.
- `--pretty` on `status`, `usage`, `requests`, `history`, `sessions` and `events` prints the human-readable form even off a terminal.

🔧 Changed
- `status`, `usage`, `requests`, `history`, `sessions` and `events` print JSON by default whenever output isn't a terminal, so agents need no `--json` flag.
- `--json` now forces JSON even on a terminal, and can't be combined with `--pretty` (or `--watch` on `usage`).
- `history` counts only real messages as requests, leaving out Claude Code's quota checks and token counts, and shows weeks reset in each account's daily notes.
- Days with no requests but account readings or week resets are now summarised too, so a week that ended on a quiet day keeps its peak.
- The request ledger no longer prunes a day's lines before that day's summary is written.
- The bundled Claude skill is updated to cover the new commands and the JSON-by-default behaviour.

🐛 Fixed
- Accounts that gain a token or switch to a new one start with fresh probe history rather than inheriting old failures and backoff.
- Table output no longer pads trailing empty cells.
