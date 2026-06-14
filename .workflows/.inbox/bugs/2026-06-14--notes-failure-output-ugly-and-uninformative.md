# `mint release` notes-failure output is ugly and uninformative

When AI notes generation fails, `mint release` renders a line like:

```
✗ notes      notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed
```

There are two distinct problems with this.

**It throws away the AI's real error.** The word "failed" appears three times and the message nests "notes generation failed (AI returned empty/invalid notes after retry): generating notes: ai generation failed" — yet none of that tells the operator what actually went wrong. In the case that surfaced this, `claude -p --model sonnet` exited non-zero with stdout `Prompt is too long`, but that real cause never reaches the screen. The transport (`internal/ai/transport.go`) maps a non-zero exit or empty body to `ErrGenerationFailed` and drops claude's captured stdout/stderr; the notes layer (`internal/notes/resolve.go:101`, `internal/notes/generate.go:185`) then wraps that into the redundant chain, and `StageFailure.Output` ends up empty, so there's nothing actionable below the ✗ line. The operator is left guessing.

**The line is visually ugly.** The failure line is `padStage`-aligned in `internal/presenter/pretty.go` `StageFailed` — `✗ <name><column-pad><message>` — and the message restates the stage, so it reads `✗ notes      notes generation failed …`: the word "notes" appears twice across a wide gap. Worth noting that the `padStage` gap is intentional column alignment, shared with the `✓` success lines, and is pinned in several presenter tests (`pretty_test.go`, `gate_forbidden_test.go`, `askline_test.go`) — so it is a deliberate convention, not a stray leftover, and changing it is a contract change.

Desired outcome: the operator should see the AI's actual output (e.g. `Prompt is too long`) verbatim under the ✗ line — have `ErrGenerationFailed` or the notes failure path carry claude's captured stdout/stderr and populate `StageFailure.Output`. The top-line message should collapse to one concise sentence that does not restate the stage name or say "failed" three times. And the failure-line layout (whether to keep the `padStage` gap for failures or drop it) should be settled, with the pinned presenter tests updated to match.

Context: this surfaced while dogfooding `mint release` on mint itself. The underlying trigger that day was a too-large notes diff — the `v0.0.2..HEAD` diff was ~867 KB, mostly the committed `.workflows/` and `.tick/` artifact trees, which pushed the prompt past the model's context window. A possibly-related sub-issue: the `max_diff_lines` guard is line-based (default 50000) and did not catch an 8.6k-line but 867 KB byte-dense diff, so a byte/token-aware ceiling — or at least surfacing claude's "Prompt is too long" distinctly — is worth considering alongside this.
