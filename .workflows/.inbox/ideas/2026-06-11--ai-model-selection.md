# AI model selection and ai_command default consolidation

The default `ai_command` is `claude -p`, which inherits whatever default model the operator's Claude CLI is set to — likely an overpowered (and more expensive/slower) model than these tasks need. The immediate want is to pin a model in the shipped default, probably via the alias form (`claude -p --model sonnet`) rather than a full model ID, since full IDs baked into the binary go stale as new model versions ship.

That small change opens a bigger discussion: should the two verbs use different models at all? Release notes are a salience-heavy task — judging what matters across a whole change map, grouping, audience-facing tone — which argues for Sonnet or Opus. Commit messages are frequent and latency-sensitive, summarising one bounded staged diff, where Haiku might be sufficient and noticeably faster. The commit-command spec currently mandates a single shared top-level `ai_command` with explicitly no per-verb override (commit spec lines 216 and 224 — "promote to a `[commit]` key only if a real need appears"). This idea is potentially that real need surfacing: decide whether per-verb model choice (e.g. a `[commit]` override falling back to the shared key) is warranted, or whether one pinned shared model serves both verbs well enough.

There's also a code-health angle: the default command string is currently spread across three sites — `internal/config/config.go:75`, `internal/ai/transport.go:45`, and the `mint init` scaffold in `internal/initgen/initgen.go:39` — plus test pins and the documented default in both specifications. Changing the default means touching all of them in step. Worth discussing whether the default should have a single source of truth so model/default changes are a one-site edit rather than a coordinated sweep.

One related constraint to keep in view: the transport's per-attempt timeout is 60 seconds and timeouts are fatal (not retried), so slower models like Opus interact directly with that deadline — model choice and timeout policy are coupled decisions.

Captured 2026-06-11 from a conversation about overriding the model used for generation.
