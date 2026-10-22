✨ Added
- A redesigned `switchboard usage` dashboard with three views (Accounts, Sessions, Runway) switched with `tab` — see every account's state, where sessions are routed, and when each account will have room.
- Account cards show the window that will stop an account first in big digits, with a burn-down, burn-rate or hourglass chart (`g`) and a window picker (`w`).
- Flip a card (`space`, or `s` for all) to see its sessions, pick one out with the arrow keys, and pin it to another account with a digit key.
- Sessions view draws a live switchboard of sessions, accounts and cords, with requests, streaming answers, refusals and re-patches shown as they happen, plus a LOG of what the router did and why.
- Runway view shows when each account has room, over the day or the week.
- Dashboard themes: `nord`, `tokyo-night`, `tokyo-night-day`, `amber`, `exchange` and `terminal`, a `t` theme picker, a light/dark pair chosen from the terminal's background, your own `.theme` files (Portal's format works as-is), and `NO_COLOR` support.
- `?` opens a help panel listing every key and what the dashboard's glyphs mean; the dashboard scrolls when cards don't fit (`j`/`k`, `PgUp`/`PgDn`, wheel).
- A phone-width layout for terminals under 100 columns.
- The router now counts each answer's streamed characters and closing token usage as it passes, and serves new history and live request-stream endpoints that feed the dashboard.
- The router keeps a list of recent events (sessions started, pressure, limits, moves, refusals, primes, restarts), shown as RECENT in the dashboard.
- `[history] keep = "14d"` sets how long the readings history is kept (8d to 400d), and older days are compressed to `.jsonl.gz`.
- `status` and the dashboard now show a session's own-pin state (yielded, or pinned since when).
- README demo clips and screenshots, with scenario-driven recording tooling under `demo/`.

🔧 Changed
- `reserve` now defaults to 0 on every account, the primary included — set `reserve = 0.1` to keep the old behaviour.
- The reserve is now explained as a general per-account feature rather than a primary-only one.
- Dashboard projections now show an account running out at its reserve, not just its limit, and are measured from when the document was read so they hold steady between reads.
- A restart waits a moment before refusing a config that fails to load, so an editor mid-save doesn't block it.
- README rewritten around tracking and analysing usage as well as routing.

🐛 Fixed
- A quiet restart no longer leaves a connection to the control socket hanging as the router shuts down.
- A 429 caused by one limit reached again while another had been reached since is now told apart, so notifications and move counts follow the right limit.
