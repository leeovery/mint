# Review Tracking: token-aware-diff-sizing - Claims Verification

## Findings

### 1. The old whitespace-split rule is stated in more comments than the spec names

**Source**: Tree measurement — `grep -rn -i 'whitespace-split\|whitespace split\|by whitespace\|strings.Fields split' internal/ai internal/config internal/aitransport --include='*.go' | grep -v _test.go`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §3.3 Compatibility (and §18.3, which points to it for the whitespace-split comments)

**Problem**:
The spec names two places whose comments say `ai_command` is split on whitespace: the transport's command parsing and config's `ai_command` field. Two more comments state the same rule. One is on config's `AICommandFor` accessor ("the transport whitespace-splits it"). The other is on `aitransport.New`, the one place production transports are built ("whitespace-splitting the command into name + args"). If only the named places are rewritten, those two keep telling the next maintainer or agent that quotes are not parsed, after quote-aware parsing has shipped. Someone who trusts the comment at the transport's construction site could then advise an operator that a quoted `--system-prompt` can't work, or write a test that expects it to fail.

**Proposal**:
List every site the measurement finds: the transport's `ai.Config` command field, `Generate` and `parseCommand` (`internal/ai/transport.go`), config's `ai_command` field and its `AICommandFor` accessor (`internal/config/config.go`), and `aitransport.New` (`internal/aitransport/aitransport.go`). The grep settles the list. CLAUDE.md's rule that comments stay true to what is built covers all of them the same way.

**Evidence**:
Claim (§3.3): "The code comments that describe a whitespace split are rewritten to the new rule, in the same change. They sit on the transport's command parsing and on config's `ai_command` field."

Measurement:
```
internal/ai/transport.go:92:// whitespace-split into name + args; see Generate for why a simple split is sufficient.
internal/ai/transport.go:171:// The ai_command is parsed into name + args by a whitespace split (strings.Fields):
internal/ai/transport.go:306:// -p`), so a strings.Fields split is sufficient — it carries no quoting or shell syntax a
internal/config/config.go:129:	// transport just whitespace-splits the resolved command into name + args (it is
internal/config/config.go:535:// so the operator's exact command survives verbatim (the transport whitespace-splits
internal/aitransport/aitransport.go:30:// is handed (whitespace-splitting the command into name + args).
```
`config.go:535` sits in the doc comment of `func (c Config) AICommandFor(verb Verb) string` (line 542), not on the field. `aitransport.go:30` sits in the doc comment of `func New(...)` (line 39). The discussion names only the transport's `parseCommand` comment ("the whitespace split is as-built only (the transport's `parseCommand` comment)"), so the spec's two-place list is its own.

**Current**:
The code comments that describe a whitespace split are rewritten to the new rule, in the same change. They sit on the transport's command parsing and on config's `ai_command` field.

**Proposed Text**:
The code comments that describe a whitespace split are rewritten to the new rule, in the same change. They sit on the transport's command parsing (the `ai.Config` command field, `Generate` and `parseCommand`), on config's `ai_command` field and its `AICommandFor` accessor, and on `aitransport.New`: `grep -rn -i 'whitespace-split\|whitespace split\|by whitespace\|strings.Fields split' internal/ai internal/config internal/aitransport --include='*.go' | grep -v _test.go` → `internal/ai/transport.go:92`, `:171`, `:306`; `internal/config/config.go:129`, `:535`; `internal/aitransport/aitransport.go:30` (2026-10-18).

**Resolution**: Pending
**Notes**:

---

### 2. The docs still promise a failed release leaves nothing behind

**Source**: Tree measurement — `grep -n -i 'unwind\|PONR\|exactly as it found' CLAUDE.md README.md`; `internal/setupguide/setupguide.go:238-240`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §18.1 The setup guide, §18.2 The README, §18.3 CLAUDE.md and code comments (the behaviour is in §6.6)

**Problem**:
Suppose an operator answers the exclusion check's pauses and the release then aborts or fails before the push. The answers commit stays on the branch, re-committed on the starting commit, because the answers are about the repo and not about this release. Four passages in the docs promise the opposite, and none of them is in the spec's lists of doc edits:

- the README's feature list: "Everything before the `git push --atomic` is unwindable … leaving the repo exactly as it found it"
- the README's surgical-unwind bullet: "mint removes *its own* commits, tag, and autostash"
- CLAUDE.md's load-bearing invariant: "every pre-push mutation is tracked and unwound on failure"
- the setup guide's pipeline section: "Everything before it is unwindable; nothing after it is"

CLAUDE.md's catch-all only covers lines that name a renamed or removed symbol, so it misses the invariant. If these stay as they are, an operator who finds the commit after an abort will read the README as saying mint broke its promise. Worse, an agent changing mint under CLAUDE.md's invariant could treat the kept commit as a bug and unwind it, which brings the same questions back on the next release.

**Proposal**:
Add each passage to its document's edit list, so each one states the single exception. Everything mint makes before the push is still unwound, except the exclusion check's answers commit, which stays and is re-committed on the starting commit. The exception is the deliberate decision on recording answers. The rule that CLAUDE.md, the README and the setup guide stay true to what is built (§18's lead, and the discussion's "edited here only where this feature makes it untrue" for the guide) fixes where the correction goes. No informed reader would want docs that promise "exactly as it found it" when a commit is left behind.

**Evidence**:
Claims: §18.1 "It is edited only where this feature makes it untrue", followed by three bullets (AI-transport paragraph, `diff_exclude` advice, config reference). §18.2 lists per-key tables, the lean command, quoting rules, and `--notes-file`/exclusion check/chunking docs. §18.3 "CLAUDE.md's config seam says … It is rewritten … Any other line naming a renamed or removed symbol is kept true the same way." §6.6: "the answers commit stays … This is the one deliberate exception to the surgical unwind."

Measurement:
```
CLAUDE.md:42:- **Surgical unwind, then PONR** (release): every pre-push mutation is tracked and unwound on failure — mint removes only what it created. ...
README.md:30:- **One atomic point of no return.** Everything before the `git push --atomic` is unwindable: if anything fails pre-push, mint surgically unwinds its own commits, tag, and stash, and only its own, leaving the repo exactly as it found it.
README.md:269:- **Surgical unwind before the point of no return.** Everything `mint release` does before the atomic push is tracked; on any pre-push failure (including Ctrl-C) mint removes *its own* commits, tag, and autostash (never your work) and reports exactly what it undid.
internal/setupguide/setupguide.go:238-240: - tag + atomic push is the POINT OF NO RETURN (PONR): the annotated tag and the branch go up together in one `git push --atomic`. Everything before it is unwindable; nothing after it is.
```

**Proposed Text**:
In §18.1, a bullet after the `diff_exclude` advice bullet:
- **The point-of-no-return line.** The pipeline section's "Everything before it is unwindable; nothing after it is" gains its one exception: after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit (§6.6).

In §18.2, a bullet after the per-key tables bullet:
- **The unwind promises.** The feature list's "Everything before the `git push --atomic` is unwindable … leaving the repo exactly as it found it" and the surgical-unwind bullet name the one exception: after an abort or failure before the push, the exclusion check's answers commit stays on the branch, re-committed on the starting commit (§6.6).

In §18.3, a bullet after the config-seam bullet:
- The load-bearing invariant "Surgical unwind, then PONR" says every pre-push mutation is tracked and unwound on failure. It is rewritten to carry the one exception: the exclusion check's answers commit, which the unwind re-commits on the starting commit (§6.6).

**Resolution**: Pending
**Notes**:

---

### 3. `on_notes_failure`'s own description still offers the fixed-string body

**Source**: Tree measurement — `grep -n -i 'fallback' README.md internal/config/metadata.go`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §14.2 Removing the key, §18.1 The setup guide (config reference bullet)

**Problem**:
Once `[release].fallback` is removed, the description of `on_notes_failure` still says `fallback` ships "the commit-subject / fixed-string body". That description appears in two places. The config-metadata source of truth holds it, and `mint setup` renders it into the guide's config reference. The README's `on_notes_failure` row says the same thing: "(or `fallback` string)". The spec removes only the `fallback` key's own row and README entries. It also says the config reference picks up the change "with no prose change", but this description is prose that sits inside the source of truth. An agent configuring mint from the guide, or an operator reading the README, is pointed at a fixed-string body that no longer exists, and setting it fails config load.

**Proposal**:
Rewrite `on_notes_failure`'s description in both its config-metadata row and its README row to say that `fallback` ships the commit-subject list. That is the one non-AI body once the fixed string is gone, so the record leaves no other wording. The config-reference bullet should also stop claiming that no prose changes: a row description that names behaviour this feature changes is edited in the source of truth itself.

**Evidence**:
Claims: §14.2 "The key's config-metadata row and its README table entries go with it. The setup guide's prose follows (§18)." §18.1 "**The config reference** renders from the config-metadata source of truth, as built, so new, renamed and removed keys reach it with no prose change."

Measurement:
```
internal/config/metadata.go:146:		{Key: "on_notes_failure", Level: LevelRelease, Default: "abort", Description: "notes-failure policy: abort fails loud, fallback uses the commit-subject / fixed-string body"},
internal/config/metadata.go:147:		{Key: "fallback", Level: LevelRelease, Description: "fixed fallback body used verbatim by on_notes_failure = fallback and --no-ai (empty uses the commit-subject list)"},
README.md:222:| `on_notes_failure` | `abort` | `abort` fails loud; `fallback` uses the commit-subject list (or `fallback` string) |
README.md:223:| `fallback` | | fixed fallback body, used verbatim by `on_notes_failure = 'fallback'` and `--no-ai` |
```
Line 147 and README line 223 are the removed key's own entries, which §14.2 already covers. Line 146 and README line 222 belong to `on_notes_failure` and are covered nowhere.

**Current**:
§14.2: "The key's config-metadata row and its README table entries go with it. The setup guide's prose follows (§18)."

§18.1: "- **The config reference** renders from the config-metadata source of truth, as built, so new, renamed and removed keys reach it with no prose change."

**Proposed Text**:
§14.2: "The key's config-metadata row and its README table entries go with it. So does the fixed string in `on_notes_failure`'s own description, in its config-metadata row (`fallback uses the commit-subject / fixed-string body`) and its README row (`(or `fallback` string)`): both say `fallback` ships the commit-subject list. The setup guide's prose follows (§18)."

§18.1: "- **The config reference** renders from the config-metadata source of truth, as built, so new, renamed and removed keys reach it with no change to the guide's own prose. A row whose description names behaviour this feature changes is edited in the source of truth itself, `on_notes_failure`'s among them (§14.2)."

**Resolution**: Pending
**Notes**:

---

## Observations

- The claude token figures (§2.1: 105,994 and 628 input tokens; §4.5: 54,654 tokens for 120,000 bytes) need a live `claude -p` call through the switchboard shim, which writes session and account state, so they were not re-measured. The flags and their help text were checked against the 2.1.302 binary: `--safe-mode`, `--tools <tools...>` (`""` disables all), `--strict-mcp-config`, `--no-session-persistence`, `--system-prompt`, and `--bare` ("Anthropic auth is strictly ANTHROPIC_API_KEY or apiKeyHelper"). All hold.
- "claude rejects an oversized prompt instantly, before sending it" (§4.5, §9.5; the discussion says "client-side"). The 2.1.302 binary matches `^prompt is too long: \d+ tokens > \d+ maximum$` in a list of API error messages, which points to a fast server-side rejection. This can't be settled read-only, and either way the reasoning for keeping the retry and leaning high holds.
- The 10 MB stdin limit holds: the binary has `ua=10485760` and the string `Error: piped stdin input exceeds ${ua/1024/1024}MB`.
- `printf '' | git tag -a v1 -F -` exits 0 (§12.2). Not re-run, because it creates a tag.
- As built, `[release].context` is ignored when `[release].prompt` is set (`ResolveInstructions`, `internal/notes/prompt.go`). So "context and prompt … ride in every … call" (§2.3) and "the `[release].prompt` override, with `context`" (§8.6) overstate it; chunk and merge calls reuse the same resolved instructions.
- Pre-existing and outside this feature: `pre_tag` runs before notes (`internal/engine/release.go` Stage 3 → Stage 4), but the setup guide's pipeline section and the `pre_tag` config-metadata row both say "after notes".
- README prose at `README.md:260` and `:264` gives the timeout default as `60` for both verbs. §18.2 names only the per-key tables for the 180s release default.
- §9.1 and §17.2 say "every commit failure already ends in the editor", but an unattended commit fails loud instead (`errNoMessageSource`). The reasoning is unaffected.
- The `~/Code/*/.mint.toml` greps in §3.3, §4.6 and §14.2 cover top-level repos only. The nested configs (`dex/dex-engine`, `packages/laravel-idempotency`, `fabric/docman/*`, depth ≤4) set none of `ai_command`, `max_diff_lines` or `fallback` either.
