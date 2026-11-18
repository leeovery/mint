🔧 Changed
- Commit-msg hook is now POSIX sh — it works under Husky and Linux `sh` (dash), which lack `[[ ]]`.
- Commit-msg hook skips entirely in CI when `$CI` is set.
- Commit-msg hook with no terminal to prompt on (GUI clients, scripts) warns on stderr and lets the commit through instead of failing.
- Trunk build workflows for Go, Laravel and Vue commit tracking updates with `--no-verify` — project developer hooks no longer break CI builds.

🐛 Fixed
- Trunk builds skip calver and build-number versions that already exist in ECR — a prior build whose tracking commit failed no longer causes an immutable-tag push failure.
- Commit-msg hook no longer fails under Husky on systems where `sh` is dash.
- Commit-msg hook only checks the subject line for merge, internal stitch and conventional-format matching, so multi-line messages are handled correctly.
