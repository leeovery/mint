# Specification: Headline First Release Notes

## Specification

### 1. Purpose and Scope

#### 1.1 What this changes

A release's notes are one body that ships byte-identical on three surfaces: the annotated tag, the `CHANGELOG.md` section under mint's `## [x.y.z] - date` header, and the provider release body. This feature changes what that body says and in what order, wherever the fix lives: in mint's default notes prompt (`DefaultPrompt`, `internal/notes/prompt.go`), or in the wording mint adds to every notes call (the closing reminder, the chunk and piece framings, the merge framing). It covers:

- a one-sentence headline above the sections that names the release's main body of work (§3);
- a 💥 Breaking section, and each change placed by comparing it with the previous release (§4);
- notes limited to what users meet, and true to the release (§5);
- ordering within sections by weight to the user, with the Change Map turned from a ranking signal into a coverage check (§6);
- shared wording that is format-neutral, so the headline reaches only repos on the default prompt (§7), and the chunked path's split of the headline between part calls and the merge (§8).

Every rule answers to the notes themselves: good, readable, coherent, useful, accurate and well ordered, for someone who uses the software. §10 makes that bar measurable.

#### 1.2 What does not change

- **The AI's inputs.** Every notes call still carries the resolved instructions, the Change Map, the post-exclusion diff (or one part's slice of it) and the closing reminder. Nothing is added. Commit history stays out, and the diff stays the only source of truth for what changed. None of the failures this feature addresses needs mint to hand the AI more (§5.2).
- **How the Change Map is built.** Its git calls, detection and rendering stay as they are (§6.2).
- **Bodies pass through verbatim.** mint never parses, reformats or adds to a notes body. The headline exists only where the AI writes it (§3.4).
- **The provider release title** stays the bare tag (§9).
- **The body selector's precedence and every body that skips the AI**: `--notes-file`, the first release, an empty diff, `--no-ai` and the fallback. None of them goes through the notes prompt (§3.4).
- **The transport and the chunked path's mechanics.** Validation, the single bad-content retry, timeouts, part sizing, halving and the saved part notes all stay as they are.
- **`mint commit`.** Its prompt and closing reminder are separate from these and out of scope.

#### 1.3 Out of scope

- The `diff_exclude` pathspec bug. Globs go to git as plain `:(exclude)` pathspecs, so `**/testdata/**` misses a `testdata/` at the repo root. It is a mint bug and is handled on its own.
- The largest releases never reaching the AI. Token-aware diff sizing closes this.
- `CHANGELOG.md` drifting after a regenerate.
- Any route for a headline supplied by an operator or an agent (§3.5). That belongs to the agent-authored release notes roadmap item.

#### 1.4 Evidence

The rules rest on two records:

- [Release-notes audit](../../imports/release-notes-audit.md): 31 AI-written releases across nine projects, all written on today's default prompt and compared with their code. The calls in it are judgement, not a controlled measurement.
- [E1 report](../../experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/report.md): today's prompt against a rewritten candidate on 26 held-out releases, two repeats each, judged blind. Its frozen candidate is the starting wording (§10).

#### 1.5 Decisions this replaces

Once shipped, this feature replaces three earlier decisions for the default prompt and the merge framing:

- from the mint-release-tool specification: the six-section default notes format, checking the notes criterion by eye with no formal rubric, and the Change Map as the ranking signal (with release-notes-quality's "rank importance from the Change Map");
- from token-aware diff sizing: the merge call "ranking by the Change Map".

---

## Working Notes
