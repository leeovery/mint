Look back over your sessions, requests and days from the command line, with the router reading your accounts around the clock.

✨ Added
- `switchboard sessions` lists today's sessions with their account, model, state, requests and API-price worth, and `sessions <id>` shows one session's accounts, moves and turns.
- `switchboard events` lists what the router decided and noticed, with `-f` to follow new events as they're filed.
- `history` now totals each account over the days, with its plan, worth against price, each week's peaks, and a verdict on whether you have as many accounts as the use needs.
- `history --windows` prints each account's window readings over the days asked for.
- Set `plan` on an account, `week_starts` at the top level, and `[prices]` overrides in the config to price accounts against their plans and change how history counts weeks.
- The router probes accounts nothing has read in half an hour, and each account shortly before its weeks reset, so usage stays current without traffic.
- The status document now includes each window's even pace and allowance, the pool of room for new sessions, and a soonest-first list of what's coming up.
- `--pretty` prints the human-readable form of `status`, `usage`, `requests`, `history`, `sessions` and `events`, even off a terminal.

🔧 Changed
- `status`, `usage`, `requests`, `history`, `sessions` and `events` print JSON whenever output isn't a terminal, so scripts and agents need no `--json` flag.
- `history` counts only messages as requests, leaving out Claude Code's quota checks and token counts, and lists weeks reset in each account's day notes.
- The Claude Code skill is updated to cover the new commands and JSON defaults.

🐛 Fixed
- Request ledger lines are no longer pruned before their day's summary is written.
- A week that ended on a quiet day now keeps its peak, since days with readings but no requests are summarised too.
- `history` no longer counts a limit a window was already at when the history began as one reached that day.
- Probe backoff is reset when an account gets a new or first token, so it is read promptly.
