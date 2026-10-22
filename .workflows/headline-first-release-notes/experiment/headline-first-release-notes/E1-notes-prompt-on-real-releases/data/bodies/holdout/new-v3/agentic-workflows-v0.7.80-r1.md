Embedding-provider rate limits no longer fail knowledge indexing and setup outright — the CLI now waits them out.

✨ Added
- Rate-limited embedding requests wait the time the provider names and retry, up to five times and 60 seconds in total per command.

🔧 Changed
- Accounts out of quota now fail immediately with a clear message and a pointer to add credit, instead of being treated as a rate limit.
- Setup shows new hints for quota and persistent rate-limit errors when validating a provider.

🐛 Fixed
- Planning no longer queries the knowledge base for cross-cutting context when no completed cross-cutting spec exists, and skips straight on when no cross-cutting work units exist.
- An invalid `--boost` directive on `knowledge query` now reports a clear error instead of being handled inconsistently.
