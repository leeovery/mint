# Review Tracking: Headline First Release Notes - Claims Verification

## Findings

### 1. The frozen candidate falls short of the decided rules in four places, not one

**Source**: Tree measurement — `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` lines 44, 79–87, 97–98 read against the headline, breaking, user-facing and true-to-the-release rules
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §10.2 Tuning and acceptance (Starting wording)

**Problem**:
The specification tells whoever writes the shipped prompt that E1's frozen candidate already carries every rule of the new notes format except one: where to look for what the previous release already had. So porting the candidate looks like a one-rule change. It is not. The candidate is also missing three rules the specification decides. It has no rule for a release with nothing users meet. It asks every breaking bullet for "what to do", with no limit to what the diff shows. And it stops a headline from pairing the main work with a lesser piece only when "And" joins them. A prompt ported on the specification's word ships without those three rules:
- a release of only CI, test or refactor changes gets notes in whatever shape the model invents, instead of the one maintenance line;
- a user upgrading can be told to take a remedy the diff never shows;
- a headline can tie the main work to a minor piece with a comma or "with".

Nothing in the specification would flag what was left out, because it says only one rule needs changing.

**Proposal**:
Name all four places where the candidate falls short, each with its measurement, so the port carries every rule the specification decides. The measurement settles it: reading new-v3 finds no maintenance-line rule, an unqualified "what to do" for breaking bullets, and a second-subject bar that covers only "And". The removed-lines difference the specification already names holds as stated.

**Evidence**:
Claim (§10.2): "Its rules are those of §2–§6 but for one, where what ships follows this specification: new-v3 looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames, while §5.2 adds the diff's removed lines"

Measurements, all against `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` (SHA-256 re-measured: `c1f149842689ba5f6b7338579514258a0fa23416c04421253eacfaae62630a0f`, matching the frozen candidate):
- `rg -n -i 'maintenance|user-facing changes' …/prompts/new-v3.md` → no output (exit 1). The specification's §5.1 says: "Where every change in the release is one this section leaves out, the notes are one plain line saying the release is maintenance with no user-facing changes … with no headline and no sections." §2.1 adds: "The one body without sections is §5.1's maintenance line."
- `rg -n 'what to do' …/prompts/new-v3.md` → `87:  usual section. Say what breaks and what to do, in the one sentence.` Nothing limits where "what to do" comes from. §4.2 says: "What to do comes only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there."
- `rg -n '"And" in a headline' …/prompts/new-v3.md` → `44:- "And" in a headline joins only two co-equal bodies of work — never two pieces`. §3.2 says: "A headline names a second subject only when it is a co-equal body of work: never two pieces of one body, and never a main body plus a lesser piece." That covers every way of joining a second subject, not only "And".
- `rg -n 'unchanged context lines|removed lines' …/prompts/new-v3.md` → `98:  unchanged context lines, or in code the diff only moves or renames.` This is the one difference the specification names. It holds.
- Spec-only: `rg -n -i 'but for one' .workflows/headline-first-release-notes/discussion/headline-first-release-notes.md .workflows/headline-first-release-notes/imports/release-notes-audit.md` → no output (exit 1). No source states how many rules the candidate lacks.

**Current**:
Its rules are those of §2–§6 but for one, where what ships follows this specification: new-v3 looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames, while §5.2 adds the diff's removed lines (`rg -n 'unchanged context lines|removed lines' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → line 98 only, no removed-lines hit).

**Proposed Text**:
Its rules are those of §2–§6 but for four, and in each what ships follows this specification: new-v3 looks for what the previous release already had only in unchanged context lines and in code the diff moves or renames, while §5.2 adds the diff's removed lines (`rg -n 'unchanged context lines|removed lines' .workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` → line 98 only, no removed-lines hit); new-v3 has no rule for a release with nothing users meet, so §5.1's maintenance line is added (`rg -n -i 'maintenance|user-facing changes' …/prompts/new-v3.md` → no hits); new-v3 asks every breaking bullet to "Say what breaks and what to do" (`rg -n 'what to do' …/prompts/new-v3.md` → line 87), while §4.2 takes what to do only from what the diff shows and stops at what breaks where it shows none; and new-v3 bars a lesser second subject only where "And" joins it (`rg -n '"And" in a headline' …/prompts/new-v3.md` → line 44), while §3.2 bars any second subject that is not a co-equal body of work.

**Resolution**: Pending
**Notes**:

---

### 2. The tag-annotation rebuild is named by a flag mint no longer has

**Source**: Tree measurement — `cmd/mint/regenerate_flags.go:20-21, 97-103`; `cmd/mint/usage.go:55-57`
**Category**: Source defect
**Move**: route
**Affects**: §9 The Provider Release Title

**Problem**:
The reasoning behind keeping the provider title bare credits `regenerate --reuse` with rebuilding a provider release from the tag annotation. mint has no `--reuse` flag. That rebuild is `mint release regenerate --source tag`, and the old flag was replaced on 2026-06-15. If the name is carried into the README, the `mint setup` guide or help text, it points users at a flag mint does not accept. The point the sentence makes still holds: the tag source reads the annotation body verbatim, so it is deterministic and does no parsing. Only the command name is out of date. The discussion's Release Title journey carries the old name, taken from the mint-release-tool specification.

**Evidence**:
Claim (§9): "The bare-tag title also keeps `regenerate --reuse`'s rebuild of the provider release from the tag annotation body deterministic and free of parsing."

Measurements:
- `rg -n 'fs\.(String|Bool)Var' cmd/mint/regenerate_flags.go` → the regenerate flag set defines `source`, `notes-file`, `target`, `all`, `y`, `yes` and `plain`. There is no `reuse`.
- `rg -n 'reuse' cmd/mint/regenerate_flags.go cmd/mint/usage.go README.md internal/setupguide/setupguide.go` → no output.
- `cmd/mint/regenerate_flags.go:20-21`: "sourceTag reads the tag annotation body verbatim — deterministic, parse-free, config-independent. Selected by --source tag."
- `cmd/mint/usage.go:55-57`: "--source SOURCE    notes source: fresh (re-diff + AI), tag (annotation body), or release (existing provider release body); tag and release run no AI. Default fresh."
- `git show -s --format='%h %ad %s' --date=short fc1cfe13` → `fc1cfe13 2026-06-15 feat(regenerate): replace --reuse/--fresh with orthogonal --source axis`

Source carrying the claim: `.workflows/headline-first-release-notes/discussion/headline-first-release-notes.md`, section "Release Title" → "Journey", line 479: "Sibling check: mint-release-tool specification — the provider release is a write-only projection of the full body, and `regenerate --reuse` recreates it from the tag annotation body "deterministic, parse-free"; a bare-tag title keeps both."

**Proposed Text**:

**Resolution**: Pending
**Notes**:

---

## Observations

- §6.2's salience count covers `internal/notes` only. `internal/engine/regenerate_fresh.go:140` also speaks of the notes task's "salience needs" in a comment.
- §1.2 says every notes call carries "the post-exclusion diff (or one part's slice of it)". The merge call carries the parts' notes in place of a diff (`internal/notes/chunk.go:562-568`).
- §2.3's rule that each change is listed once appears in new-v3 only for 💥 Breaking (line 79). The nearest general wording is "ONE bullet per notable change" (line 135).
