# `regenerate --all` prints its done summary before the changelog is committed and pushed

`mint release regenerate --all` with a `changelog` or `both` target prints its end-of-run summary (`done: {project} N regenerated, M skipped: …`) before the batch's changelog work happens. It rebuilds `CHANGELOG.md` whole, commits it as `docs(changelog): regenerate release notes`, and pushes it once, at the end of the batch. The summary comes before all of that.

The order reads wrong whenever the changelog step fails. If the commit or the push fails, the user sees a "done" line saying every version was regenerated, then a failure after it, and the run exits 1. The summary has already claimed success for a run that ends in failure. The changelog step is the batch's only git mutation, and with a `both` target the provider releases are all written by then, so the closing summary matters: it's the line a user reads to learn what landed.

It shows whenever the batch writes the changelog (`--target changelog` or `--target both`). Release-only batches are unaffected, since they have no end-of-batch step after the summary.

This came up in the docs review after the token-aware-diff-sizing feature. The batch loop emits the summary as it finishes the versions (`internal/engine/regenerate_batch.go`, around lines 219-223), and the changelog rebuild runs after the loop returns (`internal/engine/regenerate_batch.go`, around line 158, into `internal/engine/regenerate_batch_changelog.go`). The README's "Regenerating every version" section documents the end-of-batch commit and push, and says nothing about where the summary falls.
