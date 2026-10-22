🔧 Changed
- The commit-msg hook is skipped in CI (`$CI` set) — automated commits are no longer prompted or blocked.
- Trunk build workflows now commit tracking updates with `--no-verify` — project git hooks such as Husky no longer run in CI.

🐛 Fixed
- The commit-msg hook now works under Husky and Linux `sh` — it no longer fails on `[[ ]]` syntax errors.
- The commit-msg hook no longer fails commits from GUI clients and scripts — with no terminal to prompt on, it warns on stderr and lets the commit through.
- Trunk builds no longer fail on push when a version already exists in ECR — they skip ahead to the next free calver or build-number version.
- Merge and internal stitch commit detection now checks only the subject line, not the whole commit message.
