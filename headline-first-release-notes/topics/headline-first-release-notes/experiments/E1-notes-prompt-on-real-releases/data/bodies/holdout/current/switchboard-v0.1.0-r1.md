✨ Added
- Rebuilt `usage` dashboard with a title row, a heading summarising the router, and a card per account showing its state in words, a big readout of the window that will stop it first, and a chart of it.
- Three views, switched with `tab`: Accounts, Sessions (a switchboard of which account each session's requests go to, with live request pulses and a log of what the router did and why) and Runway (when each account has room, over the day or the week).
- Cards flip with `space` or `s` to show each account's sessions and recent events, and a session can be picked out and pinned to another account with a digit key.
- Three chart styles for each card's featured window, cycled with `g`: burn-down, burn rate and hourglass; `w` chooses which window every card features.
- Theme support with a picker (`t`), six built-in themes (`nord`, `tokyo-night`, `tokyo-night-day`, `amber`, `exchange`, `terminal`), your own `.theme` files, and a light/dark pair chosen from the terminal's background.
- A help panel (`?`) listing every key and what the dashboard's glyphs mean, and mouse-wheel and `j`/`k` scrolling where the cards don't fit.
- A phone-sized layout for narrow terminals, and a one-account layout with COMING UP and RECENT beside the card.
- RECENT and per-card LATELY lists of what the router has done lately: sessions starting, moving, limits reached, pressure, primes and restarts.
- The router now keeps the readings history's size in check with a `[history] keep` setting (8 to 400 days), and compresses days older than two days.
- Live request stream from the router (`GET /stream`) and window history (`GET /history`) that power the dashboard's charts and animations.
- Pin a single running session from the dashboard, the same as `pin <account> --session` does.
- Recorded demo clips and stills in the README, with `demo/` scripts to record them again.

🔧 Changed
- Reserves are now optional on any account and default to none; the primary no longer keeps 10% back by default.
- `usage` printed once now draws the full Accounts view in your chosen theme, using the router's sessions and history.
- Projections now say an account reaches its reserve rather than its limit when the reserve holds it back.
- The dashboard remembers the view, featured window, chart style and theme between runs.
- When the router stops during a restart, a launcher no longer finds a dead listener on its port.
- README reframed around tracking, routing and analysing your Claude usage across accounts.

🐛 Fixed
- A config file caught mid-save no longer makes the router refuse a restart; it waits for the next look to confirm it's invalid.
- The client's quota check no longer starts or moves a session.
- A limit's moves are now counted correctly whichever order the router hears of them in.
