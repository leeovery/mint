# Review Tracking: Token Aware Diff Sizing - Traceability

## Findings

### 1. Phase 2 pins a forward release's AI calls as if the exclusion check never runs

**Type**: Hallucinated content
**Spec Reference**: §6.1 When it runs; §6.2 The call; §7.2 The AI path, in order; §8.7 Calls that cannot be made
**Plan Reference**: planning.md Phase 2 Acceptance, first and sixth criteria; Phase 2 / token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, first criterion; Phase 2 / token-aware-diff-sizing-2-9 (tick-dc97ae), Solution and Acceptance Criteria, first criterion
**Move**: settled
**Change Type**: update-task

**Problem**:
Once Phase 7 lands, every forward release on the AI path that has a changed path left to raise asks the AI about its paths before any notes call (§6.1, §7.2). That call carries only the changed-path list, none of the operator's prompt or context (§6.2), so it is made even on a release whose notes have no room for diff. Phase 2's criteria pin the forward release's AI calls as if the check never ran: exactly one call under the cap, and no call at all when operator text leaves no room. Tests written from them fail when the check arrives. The Phase 7 implementer then finds Phase 2 saying the check call must not happen on those releases. Taken at its word, that would skip the check on a release whose notes are about to fail, and §6.1 never skips it there.

**Proposal**:
Pin the release-notes calls, which are what Phase 2 builds, and leave the check call to Phase 7:
- The under-cap case counts single-pass, chunk and merge calls.
- The no-room case says no chunk, merge or single-pass call is sent.

§8.7's "sends no call" refers to the call that has no room, not to every call the run makes. The wording follows the plan's own convention: task 2-9's third criterion, landed in cycle 1 for this reason, and task 6-6's first criterion. Phase 2's behaviour does not change, only what its criteria pin, and each criterion stays true for the finished feature.

**Current**:
In planning.md, Phase 2 Acceptance, first criterion:
```
- [ ] Given no `max_prompt_bytes` set, a release whose composed single-pass prompt (instructions, Change Map, diff and closing reminder) is at most 1,500,000 bytes makes one AI call as before. A release over that size makes no single-pass call and is chunked
```

In planning.md, Phase 2 Acceptance, sixth criterion:
```
- [ ] Given an outsized `[release].prompt` or `context` that leaves no room for diff even with the Change Map condensed, or a `max_prompt_bytes` below the fixed part of mint's own instructions with neither key set, no AI call is sent and the notes fail as `no room for the diff under max_prompt_bytes`. Given chunk notes whose merge prompt would exceed the cap, the merge is not sent and the notes fail as `part notes too large to merge`
```

In token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, first criterion:
```
- [ ] Given no `max_prompt_bytes` set and a release whose composed single-pass prompt is at most 1,500,000 bytes, including one whose diff runs past 50,000 lines, the AI tool receives exactly one call, carrying the single-pass prompt (§4.2, §7.2)
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Solution:
```
**Solution**: Send no call in either case. When a chunk call's fixed part leaves no room for diff, whether because of operator text or because `max_prompt_bytes` is below the fixed part of mint's own instructions with neither `[release].prompt` nor `[release].context` set, make no AI call at all and fail the notes as `no room for the diff under max_prompt_bytes`. When the merge prompt would exceed the cap, do not send the merge and fail the notes as `part notes too large to merge`. Both failures route like any other notes failure.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Acceptance Criteria, first criterion:
```
- [ ] Given a release whose single-pass prompt is over the cap, with a `[release].context` or `[release].prompt` override file large enough that a chunk call's fixed part leaves no room for diff, the AI tool receives no call at all. The fixed part is the instructions, the condensed Change Map, the framing and the closing reminder (§8.3, §8.7)
```

**Proposed Text**:
In planning.md, Phase 2 Acceptance, replacing the first criterion:
```
- [ ] Given no `max_prompt_bytes` set, a release whose composed single-pass prompt (instructions, Change Map, diff and closing reminder) is at most 1,500,000 bytes makes exactly one single-pass call and no chunk or merge call, as before. A release over that size makes no single-pass call and is chunked
```

In planning.md, Phase 2 Acceptance, replacing the sixth criterion:
```
- [ ] Given an outsized `[release].prompt` or `context` that leaves no room for diff even with the Change Map condensed, or a `max_prompt_bytes` below the fixed part of mint's own instructions with neither key set, no chunk, merge or single-pass call is sent and the notes fail as `no room for the diff under max_prompt_bytes`. Given chunk notes whose merge prompt would exceed the cap, the merge is not sent and the notes fail as `part notes too large to merge`
```

In token-aware-diff-sizing-2-4 (tick-5d01e7), Acceptance Criteria, replacing the first criterion:
```
- [ ] Given no `max_prompt_bytes` set and a release whose composed single-pass prompt is at most 1,500,000 bytes, including one whose diff runs past 50,000 lines, the AI tool receives exactly one single-pass call, carrying the single-pass prompt, and no chunk or merge call (§4.2, §7.2)
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), replacing the Solution:
```
**Solution**: Send no call in either case. When a chunk call's fixed part leaves no room for diff, whether because of operator text or because `max_prompt_bytes` is below the fixed part of mint's own instructions with neither `[release].prompt` nor `[release].context` set, send no chunk, merge or single-pass call and fail the notes as `no room for the diff under max_prompt_bytes`. When the merge prompt would exceed the cap, do not send the merge and fail the notes as `part notes too large to merge`. Both failures route like any other notes failure.
```

In token-aware-diff-sizing-2-9 (tick-dc97ae), Acceptance Criteria, replacing the first criterion:
```
- [ ] Given a release whose single-pass prompt is over the cap, with a `[release].context` or `[release].prompt` override file large enough that a chunk call's fixed part leaves no room for diff, no chunk, merge or single-pass call is sent. The fixed part is the instructions, the condensed Change Map, the framing and the closing reminder (§8.3, §8.7)
```

**Resolution**: Fixed
**Notes**:

---

## Observations

- Task 2-8's one-call criterion ("the narration is as built") and Phase 2's acceptance ("A single-pass run narrates as before") say nothing of the check line Phase 7 puts ahead of a forward release's notes. §16.1 says a single-pass run "otherwise narrates as today", and task 7-5 already pins the check line.
- The setup guide's "diff_exclude scope" section opens "diff_exclude is for release-notes NOISE, not generated code", while the floor's size move (§12.4, task 5-4) says to exclude generated paths with `[release].diff_exclude`. §18.1's rewrite of that section names only the levels, so the guide's advice and the floor's guidance pull in opposite directions.
- The README documents chunking only in its `release` section (task 2-4), though regenerate's fresh source chunks too and §18.2 documents chunking "with the commands they belong to". Task 4-13's regenerate text about the saved file implies chunking without naming it.
