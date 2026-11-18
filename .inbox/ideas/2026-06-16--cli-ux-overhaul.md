# Mint CLI UI/UX overhaul

Now that mint is functionally complete, its output experience deserves a deliberate, end-to-end pass. The tool does the right things, but the presentation has grown organically alongside the features and the seams show. This is a holistic UI/UX overhaul of the whole CLI — every verb and every shared surface — not a one-off fix to a single command.

The surfaces in scope are all of them: the release spine (`mint release`), regenerate in both single and `--all` batch modes, `mint commit`, `mint init`, and version output. Underneath those sit the shared presenter primitives that every verb composes from — the brand/run lines, the `Plan` block, blocking and short stages, spinners, gates and prompts, warnings, content panels (the gutter), and end-of-run summaries — across both the pretty (TTY) and plain (pipe/script) renderers. The goal is a coherent, considered experience everywhere, where each surface reads at the right altitude and the vocabulary stays consistent verb to verb.

The clearest worked example — the thing that triggered this — is `mint release regenerate --all --source tag --target changelog --yes`. Run against a 39-tag repo it worked correctly (14 generated, 25 skipped, clean changelog rebuild and push) but produced ~234 lines of scrollback to say "I read 39 tags, generated 14, skipped 25," then closed with one flattened summary line repeating the same 38-character skip reason 25 times. The root cause: the batch path has no narration of its own, so it replays the single-version events once per version — the brand banner and `Plan` block repeat 39 times, and `⚠` is used for skips that are entirely normal for `--source tag`. That's one symptom of patterns that likely recur across the other surfaces.

Cross-cutting themes to carry through the whole overhaul, drawn from that example but not specific to it:

- **Altitude and repetition** — emit once-per-run things once; don't replay single-item narration across a batch.
- **Glyph semantics** — `⚠` should mean something is wrong, not flag an expected outcome.
- **Label honesty** — "Generated" vs "read/reused" for deterministic sources; spinners and `(0.0s)` timings on instant operations are noise.
- **Single vs batch** — batch is a first-class shape (header / per-item line / grouped footer), not a loop of single-run blocks.
- **Structured summaries** — `RunResult.Summary` as a pre-formatted string (`internal/engine/regenerate_batch.go:401`) forces the engine to do presentation and leaves the presenter nothing to lay out; summaries want structured payloads the presenter formats and groups.
- **Consistency** — the same events render coherently across verbs and degrade gracefully between pretty and plain.

Relevant code: `internal/presenter` (the only output surface; `pretty.go`, `plain.go`), `internal/engine` (event emission, e.g. `regenerate_batch.go:256`). Open question for planning: sequence the overhaul surface by surface, or start by reworking the shared presenter contract (batch events, structured summaries) and let the verbs fall out of it.
