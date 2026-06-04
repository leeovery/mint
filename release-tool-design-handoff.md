# Mint — Release Tool Design Handoff

> **Tool name: `mint`.** "Mint a release" — stamp a fresh, versioned, official artifact
> (bump → tag → publish). Installed as `brew install leeovery/tools/mint`; invoked as
> `mint`, `mint init`, etc. The per-project shim stays named `release` (muscle memory),
> so `./release -m` continues to work and delegates to `mint`.

> **Purpose of this doc.** Hand-off for building a reusable, configuration-driven release
> tool that replaces the per-project `release` bash scripts currently copy-pasted (and
> drifting) across many repos. This is a *design conversation* artifact — nothing is built
> yet. It contains: the decisions reached so far, the open forks, and the **reference
> implementation** (the current `agentic-workflows/release` script) to be used as the
> behavioral spec / test oracle for the rewrite. Self-contained — no need to access the
> original repos.

---

## 1. The problem

A `release` bash script started life in one project and was copy-pasted across ~8 repos.
Over months it diverged. It does mostly generic work (AI release notes via Claude, git
tag, atomic push, GitHub release, safe-git lock handling, changelog) plus, in one repo,
a single project-specific step (a knowledge-base bundle build).

Goal: extract the generic engine into **one reusable tool**, distributed via the existing
Homebrew tap (`leeovery/homebrew-tools`), with **per-project configuration** and a **hook
system** for project-specific steps. Each project keeps a tiny shim that delegates to the
globally-installed tool (and tells the user to `brew install` it if missing). The tool
should also be able to "activate" / scaffold itself into a project.

---

## 2. Decisions reached

### 2.1 Language: **Go** (decided)

Rewrite as a single Go binary rather than porting the bash.

**Why Go, for this tool specifically:**

- **Blast radius vs. testability.** A release tool that mis-cuts a tag, corrupts a
  CHANGELOG insertion, or mishandles a git lock is high-consequence and runs across many
  repos. The genuinely fragile logic is exactly what bash tests badly but Go tests cheaply:
  - `git_safe` — classifying git's stderr to tell a lock collision from a real error, with
    retry + stale-lock clearing.
  - the `awk` CHANGELOG insertion (insert above first `## [` section; create-if-absent).
  - version extraction across `file` / `embedded` / `none` strategies.
  - the AI diff-exclude + max-diff-lines guard.
  In Go these become fast **table-driven unit tests**; in bash they're slow, coarse,
  integration-only tests against temp git repos.
- **Typed, validated config** (TOML) with real error messages, vs. a sourced bash file that
  fails silently on a typo.
- **Public dual-arch Homebrew formula** like `tick`/`portal` — no `HOMEBREW_GITHUB_API_TOKEN`,
  no private download strategy. Simpler install than the `bash-toolkit` private pattern.
- **Clean pipeline architecture** — discrete stages + injectable hooks map naturally to Go.

**Where Go won't magically help (be clear-eyed):**

- The bulk of the tool is orchestration — shelling out to `git`, `gh`, `claude`. That's
  `os/exec` glue, more verbose than bash. **Mitigation / key design rule:** put *all*
  process execution behind a single `CommandRunner` interface so the pipeline logic is
  tested with a mock runner and only a thin layer touches real binaries. Do this and ~90%
  of the code is unit-testable; skip it and you've written verbose bash.
- **Rewrite regression risk** on the subtle bits. **Mitigation:** the existing bash is the
  battle-tested reference. Treat it as the **test oracle** — port the existing
  `tests/scripts/test-release-*.sh` cases as Go golden/integration tests *first*, confirm
  bash and Go produce byte-identical changelog/tag output on the same fixtures, then
  refactor freely.

> Note: bash would have been the pragmatic "good enough" choice for a one-off in a single
> repo (it reuses the proven logic + the `bash-toolkit` UI library). Go wins *because* this
> is being productized: brew + config + hooks + many repos + evolve-over-years.

### 2.2 Distribution (leaning, to confirm)

- Add as a **new public formula** in the existing `leeovery/homebrew-tools` tap, alongside
  `tick`, `portal`, `stitch`, `bash-toolkit`. Reuse the tap's GitHub Actions auto-update
  workflow (dispatch on release → bumps formula version/sha).
- Source lives in its **own repo** (e.g. `leeovery/mint`).
- Public dual-arch formula (like `tick`/`portal`) — no auth token needed.

### 2.3 Per-project shim + activation (leaning)

- Each project commits a tiny `release` shim that `exec`s `mint`, and if it's missing
  prints `brew install leeovery/tools/mint`. Preserves `./release -m` muscle memory.
- `mint init` = "activate this project": scaffolds the config file, the shim, and a
  commented `.release/hooks/` example. For a project like agentic-workflows it would
  pre-fill the knowledge `pre-tag` hook + diff-exclude.

---

## 3. Open forks (still to decide in the new session)

1. **Hook mechanism.** Three options:
   - **(a) Hook scripts** in `.release/hooks/` (e.g. `pre-tag.sh`, `post-release.sh`),
     invoked with env vars (`NEW_VERSION`, `CURRENT_VERSION`, `DIR`, …) if present. Most
     flexible; project owns its build+commit logic. **Recommended default.**
   - **(b) Inline commands** in config (`pre_tag = "npm ci && npm run build"`). Simple for
     one-liners; awkward for conditional-commit logic.
   - **(c) Both** — config string if set, else fall through to a hook script file.
2. ~~Tool name~~ — **decided: `mint`** (global binary). Local shim stays `release`.
3. **Config format.** TOML assumed (Go-native, validated). Confirm vs. YAML.

---

## 4. What's generic vs. project-specific

**Generic engine (the reusable core):**

- semver bump: major / minor / patch (default patch), `--dry-run`
- dirty working-tree gate
- version strategy: `file` (plain `release.txt`), `embedded` (`RELEASE_VERSION="x.y.z"` in a
  source file), or `none` (tag-only, derive from `git describe`)
- AI release notes via the `claude` CLI (with diff-exclude paths + max-diff-lines guard +
  60s timeout + graceful fallback when unavailable)
- `git_safe` — lock-resilient git wrapper (retry on contended lock, clear provably-stale lock)
- `gh` preflight gate (installed + authenticated) *before* any mutation
- CHANGELOG.md generation (Keep a Changelog format; insert newest entry above the first
  existing `## [` section; create file with header if absent)
- annotated tag + atomic push (`git push --atomic origin HEAD vX.Y.Z`)
- `gh release create` (reusing the generated notes body)
- interactive "Proceed? [Y/n]" confirmation + a plan summary

**Project-specific (→ becomes hooks/config), seen only in `agentic-workflows`:**

- **Knowledge bundle build** — `npm ci` + `npm run build`, then conditionally commit the
  regenerated `skills/workflow-knowledge/scripts/knowledge.cjs` *before* tagging. → a
  **`pre-tag` hook**.
- **Diff-exclude** of that generated bundle from the AI diff, and a raised
  `max_diff_lines` (60000 vs 25000) because the bundle no longer counts. → **config**
  (`diff_exclude = [...]`, `max_diff_lines = 60000`).

**Proposed release pipeline (lifecycle) + hook points:**

```
preflight            (validate gh auth, custom checks)        [hook: preflight]
  -> compute version
  -> pre-tag          (build + commit generated artifacts)    [hook: pre-tag]   <- knowledge build
  -> generate notes   (AI)
  -> update changelog (commit)
  -> update version file (commit, if strategy != none)
  -> create tag
  -> push (atomic)
  -> publish          (gh release; could host npm publish etc) [hook: post-tag]
  -> post-release     (notifications, tap dispatch, etc.)      [hook: post-release]
```

---

## 5. Variations across the existing scripts (informational only)

We're rebuilding, so these are just clues about scope. Three tiers exist:

| Project(s) | Lines | Capabilities |
|---|---|---|
| `nuxt-layers` | 170 | Oldest. `release.txt` only, simple commit/tag/push, basic `gh release`, **no AI, no changelog, no lock-safety**. |
| **canonical**: `agntc`, `tick`, `portal`, `stitch-cli`, `bash-toolkit`, `agentic-skills` | 355 | AI notes via `claude`, version strategy (file/embedded/none), tag + atomic push. **No changelog, no `gh release`, no `git_safe`.** |
| `agentic-workflows` | 552 | The **superset / most advanced**. Everything above **plus** CHANGELOG.md, `git_safe` lock resilience, `gh` preflight gate, `gh release create`, and the project-specific knowledge build. |

**Implication:** the 552-line `agentic-workflows` version is the most complete and is the
right **reference implementation** to port from. Its generic features (`git_safe`,
changelog, gh release, gh preflight) are strict improvements the other 6 repos never
received — the rebuilt tool gives all repos these for free and ends the drift.

**Adjacent context — `stitch` is NOT this tool.** The tap already contains `stitch`, a
heavyweight Laravel/Vue **deploy** orchestrator (environments, Docker images, branch
strategies, release-train vs trunk). This `release` tool's niche — cut a tagged release of
a library/tool repo — is *complementary*, not redundant. Don't conflate them.

**Reusable precedent — `bash-toolkit`.** Already a brew-installed library sourced via
`source $(bash-toolkit)` (message/prompt/layout/execution modules). Confirms the tap +
formula machinery works; not needed by a Go rewrite but worth knowing.

---

## 6. Reference implementation — `agentic-workflows/release` (verbatim)

This is the 552-line superset. **Use it as the behavioral spec and test oracle for the Go
rewrite.** The project-specific knowledge-build section (in `perform_release`) is the part
to extract into a `pre-tag` hook; the `bundle_exclude` / `max_diff_lines` are the
diff-exclude config.

```bash
#!/usr/bin/env bash

set -euo pipefail

# =============================================================================
# Configuration - adjust these for your project
# =============================================================================

# Version strategy: "file", "embedded", or "none"
#   file     - version stored in plain text file (just the version number)
#   embedded - version embedded in source file as RELEASE_VERSION="x.y.z"
#   none     - tag-only, no version file to update
VERSION_STRATEGY="none"

# Path to version file (used by "file" and "embedded" strategies)
VERSION_FILE="release.txt"

# =============================================================================

# Get the script directory
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
readonly script_dir

# Get the Git repository root directory
dir="$(git rev-parse --show-toplevel 2> /dev/null || echo "$script_dir")"
readonly dir

cd "$dir" || exit

usage() {
    cat << EOF
Usage: $(basename "$0") [-M|--major|-m|--minor|-p|--patch] [-d|--dry-run] [--no-ai] [-h|--help]

Options:
  -M, --major      Increment major version
  -m, --minor      Increment minor version
  -p, --patch      Increment patch version (default if no option specified)
  -d, --dry-run    Dry run (don't make any changes, shows AI-generated message)
  --no-ai          Skip AI commit message generation
  -h, --help       Display this help message

Examples:
  $(basename "$0")       Create a patch release (default)
  $(basename "$0") -m    Create a minor release
  $(basename "$0") -d    Preview release without making changes

Requires Claude CLI for AI-powered release notes (falls back to simple message).
EOF
}

semver_increment() {
    local -a parts
    IFS='.' read -ra parts <<< "$1"

    if [ ${#parts[@]} -ne 3 ]; then
        echo "Semantic versions should have 3 components (for example: 1.2.3)" >&2
        return 1
    fi

    case $2 in
        major)
            ((parts[0]++))
            parts[1]=0
            parts[2]=0
            ;;
        minor)
            ((parts[1]++))
            parts[2]=0
            ;;
        patch) ((parts[2]++)) ;;
        *)
            echo "Invalid increment type. Use major, minor, or patch." >&2
            return 1
            ;;
    esac

    echo "${parts[0]}.${parts[1]}.${parts[2]}"
}

get_current_version() {
    local version=""

    case "$VERSION_STRATEGY" in
        file)
            # Read version from plain text file
            if [[ -f "$VERSION_FILE" ]]; then
                version=$(tr -d '[:space:]' < "$VERSION_FILE")
            fi
            ;;
        embedded)
            # Extract version from RELEASE_VERSION="x.y.z" in source file
            if [[ -f "$VERSION_FILE" ]]; then
                version=$(grep -oE '^RELEASE_VERSION="[0-9]+\.[0-9]+\.[0-9]+"' "$VERSION_FILE" 2>/dev/null | sed 's/RELEASE_VERSION="//;s/"$//' || echo "")
            fi
            ;;
        none)
            # Tag-only: use git tags
            version=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "")
            ;;
    esac

    # Validate version format, fallback to 0.0.0
    if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        version="0.0.0"
    fi

    echo "$version"
}

update_version_file() {
    local new_version=$1

    case "$VERSION_STRATEGY" in
        file)
            echo "$new_version" > "$VERSION_FILE"
            echo "Updated $VERSION_FILE to $new_version"
            ;;
        embedded)
            sed -i.bak 's/^RELEASE_VERSION=".*"$/RELEASE_VERSION="'"$new_version"'"/' "$VERSION_FILE"
            rm -f "${VERSION_FILE}.bak"
            echo "Updated RELEASE_VERSION in $VERSION_FILE to $new_version"
            ;;
        none)
            echo "No version file to update (tag-only strategy)"
            ;;
        *)
            echo "Error: Unknown VERSION_STRATEGY '$VERSION_STRATEGY'" >&2
            exit 1
            ;;
    esac
}

# Generate release notes using Claude AI
# Args: version, base_ref, target_ref (defaults to HEAD)
# Returns: generated message or empty string on failure
generate_release_notes() {
    local version="$1"
    local base_ref="$2"
    local target_ref="${3:-HEAD}"

    # Check if Claude CLI is available
    if ! command -v claude &> /dev/null; then
        return 1
    fi

    # Exclude the generated knowledge bundle from the diff — it is minified,
    # huge, and not human-meaningful, so it only wastes the token budget and
    # pollutes the summary.
    local bundle_exclude=":(exclude)skills/workflow-knowledge/scripts/knowledge.cjs"

    # Get actual code changes for AI analysis
    local files_changed changes_summary code_diff
    files_changed=$(git diff "$base_ref..$target_ref" --name-status -- . "$bundle_exclude" 2>/dev/null || echo "")
    changes_summary=$(git diff "$base_ref..$target_ref" --stat -- . "$bundle_exclude" 2>/dev/null || echo "")

    # Check if diff is too large. The generated bundle is already excluded
    # above, so this guards against genuinely large source changes only —
    # raised from 25000 since the bundle (the usual culprit for blowing the
    # token budget on knowledge-base releases) no longer counts.
    local max_diff_lines=60000
    local diff_line_count
    diff_line_count=$(git diff "$base_ref..$target_ref" -- . "$bundle_exclude" 2>/dev/null | wc -l)

    if [[ "$diff_line_count" -gt "$max_diff_lines" ]]; then
        echo "⚠️  Diff too large for AI analysis ($diff_line_count lines)" >&2
        return 1
    fi

    code_diff=$(git diff "$base_ref..$target_ref" -- . "$bundle_exclude" 2>/dev/null || echo "")

    # Build prompt for Claude
    local prompt="Generate release notes for version v${version}.

## Files Changed:
${files_changed}

## Change Statistics:
${changes_summary}

## Actual Code Diff:
${code_diff}

## Output Format

A markdown bullet list summarising the meaningful technical changes. No
heading, no preamble, no version line — just the bullets. This body is reused
verbatim for both the git tag message and the CHANGELOG.md entry.

**Rules:**

- Summarize the meaningful technical changes (not every file/function)
- This is for users and developers to understand what this release contains
- Number of bullet points should match release scope - be terse
- Be technically accurate but concise
- Base on ACTUAL CODE CHANGES, ignore 'wip' commit messages

## Example:

- Add AI-powered release notes generation
- Fix config validation edge case
- Refactor deploy workflow error handling

## Notes

- NO Claude signature
- Return ONLY the bullet list
- No explanations

Output:"

    # Write prompt to temp file to avoid shell escaping issues
    local prompt_file
    prompt_file=$(mktemp)
    echo "$prompt" > "$prompt_file"

    # Call Claude with timeout (pipe content, don't pass as argument - avoids CLI hangs)
    local claude_output
    if claude_output=$(timeout 60 bash -c "cat '$prompt_file' | claude -p" 2>/dev/null); then
        rm -f "$prompt_file"
        echo "$claude_output"
    else
        rm -f "$prompt_file"
        return 1
    fi
}

# Wrapper for new releases — resolves the base ref and returns the release
# notes BODY (markdown bullet list) on stdout, or an empty string if notes
# can't be generated (no base tag, Claude unavailable, diff too large). The
# callers wrap this body for the tag message and the changelog entry.
generate_release_body() {
    local new_version="$1"
    local last_tag="$2"
    local last_tag_no_v="${last_tag#v}"  # Strip v prefix if present

    # Find the matching tag (try both with and without v prefix)
    local base_ref=""
    if [[ -n "$last_tag" ]] && git rev-parse "v${last_tag_no_v}" &>/dev/null; then
        base_ref="v${last_tag_no_v}"
    elif [[ -n "$last_tag_no_v" ]] && git rev-parse "$last_tag_no_v" &>/dev/null; then
        base_ref="$last_tag_no_v"
    fi

    # If no matching tag found, skip AI (can't reliably diff with parallel version lines)
    if [[ -z "$base_ref" ]]; then
        echo "⚠️  No tag found for v${last_tag_no_v}, skipping AI generation" >&2
        return 0
    fi

    local body
    if body=$(generate_release_notes "$new_version" "$base_ref" "HEAD"); then
        echo "$body"
    fi
    return 0
}

# Compose the annotated-tag / release-commit message from a version and an
# optional notes body. Format is unchanged from prior releases: a
# "🔖 Release vX.Y.Z" subject, then the bullet body when present.
compose_tag_message() {
    local version="$1" body="$2"
    if [[ -n "$body" ]]; then
        printf '🔖 Release v%s\n\n%s\n' "$version" "$body"
    else
        printf '🔖 Release v%s\n' "$version"
    fi
}

# Prepend a new release section to CHANGELOG.md (Keep a Changelog format),
# creating the file with a header if it does not yet exist. The entry is
# inserted above the most recent existing "## [" section.
update_changelog() {
    local version="$1" body="$2"
    local changelog="CHANGELOG.md"
    local date
    date=$(date +%Y-%m-%d)

    if [[ ! -f "$changelog" ]]; then
        {
            echo "# Changelog"
            echo
            echo "All notable changes to this project are documented in this file."
            echo
            echo "The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),"
            echo "and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)."
            echo
        } > "$changelog"
    fi

    local entry_body="${body:-- Maintenance release (no notes recorded).}"

    # Build the entry in a temp file so the body (which may contain any
    # characters) is never reinterpreted by awk -v escaping.
    local entry_file tmp
    entry_file=$(mktemp)
    tmp=$(mktemp)
    printf '## [%s] - %s\n\n%s\n\n' "$version" "$date" "$entry_body" > "$entry_file"

    awk -v ef="$entry_file" '
        /^## \[/ && !ins { while ((getline line < ef) > 0) print line; close(ef); ins=1 }
        { print }
        END { if (!ins) { while ((getline line < ef) > 0) print line; close(ef) } }
    ' "$changelog" > "$tmp"

    mv "$tmp" "$changelog"
    rm -f "$entry_file"
}

# How long (seconds) to wait out a contended git lock before treating it as
# stale and removing it. A real local index/ref mutation finishes well within
# this window — overridable via the environment for testing.
: "${GIT_LOCK_WAIT_SECONDS:=30}"

# Run a git command, transparently surviving a contended or stale lock file.
#
# Git takes a lock (.git/index.lock for index ops, refs/.../*.lock for ref ops)
# at the start of a mutation and releases it on completion. A second git caller
# running concurrently — an editor, an auto-commit, a background agent — or a
# process killed mid-op leaves us either racing the lock or staring at a stale
# one. Either way the bare command aborts with "Unable to create '...lock':
# File exists", which would break a release. This wrapper instead waits the
# lock out silently, retrying until the holder finishes, and as a last resort
# removes a lock that is provably stale (still present after the whole wait).
# The caller never sees the lock error; genuine (non-lock) git failures are
# surfaced and returned unchanged.
git_safe() {
    local deadline=$(( SECONDS + GIT_LOCK_WAIT_SECONDS ))
    local out status lock_path

    while true; do
        # Combine streams so we can inspect git's diagnostics. The capture sits
        # in an `if` condition because `set -e` would otherwise abort the whole
        # script the moment git exits non-zero — defeating the retry entirely.
        if out=$(git "$@" 2>&1); then status=0; else status=$?; fi

        if [[ $status -eq 0 ]]; then
            if [[ -n "$out" ]]; then printf '%s\n' "$out"; fi
            return 0
        fi

        # Not a lock collision? Surface the real error and fail as normal.
        if ! printf '%s' "$out" | grep -qE "Unable to create '.*\.lock'|cannot lock ref|Another git process"; then
            if [[ -n "$out" ]]; then printf '%s\n' "$out" >&2; fi
            return "$status"
        fi

        # Lock collision: stay quiet and wait for the holder to release it.
        if (( SECONDS < deadline )); then
            sleep 1
            continue
        fi

        # Waited the full window and the lock is still here — treat as stale,
        # remove it, and make one final attempt.
        lock_path=$(printf '%s' "$out" | grep -oE "Unable to create '[^']*\.lock'" | head -1 | sed "s/Unable to create '//; s/'\$//")
        if [[ -n "$lock_path" && -e "$lock_path" ]]; then
            echo "Clearing stale git lock: $lock_path" >&2
            rm -f "$lock_path"
            if out=$(git "$@" 2>&1); then status=0; else status=$?; fi
            if [[ -n "$out" ]]; then
                if [[ $status -eq 0 ]]; then printf '%s\n' "$out"; else printf '%s\n' "$out" >&2; fi
            fi
            return "$status"
        fi

        # Couldn't identify a lock file to clear — surface the original error.
        if [[ -n "$out" ]]; then printf '%s\n' "$out" >&2; fi
        return "$status"
    done
}

# Preflight gate: require the GitHub CLI to be installed and authenticated.
# Called before any mutation so a missing/unauthenticated gh aborts the release
# loudly *before* a tag is ever created — recoverable, no stray tags left behind.
require_github_cli() {
    if ! command -v gh >/dev/null 2>&1; then
        echo "Error: gh (GitHub CLI) is not installed. Install it before releasing: https://cli.github.com" >&2
        exit 1
    fi
    if ! gh auth status >/dev/null 2>&1; then
        echo "Error: gh is not authenticated. Run 'gh auth login' before releasing." >&2
        exit 1
    fi
}

perform_release() {
    local new_version=$1
    local current_version=$2
    local skip_ai=${3:-false}

    if [ "$(git status --porcelain)" ]; then
        echo "Error: The git working directory is dirty (has uncommitted changes)."
        echo "Please commit or stash your changes and run the script again to release."
        exit 1
    fi

    # Verify gh is ready before mutating anything, so no tag is ever pushed
    # without the means to create its Release.
    require_github_cli

    # Rebuild the knowledge CLI bundle so every tagged release ships a fresh
    # skills/workflow-knowledge/scripts/knowledge.cjs. AGNTC installs from tags
    # with no build step, so the bundle must be committed before the tag.
    # Use `npm ci` (not `npm install`) so the lockfile can never drift mid-release.
    # `npm install` would mutate package-lock.json on any resolvable version change,
    # bypassing the dirty-tree gate and leaving uncommitted state after the tag.
    echo "Installing build dependencies..."
    if ! npm ci; then
        echo "Error: npm ci failed. Aborting release." >&2
        exit 1
    fi

    echo "Building knowledge bundle..."
    if ! npm run build; then
        echo "Error: npm run build failed. Refusing to tag with a stale or missing bundle." >&2
        exit 1
    fi

    local bundle_path="skills/workflow-knowledge/scripts/knowledge.cjs"
    if ! git diff --quiet -- "$bundle_path"; then
        echo "Knowledge bundle changed — committing."
        git_safe add "$bundle_path"
        git_safe commit -m "chore(release): rebuild knowledge bundle for v${new_version}"
    else
        echo "Knowledge bundle unchanged — no commit needed."
    fi

    # Generate the release-notes body once — shared by CHANGELOG.md and the tag.
    local body=""
    if ! $skip_ai; then
        echo "Generating release notes..."
        body=$(generate_release_body "$new_version" "v${current_version}")
    fi

    # Prepend the entry to CHANGELOG.md and commit it before tagging.
    update_changelog "$new_version" "$body"
    git_safe add CHANGELOG.md
    if ! git diff --cached --quiet -- CHANGELOG.md; then
        git_safe commit -m "chore(release): update changelog for v${new_version}"
    fi

    local commit_message
    commit_message=$(compose_tag_message "$new_version" "$body")

    # Update version file and commit
    update_version_file "$new_version"

    if [[ "$VERSION_STRATEGY" != "none" ]]; then
        git_safe add "$VERSION_FILE"
        git_safe commit -m "$commit_message"
    fi

    # Create annotated tag with the same message
    git_safe tag -a "v${new_version}" -m "$commit_message"

    # Push commit and tag atomically
    git_safe push --atomic origin HEAD "v${new_version}"

    # Create the GitHub Release, reusing the already-computed notes body. The
    # tag is already live, so a failure here is non-fatal — but the preflight
    # gh gate makes this branch rare.
    echo "Creating GitHub release..."
    local notes="${body:-See CHANGELOG.md for details.}"
    if gh release create "v${new_version}" \
           --title "v${new_version}" \
           --notes "$notes" \
           --verify-tag --latest; then
        echo "✅ GitHub release v${new_version} created"
    else
        echo "⚠️  GitHub release creation failed — the tag is already pushed." >&2
        echo "    Re-run manually: gh release create v${new_version} --title v${new_version} --notes '...' --verify-tag --latest" >&2
    fi

    echo -e "\n"
    echo "✅ Released v${new_version}"
}

main() {
    local semver_type="patch"
    local dry_run=false
    local skip_ai=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            -M | --major) semver_type="major" ;;
            -m | --minor) semver_type="minor" ;;
            -p | --patch) semver_type="patch" ;;
            -d | --dry-run) dry_run=true ;;
            --no-ai) skip_ai=true ;;
            -h | --help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1" >&2
                usage
                exit 1
                ;;
        esac
        shift
    done

    local current_version
    current_version=$(get_current_version)

    local new_version
    new_version=$(semver_increment "${current_version}" "${semver_type}")

    echo -e "\nRelease"
    echo " - this will be a ${semver_type} increment"
    echo " - the current version is v${current_version}"
    echo " - the new version will be v${new_version}"

    if $dry_run; then
        echo -e "\nDRY RUN: No changes will be made."
        if ! $skip_ai; then
            echo -e "\nGenerating release notes preview..."
            local preview_body
            preview_body=$(generate_release_body "$new_version" "v${current_version}")
            echo -e "\n--- Tag message preview ---"
            compose_tag_message "$new_version" "$preview_body"
            echo "--- CHANGELOG.md entry preview ---"
            printf '## [%s] - %s\n\n%s\n' "$new_version" "$(date +%Y-%m-%d)" "${preview_body:-- Maintenance release (no notes recorded).}"
            echo "--- End preview ---"
        fi
    else
        echo -e "\nThis will:"
        if [[ "$VERSION_STRATEGY" != "none" ]]; then
            echo " 1. Update CHANGELOG.md"
            echo " 2. Update version in $VERSION_FILE"
            echo " 3. Create a new commit"
            echo " 4. Create git tag v${new_version}"
            echo " 5. Push changes"
            echo -e " 6. Create GitHub release\n"
        else
            echo " 1. Update CHANGELOG.md"
            echo " 2. Create git tag v${new_version}"
            echo " 3. Push changes"
            echo -e " 4. Create GitHub release\n"
        fi

        echo -n "Proceed? [Y/n] "
        read -r perform_update
        if [[ $perform_update =~ ^[Nn]$ ]]; then
            echo -e "\nABORTED"
        else
            perform_release "$new_version" "$current_version" "$skip_ai"
        fi
    fi
}

main "$@"
```

---

## 7. Canonical variant — key diffs from the reference (for context)

The 355-line canonical script (`agntc`, `tick`, `portal`, `stitch-cli`, `bash-toolkit`,
`agentic-skills`) is the same skeleton **minus** these blocks present in the reference above:

- No `update_changelog()` / CHANGELOG.md handling at all.
- No `git_safe()` — uses bare `git add/commit/tag/push` (no lock resilience).
- No `require_github_cli()` preflight and **no `gh release create`** — it only creates and
  pushes the annotated tag.
- No knowledge bundle build, no `bundle_exclude`; `max_diff_lines=25000` (not 60000).
- The AI step returns a full **commit message** (with the `🔖 Release vX.Y.Z` subject baked
  into the Claude prompt) rather than a reusable **body** that's separately composed for tag
  + changelog.

The `nuxt-layers` script (170 lines) is older/simpler still: `release.txt` only, a plain
`Release X` notes string, basic `gh release create --target HEAD`, no AI, no changelog, no
lock-safety.

---

## 8. Suggested first steps for the new session

1. Stand up the Go project skeleton + `CommandRunner` interface (mockable git/gh/claude).
2. Port the existing bash tests as golden tests; lock behavior parity (changelog insertion,
   semver, version extraction, `git_safe` classification) against the reference above.
3. Define the TOML config schema + the pipeline/hook lifecycle (§4).
4. Decide the remaining open forks (§3): hook mechanism, config format.
5. Add the public dual-arch `mint` formula to `leeovery/homebrew-tools` + reuse the
   auto-update action; build `mint init` to scaffold config + shim + hooks.
