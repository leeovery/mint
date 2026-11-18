✨ Added
- Rate-limited embedding requests now wait the time the provider names and retry — within a 60-second budget per command, so a throttled index no longer fails outright.
- Retrieval eval harness with judged cases over a frozen corpus from three real projects — keyword mode gates the test suite against a pinned baseline.

🔧 Changed
- Accounts out of quota now fail immediately with a clear "add credit" message instead of being retried.
- Setup's rate-limit and quota hints now say what to do next rather than "wait and retry".
- Planning's cross-cutting context step now skips the knowledge query when no completed cross-cutting spec exists.
- Invalid `--boost` directives on `query` are reported up front as a clear error.

🐛 Fixed
- Filtered knowledge-store reads no longer allocate a fixed million-entry results array — they size to the store.
