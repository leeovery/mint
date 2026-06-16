// Package setupguide holds the embedded AI setup guide that `mint setup`
// prints. It is a PURE string emitter: the single exported Guide() returns the
// version-matched static body and performs NO filesystem, git, or stdout IO —
// writing the string to a terminal (and the help/dispatch wiring) lives in the
// `mint setup` cmd-layer tasks, never here. The package deliberately takes no
// presenter dependency (CLAUDE.md seam 3 leaves the emission surface to the
// write site). It imports `config` in exactly ONE path — renderConfigReference()
// reads the config-metadata source of truth (config.MetadataRows()) to render
// the config-reference table; every prose-authoring helper stays config-free, so
// the SoT remains the single source of key metadata.
//
// Why emit the guide from the binary at all: the instructions are version-
// matched to the installed mint, so an old mint emits old-but-correct guidance
// and there is no N-versions doc maintenance — the same anti-drift discipline
// initgen follows against the schema. The guide teaches an AI agent how to
// configure mint for a project: mint's pipeline/hook model, the etiquette of
// proposing changes, the minimalism rule, handling an existing .mint.toml, and
// the ordered inspect-and-map procedure. mint itself never prompts; the agent
// runs the natural-language session.
//
// Each required section is preceded by a STABLE, greppable marker (an HTML
// comment in the `<!-- mint:section:NAME -->` namespace that cannot collide
// incidentally with prose), emitted on its own line immediately before its
// section. A structural test greps these marker CONSTANTS — not body prose — so
// a wording change inside a section never breaks the test and removing a marker
// always does.
//
// The config-reference section is assembled from configReferenceSection(): it
// authors the marker and the framing line, then splices in the rendered
// key·level·default·description table (renderConfigReference(), sourced from the
// config-metadata source of truth). Guide() composes one finished string so
// callers never re-assemble the parts.
package setupguide

import (
	"strings"

	"mint/internal/config"
)

// Section markers. Each is a fixed HTML-comment anchor in the
// `<!-- mint:section:NAME -->` namespace, decoupled from the section prose so
// the structural test keys on these constants rather than representative text.
// Every marker is emitted on its own line immediately before its section.
const (
	// MarkerPipeline precedes mint's pipeline/stage model (the ordered stages,
	// where each hook fires, the PONR, and the release-shim role mention).
	MarkerPipeline = "<!-- mint:section:pipeline -->"
	// MarkerEtiquette precedes the AI etiquette rules for proposing and
	// confirming changes.
	MarkerEtiquette = "<!-- mint:section:etiquette -->"
	// MarkerMinimalism precedes the minimalism rule (only set what varies).
	MarkerMinimalism = "<!-- mint:section:minimalism -->"
	// MarkerExistingConfig precedes the existing-`.mint.toml` / upgrade branch.
	MarkerExistingConfig = "<!-- mint:section:existing-config -->"
	// MarkerConfigReference precedes the config reference: the framing line plus
	// the key·level·default·description table rendered from the config-metadata
	// source of truth, spliced in via configReferenceSection().
	MarkerConfigReference = "<!-- mint:section:config-reference -->"
)

// Guide returns the embedded setup-guide body as one static, version-matched
// string. It performs NO IO. The body carries every required section, each
// preceded by its stable marker, and threads the ordered inspect-and-map
// procedure through the marked sections. The config-reference section is
// composed from configReferenceSection() so the rendered table has a single,
// stable splice point.
func Guide() string {
	return strings.Join([]string{
		header(),
		procedure(),
		pipelineSection(),
		etiquetteSection(),
		minimalismSection(),
		existingConfigSection(),
		configReferenceSection(),
	}, "\n\n")
}

// header introduces the guide and frames the agent's role: a knowledgeable
// collaborator, not a magic one-shot configurer (the cross-cutting principle).
func header() string {
	return `# mint setup — AI-assisted configuration guide

You are configuring mint (an AI tool for cutting releases and writing commits)
for this project. mint never prompts; YOU run the setup conversation. Act as a
knowledgeable collaborator, not an auto-configurer: explain mint's model,
propose a fit, flag anything uncertain, and never silently drop or clobber the
user's release process. The honest outcome of this conversation may legitimately
be "mint fits cleanly", "your process needs a small adaptation", or "mint isn't
the right tool here" — surface whichever is true rather than forcing a mapping.

mint ships sensible defaults compiled into the binary, so the whole .mint.toml
is OPTIONAL. Your job is to add only the keys this project has a concrete reason
to set, after inspecting what the project actually does and confirming each
change with the user.`
}

// procedure emits the ordered inspect-and-map flow. Step 1 (cwd-confirm) is the
// in-instructions safety net that replaces mint setup's missing cwd guard;
// step 3 (read the config reference) is an explicit early step BEFORE any
// inspect/edit, pointing at the embedded config-reference section in this same
// output. The numbered steps thread through the marked sections that follow.
func procedure() string {
	return `## Setup procedure (follow in order)

1. Confirm the working directory is the intended repo root BEFORE inspecting or
   writing anything. mint setup runs anywhere (it is a pure text emitter with no
   repo guard), so verify with the user that you are in the project you mean to
   configure — this is your safety net. ` + "`mint init`" + ` (run later, if needed)
   is the loud-fail backstop: it refuses to run outside a git work tree.
2. Learn mint. Read mint's README (the human config reference) for mint's
   commands and surface — this means mint's OWN README, not the target project's
   — and internalise mint's minimalist philosophy: only set what varies from the
   compiled defaults.
3. Read the config reference NOW — before any inspect or edit. The reference is
   the config-reference section embedded further down in THIS output (mint help
   deliberately omits it, and the README is the human surface). Internalise the
   key · level · default · description table so that, when you inspect the
   project, you already know each key's real default and can judge "is the
   default fine here?" against it. Do not guess defaults or restate them from
   illustrative examples elsewhere.
4. Inspect the project and map findings to config:
   - existing release process / scripts -> mint's hooks (the centrepiece — see
     the pipeline/hook model below);
   - noise directories that pollute the release-notes diff -> diff_exclude (see
     diff_exclude scope below);
   - a version file the release bumps -> version_file / version_pattern;
   - the AI model the project wants per verb -> ai_command. Ask in natural
     language which model each verb should use. SAME model for both verbs ->
     write the shared top-level ai_command. DIFFERENT models -> write
     [release].ai_command and [commit].ai_command as per-verb overrides. A
     non-Claude command is fine: ai_command is any executable that takes the
     prompt on stdin and returns the message on stdout.
   - provider / release_branch -> ONLY if mint's auto-detection would be wrong.
5. Propose -> explain -> approve. Present the proposed config, explain each
   activated key's project-specific reason, and get the user's approval per the
   etiquette rules below. Nothing is written before this.
6. Sanity-check. mint's schema is strict: an unknown, renamed, or mistyped key
   fails loudly at the next mint run (DisallowUnknownFields), so a stale key
   never passes silently. After writing, do a natural "verify the config loads"
   step (run a mint command, or mint init's backstop) to confirm the file is
   accepted.`
}

// pipelineSection carries mint's pipeline/stage model — the ordered stages and
// which hook fires where — plus the one-line release-shim role mention. This
// content is drift-sensitive: it must match the engine. Hook-detection guidance
// (best-fit mapping, never silently skip, pre_tag-as-array widens what fits) is
// woven in here because it depends on the model.
func pipelineSection() string {
	return MarkerPipeline + `
## mint's pipeline / stage model

A release runs these stages in order, with the three hook phases bracketing the
core:

    preflight -> notes -> pre_tag -> tag + push (PONR) -> publish -> post_release

- preflight runs BEFORE any release work (clean tree, right branch, tag free,
  remote in sync, provider auth). Its failure ABORTS the release.
- notes generates the AI release notes from the diff, reviewed at a gate.
- pre_tag runs AFTER notes and BEFORE the tag. It accepts a single command OR an
  ARRAY of commands run in order — so a linear multi-step build/test sequence
  (e.g. install -> build -> test) maps cleanly onto a pre_tag array.
- tag + atomic push is the POINT OF NO RETURN (PONR): the annotated tag and the
  branch go up together in one ` + "`git push --atomic`" + `. Everything before it is
  unwindable; nothing after it is.
- publish creates the provider (e.g. GitHub) release.
- post_release runs AFTER the release is published (notifications, downstream
  triggers).

The release shim: ` + "`./release`" + ` is the small executable wrapper mint init drops
at the repo root alongside .mint.toml; it invokes mint release so the project
keeps a familiar ./release entry point. mint init creates it — you do not write
it by hand.

Mapping an existing release process onto these hooks is the centrepiece of
setup:
- Propose a best-fit mapping and FLAG it to the user. Never silently skip a
  step: if it is in the customer's release script, it is important.
- When a step does not fit a hook slot, surface it HONESTLY rather than papering
  over it. The right answer may be an adaptation to the process, or recognising
  that mint is not suitable for this project — both are acceptable outcomes.
- Explain mint's model clearly (the stages above and where each hook slots in)
  so a technical user can collaborate on a workaround, an adaptation, or a clean
  fit. You facilitate that conversation; you do not force a mapping.
- pre_tag-as-array widens what fits. The genuinely unmappable cases narrow to:
  needing a step where mint has no hook, a non-linear pipeline, or a
  mid-pipeline approval gate.`
}

// etiquetteSection carries the AI etiquette rules: ask interactively
// (AI-agnostic), confirm comfort and state exactly what changes before writing,
// never remove without explicit permission, and surface diff_exclude inside the
// confirmation.
func etiquetteSection() string {
	return MarkerEtiquette + `
## Etiquette — how to propose and confirm changes

- Ask interactively. If you are Claude, use your Ask-User tool; otherwise, if
  you have ANY tool for asking the user questions, use it. Do not configure
  silently.
- Before writing anything, confirm the user is comfortable and state EXACTLY
  what you are about to change or update — which keys, which values, and why.
- Never remove anything without explicit permission. Removing or overwriting a
  key the user already has is always the user's call.
- Surface the diff_exclude patterns you are proposing INSIDE this interactive
  confirmation — the right set is project-specific, so the user signs off on it.

### diff_exclude scope

diff_exclude is for release-notes NOISE, not generated code. Keep its scope
honest:

- .gitignore'd paths (node_modules, vendor, build output) are already absent
  from the diff — do not list them.
- The real targets are TRACKED process / meta / doc files that churn but are not
  user-facing release content: .workflows/, .claude/, other agent directories,
  docs/, and lockfiles.
- The right set is project-specific, so propose patterns and surface them in the
  confirmation step above rather than assuming a default — mint ships none.`
}

// minimalismSection carries the "only set what varies" rule, reconciled to the
// post-strip empty-body file: add a key only to set a non-default value, omit it
// otherwise (defaults are compiled in and apply regardless), justify every
// activated key at the confirmation step, and read real defaults from the config
// reference rather than guessing or restating them (DRY).
func minimalismSection() string {
	return MarkerMinimalism + `
## Minimalism — only set what varies

- Add (activate) a key ONLY to set a value that differs from mint's compiled
  default. If the default is fine, OMIT the key entirely. Omitting is NOT
  skipping or disabling: mint's defaults are compiled into the binary and apply
  whether or not the key appears in the file. The whole .mint.toml is optional,
  and mint init leaves it with an empty body for exactly this reason — you add
  only the keys you have a concrete project reason to set.
- For EVERY key you do activate, state the project-specific reason at the
  confirmation step, so over-configuration is visible (each active key carries a
  justification) rather than silent. A thorough agent that populates every
  inferable key hands back a bloated file and defeats mint's good-defaults
  design — resist that.
- Judge "is the default fine?" against the REAL default from the config-
  reference table (the default column), never a guess and never an illustrative
  example value seen elsewhere. Read defaults from that table; this guide does
  not restate them (DRY).`
}

// existingConfigSection carries the existing-`.mint.toml` / upgrade branch:
// never silently overwrite, offer work-with-existing vs start-fresh-reusing-
// values, and the config upgrade/migration capability (detect removed/renamed
// keys that would fail DisallowUnknownFields, values that no longer fit, new
// keys worth considering) — setup doubles as upgrade-my-config-to-this-version.
func existingConfigSection() string {
	return MarkerExistingConfig + `
## Existing .mint.toml — diff, discuss, upgrade

Re-running setup on an already-configured repo is a likely motion. mint's own
generator is strictly non-clobbering (mint init skips an existing file unless
--force); carry the same instinct:

- If a .mint.toml already exists, bring it into context and DISCUSS it — never
  silently overwrite. Surface what is there and ask.
- Offer the choice: work WITH the existing config (targeted, key-by-key,
  reviewed changes) OR start fresh reusing the existing values to build a clean
  current file. It is always the user's call; nothing is removed without
  explicit permission.
- Config upgrade / migration: you hold the current canonical reference (the
  config-reference table below) plus the user's file, so you can bring an old
  config current. Detect:
  - removed or renamed keys that would otherwise fail DisallowUnknownFields
    loudly at the next mint run;
  - values that no longer fit the current schema;
  - new keys worth considering for this project.
  Setup therefore doubles as "upgrade my config to this mint version."`
}

// configReferenceSection emits the config-reference marker plus the intro
// framing line, then splices in the rendered key·level·default·description table
// (renderConfigReference()) so the finished section lands under the marker. The
// prose-authoring framing here hand-writes NO config key metadata — the table is
// the only metadata carrier and it reads the SoT, so the SoT stays the single
// source.
func configReferenceSection() string {
	return MarkerConfigReference + `
## Config reference

Every mint config key, its level (top-level shared, [release], [release.hooks],
or [commit]), its real compiled default, and a one-line meaning. Read the
default column to judge whether a key needs setting at all (see Minimalism).

` + renderConfigReference()
}

// renderConfigReference renders the key·level·default·description table that
// lands inside the config-reference section, sourced ENTIRELY from the
// config-metadata source of truth (config.MetadataRows()). This is the single
// place the setupguide package reads config: the prose-authoring helpers above
// hand-write NO key metadata, so the SoT stays the one source and cannot drift
// from what the binary accepts (the drift test in the config package guards the
// SoT itself against the schema).
//
// The render performs ZERO metadata logic — it iterates the SoT rows in SoT
// order and copies each row's columns through verbatim:
//   - Key: the TOML key name as-is.
//   - Level: row.Level.String() (the canonical TOML form — [release],
//     [release.hooks], [commit], or the empty shared form). Because a blank
//     markdown cell would be ambiguous to the reading agent, the empty shared
//     form is surfaced as "top-level" (NOT "shared", which the Default column
//     already uses for the inherit token — see LevelCell); the rendered value
//     stays DRIVEN by MetadataLevel.String() (the placeholder applies only when
//     String() is empty), so level identity remains single-sourced in config.
//   - Default: row.Default carried VERBATIM — blank stays blank, "auto" stays
//     "auto", "[]" stays "[]", "shared" stays "shared". An empty default is
//     emitted as a single space so the markdown cell is well-formed without
//     changing the SoT token. No re-defaulting, no re-typing.
//   - Description: row.Description as-is.
//
// The dual-level ai_command/timeout keys are NOT collapsed: each (level, key)
// pair the SoT carries renders as its own line, so the agent sees the
// same->shared, different->per-verb choice in the reference.
func renderConfigReference() string {
	var b strings.Builder

	b.WriteString("| Key | Level | Default | Description |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, row := range config.MetadataRows() {
		b.WriteString("| ")
		b.WriteString(row.Key)
		b.WriteString(" | ")
		b.WriteString(LevelCell(row.Level))
		b.WriteString(" | ")
		b.WriteString(defaultCell(row.Default))
		b.WriteString(" | ")
		b.WriteString(row.Description)
		b.WriteString(" |\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// LevelCell renders a row's level column from row.Level.String() (the canonical
// TOML form), surfacing the empty shared form as the "top-level" placeholder so
// the markdown cell is never blank/ambiguous. The placeholder deliberately is
// NOT "shared": the Default column already uses "shared" to mean "inherit the
// shared top-level value" for the per-verb override rows, so a "shared" level
// cell would collide with that token in the same table and read as two
// meanings. The cell stays driven by MetadataLevel.String() — the placeholder
// applies ONLY when it is empty — so the level identity is single-sourced in
// config. This is the ONE place the placeholder string lives; the structural
// test calls it rather than re-deriving it, so a change here flows into the
// test and a drift turns it red.
func LevelCell(level config.MetadataLevel) string {
	if s := level.String(); s != "" {
		return s
	}
	return "top-level"
}

// defaultCell carries the SoT default token verbatim, emitting a single space
// for an empty default so the markdown cell is well-formed. It performs NO
// metadata logic: the SoT already encodes the representation convention (blank /
// auto / [] / shared / scalar), so this never re-defaults or re-types a token.
func defaultCell(def string) string {
	if def == "" {
		return " "
	}
	return def
}
