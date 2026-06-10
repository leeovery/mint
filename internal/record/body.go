// Package record persists a finished release into the repo (Stage 5): it writes
// the CHANGELOG entry and builds the single release-bookkeeping commit that the
// tag will point at.
//
// Two concerns live here, both kept testable without touching the host:
//   - The changelog writer (WriteChangelog) operates on a repo-root directory
//     passed in as a parameter and injects the release date, so file effects are
//     driven against a t.TempDir() and the section header is exactly assertable.
//   - The bookkeeping commit (Commit) stages and commits through the
//     runner.CommandRunner seam, so the git invocations — including the exact
//     commit subject — are scripted and asserted with a FakeRunner, never real
//     git.
//
// FirstReleaseBody is the fixed no-AI notes body for the first release. Per
// Notes-path precedence (1), a first release (no prior tag) has no diff base, so
// mint skips the AI and records this constant. It is a pure value with no runner
// dependency, so obtaining it can never spend an AI call.
package record

// FirstReleaseBody is the fixed release-notes body for a first release. The first
// release has no prior tag and therefore no diff to summarise, so mint never
// invokes the AI and records this verbatim. It is deliberately a plain constant
// (not derived from any command) so the no-AI guarantee is structural: reading it
// touches nothing external.
const FirstReleaseBody = "Initial release."
