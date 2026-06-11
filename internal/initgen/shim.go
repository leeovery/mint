package initgen

import "os"

// ShimMode is the executable file mode the `release` shim must be written with so
// `./release` is runnable straight after clone. The actual write/chmod lives in the
// `mint init` writer task; this generator owns the content and the documented mode.
const ShimMode os.FileMode = 0o755

// ReleaseShim returns the per-project `release` shim as a single POSIX `sh` script.
// It is a PURE generator: it returns a string and performs NO filesystem or git IO.
//
// The shim is committed to a project so `./release` works for anyone who clones,
// delegating to the globally-installed `mint`:
//
//   - It guards on `command -v mint` — the portable presence check (POSIX, no
//     reliance on `which`).
//   - When mint is ABSENT it prints the exact install hint to STDERR and exits
//     non-zero, so a fresh clone gets an actionable message rather than a cryptic
//     "command not found".
//   - When mint is PRESENT it `exec`s `mint release "$@"`: `exec` REPLACES the shim
//     process so signals and the exit code propagate cleanly, and `"$@"` forwards
//     every argument verbatim (`./release -m` becomes `mint release -m`).
func ReleaseShim() string {
	return `#!/usr/bin/env sh
if ! command -v mint >/dev/null 2>&1; then
  echo "mint is not installed. Install it with: brew install leeovery/tools/mint" >&2
  exit 1
fi
exec mint release "$@"
`
}
