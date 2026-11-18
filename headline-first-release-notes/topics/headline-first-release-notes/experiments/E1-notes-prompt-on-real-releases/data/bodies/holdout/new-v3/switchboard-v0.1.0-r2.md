The dashboard now shows where your Claude usage is heading, which sessions are on which account, and when each account will have room, in one redesigned screen.

💥 Breaking
- Accounts other than the primary and the primary itself no longer get a default reserve — the primary's default of `0.1` is gone, so set `reserve = 0.1` on it in the config if you want the router to keep leaving a tenth for the Claude apps.

✨ Added
- Redesigned `usage` dashboard with three views, switched by `tab` — Accounts (a card per account), Sessions and Runway.
- Account cards feature one window in big digits with a chart, drawn as a burn-down, burn rate or hourglass (`g`), with the featured window chosen by `w`.
- Cards flip (`space`, `s`) to show their sessions, and a session can be picked out, then pinned to another account with a digit or released with `a`.
- Sessions view draws a switchboard of where each session's requests go, with live requests, answers streaming back, refusals and re-patching at a limit, plus a LOG of why.
- Runway view shows when each account has room, over the day or the week (`w`).
- Themes for the dashboard: built-in `nord`, `tokyo-night`, `tokyo-night-day`, `amber`, `exchange` and `terminal`, a picker on `t`, your own `.theme` files, and light/dark pairs chosen from the terminal's background; `NO_COLOR` is honoured.
- `?` opens every key and what the glyphs mean, and the dashboard scrolls with `j`/`k`, `PgUp`/`PgDn` and the wheel.
- The router tells of recent events (sessions starting, pressure, limits, moves, refusals, primes) and streams requests live, with tokens counted from each answer.
- The router serves a window's usage history, which the charts draw from.
- `[history] keep` in the config sets how long the readings history is kept, from `8d` to `400d`.

🔧 Changed
- Any account can keep a reserve, not just the primary.
- Readings history files are compressed once two days old.
- Projections now show a window as running out only when it reaches its limit before it resets.

🐛 Fixed
- Claude Code's startup quota check no longer registers a session or moves one between accounts.
- A config file caught half-saved no longer triggers a refused change or an early restart.
- A stopping router no longer leaves launchers hanging on its proxy address.
