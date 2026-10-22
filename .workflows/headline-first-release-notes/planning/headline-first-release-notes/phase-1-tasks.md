# Phase 1: Headline-first notes on a single call — 13 tasks

## headline-first-release-notes-1-1

### Task 1-1: Headline-or-first-section format led by 💥 Breaking

**Problem**: Today's `DefaultPrompt` asks for six sections led by ✨ Added and tells the model to "start directly with the first section header". The notes have no slot for a line that says what the release is, and no section that puts a breaking change in front of a reader upgrading (§2.1).

**Solution**: Rewrite the format part of the default prompt. The body may open with an optional one-sentence headline, followed by one blank line. Then come seven sections in a fixed order led by 💥 Breaking, with empty sections left out. The headline-or-first-section rule replaces "start directly with the first section header" and still forbids every other preamble. Every change is listed once. The bullet rules carry over unchanged.

**Outcome**: On the default prompt, the model is asked for a body that opens either with a headline line and one blank line or with the first section. The sections always run 💥 Breaking, ✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security, and nothing else surrounds them.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives ask for an optional one-sentence headline, only where the release has one, as the body's first line, followed by one blank line and then the first section. With no headline, the body opens with the first section (§2.1)
- [ ] The same instructions list the sections in the order 💥 Breaking, ✨ Added, 🔧 Changed, ⚠️ Deprecated, 🗑️ Removed, 🐛 Fixed, 🔒 Security. They ask for a section with no items to be left out, and keep that order in every release, whatever its main work (§2.1)
- [ ] The same instructions allow nothing above the first section but the headline: no summary paragraph, no TL;DR, no label and no other preamble. They ask for no wrapper, markers or machine-parseable labels, and no commentary or sign-off. The words "start directly with the first section header" no longer appear (§2.1)
- [ ] The same instructions ask for every change to be listed once, in one section, with the headline's summary of the main work the only restatement in the body (§2.3)
- [ ] The same instructions carry the bullet rules unchanged. Each bullet is one line and one sentence, with the user-facing effect after an em dash where it helps, and no second sentence, bold lead-in, sub-bullets, implementation detail or rationale. The three altitude examples are as shipped. There is one bullet per notable change, with "Combine closely-related changes into a single bullet." word for word. Version bumps and bookkeeping churn are ignored, and the notes are not padded. Deprecated and Security take an item only on an explicit marker in the diff, never inferred (§2.2)

**Do**:
- The format lives in `DefaultPrompt` in `internal/notes/prompt.go`. It and every other rule of §2–§6 live in the default prompt alone, never in mint's shared wording (§7.1).
- The starting wording is E1's frozen candidate, `.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md` (SHA-256 `c1f149842689ba5f6b7338579514258a0fa23416c04421253eacfaae62630a0f`): its opening and format, bullet and altitude-example passages (lines 1–33), and its scope, Deprecated/Security and output-discipline passages (lines 133–151). Where it and §2 differ, §2 governs (§10.2).

**Context**:
> §2.1: the section order is the same in every release because the headline already lifts the main work above every section. Reordering the sections would answer the same problem twice and cost readers an order they can rely on across versions. The one body without sections is §5.1's maintenance line (task 1-8).
>
> §2.2: "closely related" means the small tweaks of one change, such as three commits that fix one gate, never a feature's separate capabilities. The combine rule's wording stays as it is; what would reopen it is evidence on real releases of the model collapsing a large feature into one bullet.
>
> When and how a headline is written are tasks 1-2 to 1-4. What belongs in each section is tasks 1-5 and 1-6. The Change Map's description and ranking rule are task 1-11's, and the closing reminder is task 1-12's.
>
> §10.2: the wording stays general. No prompt text names, quotes or paraphrases a change from the audit's 31 sampled releases, and any example added is invented (the three shipped altitude examples are exempt). Phase 3 tunes the wording; this task carries the rules.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §2.1 Shape; §2.2 Bullets (unchanged); §2.3 Each change once; §7.1 mint's shared wording stays format-neutral; §10.2 Tuning and acceptance (starting wording)

## headline-first-release-notes-1-2

### Task 1-2: Headline written at the bullets' altitude

**Problem**: The headline is the most-read line in the notes. Written loosely, it grows back into the essay-length TL;DR that 886c6100 (2026-06-13) removed for length, or into a claim about the scale of the work, or an inventory of its pieces. A model that thinks the top line has already summed things up also thins the sections beneath it (§3.1).

**Solution**: Add the default prompt's rules for what a headline says and how it is picked. It is one terse, plain-language sentence at the bullets' altitude. It names the main body of work and leads with what that gives its user. It is never a synthesis or a narrative thread, carries no scale or praise words and no inventory, and may name any kind of change. It is plain text, picked from what the release changes for its user as read from the diff, and every notable change is still listed beneath it.

**Outcome**: On the default prompt, the model is told how a headline reads, how it is chosen, and that the sections stay complete below it.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives hold the headline to the bullets' altitude. It is one terse sentence in plain language that names the main body of work and leads with what users can now do, or no longer suffer, in the terms they meet it, never in files, formats or internals. It is never a synthesis of everything in the release, and never a narrative thread that ties the changes together (§3.1)
- [ ] The same instructions bar scale and praise words such as "major", "overhaul" or "powerful", and any inventory of the work's pieces, leaving the scale to the sections (§3.1)
- [ ] The same instructions say the main body of work may be a feature, a rework, a fix or a large epic, and the headline names it whatever section its pieces sit in (§3.1)
- [ ] The same instructions keep the headline plain text: no bold, no emoji, and no label such as "Highlights:" or "TL;DR:" (§3.1)
- [ ] The same instructions say the headline does not relieve the sections: every notable change, the pieces of the main work included, is listed below it exactly as it would be without one (§3.1)
- [ ] The same instructions say the headline is picked by what the release changes for its user, read from the diff. A new folder or a large file is not, of itself, the release's main thing, and the Change Map does not pick the headline (§3.3, §6.2)

**Do**:
- The headline rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's headline passage (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 46–55 and 61–63). Where it and §3.1 or §3.3 differ, the specification governs (§10.2).

**Context**:
> §3.1: in the common release, one body of work built from many pieces plus some sideline changes, the main work takes several bullets, so the headline cannot be a lifted bullet. Where the main work is a single bullet, the overlap costs one repeated line. The headline and the sections overlap by design: summary, then detail.
>
> §3.3: the inputs stay those of §1.2, with no commit history. The session that built a feature knows its headline better than mint's diff-only view does; that is the ground of §3.5, and this feature does not close it.
>
> When a headline is written at all is tasks 1-3 and 1-4.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented. §3.1's example of the shape illustrates the rule for the specification's reader.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §3.1 Shape; §3.3 How the headline is picked; §6.2 The Change Map is coverage, not ranking; §10.2 Tuning and acceptance

## headline-first-release-notes-1-3

### Task 1-3: Headline only for a main body of work

**Problem**: A headline is right only where one body of work stands out. Written on the largest of several separate changes, or naming a lesser piece beside the main work, it misstates the release in its most-read line. new-v3, the starting wording, runs the two-body test without setting the other body aside, and bars a lesser second subject only where "And" joins it (§3.2, §10.2).

**Solution**: Add the default prompt's test for when a headline is written: a main body of work, tested against the notes' own bullets. Two co-equal bodies are named together in one benefit-led sentence only where each passes the test with the other set aside. A second subject that is not a co-equal body is never named, however it is joined, and a headline names at most two.

**Outcome**: On the default prompt, the model writes a headline only for a main body of work, or for two co-equal bodies, by §3.2's test.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives write a headline only for a main body of work: one feature, rework or fix that anyone asked "what is this release?" would name. The test is run against the notes' own bullets: most of the notable changes serve it, or it alone clearly outweighs all the other changes put together (§3.2)
- [ ] The same instructions say the largest of several separate changes is not a main body of work (§3.2)
- [ ] The same instructions name two bodies of work of equal weight together, in the one sentence and led by the benefit, only where each passes the test once the other is set aside (§3.2, §10.2)
- [ ] The same instructions name a second subject only when it is a co-equal body of work: never two pieces of one body, and never a main body plus a lesser piece, whether "And" or anything else joins them (§3.2, §10.2)
- [ ] The same instructions cap a headline at two named bodies in one sentence, with no exception for a release with more than two co-equal bodies (§3.2)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's headline test (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 37–45), changed where §10.2 lists. Line 42 names two bodies together where each "pass the test"; §3.2 runs each body's test with the other set aside. Line 44 bars a lesser second subject only where "And" joins it; §3.2 bars any second subject that is not a co-equal body of work.

**Context**:
> §3.2: a release with more than two co-equal bodies is not designed for. The one-sentence limit stands, and the model handles the case as best it can.
>
> §10.3 lists two-body releases losing their headline, or getting half of one, as a known weak spot. It is tuned in Phase 3 by wording only; the rules stay as specified.
>
> Where the headline is left out (a single bullet stating the whole release, a grab-bag, doubt) is task 1-4.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §3.2 When a release has one; §10.2 Tuning and acceptance (differences from new-v3); §10.3 Where the tuning goes (known weak spots)

## headline-first-release-notes-1-4

### Task 1-4: Headline left out where it would repeat, stitch or guess

**Problem**: A headline costs the reader where it only repeats the one bullet that already states the whole release, where it stitches a theme across unrelated changes, or where the model guesses. On an unattended run (`mint release -y`, as agent sessions run it) nobody reads the notes before they ship, so a wrong headline goes out in the release's most-read line. The opposite mistake matters too: where the main work is spread over several bullets and none names the whole, dropping the headline loses the one line that says what the release is (§3.2).

**Solution**: Add the default prompt's rules for leaving the headline out. There is none where a single bullet states the whole release and nothing else stands beside it; the test is whether one bullet states the whole, not how many bullets there are. The headline stays where the main work is spread over several bullets none of which names the whole, and for a main feature written as one bullet beside unrelated changes. There is none on a grab-bag, and none when in doubt.

**Outcome**: On the default prompt, the model leaves the headline out where it would repeat a bullet, stitch a theme or guess, and keeps it where the main work needs one line to name it.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives leave the headline out where one bullet already states the whole release and nothing else stands beside it. That is judged by whether a single bullet states the whole release, not by how many bullets there are (§3.2)
- [ ] The same instructions keep the headline where the main work is spread over several bullets and none of them names the whole, even when the notes hold nothing else (§3.2)
- [ ] The same instructions keep the headline for a main feature written as a single bullet beside unrelated changes (§3.2)
- [ ] The same instructions write no headline on a grab-bag of small unrelated changes, however large the largest of them, and stitch no theme across such changes (§3.2)
- [ ] The same instructions write a headline only when one body of work, or two co-equal bodies, clearly stands out, and otherwise start the notes with the first section: when in doubt, leave it out (§3.2)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's leave-out passage (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 56–60). Where it and §3.2 differ, §3.2 governs (§10.2).

**Context**:
> §3.2: a missing headline costs no more than today's notes do, while a wrong one misstates the release in its most-read line. Accepted cost: some releases that deserved a headline go without one. On an attended run the review gate's existing correctives remain unchanged: `e` to edit, and `r` to regenerate with a one-time nudge such as "lead with the auth work".
>
> §3.1: a theme stitched across a grab-bag is the narrative thread the headline rules out (task 1-2).
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §3.2 When a release has one; §3.1 Shape (narrative thread); §10.2 Tuning and acceptance

## headline-first-release-notes-1-5

### Task 1-5: Each change placed against the previous release

**Problem**: Today's prompt glosses each section ("new features and capabilities", "changes to existing behaviour", "bug fixes") without saying what a change is judged against. Nothing in it stops a bug in something built during this release being reported as Fixed, a tweak to a new feature as Changed, or a rework of existing behaviour as Added. On E1's 52 bodies, today's notes carried 84 misfiled items and 36 reworks of existing behaviour filed under ✨ Added (§10.1, §10.3). A reader upgrading shares only one baseline: the previous release (§4.1).

**Solution**: Add the default prompt's placement rules, each judged against the previous release and never against earlier states inside this one. Added covers what the previous release did not have, its adjusted pieces included. Changed covers a rework of something it had and any other non-fix change to how that works. Fixed covers what behaved wrongly in it and now behaves correctly, however the diff frames it. Removed covers what it had that is gone, unless the removal breaks existing use.

**Outcome**: On the default prompt, the model places every change by comparing it with the previous release.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives place every change by comparing it with the previous release, never with earlier states inside this release (§4.1)
- [ ] The same instructions file under ✨ Added what the previous release did not have, its pieces included even where the diff shows them adjusted, capped or fixed along the way. A bug in something new in this release is never Fixed, and a tweak to it is never Changed (§4.1)
- [ ] The same instructions file under 🔧 Changed a rework or redesign of something the previous release already had, and any smaller change to how it works that is not a fix, such as a speed-up users notice or a changed default that is not breaking. A genuinely new capability is Added, whether it ships inside a rework or is added to something the previous release had, such as a new option on an existing command (§4.1)
- [ ] The same instructions file under 🐛 Fixed something that behaved wrongly in the previous release and now behaves correctly, however the diff or its comments frame it. Behaving wrongly covers wrong output or placement, a crash or error, dropped or lost data, and something not working. A rework whose purpose is the fix, and something an earlier release lost that this one brings back, are Fixed too (§4.1)
- [ ] The same instructions file under 🗑️ Removed something the previous release had that is gone, and put a removal that breaks existing use under 💥 Breaking only (§4.1)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's placement passage and section glosses (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 65–78, and lines 15–21). Where it and §4.1 differ, §4.1 governs (§10.2).

**Context**:
> §4.1: the previous release is the baseline a reader upgrading from it shares. ⚠️ Deprecated and 🔒 Security keep their opportunistic rule (§2.2, carried by task 1-1). What qualifies as breaking, and the 💥 Breaking section's own rules, are task 1-6.
>
> §5.2: no inputs are added. The previous release's state is already in front of the AI, in the diff's removed and context lines.
>
> §10.3 lists a rework of existing behaviour filed under ✨ Added (36 in today's notes, 34 in E1's candidate) as a known weak spot, tuned in Phase 3 by wording only.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §4.1 Judged against the previous release; §5.2 True to the release (no new inputs); §10.1 The bar; §10.3 Where the tuning goes (known weak spots)

## headline-first-release-notes-1-6

### Task 1-6: 💥 Breaking held narrow

**Problem**: A reader upgrading must meet what breaks before anything else, and today's prompt has no place for it: today's notes flagged 1 of E1's 10 breaking changes (§4.2, §10.1). The section sits at the top of the notes, though, so a false alarm stands in the opening lines where every reader looks. It has to be narrow to keep from crying wolf.

**Solution**: Add the default prompt's 💥 Breaking rules. A breaking change goes there only. It qualifies only where something users already do stops working or gives a different result they must act on, with the spec's examples. A change that only adds or improves is not breaking, and an unsure change is not breaking. Each bullet says what breaks and, only from what the diff shows, what to do.

**Outcome**: On the default prompt, the model lists a breaking change once, under 💥 Breaking, only when it clearly qualifies, and tells the reader what to do only where the diff shows it.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives put a breaking change under 💥 Breaking only, never repeated in another section (§4.2)
- [ ] The same instructions count a change as breaking only where something users already do stops working, or gives a different result they must act on. They give the qualifying examples: a removed or renamed command, flag, option, setting or key binding; a default changed so that existing setups behave differently in a way that matters; a changed output or file format that scripts or tools read; data or configuration from the previous release no longer read (§4.2)
- [ ] The same instructions say a change that only adds, keeps more, allows more or improves is not breaking (§4.2)
- [ ] The same instructions say that when it is unsure whether a change is breaking, it is not, and it goes in its usual section (§4.2)
- [ ] The same instructions ask each Breaking bullet to say, in its one sentence, what breaks and what to do. What to do comes only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour. Where the diff shows none, the bullet says what breaks and stops there (§4.2, §10.2)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's breaking-change rule (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 79–87), changed where §10.2 lists: line 87 asks every breaking bullet to "Say what breaks and what to do", and §4.2 takes what to do only from what the diff shows.

**Context**:
> §4.2: "when unsure, it is not breaking" is the same asymmetry as the headline's (§3.2). A missed flag costs what today's notes already cost, while a false one misstates the release where every reader looks. The section comes first among the sections, after the headline where there is one, and is left out when empty (§2.1, task 1-1).
>
> §10.3 lists 💥 Breaking misfilings, and false breaks that assert a previous state the diff contradicts, as a known weak spot, tuned in Phase 3 by wording only.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §4.2 💥 Breaking; §2.1 Shape (section order); §10.1 The bar (breaking changes flagged); §10.2 Tuning and acceptance (differences from new-v3); §10.3 Where the tuning goes

## headline-first-release-notes-1-7

### Task 1-7: Notes limited to what users meet

**Problem**: Release notes are for the people who use the software, but today's prompt sets no line between what they meet and how the software is built. CI, test harnesses, internal packages and refactors reach the notes: on E1's 52 bodies, today's notes carried 77 internal items (§5.1, §10.1).

**Solution**: Add the default prompt's rules limiting the notes to what a user meets. There is an in list and an out list. The test is whether a user meets or notices the change, not whether the code behind it is internal, and shipped prompts, skills and templates count. Every change is described in the user's terms.

**Outcome**: On the default prompt, the model lists only changes a user of the software meets or notices, described as users see them.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives include what a user of the software meets: what the software does, its commands, options, configuration, output, errors, performance and compatibility (§5.1)
- [ ] The same instructions leave out how the software is built and maintained: CI and release tooling, linting, build scripts, test suites and harnesses, internal packages, modules and refactors, internal flags and plumbing commands, data-migration numbering, contributor docs, and repo housekeeping such as restructuring the README. A new internal package is described by what it lets users do, or left out (§5.1)
- [ ] The same instructions make the test whether a user meets or notices the change, not whether the code behind it is internal. A speed-up users notice is listed. Where the shipped files are the product (prompts, skills or templates the software ships and uses), changes to those files count (§5.1)
- [ ] The same instructions describe every change as users see and use it, never in the code's names, the mechanism, or a design document's jargon (§5.1)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's users-meet passage (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 107–119). Where it and §5.1 differ, §5.1 governs (§10.2).

**Context**:
> §10.2 names new-v3's internal-efficiency leave-out line ("internal efficiency changes a user would not notice", line 113) a tuning point. §10.3: E1's candidate dropped a speed-up users notice under that line, which is why §5.1 draws the line at what users notice. Phase 3 tunes that line; whatever wording this task carries, a speed-up users notice is listed.
>
> A release where every change is one this task leaves out gets the maintenance line (task 1-8). Claims drawn from documentation, comments or tests are task 1-9.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.1 Only what users meet; §10.1 The bar (internal items); §10.2 Tuning and acceptance; §10.3 Where the tuning goes

## headline-first-release-notes-1-8

### Task 1-8: Maintenance line for a release with nothing users meet

**Problem**: Once the notes leave out everything users do not meet, a release made only of such changes leaves the model nothing to list, and new-v3 has no rule for it (§5.1, §10.2). The notes can never be empty, since the transport rejects an empty body as a failed call. Guessing the other way is worse: a maintenance line on a release with a change users meet drops that change and tells every reader, in the release's only line, that nothing changed for them.

**Solution**: Add the default prompt's rule for a release with nothing users meet. Where every change in the release is one the notes leave out, the notes are one plain line, in the shape of the empty-diff stub, saying the release is maintenance with no user-facing changes. That body has no headline and no sections. When unsure, the change in doubt is listed in its section and the line is not written.

**Outcome**: On the default prompt, a release with nothing users meet is asked for the one maintenance line, and a release with a change in doubt is asked to list it instead.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives say that where every change in the release is one the notes leave out, the notes are one plain line saying the release is maintenance with no user-facing changes, in the shape of `Maintenance release — no notable source changes`, never an empty body (§5.1)
- [ ] The same instructions give that line no headline and no sections, and it is the one body the format allows without sections (§5.1, §2.1)
- [ ] The same instructions say that when unsure whether the release holds any change users meet, the change in doubt is listed in its section and the maintenance line is not written (§5.1)

**Do**:
- The rule lives in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1). new-v3 has no such rule, so it is added (§10.2).
- The empty-diff stub whose shape the line follows is `stubBody` in `internal/notes/degenerate.go`. That stub and its no-AI route stay as they are (§3.4).

**Context**:
> §5.1: a line left out costs what today's notes cost, while a false one drops a change users meet. That asymmetry is why the change in doubt is listed.
>
> On the chunked path only the merge can say this of the release. A part whose slice holds nothing users meet returns its own one line, and the merge writes the maintenance line only when every part returned one. That is Phase 2 (§8).
>
> §10.3: Phase 3 checks a release with nothing users meet on the frozen wording. A headline on that release, an empty body that fails the run, or a maintenance line written for a release with a change users meet, holds shipping.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.1 Only what users meet (a release with nothing users meet); §2.1 Shape; §3.4 Bodies that carry no headline; §10.2 Tuning and acceptance; §10.3 Where the tuning goes

## headline-first-release-notes-1-9

### Task 1-9: Notes true to the release

**Problem**: From the diff alone, the model reads edited documentation, comments and tests as changed behaviour. It also presents work built in this release as a change to the previous one, and announces as new what the previous release already had. On E1's 52 bodies, today's notes carried 25 false claims and 22 items built in this release presented as changes, and E1's candidate raised false claims to 34 (§5.2, §10.1, §10.3). new-v3 looks for what the previous release had only in unchanged context lines and moved or renamed code, not in the diff's removed lines (§10.2).

**Solution**: Add the default prompt's truthfulness rules. No behaviour is claimed from documentation, READMEs, comments, examples or tests alone, except where those files are the product, and a change to tests alone is not a fix. Work built in this release is never presented as a change to or fix of the previous release. Nothing the previous release had is presented as new, wherever the diff shows it.

**Outcome**: On the default prompt, the model claims only what the code in the diff changes, judged against what the previous release already had.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives claim no behaviour from documentation, READMEs, comments, examples or tests alone, because those describe behaviour and editing them does not change it. Files that are the product (prompts, skills or templates the software ships and uses) are the exception (§5.2, §5.1)
- [ ] The same instructions say a change to tests alone is not a fix (§5.2)
- [ ] The same instructions never present work built in this release as a change to, or fix of, the previous release (§5.2)
- [ ] The same instructions never present as new anything the previous release already had, whether the diff shows it in unchanged context lines, in removed lines, or in code the diff only moves or renames (§5.2, §10.2)

**Do**:
- The rules live in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's truthfulness passage (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 89–105), changed where §10.2 lists: lines 97–98 look for what the previous release already had only in unchanged context lines and moved or renamed code, and §5.2 adds the diff's removed lines.

**Context**:
> §5.2, no new inputs: the previous release's state is already in front of the AI, in the diff's removed and context lines. E1's judges called only 3 of the 47 accuracy residuals in the rewritten notes input-absent, and three "the previous release already had it" calls, spot-checked, each sat in the diff's removed lines. So the fix is how the prompt gets the diff read, not what mint sends. The composed prompt's parts stay as they are (task 1-12).
>
> §10.3: false claims are furthest from the bar (34 against at most 25). Phase 3 tunes them back by wording only.
>
> Where a change is placed (Added, Changed, Fixed) is task 1-5.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §5.2 True to the release; §5.1 Only what users meet (files that are the product); §10.1 The bar; §10.2 Tuning and acceptance (differences from new-v3); §10.3 Where the tuning goes

## headline-first-release-notes-1-10

### Task 1-10: Each section ordered by weight to the user

**Problem**: Today's notes order changes within a section by stand-ins for importance: lines, files, commits, and how easy a change is to name. The audit traced the inversions within sections to exactly those stand-ins, granularity and nameability, and on E1's 52 bodies today's notes carried 30 inversions (§6.1, §10.1).

**Solution**: Add the default prompt's ordering rule. Within each section, changes run by what they mean to the user, most important first. Weight is how much of the product a change touches, how often users meet it, and how much it changes their experience. The pieces of the main work lead their sections. Lines, files, commits and nameability are not weight.

**Outcome**: On the default prompt, the model orders each section by weight to the user in the user's terms.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives order the changes in each section by what they mean to the user, most important first (§6.1)
- [ ] The same instructions define weight as how much of the product a change touches, how often users meet it, and how much it changes their experience. A change to something users do every day outranks a change to a diagnostic or rarely used command (§6.1)
- [ ] The same instructions put the pieces of the release's main work at the head of their sections (§6.1)
- [ ] The same instructions say lines, files, commits, and how easy a change is to name, are not weight (§6.1)

**Do**:
- The rule lives in `DefaultPrompt` in `internal/notes/prompt.go` alone (§7.1).
- Start from new-v3's ordering passage (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 121–126). Where it and §6.1 differ, §6.1 governs (§10.2).

**Context**:
> §6.2: the Change Map is never a measure of importance for ordering; its role as a coverage check is task 1-11. The section order itself is fixed (§2.1, task 1-1); this rule orders changes within a section.
>
> §10.2: no prompt text names, quotes or paraphrases a change from the 31 sampled releases, and any example added is invented.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §6.1 Ordering within sections; §6.2 The Change Map is coverage, not ranking; §10.1 The bar (inversions); §10.2 Tuning and acceptance

## headline-first-release-notes-1-11

### Task 1-11: Change Map described as a coverage check, not a ranking signal

**Problem**: The default prompt calls the Change Map "a diff-derived salience summary" and tells the model to "Rank importance using the Change Map: new structure (a whole new package or directory) is the strongest signal — weight it above raw size". mint's own doc comments describe the map the same way, as what tells the AI what to prioritize. But detection reads only a path's first segment, so a new package under an existing `internal/`, `src/` or `skills/` never registers. The audit found the novelty line fired in 6 of 31 releases and pointed the wrong way each time, and the largest-file line is often a test, a deletion or a dev script (§6.2).

**Solution**: Replace the ranking rule and the salience description in the default prompt with the map's coverage role: a check on where the changes landed, so that nothing large is missed, never a measure of importance. Change mint's own descriptions of the map to match: the doc comments on how it is built, composed and generated, and the novelty comments that weight it above magnitude or call a new directory its headline. Leave the map exactly as it is built.

**Outcome**: The instructions describe the Change Map as a coverage check, nothing in `internal/notes` outside the tests calls it a salience or priority signal, and the map the AI receives is byte for byte what it was.

**Acceptance Criteria**:
- [ ] Given a repo on the default prompt, when a release's notes are written in one call, the instructions the AI tool receives describe the Change Map as a check on where the changes landed, used so that nothing large is missed, and never a measure of importance. Every change is described from the diff, and the map is metadata, never content (§6.2)
- [ ] The same instructions carry no ranking rule for the map. "Rank importance using the Change Map" and "new structure (a whole new package or directory) is the strongest signal — weight it above raw size" are gone, and nothing calls the map a salience summary (§6.2)
- [ ] Given the same range and exclusions, when the notes prompt is composed, its Change Map is byte-identical to the one mint built before this change, from the same git calls. A release that adds every file under `internal/foo/` while modifying another file under `internal/` still flags no new package or directory (§1.2, §6.2)
- [ ] `rg -n -i 'salience|prioriti' internal/notes --glob '!*_test.go'` prints nothing, and no comment in `internal/notes/changemap.go` weights novelty above magnitude or calls a new directory the map's headline (§6.2)

**Do**:
- The prompt's description and the removed ranking rule live in `DefaultPrompt` in `internal/notes/prompt.go` (§7.1). Start from new-v3's description of the map (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3.md`, lines 1–3 and 128–131) (§10.2).
- The descriptions to change are the 11 lines `rg -n -i 'salience|prioriti' internal/notes --glob '!*_test.go'` finds, and the novelty comments `rg -n -i 'weighted above|headline' internal/notes/changemap.go` finds at lines 26, 198, 199, 238 and 239 (§6.2).
- The map's git calls, detection (`leadingDir`, `newDirectories`) and rendering in `internal/notes/changemap.go` stay as they are (§1.2, §6.2).

**Context**:
> §6.2: detection is not fixed on purpose. Making it find new packages at any depth would make it fire on exactly the internal packages §5.1 keeps out of the notes, and it would still flag novelty rather than what a release changes for its user. The map stays in every notes call, single-pass, chunk, piece and merge alike.
>
> §1.5: this replaces the mint-release-tool specification's Change Map as the ranking signal, and release-notes-quality's "rank importance from the Change Map", for the default prompt.
>
> The merge framing's "rank by the Change Map above" is dropped in Phase 2 (§7.1). That the Change Map does not pick the headline is task 1-2, and ordering by weight is task 1-10.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §6.2 The Change Map is coverage, not ranking; §1.2 What does not change; §1.5 Decisions this replaces; §10.2 Tuning and acceptance

## headline-first-release-notes-1-12

### Task 1-12: Format-neutral closing reminder on every notes call

**Problem**: The closing reminder is the last thing the model reads on every notes call, on every repo, including repos on a full `[release].prompt` override. It is the hardening against models narrating before the notes, observed in real `mint commit` runs. Today it does not point back to the format that governs. E1's version named the headline, and on an override repo that line would tell the model to put a headline above sections its format may not have (§7.1).

**Solution**: Change the notes reminder to §7.1's format-neutral wording, so that every single-pass, chunk, piece and merge call closes with it. "The format above" means whichever instructions govern: on the default prompt, a format that opens with the headline. Every headline rule, and every rule of §2–§6, stays in the default prompt alone, so an override repo's calls carry nothing that asks for a headline.

**Outcome**: Every notes call on every repo ends with the same format-neutral reminder. Default-prompt repos, with or without `[release].context`, get the headline rules; override repos get only their own instructions.

**Acceptance Criteria**:
- [ ] Given repos on the default prompt, on the default prompt with `[release].context`, and on a full `[release].prompt` override, when each writes a release's notes in one call, the prompt the AI tool receives ends with exactly: "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes." (§7.1)
- [ ] Given the override repo, the instructions in its prompt are its prompt file verbatim, and nothing mint adds to the call, the reminder included, names or asks for a headline (§7.2)
- [ ] Given the context repo, its prompt carries every rule of the default prompt, with its context appended under the existing "Additional project context (apply the rules above with this in mind):" header (§7.2)
- [ ] Given a release whose notes are written in parts, every chunk and piece call, and the merge call, ends with the same reminder (§7.1)
- [ ] Given any of these repos, the single-call prompt holds the instructions, the Change Map, the diff and the reminder, in that order, and nothing else: no commit history and no added input (§1.2)
- [ ] Given `mint commit`, its prompt and its own closing reminder are unchanged (§1.2)

**Do**:
- The reminder is `OutputReminder` in `internal/notes/prompt.go`. It closes the single-pass prompt (`ComposePrompt`, `prompt.go`), the chunk and piece calls and the merge call (`chunk.go`): `rg -n 'OutputReminder' internal/notes/chunk.go internal/notes/prompt.go | grep -v '//'` (§7.1).
- The wording is §7.1's, not new-v3's reminder (`.workflows/headline-first-release-notes/experiment/headline-first-release-notes/E1-notes-prompt-on-real-releases/prompts/new-v3-reminder.txt`, which names the headline) (§10.2).
- `mint commit`'s reminder is its own constant in `internal/commit/prompt.go` and stays as it is (§1.2).

**Context**:
> §7.1: whether the model keeps the headline under the neutral wording is measured in Phase 3 (§10.3). What would reopen this decision: evidence that the model keeps a headline only when the last thing it reads names one.
>
> The chunk and piece framings and the merge framing get their own format-neutral changes in Phase 2 (§7.1, §8). This task changes only the reminder that closes those calls.
>
> §7.2: `[release].context` is appended under its labelled header and leaves the default's rules intact. A full override is the operator's format and gets no headline unless the operator's prompt asks for one.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §7.1 mint's shared wording stays format-neutral; §7.2 What reaches which repo; §1.2 What does not change; §10.2 Tuning and acceptance; §10.3 Where the tuning goes (the neutral reminder)

## headline-first-release-notes-1-13

### Task 1-13: Headline-led bodies ship byte-identical under a bare-tag title

**Problem**: A headline reaches readers only if the body that opens with it ships untouched on all three surfaces: the tag annotation, the `CHANGELOG.md` section and the provider release body. The provider release title stays the bare tag rather than lifting the headline, and the six bodies that never go through the notes prompt must ship as before, with no headline from mint (§1.2, §3.4, §9).

**Solution**: Prove, on the forward release and both regenerate paths, that a body opening with a headline line passes through byte for byte to every surface with the bare tag as the provider release title. Prove too that the first-release, empty-diff, `--no-ai`, fallback, `--notes-file` and notes-floor bodies carry nothing mint added. mint's pass-through and its bare-tag title are as built (§1.2, §9); this task pins them for a headline-led body.

**Outcome**: A headline-led body ships identically everywhere, every provider release title is the bare tag, and no body that skips the AI gains a headline.

**Acceptance Criteria**:
- [ ] Given an AI body whose first line is a headline, followed by one blank line and the sections, when `mint release` ships it, the tag annotation, the `CHANGELOG.md` section under its `## [x.y.z] - date` header and the provider release body each carry that body byte for byte, the headline line included. The provider release title is the bare tag (§1.2, §9)
- [ ] Given a fresh-source `mint release regenerate` whose AI body opens with a headline, for a single version and for an `--all` batch, the provider release is published with that body byte for byte and the bare tag as its title (§9)
- [ ] Given a tag whose annotation body opens with a headline, when `mint release regenerate --source tag` rebuilds its provider release, for a single version and for an `--all` batch, the release body is the annotation body byte for byte and the title is the bare tag (§9)
- [ ] Given a first release, a release whose post-exclusion diff is empty, a `--no-ai` release, a release whose AI call fails under `on_notes_failure = "fallback"`, and an attended release whose AI call fails and whose operator chooses the commit subjects at the notes floor, the body that ships is `Initial release.`, `Maintenance release — no notable source changes`, or the commit-subject list, exactly as built, with no headline or other line added (§3.4)
- [ ] Given a `--notes-file` body, or notes the operator writes in the editor at the notes floor after the AI fails on an attended run, the body ships byte for byte as the operator wrote it: it opens with a headline only where the operator wrote one, and mint adds none (§3.4)

**Do**:
- The title call sites (§9): `CreateRelease(ctx, tag, tag, body)` at `internal/engine/release.go:786`; `DispatchRelease(…, res.Tag, res.Tag, …)` at `internal/engine/regenerate_batch.go:361` and `DispatchRelease(…, req.Tag, req.Tag, …)` at `internal/engine/regenerate_write.go:200`.
- The body kinds are in `internal/notes/select.go` (`rg -n '// Kind[A-Za-z]+:' internal/notes/select.go` → 7 lines); only the AI-written kind goes through the notes prompt (§3.4).

**Context**:
> §9: lifting the headline into the title is ruled out. Titles would vary with whether the AI wrote a headline, so the releases list would mix summaries with bare numbers. mint would have to read into a body it otherwise passes through untouched. Both regenerate paths re-publish with a title, so regenerating would retitle releases already out. The gain is confined to the releases list, since the release page already shows the headline as the first line under the title. The bare-tag title also keeps `--source tag`'s rebuild of the provider release from the annotation body deterministic and free of parsing.
>
> §1.2 and §3.4: mint never parses, reformats or adds to a notes body, and never writes a headline. A full `[release].prompt` override gets a headline only if it asks for one (§7.2, task 1-12). The saved part notes of a failed chunked run carry no headline (§8, Phase 2).
>
> §3.5: no route for a headline supplied by an operator or an outer agent is designed here. The headline slot, one plain sentence above the first section, is what a later route fills.

**Spec Reference**: `.workflows/headline-first-release-notes/specification/headline-first-release-notes/specification.md` — §1.2 What does not change; §3.4 Bodies that carry no headline; §3.5 Room for a supplied headline; §9 The Provider Release Title
