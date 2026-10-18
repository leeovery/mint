# Review Tracking: token-aware-diff-sizing - Claims Verification

## Findings

### 1. claude sends an oversized prompt and the API rejects it; claude does not refuse it locally

**Source**: Tree measurement — `strings -n 12 ~/.local/share/claude/versions/2.1.302` searched for the overflow message and the code that builds it
**Category**: Source defect
**Move**: route
**Affects**: §4.5 The default's grounds ("Too high for the model is loud and corrects itself"); §9.5 Edges ("The transport's single retry stays")

**Problem**:
The specification says claude refuses an oversized prompt on the machine, before sending anything, so repeating an overflowing call costs nothing. Two decisions lean on that: the 1.5 MB default is allowed to run high because an overflow is "instant", and the transport's single retry stays on every call because the identical repeat is "near-free". The installed claude does something else. It uploads the prompt, the API counts the tokens and rejects the request, and claude builds its "Prompt is too long" message from that API error. That includes the token breakdown in the record's own observed failure. So an overflow under the cap costs a full upload and a server rejection of up to 1.5 MB twice (the call and its transport retry) before the classifier sees it, and again on every halving round. The record already names this condition as the trigger to revisit the retry rule: "providers that reject server-side, at a full round trip each". claude is one of them, so the trigger is met on the shipped default. An operator whose cap is too high for their model waits through repeated round trips on each overflow, where the record expects instant local refusals.

**Evidence**:
Claims, verbatim:
- §4.5: "claude rejects an oversized prompt instantly, before sending it."
- §9.5: "On claude the repeat is near-free, since it rejects an oversized prompt instantly before sending it, and the cap makes under-cap overflows rare. … This is revisited if providers that reject server-side, at a full round trip each, become a target."

Measurement: `strings -n 12 ~/.local/share/claude/versions/2.1.302` (claude 2.1.302, the binary behind `~/.local/bin/claude`), then `grep -F` on the dump.

- The overflow breakdown message the record quotes from its observed failure is built by one function, which takes its token counts as arguments:
  `function Yms(e){let{actualTokens:n,limitTokens:r,conversationTokensEstimate:s}=e;… return`${aU} \xB7 the request is ~${n} tokens (limit ${r}) but this conversation is only ~${s} tokens — the rest is system prompt, `+"tool definitions, and attachment content. …"}` (with `var aU="Prompt is too long"`).
- Its only caller takes the actual and limit counts from the API error's details. It takes the conversation figure from a local estimate and logs the difference as an estimate:
  `{actualTokens:eo,limitTokens:Ao}=F3e(b.errorDetails??""),Ko=Lg(Tt.flat()),nr=eo!==void 0?Math.max(0,eo-Ko):void 0;i("tengu_ptl_surfaced_to_user",{…overheadTokensEstimate:nr});let Vo=Zt==="prompt_too_long"?Ilo(b,Yms({actualTokens:eo,limitTokens:Ao,conversationTokensEstimate:Ko}),A):b`
- `F3e` parses the API's own error text: `function F3e(e){let n=e.match(/prompt is too long[^0-9]*(\d+)\s*tokens?\s*>\s*(\d+)/i);…}`. That format is listed with the API's error messages: `var Q_=[/^prompt is too long: \d+ tokens > \d+ maximum$/,/^max_tokens: \d+ > \d+, which is the maximum allowed number of output tokens for this model$/,…`.
- `Ilo` copies the API response's identity onto the surfaced message: `function Ilo(e,n,r){return{...Os({content:n,error:"invalid_request",apiError:e.apiError,apiErrorParams:e.apiErrorParams,errorDetails:e.errorDetails,…}),requestId:e.requestId,apiErrorStatus:e.apiError…`. A message that carries a `requestId` comes from a request that was sent.
- What follows is the reactive path, acting on the server's answer: `Reactive compact: attempt ${Q} hit prompt-too-long (gap=${mt.tokenGap??"?"} …)`.
- The record's observed failure carries exactly this API-derived form (research, `token-aware-diff-sizing.md` lines 265–266): "Prompt is too long · the request is ~1335248 tokens (limit 1000000) but this conversation is only ~532481 tokens — the rest is system prompt, tool definitions, and attachment content." claude has no local tokenizer to produce an exact "1335248", and the figure is parsed from the API's error.

Source carrying the claim:
- discussion `token-aware-diff-sizing.md` line 197: "Cheap with claude (its overflow rejection is instant and client-side)"
- line 239: "claude rejects the oversized prompt client-side and instantly"
- line 347: "research found claude rejects an oversized prompt client-side before sending anything"
- line 351: "on claude the repeat is near-free … revisit if non-claude providers with server-side rejection become a target"
- research `token-aware-diff-sizing.md` line 163 (F1): "rejects **client-side** (instant, `input_tokens: 0`, no network variability)"

**Proposed Text**:

**Resolution**: Pending
**Notes**:

---

## Observations

- §1.2's "About 803k tokens belonged to the tool itself" is claude's own `overheadTokensEstimate`: the API's actual count minus a local estimate of the conversation (`nr=Math.max(0,eo-Ko)` in claude 2.1.302). It is not a measured overhead, and it sits far above the plain form's measured overhead of 105,994 tokens in §2.1.
- The claude API measurements in §2.1 (105,994 vs 628 input tokens) and §4.5 (54,654 tokens, 2.2s) were not re-run, because a claude call writes its own state and the plain form runs the user's hooks. Both carry dated commands, match the discussion (lines 601, 673, 237), and the §4.5 sample command does yield exactly 120,000 bytes (`git log --all -p --no-color -- '*.go' | head -c 120000 | wc -c` → `120000`).
- §16.4's "The engine never touches spinners or TTY state": as built, the engine calls the presenter's `SuspendSpinner`/`ResumeSpinner` hooks around its own gates and editor hand-off (for example `cacheReuse` in `internal/engine/release.go`). The pretty `Prompt` does not suspend a live spinner by itself, so the check's pauses and the floor's choice need those calls.
- §9.1 and §11.5's "never falls to `on_notes_failure`" for a cancel is new behaviour to build, not as built. Today `notes.(*Selector).SelectBodyWithReuse` routes any notes-call error, a wrapped `context.Canceled` included, into `ResolveFailure` (`internal/notes/select.go` `resolveNormalPathFailure`). No `Canceled` check exists in `internal/notes`.
- §8.6's "the default prompt or the `[release].prompt` override, with `context`" and §2.3's "`context` and `prompt` keys, which ride in every … call": as built, `ResolveInstructions` ignores `[release].context` when a `[release].prompt` override is set (`internal/notes/prompt.go:115-128`). Chunk and merge calls built from the same resolver keep single-pass parity.
- §12.2's `printf '' | git tag -a v1 -F -` exits 0 was not run, because it creates a tag.
