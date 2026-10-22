# Review Tracking: headline-first-release-notes - Claims Verification

## Findings

### 1. The starting prompt does not carry two of the decided rules

**Source**: Tree measurement — `rg -n 'internal efficiency|unchanged context lines|removed lines' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §10.2 Tuning and acceptance (Starting wording); bears on §5.1 Only what users meet and §5.2 True to the release

**Problem**:
The specification tells the implementer that E1's frozen prompt already holds every notes rule this feature decides, and that only the closing reminder and the chunked-path framings differ from it. Two of its rule lines differ from the decided rules. First, its list of things to leave out still includes "internal efficiency changes a user would not notice". That is the line under which E1's candidate dropped a speed-up users notice from both repeats, while the decided rule keeps every change a user notices, speed-ups included. Second, it looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames. The decided rule also covers the diff's removed lines, which is where E1's three spot-checks found the earlier state behind false "this is new" claims. An implementer who trusts the statement and changes only the reminder and the framings ships both lines as they are. Release readers would then keep losing speed-ups they notice and keep seeing last release's behaviour announced as new. Missing items and false claims are the two counts already over the shipping bar, and these are the lines that feed them.

**Proposal**:
Correct the starting-wording statement to name the two lines where what ships follows the specification rather than new-v3. The measurement decides it: the frozen file (SHA-256 confirmed as the one the specification names) carries the internal-efficiency leave-out at line 113, and its already-existed rule names only context lines and moved or renamed code at lines 97–98, with no mention of removed lines.

**Evidence**:
Claim (§10.2): "Its rules are those of §2–§6. Two parts of what ships are not new-v3's: the closing reminder is §7.1's format-neutral wording, not new-v3's headline-naming one, and the chunk, piece and merge framings follow §7.1 and §8."

`shasum -a 256 prompts/new-v3.md` → `c1f149842689ba5f6b7338579514258a0fa23416c04421253eacfaae62630a0f` (the file the specification names).

`rg -n 'internal efficiency|unchanged context lines|removed lines' …/prompts/new-v3.md` →
```
98:  unchanged context lines, or in code the diff only moves or renames.
113:  and refactors, internal efficiency changes a user would not notice, internal
```
(no `removed lines` hit). The full lines are new-v3.md:97–98: "- Do not present behaviour as new when the diff shows it already existed — in unchanged context lines, or in code the diff only moves or renames." and new-v3.md:111–114: "- Leave out how the software is built and maintained: … internal packages, modules and refactors, internal efficiency changes a user would not notice, internal flags and plumbing commands …".

Against the specification's rules. §5.1: "The test is whether a user meets or notices the change, not whether the code behind it is internal. A speed-up users notice belongs to them and is listed." Its Out list does not carry the internal-efficiency item. §5.2: "That holds whether the diff shows it in unchanged context lines, in removed lines, or in code the diff only moves or renames."

The cost E1 measured under the new-v3 line: `jq --slurpfile key data/blind-key.json -c '.[] | select(.release=="agentic-workflows-v0.7.80") | {label, arm: $key[0].key[.release][.label].arm, missing}' data/judgements/agentic-workflows.json` → both `new` bodies (A, C) list `"Filtered store reads size to the store instead of pre-allocating a million rows, making knowledge indexing much faster"` as missing, and both `current` bodies (B, D) list nothing.

Source check: the discussion never says new-v3's rules equal the decided rules (`rg -n -i "rules are those|starting point|two parts of what ships" .workflows/headline-first-release-notes/discussion/headline-first-release-notes.md` → no output). Its Scope decision draws the line at "whether a user notices the change", and its accuracy decision places the previous release's state "in the diff's removed and context lines". The claim exists only in the specification.

**Current**:
Its rules are those of §2–§6. Two parts of what ships are not new-v3's: the closing reminder is §7.1's format-neutral wording, not new-v3's headline-naming one, and the chunk, piece and merge framings follow §7.1 and §8.

**Proposed Text**:
Its rules are those of §2–§6 but for two lines, where what ships follows this specification. new-v3's leave-out list names "internal efficiency changes a user would not notice", where §5.1 keeps every change a user notices, a speed-up included. new-v3 looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames, where §5.2 adds the diff's removed lines (`rg -n 'internal efficiency|unchanged context lines|removed lines' …/prompts/new-v3.md` → lines 98 and 113, no removed-lines hit). Two more parts of what ships are not new-v3's: the closing reminder is §7.1's format-neutral wording, not new-v3's headline-naming one, and the chunk, piece and merge framings follow §7.1 and §8.

**Resolution**: Pending
**Notes**:

---

## Observations

- §9 names `regenerate --reuse`, but the flag mint ships is `--source tag` (`rg -n 'reuse' cmd/mint -g '!*_test.go'` finds no such flag). The discussion carries the same name (line 479). The behaviour holds: the tag annotation body is re-published with a bare-tag title.
- §1.2 says every notes call carries the diff or a part's slice of it. The merge call carries the parts' notes instead (`composeMergePrompt`, chunk.go:563–568), which §8 already describes correctly.
- §6.2's comment scope is pinned by a regex that matches `salience|prioriti` (11 lines, measured). Comments that call the map a headline or strongest signal in other words fall outside it: changemap.go:198–200 ("headline first (the strongest language-agnostic signal)") and 237–239 ("headlines", "false novelty headline").
- §10.3 quotes new-v3's line as "internal efficiency a user would not notice". The file reads "internal efficiency changes a user would not notice" (new-v3.md:113).
