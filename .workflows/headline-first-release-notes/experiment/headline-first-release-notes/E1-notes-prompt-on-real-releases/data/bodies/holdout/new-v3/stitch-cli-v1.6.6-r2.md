🔧 Changed
- The commit-msg hook skips itself in CI (when `$CI` is set), so automated commits are never blocked.
- The commit-msg hook warns on stderr and lets the commit through when no terminal is available — GUI clients and scripts no longer fail on a prompt.
- Trunk build workflows commit tracking updates with `--no-verify` — project git hooks such as Husky no longer break CI builds.

🐛 Fixed
- The commit-msg hook now works under Husky and Linux `sh` — it no longer fails on `[[ ]]` syntax errors.
- Trunk build workflows skip past versions already in ECR — a build whose tracking commit failed no longer causes later builds to fail on an immutable tag.
