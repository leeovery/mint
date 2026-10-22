🔧 Changed
- The commit-msg hook now runs under POSIX sh — it works with Husky and on Linux systems where `sh` is dash.
- Trunk build workflows (Go, Laravel, Vue) commit tracking updates with `--no-verify` — project developer hooks no longer run in CI.

🐛 Fixed
- The commit-msg hook is skipped in CI and, with no terminal to prompt on (GUI clients, scripts), warns instead of failing the commit.
- Trunk builds skip calver and build-number versions that already exist in ECR — no more push failures after a build whose tracking commit failed.
