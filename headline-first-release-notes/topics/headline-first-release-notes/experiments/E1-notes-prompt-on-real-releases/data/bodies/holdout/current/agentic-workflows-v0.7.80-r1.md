✨ Added
- Retrieval eval harness with judged cases over a frozen corpus — keyword-mode results are gated against a pinned baseline, so retrieval changes show their before and after.

🔧 Changed
- Rate-limited embedding requests (HTTP 429) now wait the time the provider names and retry, up to a 60-second total per command — large indexing runs survive rate limits instead of failing.
- Setup and indexing report an out-of-quota account as its own error with billing guidance — no pointless retries.
- Planning's cross-cutting context step skips the knowledge query when no completed cross-cutting spec exists — less noise and no wasted lookup.
- An invalid `--boost` directive on `query` now reports a clear error before any search runs.

🐛 Fixed
- Filtered store reads no longer rely on a fixed one-million result cap — removals and counts stay correct however large the store grows.
