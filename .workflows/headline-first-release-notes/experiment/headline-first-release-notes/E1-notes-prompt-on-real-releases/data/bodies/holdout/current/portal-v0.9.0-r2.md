✨ Added
- Multi-select mode in the session picker — press `m` to mark sessions, Enter to open them all, Esc to cancel.
- Open several sessions at once in separate host-terminal windows, with the last one attaching in the current window.
- New `portal spawn` command to open one or more sessions in host-terminal windows from the command line.
- `portal spawn --detect` prints the detected host terminal without opening anything.
- Ghostty support for opening session windows natively on macOS.
- Custom terminal support via `terminals.json` — define your own argv or script recipe for terminals Portal doesn't know.
- Picker shows a proactive warning banner when your terminal can't open extra windows (or when no host-local terminal is detected, such as over mosh).
- A live `Opening n/N…` progress band while windows open, cancellable with Ctrl-C or Esc.
- Multi-select footer with navigate, toggle, preview, open and cancel hints.

🔧 Changed
- If a session disappears before opening, the batch aborts with a "gone — nothing opened" banner and the remaining marked sessions stay selected for retry.
- If some windows fail to open, the ones that opened are left in place and only the failed sessions stay marked for a retry.
- Row actions that don't combine with a marked set (kill, rename, new session, page toggle) are disabled in multi-select mode.
