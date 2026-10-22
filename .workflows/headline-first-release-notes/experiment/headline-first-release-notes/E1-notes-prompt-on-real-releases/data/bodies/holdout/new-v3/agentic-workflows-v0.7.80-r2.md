Rate-limited embedding requests now wait and retry instead of failing the knowledge index straight away.

🔧 Changed
- Rate-limited embedding requests (HTTP 429) wait the time the provider names and retry, up to 60 seconds in all per command — indexing no longer fails on a brief rate limit.
- Cross-cutting context in planning is only queried when a completed cross-cutting spec exists, and otherwise proceeds without it.
- Setup's rate-limit message now says the limit held through its own waits and points to your plan's rate limits.

🐛 Fixed
- An account out of quota (HTTP 429) now fails at once with a clear message to add credit, instead of being retried pointlessly.
- An invalid `--boost` directive on `query` now reports its error cleanly.
