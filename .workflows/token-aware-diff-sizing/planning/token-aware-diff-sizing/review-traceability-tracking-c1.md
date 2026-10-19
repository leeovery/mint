# Review Tracking: Token Aware Diff Sizing - Traceability

## Findings

### 1. An old claude's release is pinned to two AI invocations the finished feature does not make

**Type**: Hallucinated content
**Spec Reference**: §2.5 A claude too old for the flags; §6.2 The call; §9.5 Edges; §17.2 What commit does not get
**Plan Reference**: Phase 1 / token-aware-diff-sizing-1-2 (tick-3acb4d), Acceptance Criteria, fourth criterion
**Move**: settled
**Change Type**: update-task

**Problem**:
The criterion says that when claude rejects a lean flag, the AI tool's only invocations are the lean command's first attempt and its one retry, with no verb named. That holds on `mint commit`, but not on a release once the later phases land. §2.5 has the release's failure classifier run the same command, fail the same way and take the OTHER path, and Phase 7's exclusion check runs before the notes call through that command too. A test written from this criterion for a release pins a count the finished feature contradicts. The Phase 4 implementer then finds a Phase 1 criterion saying that the classify call §2.5 requires must not happen.

**Proposal**:
Keep what §2.5 decides as a rule that every invocation meets: no version probe, and no fallback to the plain command or any other command form. Keep the exact two-invocation count only for `mint commit`, which makes no classify call (§17.2) and runs no exclusion check (§6.1), so the count stays true for the finished feature. Nothing else in the task changes.

**Current**:
```
- [ ] Given a claude too old for a lean flag, one that exits non-zero rejecting `--safe-mode` as an unknown option, mint runs no version probe and no other command form. The AI tool's only invocations are the lean command's first attempt and the transport's one retry (§2.5)
```

**Proposed Text**:
```
- [ ] Given a claude too old for a lean flag, one that exits non-zero rejecting `--safe-mode` as an unknown option, mint runs no version probe and no other command form: every invocation of the AI tool is the lean command, and none is the plain `claude -p --model sonnet` or any other form without the lean flags. On `mint commit` the AI tool's only invocations are the lean command's first attempt and the transport's one retry (§2.5, §17.2)
```

**Resolution**: Pending
**Notes**:

---

### 2. Chunk calls are told to carry `context` alongside a `[release].prompt` override

**Type**: Incomplete coverage
**Spec Reference**: §8.6 What each call is told; Corrigendum 2026-10-19 on §12.4 and §8.6 (a set `[release].prompt` replaces the default prompt and `context` with it)
**Plan Reference**: Phase 2 / token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, third criterion; planning.md Phase 2 task table, row token-aware-diff-sizing-2-4, Edge Cases cell
**Move**: settled
**Change Type**: update-task

**Problem**:
The criterion lists a chunk call's instructions as "the default prompt or the `[release].prompt` override, with `context`", and the task table repeats that wording. This is the reading the corrigendum to §8.6 corrected. As built, and as §8.6 now says, a set `[release].prompt` replaces the default prompt and `context` together (`ResolveInstructions`, `internal/notes/prompt.go`). An implementer writing chunk and merge calls from this criterion would append `context` to an override in those calls only. An over-cap release would then be written under different instructions from a single-pass one. Phase 5's no-room move, which names only `[release].prompt` when it is set, would also miss the `context` text those calls were carrying.

**Proposal**:
Carry §8.6's corrected wording into the criterion and the table row: the default prompt with `context`, or the override, which replaces both, as built. Add one criterion that proves the override case, since that is the case the old wording gets wrong. This text comes from §8.6 and its corrigendum. Task 6-1's Context already states it this way.

**Current**:
In token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, third criterion:
```
- [ ] Given a release whose single-pass prompt is over the cap, no single-pass call is made. The AI tool receives chunk calls, none bigger than the cap, each carrying (§4.1, §8.2, §8.6):
  - the release's instructions: the default prompt or the `[release].prompt` override, with `context`
  - the whole-release Change Map
  - framing that names its part (part k of n)
  - its slice of the diff
  - the closing reminder, last
```

In planning.md, Phase 2 task table, row token-aware-diff-sizing-2-4, Edge Cases cell, the clause:
```
a chunk call carries the release's instructions (default prompt or `[release].prompt` override, with `context`), part-k-of-n framing, its slice and the closing reminder (§8.6)
```

**Proposed Text**:
In token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, replacing the third criterion:
```
- [ ] Given a release whose single-pass prompt is over the cap, no single-pass call is made. The AI tool receives chunk calls, none bigger than the cap, each carrying (§4.1, §8.2, §8.6):
  - the release's instructions: the default prompt with `context`, or the `[release].prompt` override, which replaces both, as built
  - the whole-release Change Map
  - framing that names its part (part k of n)
  - its slice of the diff
  - the closing reminder, last
- [ ] Given an over-cap release with both `[release].prompt` and `[release].context` set, every chunk call and the merge call carry the override file's contents as the release's instructions and none of the `context` text, as the single-pass call does (§8.6)
```

In planning.md, Phase 2 task table, row token-aware-diff-sizing-2-4, Edge Cases cell, replacing the clause:
```
a chunk call carries the release's instructions (the default prompt with `context`, or the `[release].prompt` override, which replaces both), part-k-of-n framing, its slice and the closing reminder (§8.6)
```

**Resolution**: Pending
**Notes**:

---

### 3. A cap below mint's own fixed part has no no-room behaviour in the task that builds it

**Type**: Incomplete coverage
**Spec Reference**: §8.7 Calls that cannot be made; Corrigendum 2026-10-19 on §8.7 (the third no-room cause); §4.1 The key; §12.4 The ✗ line and the next moves
**Plan Reference**: Phase 2 acceptance; Phase 2 / token-aware-diff-sizing-2-9 (tick-dc97ae), Problem, Solution, Acceptance Criteria, Context; planning.md Phase 2 task table, row token-aware-diff-sizing-2-9
**Move**: settled
**Change Type**: update-task

**Problem**:
§8.7, as corrected, gives no room for diff three causes. The third is a `max_prompt_bytes` set below the fixed part of mint's own instructions with neither `[release].prompt` nor `context` set, a value config load accepts (§4.1). Task 2-9 builds the no-call behaviour but names only outsized operator text, and its Solution makes the no-call depend on it ("because of operator text"). An implementer who keys the check on operator text leaves this run with a chunk room below zero. The chunker then sends an over-cap call, against §4.1, or packs nothing. Task 5-4 already assumes this run fails as `no room for the diff under max_prompt_bytes`, so the gap would surface only there, three phases later. The task's Context also still carries guidance the spec superseded: "shorten whichever of `[release].prompt` or `context` is outsized".

**Proposal**:
Bring the third cause into the task that builds the no-call behaviour, the task table and Phase 2's acceptance, with the same phrase §12.4 gives it. Add the criterion that proves no notes call is sent. Replace the superseded guidance in the Context with §12.4's corrected moves. All of this text comes from §8.7, its corrigendum and §12.4. The new criterion says "no chunk, merge or single-pass call" rather than "no call at all", because Phase 7's exclusion check, which carries no operator text and may fit under a small cap, still runs ahead of the notes.

**Current**:
In planning.md, Phase 2 Acceptance:
```
- [ ] Given an outsized `[release].prompt` or `context` that leaves no room for diff even with the Change Map condensed, no AI call is sent and the notes fail as `no room for the diff under max_prompt_bytes`. Given chunk notes whose merge prompt would exceed the cap, the merge is not sent and the notes fail as `part notes too large to merge`
```

In planning.md, Phase 2 task table:
```
| token-aware-diff-sizing-2-9 | Calls that cannot be made | an outsized `[release].prompt` or `context` that leaves no room even with the Change Map condensed sends no call and fails as `no room for the diff under max_prompt_bytes` (§8.7, §12.4), mint never trims operator text (§8.7), a merge prompt over the cap is not sent and fails as `part notes too large to merge` (§8.7, §11.3, §12.4), both failures route through `on_notes_failure` as a single-pass failure does (§12.1) |
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Problem:
```
**Problem**: Some chunked runs need a call that no prompt under the cap can carry. An outsized `[release].prompt` or `context` can leave a chunk call no room for diff, even with the Change Map condensed. And a release's chunk notes together can make a merge prompt bigger than the cap (§8.7). Sending either call would break the rule that no call is bigger than the cap (§4.1), and mint never trims operator text to make room.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Solution:
```
**Solution**: Send no call in either case. When a chunk call's fixed part leaves no room for diff because of operator text, make no AI call at all and fail the notes as `no room for the diff under max_prompt_bytes`. When the merge prompt would exceed the cap, do not send the merge and fail the notes as `part notes too large to merge`. Both failures route like any other notes failure.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Acceptance Criteria, second criterion:
```
- [ ] Given that release under `on_notes_failure = abort`, the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§8.7, §12.4)
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Context, first paragraph:
```
> §8.7: no room for diff happens with an outsized `[release].prompt` or `context` (operator text mint never trims), or with a halving round aimed below the fixed part. Halving is Phase 4, which renders that second case as `too large for the model, even in smaller parts`. Phase 5's floor guidance names which of the two it was, and for operator text says to shorten whichever of `[release].prompt` or `context` is outsized.
```

**Proposed Text**:
In planning.md, Phase 2 Acceptance, replacing the criterion:
```
- [ ] Given an outsized `[release].prompt` or `context` that leaves no room for diff even with the Change Map condensed, or a `max_prompt_bytes` below the fixed part of mint's own instructions with neither key set, no AI call is sent and the notes fail as `no room for the diff under max_prompt_bytes`. Given chunk notes whose merge prompt would exceed the cap, the merge is not sent and the notes fail as `part notes too large to merge`
```

In planning.md, Phase 2 task table, replacing the row:
```
| token-aware-diff-sizing-2-9 | Calls that cannot be made | an outsized `[release].prompt` or `context` that leaves no room even with the Change Map condensed sends no call and fails as `no room for the diff under max_prompt_bytes` (§8.7, §12.4), a `max_prompt_bytes` below the fixed part of mint's own instructions, with neither `[release].prompt` nor `context` set, sends no call and fails the same way (§8.7), mint never trims operator text (§8.7), a merge prompt over the cap is not sent and fails as `part notes too large to merge` (§8.7, §11.3, §12.4), each failure routes through `on_notes_failure` as a single-pass failure does (§12.1) |
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), replacing the Problem:
```
**Problem**: Some chunked runs need a call that no prompt under the cap can carry. An outsized `[release].prompt` or `context` can leave a chunk call no room for diff, even with the Change Map condensed, and so can a `max_prompt_bytes` set below the fixed part of mint's own instructions. And a release's chunk notes together can make a merge prompt bigger than the cap (§8.7). Sending any such call would break the rule that no call is bigger than the cap (§4.1), and mint never trims operator text to make room.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), replacing the Solution:
```
**Solution**: Send no call in either case. When a chunk call's fixed part leaves no room for diff, whether because of operator text or because `max_prompt_bytes` is below the fixed part of mint's own instructions with neither `[release].prompt` nor `[release].context` set, make no AI call at all and fail the notes as `no room for the diff under max_prompt_bytes`. When the merge prompt would exceed the cap, do not send the merge and fail the notes as `part notes too large to merge`. Both failures route like any other notes failure.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Acceptance Criteria, replacing the second criterion with it and a new criterion after it:
```
- [ ] Given that release under `on_notes_failure = abort`, the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§8.7, §12.4)
- [ ] Given neither `[release].prompt` nor `[release].context` set and a `max_prompt_bytes` below the fixed part of mint's own instructions, no chunk, merge or single-pass call is sent. Under `on_notes_failure = abort` the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§4.1, §8.7, §12.4, Corrigendum 2026-10-19 on §8.7)
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Context, replacing the first paragraph:
```
> §8.7 and its corrigendum: no room for diff happens with an outsized `[release].prompt` or `context` (operator text mint never trims), with a `max_prompt_bytes` below the fixed part of mint's own instructions when neither key is set, or with a halving round aimed below the fixed part. Halving is Phase 4, which renders that last case as `too large for the model, even in smaller parts`. Phase 5's floor guidance names which cause it was. For operator text it says to shorten `[release].prompt` when the release sets it, since the override replaces the default prompt and `context`, and otherwise `[release].context`. With neither key set it says to raise `max_prompt_bytes`.
```

**Resolution**: Pending
**Notes**:

---

## Observations

- token-aware-diff-sizing-4-1's Context table gives `no room for the diff under max_prompt_bytes` only for operator text, while §12.4's row also covers a cap below mint's own fixed part. This is Context only: the task's phrase and its no-suffix rule are right.
- token-aware-diff-sizing-4-7's Context says the floor guidance names "which of the two no-room causes" a failure was, but §8.7 as corrected has three. This is Context only, and task 5-4 carries all three.
- token-aware-diff-sizing-4-10's Context says Phase 5 names the saved file in the `fallback` warning with the regenerate command. The corrected §12.1 warning names both commands, and task 5-6 carries both.
- token-aware-diff-sizing-2-2's Context lists commit's instructions as "the default prompt or the `[commit].prompt` override, with `[commit].context`", which reads as if `context` rides along with the override. As built the override replaces `context`, and the cap measures whatever `ComposePrompt` composes.
