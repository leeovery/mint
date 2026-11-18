The dashboard is rebuilt around views, charts and themes, so you can see where each account's usage is heading and where your sessions go.

💥 Breaking
- Only the primary keeps no default reserve any more — the primary's built-in 10% reserve is gone, so set `reserve = 0.1` on the account if you want the router to keep leaving that share for the Claude apps.

✨ Added
- Three dashboard views, switched with `tab`: Accounts, Sessions (where each session's requests go, drawn as cords from sessions to accounts) and Runway (when each account has room, over the day or the week).
- Account cards now show their state in words, the window that will stop them first in big digits, and a chart of it — a burn-down, burn rate or hourglass, cycled with `g` — with `w` choosing the window.
- Flip a card with `space` or `s` to see its sessions, pick one with `↑↓`, and pin it to another account with a digit or clear its pin with `a`.
- Themes for the dashboard: built-in `nord`, `tokyo-night`, `tokyo-night-day`, `amber`, `exchange` and `terminal`, a `t` picker with live preview, and your own `.theme` files in the themes directory.
- The dashboard picks a light or dark theme to match your terminal, and `NO_COLOR` draws it without colour.
- A heading, COMING UP and RECENT strips, and a `?` help showing every key and what the glyphs mean.
- Charts draw from the router's history of each window's use, and `usage` printed once now includes them.
- The router keeps a recent-events list and a live request stream, so the dashboard shows requests going out, answers streaming back with token counts, limits, refusals and session moves as they happen.
- Any account can keep a `reserve`, not just the primary.
- `[history] keep = "14d"` sets how long the readings history is kept, from `8d` to `400d`.
- Cards scroll with `j`/`k`, `PgUp`/`PgDn` or the wheel when they don't fit, and the layout adapts down to a phone-width terminal.
- The dashboard remembers your view, featured window, chart style and theme between runs.

🔧 Changed
- Readings history files are compressed to `.jsonl.gz` once their day ended two days ago.
- Window projections now say a window runs out only if it reaches its limit before it resets, and use the reserve as the floor where one holds an account back.

🐛 Fixed
- A restart racing a config file caught mid-save no longer refuses the config; it looks again before rejecting it.
- A router stopping while handing over to a new one no longer leaves launchers hanging on its address.
