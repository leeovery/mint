// Package initgen generates the scaffolding content `mint init` writes into a
// project. It is a PURE generator: every function returns a string and performs
// NO filesystem or git IO — writing those strings to disk, the idempotency /
// --force behaviour, and the `release` shim live in their own `mint init` tasks.
//
// MintTOML returns the commented `.mint.toml` template: the common keys shown at
// their out-of-the-box defaults (active), plus every optional key present-but-
// commented with a one-line explanation, so a project tunes mint by uncommenting
// rather than reading docs. The template is DELIBERATELY static — initgen does NOT
// sniff package.json or any project file to pre-fill values, because that guesswork
// surprises; a clean, honest commented template is the chosen design (see the spec's
// "No project auto-detection"). The key names, defaults, and value shapes mirror the
// canonical config schema exactly, and the package's tests prove the full template,
// once uncommented, loads cleanly through the real config.Load.
package initgen

// MintTOML returns the commented `.mint.toml` scaffolding template as a single
// string. The content is static — NO project auto-detection, NO file reads, NO IO.
//
// The split is deliberate:
//   - Common keys are ACTIVE at their defaults so the file documents the
//     out-of-the-box behaviour and is immediately valid.
//   - Optional keys are present-but-COMMENTED, each with a one-line explanation, so
//     enabling one is uncommenting a line rather than consulting docs. The example
//     values are illustrative and chosen to be schema-valid (so the uncommented
//     template still loads), not auto-detected from the project.
//
// The `[release].prompt` full-prompt override is only MENTIONED in a comment — this
// generator emits exactly one string and never a second prompt file. Hooks appear
// only under `[release.hooks]`, never a top-level `[hooks]` table.
func MintTOML() string {
	return `# .mint.toml — mint configuration. This file is fully optional: every key shown
# here is set to its default, so deleting it changes nothing. Optional keys are
# present but commented out — uncomment a line to enable that setting. Examples are
# static; mint never inspects your project files to pre-fill anything.

# --- Engine-level keys (shared by every mint verb) ---

ai_command = 'claude -p'
max_diff_lines = 50000

# diff_exclude = ['skills/**/knowledge.cjs', '*.min.js']  # tracked generated files to keep out of the notes diff

[release]
tag_prefix = 'v'
commit_prefix = '🌿'
changelog = true
publish = true
on_notes_failure = 'abort'

# release_branch = 'main'                          # branch releases must run on (default: auto-derived from origin/HEAD)
# version_file = 'bin/tool'                        # write the new version into this file (omit = tag-only release)
# version_pattern = 'RELEASE_VERSION="{version}"'  # version line to replace inside version_file (omit = the whole file is the version)
# provider = 'github'                              # publishing driver to force (default: auto-detected from the remote host)
# context = 'Emphasise user-facing changes.'       # project guidance injected into the notes prompt
# prompt = '.mint/notes-prompt.md'                 # full prompt-override file — create it yourself; mint init does NOT scaffold it

# --- Lifecycle hooks (always under [release.hooks], never a top-level [hooks]) ---
# [release.hooks]
# preflight = 'scripts/check.sh'                   # runs before any release work; failure aborts the release
# pre_tag = 'npm run build'                        # runs after notes, before the tag (single-command form)
# pre_tag also accepts an array of commands run in order: ['npm ci', 'npm run build']
# post_release = 'scripts/notify.sh'               # runs after the release is published
`
}
