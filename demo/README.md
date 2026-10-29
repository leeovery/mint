# The README's demos

Records the clip and stills for the project README by typing real commands into mint built from
this checkout, in a world of its own: a home folder with a project in it, a git remote that mint
reaches as a GitHub repo but that is a local bare repo, and shims for the AI, `gh` and the build.
Nothing reaches the network, the AI, GitHub, the real home folder or the real git config, so it's
safe to record anywhere.

The same recordings are how a change to mint's pretty output checks its own work: record the
verb's tapes, look at the frames, and fix what reads wrong before review.

```bash
demo/record.sh               # every tape: record it, then make its art
demo/record.sh tour commit   # the tapes named
```

## What's recorded

| Tape | Art |
|---|---|
| `tour` | `tour.webp`, `.mp4`: the README's lead, `mint release -m` from start to finish: the version confirmed, the exclusion check's pause on `docs/specs/**` answered exclude, the notes written in 4 parts and merged, their review answered release, then the push, the GitHub release and the `post_release` hook |
| `failed` | `failed.webp`, a still: `mint release -m -y` with part 3 of the notes failing on a usage limit; the run stops on its notes and unwinds |
| `dry-run` | `dry-run.webp`, a still: `mint release -m --dry-run` |
| `commit` | `commit.webp`, a still: `mint commit -Ap`, its message accepted, then committed and pushed |
| `regenerate` | `regenerate.webp`, a still: `mint release regenerate --all --target both -y`, v1.2.0 skipped |
| `init` | `init.webp`, a still: `mint init` in an empty repo |

The art goes to `demo/out/art/`. `look.tape` holds the settings every tape sources; each tape
sets the height its run needs.

## The world

`demo/world.sh` builds it under `demo/out/world/`, and each tape builds it afresh, out of sight,
before it shows anything, so a tape records the same thing every time. `pigeon`, a webhook
delivery service, lives at `~/Code/pigeon`, released with mint from v1.0.0 to v1.3.2. Its
`.mint.toml` names the AI command, a `pre_tag` hook (`npm ci && npm run build`, which writes
`dist/`), a `post_release` hook, `package.json` as the version file, and a `max_prompt_bytes`
low enough that the notes of v1.4.0 are written in 4 parts. Each variant is a mode of `world.sh`:

- **`release`**: pigeon with the work since v1.3.2 committed and pushed, ready for v1.4.0: the
  retry, replay, dashboard and signing changes, and six design documents under `docs/specs/` for
  the exclusion check to flag. `tour`, `failed` and `dry-run` record in it.
- **`commit`**: the same, the retry change left in the tree: four files, none staged. `commit`
  records in it.
- **`regenerate`**: pigeon released to v1.4.0, `docs/specs/**` excluded, with a GitHub release for
  every version. Its earlier releases carry terser notes than the AI writes now, so regenerate
  has a `CHANGELOG.md` to rewrite. `regenerate` records in it.
- **`init`**: an empty repo at `~/Code/pigeon`. `init` records in it.

Every commit and tag is dated, and the profile the tapes source dates every commit mint makes, so
the hashes are the same in every recording. The shell runs in `TZ=UTC` with `HOME` the world's
home, so paths read `~/Code/pigeon`. The remote is reached through a `url.insteadOf` in the
world's own gitconfig, which the shell names in `GIT_CONFIG_GLOBAL`, with `GIT_CONFIG_NOSYSTEM=1`.

The shims on the world's `PATH`:

- **`mint`**, built from this checkout with its version pinned (`mint_version` in `world.sh`), so
  the header shows a release as an installed mint would.
- **`claude`**, the world's `ai_command`. It reads the prompt mint pipes it, waits a moment so the
  spinners show, and answers from canned replies chosen by what the prompt asks: the exclusion
  check, a part of the notes, the merge, single-pass notes, a commit message, or what a failed
  call's output means. `world.sh` lists the text that tells each apart. `DEMO_AI_FAIL_PART=3`
  makes part 3 fail with a usage-limit message, for `failed`. Single-pass notes are chosen by the
  file each version added; v1.2.0's come back empty, so `regenerate --all` skips it.
- **`gh`**: signed in; `release create`, `edit` and `view` answer as GitHub would, from releases
  kept under `demo/out/world/github/`, with `https://github.com/acme/pigeon/releases/tag/<tag>`
  where a URL is due.
- **`git`**: the real git, waiting a moment before a push, as the network would, so the spinner
  shows. `git remote get-url origin` answers with the URL as configured: git expands `insteadOf`
  there, which would show mint the bare repo's path and no GitHub to publish to.
- **`npm`**: the `pre_tag` hook's install and build, which takes a moment and writes `dist/`.

## The look

agntc's, which is Switchboard's: JetBrains Mono, the nord theme, 100 columns at 28px, frames twice
the width the README shows them at, so a high-density screen draws them sharp. A cell is
16 × 37 px, so a frame is 1600 px wide and, with its 40 px margin, the tour is 1680: twice 840.
The shell reports `COLORTERM=truecolor`, as Ghostty and iTerm do.

## How a tape becomes art

`record.sh` deletes the tape's old frames, runs it through vhs, and hands its frames to
`finalize.sh`, which makes the art:

- **The frames, not a video.** vhs writes every frame it shows to `demo/out/<name>/`, as the
  terminal drew it. A video's compression shifts every pixel a little from frame to frame, which an
  animated WebP then stores whole; from the frames themselves it stores only what changed,
  losslessly.
- **The art.** For a clip, an animated WebP, which GitHub shows inline, at the frames' 30 a second,
  and an H.264 MP4 of the same frames; for a still, a lossless WebP of the last frame. Each frame
  is set in a margin of the theme's background.

`demo/out/` is remade each time and never committed.

## Setting up

```bash
brew install vhs ffmpeg webp
ls ~/Library/Fonts /Library/Fonts | grep JetBrainsMono-Regular
```

vhs 0.12 or later. `world.sh` builds mint with `go`, so Go must be on the `PATH`.

## Gotchas

- **The sandbox.** vhs drives a headless browser that connects to `ttyd` over loopback, which an
  agent's sandbox refuses (`could not open ttyd: ... ERR_CONNECTION_REFUSED`): run `record.sh`
  with the sandbox off.
- **`TMPDIR` on another volume.** vhs records a tape's frames into a directory of its own under
  `$TMPDIR`, then moves it to the tape's `Output`. Where `$TMPDIR` is on another volume than the
  repository, the move fails without a word, and no frames come: `record.sh` gives vhs a `TMPDIR`
  in `demo/out`.
- **Silent failures.** vhs can run a tape, report nothing wrong, and write nothing, leaving the
  last recording's frames to be judged as new. `record.sh` deletes a tape's output before it
  records, and `finalize.sh` stops when there are no frames.
- **Settings come first.** A `Set` after any other command is ignored, and `LetterSpacing` takes
  no negative value.
- **Quoted paths.** A path with a `/` in a tape must be quoted, or the tape doesn't parse.
- **`Wait+Screen` reads the top.** It matches against the first screenful of output, not what
  shows once the output has scrolled, so a tape that waits on the screen is tall enough for its
  whole run. `Wait`, which looks for the prompt on the cursor's line, isn't affected.
