## Attempt 1

ISSUES:
- README.md:129 quotes the hint as `max_prompt_bytes: set higher than this model accepts; set max_prompt_bytes = N, the largest call that succeeded`. mint prints that line in neither mode. `hintLowerCap` (internal/engine/notesstage.go:272-280) emits a Warning with Label `max_prompt_bytes` and the Message on its own:
  - pretty mode renders it as `⚠ max_prompt_bytes  set higher …`, with two spaces and no colon (internal/presenter/pretty.go:705-710);
  - plain mode renders it as `max_prompt_bytes: WARN - set higher …` (internal/presenter/plain.go:306).
  An operator grepping a CI log or a terminal for the quoted line finds nothing. That breaks §18's rule that the README stays true to what is built, and it breaks the README's own habit of quoting a warning by its message (:119).
  FIX: Drop the label from the quote, matching :119. Replace "mint then warns `max_prompt_bytes: set higher than this model accepts; set max_prompt_bytes = N, the largest call that succeeded`, the lower-the-cap hint," with "mint then warns `set higher than this model accepts; set max_prompt_bytes = N, the largest call that succeeded` under `max_prompt_bytes`, the lower-the-cap hint,". Nothing else in the sentence changes, and no pin touches this text.
  ALTERNATIVE: Quote only `set max_prompt_bytes = N`, the fragment AC3 names. It is shorter, but it loses the "set higher than this model accepts" text an operator would recognise in the output. I recommend the full message-only quote.
  CONFIDENCE: high

NOTES:
- Two places where the paragraph says less than the spec, neither required by the AC:
  - Under `--all`, N is the largest diff-carrying call that succeeded across the whole batch, including calls from versions whose notes then failed (notesstage.go:242-245). The unqualified "the largest diff-carrying call that succeeded" covers this.
  - A halving re-cut with no room for diff also fails the slice (chunk.go:236-237; generateHalved at :43-45).
- In "SIZE on the merge call is a failed merge (part notes too large to merge), and any other answer goes to the notes failure path", the "any other answer" clause could be read as saying a merge SIZE does not reach the floor. The paragraphs at :111 and :131 make clear that a failed merge does reach it, so I am not raising it.
- The executor's line references are pre-insert numbers: :195 is the commit section's "one-time context line", now at :197. This affects only the report.
- `.tick/tasks.jsonl` is also modified in the worktree. That is task tracking, not the executor's change.
- Nothing arrived beyond the enumerated inputs.
