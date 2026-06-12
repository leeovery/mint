package main

// The hand-written --help surface. Each verb's flag set parses with
// flag.ContinueOnError and its default output discarded (main prints its own
// errors), so a -h/--help surfaces as flag.ErrHelp with NOTHING rendered — the cmd
// layer catches it and prints these texts instead. They are hand-written rather than
// fs.PrintDefaults output because every boolean is registered twice (the paired
// short/long spelling), which PrintDefaults would list as separate single-dash
// entries; a curated text keeps the pairs on one line and the wording consistent
// with the spec's flag descriptions. A help request is a REQUESTED action, not a
// usage error: it prints to stdout and exits 0 (usage errors stay on stderr with
// exit 2).

// rootUsage is the top-level `mint help` / `mint --help` surface: the command list.
const rootUsage = `usage: mint <command> [options]

commands:
  release               cut a release (bump, notes, changelog, tag, push, publish)
  release regenerate    regenerate the notes for an existing release
  commit                mint an AI commit message and create the commit
  init                  scaffold .mint.toml and the release shim into a repo
  version               print mint's own version (also: mint --version)

Run 'mint <command> --help' for that command's options.
`

// releaseUsage documents the `mint release` flag surface (flags.go).
const releaseUsage = `usage: mint release [-p | -m | -M | --set-version X.Y.Z] [options]

Cut a release: bump the version, generate the AI release notes, update the
changelog, commit, tag, push atomically, and publish the provider release.

  -p, --patch          patch bump (default)
  -m, --minor          minor bump
  -M, --major          major bump
      --set-version V  explicit version X.Y.Z (mutually exclusive with bump flags)
  -d, --dry-run        read-only run: print the plan, make no changes
  -y, --yes            skip the confirmation/notes-review gate
      --no-ai          skip the AI notes path; use the commit-subject fallback body
      --autostash      stash/restore unrelated WIP around the run
      --any-branch     bypass the release-branch gate
      --plain          force plain (un-styled) output
`

// regenerateUsage documents the `mint release regenerate` flag surface
// (regenerate_flags.go).
const regenerateUsage = `usage: mint release regenerate <version> [options]
       mint release regenerate --all [options]

Regenerate the notes for an existing release and rewrite the chosen surface(s).

      --reuse            source = tag annotation body (no AI); implies --target release
      --fresh            source = re-diff + AI (default)
      --target SURFACE   surface(s) to write: release, changelog, or both
      --all              regenerate every version, oldest → newest
  -y, --yes              skip the confirmation / per-version review gate
      --plain            force plain (un-styled) output
`

// commitUsage documents the `mint commit` flag surface (commit_flags.go).
const commitUsage = `usage: mint commit [-a | -A] [-p] [-y] [--no-ai] [--plain]

Mint an AI-generated Conventional Commits message from the would-be-committed
diff, review it at the Continue? gate, and create the commit.

  -a, --all      stage tracked changes at accept (git commit -a semantics)
  -A, --add-all  stage everything incl. untracked at accept (git add -A)
  -p, --push     push after a successful commit (never pushes without this flag)
  -y, --yes      auto-accept the review gate
      --no-ai    skip AI generation; write the message in $EDITOR
      --plain    force plain (un-styled) output
`

// initUsage documents the `mint init` flag surface (init.go).
const initUsage = `usage: mint init [--force] [--plain]

Scaffold mint into the current repo: drop the .mint.toml template and the
release shim at the git-resolved repo root.

      --force    regenerate (overwrite) existing files
      --plain    force plain (un-styled) output
`

// isHelpCommand reports whether the invocation is a top-level help request:
// `mint help`, `mint -h`, or `mint --help`.
func isHelpCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "help" || args[0] == "-h" || args[0] == "--help"
}
