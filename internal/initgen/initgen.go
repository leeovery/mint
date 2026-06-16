// Package initgen generates the scaffolding content `mint init` writes into a
// project. It is a PURE generator: every function returns a string and performs
// NO filesystem or git IO — writing those strings to disk, the idempotency /
// --force behaviour, and the `release` shim live in their own `mint init` tasks.
//
// MintTOML returns the MINIMAL `.mint.toml` template: an empty body (no active keys,
// no commented example keys) preceded by a short header comment. Because every
// default is compiled into the binary, an empty file is valid and changes nothing —
// the file is fully optional. The template is DELIBERATELY static — initgen does NOT
// sniff package.json or any project file to pre-fill values, because that guesswork
// surprises; a clean, honest minimal template is the chosen design (see the spec's
// "Generated config: strip to minimal" and "No project auto-detection").
//
// The config DOCS live in the BINARY, not in template comments: the agent reads the
// drift-tested config reference from `mint setup`, and humans read it from the GitHub
// docs / README. The header is the cold-arrival RECOVERY NET — it points to both, so
// an agent or human who opens `.mint.toml` directly (without having run `mint setup`)
// still finds where the config reference lives. Stripping the old commented template
// removes a pure-duplication drift surface; the package's tests pin the minimal shape
// (empty body) and both header pointers.
//
// initgen does NOT import config: the canonical defaults and their drift discipline
// live in the config-metadata source of truth, whose default column is the drift-pinned
// carrier of the canonical values (the package's value-drift pins were subsumed there
// when the template stopped carrying default values). ReleaseShim() is unaffected.
package initgen

// MintTOML returns the minimal `.mint.toml` scaffolding template as a single string.
// The content is static — NO project auto-detection, NO file reads, NO IO.
//
// The body is EMPTY: it carries no active `key = value` lines and no commented
// example keys, because every default is compiled into the binary, so an empty file
// is valid and changes nothing. The only content is a short header comment — the
// cold-arrival recovery net — that points to BOTH the GitHub docs (the human config
// reference) and `mint setup` (the AI-assisted setup guide). The contract pinned by
// the package's tests is that both pointers are present and the body carries no key,
// not the exact header wording.
func MintTOML() string {
	return `# .mint.toml — mint configuration. This file is fully OPTIONAL: every default is
# compiled into mint, so an empty file is valid and changes nothing. Add a key only
# to set a value that differs from the default.
#
# Config reference (every key, level, and default): https://github.com/leeovery/mint
# AI-assisted setup (inspects your project and proposes config): run ` + "`mint setup`" + `
`
}
