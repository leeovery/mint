## Attempt 1

ISSUES:
- `/Users/leeovery/Code/mint/internal/commit/run_aifail_output_test.go:234` and `:247`: `TestRun_NonAIFallbackTriggers_CarryNoAIOutput` injects `Transport: scriptedTransport("must never be returned")`. That makes the failing `claude` seeded at line 219 unreachable: `commitTransport` takes the injected transport, so no call ever reaches the runner's `claude`. The guard passes while checking nothing.
  - I reproduced it in a scratch copy by deleting `Run`'s `if deps.NoAI { return runEditorFallback(...) }` early return. As shipped, `NoAI/Attended` still passes, because the run silently commits the scripted body. With the transport left nil it fails.
  - It also makes two comments in the file false. The header at lines 14-16 says "These proofs ride the REAL ai.Transport (deps.Transport nil)". Line 218 says "A failing claude that must never be reached: if it were, its output would leak".
  FIX: Remove `, Transport: scriptedTransport("must never be returned")` from both `editorDeps(...)` calls (lines 234 and 247). The real transport then runs over the seeded failing claude, so a misrouted non-AI trigger shows up as an AI-failure warn or StageFailure carrying `error: unknown option '--safe-mode'`, and the assertions catch it. I checked this in a scratch copy: all six subtests still pass on the shipped `run.go`, and both NoAI subtests fail under the mutation above. Both comments become true with no edit.
  ALTERNATIVE: Keep the scripted transport, delete the `seedClaudeFailing` line and its comment, and reword the header. That is weaker: the NoAI/Attended misroute stays undetected.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `/Users/leeovery/Code/mint/internal/commit/run.go:438-440`: "reuse it unchanged" is false now that the AI-failure caller passes `aiOutput`. It also carries task ids and a cardinality claim.
  OLD: // runEditorFallback is the shared $EDITOR degradation path the three "no AI message"
// cases converge on (the --no-ai entry is wired here; AI-failure 3-3 and oversized 3-4
// reuse it unchanged). It opens the editor directly via the reusable OpenEditor
  NEW: // runEditorFallback is the shared $EDITOR degradation path for a run with no AI message.
// It opens the editor directly via the reusable OpenEditor
- `/Users/leeovery/Code/mint/internal/commit/run.go:455-457`: "identically across all three converging triggers" is false now that the AI-failure trigger's fail-loud carries the tool output.
  OLD: // All three collapse to errNoMessageSource (the spec's "no AI message and no interactive
// editor available"), surfaced and aborted — identically across all three converging
// triggers, since they all reach here. There is NO -m/--message escape.
  NEW: // All three collapse to errNoMessageSource ("no AI message and no interactive editor
// available"), surfaced and aborted. There is NO -m/--message escape.
- `/Users/leeovery/Code/mint/internal/commit/run.go:801-806`: most of the comment restates the code below it. Only the link to release's rule and the "no streams" case carry anything the code cannot show.
  OLD: // aiFailureOutput composes a failed AI call's captured streams for Warning.Output /
// StageFailure.Output, by the same rule release applies beneath its ✗ line: a stream
// counts only when it is not whitespace-only, the counted streams are kept verbatim,
// stdout first then stderr joined by a newline, and only the composed result's trailing
// whitespace is trimmed. A cause carrying no *ai.GenerationError (a timeout, a missing
// binary) yields "".
  NEW: // aiFailureOutput composes a failed AI call's captured streams by the rule release
// applies beneath its ✗ line (notesFailureOutput), so the same tool output reads alike on
// both verbs. A timeout or a missing binary carries no streams and yields "".
- `/Users/leeovery/Code/mint/internal/commit/surface.go:19-20`: since the diff added a caller that passes the AI tool's composed stdout-then-stderr output, "captured stderr" no longer describes what this function carries.
  OLD: // surfaceOutput is surface with the failed command's captured stderr passed through
// VERBATIM as StageFailure.Output. The mutation failures use it (stage/commit) because
  NEW: // surfaceOutput is surface with the failed command's captured output passed through
// VERBATIM as StageFailure.Output. The mutation failures use it (stage/commit) because
- `/Users/leeovery/Code/mint/internal/commit/surface.go:23-24`: same staleness as the item above.
  OLD: // renders as an unexplained failure. An empty output degrades to exactly surface's
// rendering, so callers pass whatever stderr they captured unconditionally. (The push
  NEW: // renders as an unexplained failure. An empty output degrades to exactly surface's
// rendering, so callers pass whatever output they captured unconditionally. (The push
- `/Users/leeovery/Code/mint/internal/commit/run_aifail_output_test.go:20-21`: restates the three-line function below it.
  OLD: // seedClaudeFailing scripts every `claude` invocation to exit non-zero with output on
// stdout and stderr.
  NEW:

NOTES:
- Criterion 3's unattended leg is tested only with a whitespace-only stdout, not an empty one. The attended table covers both, and both runs take the same composed string, so the risk is negligible.
- An attended TTY run where the AI fails and no editor can be launched shows the output in the warn, then a bare fail-loud with nothing beneath it. That fits §17.1, which places attended output in the warning. No test pins this path, and no criterion asks for one.
- Nothing arrived beyond the enumerated inputs.
