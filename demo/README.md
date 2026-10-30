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

The art goes to `art/` at the repository's root, where the README takes it from, so `git diff`
shows what a recording changed and the art is committed with the change it shows. `look.tape`
holds the settings every tape sources; each tape sets the height its run needs at its tallest,
and a still is cropped to what its last frame drew. Each tape ends on a `Wait+Screen` for a line
only its run's real flow prints, so a shim answering wrongly, or mint failing early, fails the
recording rather than leaving a still that looks right. Those lines are mint's own words: a
change to the output changes them too.

## The review tapes

The README's tapes show a run answered the way a user usually answers it. A change to mint's
questions needs more: every question at every state it passes through. `demo/review/` holds a
tape for that per run, each taking a `Screenshot` at each state, and `review.sh` records them:

```bash
demo/review.sh                   # every review tape
demo/review.sh release plain     # the tapes named
```

| Tape | Screenshots, in `demo/out/review/<tape>/` |
|---|---|
| `release` | the version asked; the exclusion check's two questions, answered `e` then `k`; the notes running once both are answered; the review with its keys, edited with `e` and back; aborted with `n`, the unwind and the close |
| `dry-run` | `mint release -m --dry-run`: the check's questions and the review's, where a real run asks them, skipped |
| `commit` | `mint commit -Ap`: the message with its keys, then committed and pushed |
| `regenerate` | `mint release regenerate v1.4.0`: Source asked and answered `t`, Target taken at its default, the review, the close |
| `plain` | `mint release -m --plain` at a terminal: each question one line, its hint marking the default |

They make no art: the screenshots are never committed, and each run deletes its tape's old ones
first. The tapes record in the `release`, `commit` and `regenerate` worlds, with two extras the
README's tapes never use: `DEMO_AI_CHECK_TESTS=1` makes the exclusion check flag `test/` as well
as `docs/specs/**`, for a second question, and `demo-edit`, set as `VISUAL` and `EDITOR`, is an
editor that adds a few words to the notes' first item, so the review's `e` comes back changed.

## The world

`demo/world.sh` builds it under `/tmp/mint-demo`, outside the checkout, and each tape builds it
afresh, out of sight, before it shows anything, so a tape records the same thing every time. The
fixed path keeps every path mint prints, such as where it saved a failed run's part notes, the
same on every machine and from every checkout, and free of the recorder's own home folder. The
art is recorded there. `DEMO_WORLD=<dir>` moves the world, so that two checkouts can record at
once, but the paths in its frames then name that directory: record the art at the default.
`world.sh` refuses a `DEMO_WORLD` that holds anything but a world, since it empties it first.

`pigeon`, a webhook
delivery service, lives at `~/Code/pigeon`, released with mint from v1.0.0 to v1.3.2. Its
`.mint.toml` names the AI command, a `pre_tag` hook (`npm ci && npm run build`, which writes
`dist/`), a `post_release` hook, `package.json` as the version file, and a `max_prompt_bytes`
low enough that the notes of v1.4.0 are written in 4 parts (in 5 where `docs/specs/` stays in
the diff, as in `failed` and `dry-run`). Each variant is a mode of `world.sh`:

- **`release`**: pigeon with the work since v1.3.2 committed and pushed, ready for v1.4.0: the
  retry, replay, dashboard and signing changes, and six design documents under `docs/specs/` for
  the exclusion check to flag. `tour`, `failed` and `dry-run` record in it.
- **`commit`**: the same, the retry change left in the tree: four files, none staged. `commit`
  records in it.
- **`regenerate`**: pigeon released to v1.4.0, `docs/specs/**` excluded, with a GitHub release for
  every version. Its earlier releases carry terser notes than the AI writes now, so regenerate
  has a `CHANGELOG.md` to rewrite. `regenerate` records in it.
- **`init`**: an empty repo at `~/Code/pigeon`. `init` records in it.

Every commit and tag the world is built with is dated, so their hashes are the same in every
recording, and the profile the tapes source dates the commits mint makes too. All but one: mint
dates a release's `CHANGELOG.md` entry by the clock, so the bookkeeping commit's hash changes from
day to day. No frame shows it.

The shell runs in `TZ=UTC` with `HOME` the world's home, `/private/tmp/mint-demo/home`: the world
is named by its physical path, as git names the repo's root (macOS keeps `/tmp` at
`/private/tmp`), so the repo sits at `~/Code/pigeon` under `HOME`, and mint prints the paths it
names in full, such as `/private/tmp/mint-demo/home/Library/Caches/mint/chunk-notes/…`. The
user's own git and config variables (`GIT_AUTHOR_*`, `GIT_COMMITTER_*`, `GIT_DIR`,
`GIT_WORK_TREE`, `GIT_INDEX_FILE`, `XDG_CONFIG_HOME`) are unset, and the history is built with
the world's `HOME`, so the user's git ignore and attributes files can't change it. The remote is
reached through a `url.insteadOf` in the world's own gitconfig, which the shell names in
`GIT_CONFIG_GLOBAL`, with `GIT_CONFIG_NOSYSTEM=1`.

**The part count.** v1.4.0's notes come in 4 parts because `max_prompt_bytes` (in `mint_toml`
in `world.sh`, 20800) leaves only so much room for diff beside mint's notes prompt, its Change
Map and its framing. A change to mint's prompt, or to pigeon's history, shifts the room and so the
count, without a word. To recalibrate, record `tour` and read its notes row, `generated in N parts,
merged`: raise `max_prompt_bytes` for fewer parts, lower it for more, a few hundred bytes at a
time. The exclusion check's pause must still come, and `failed` needs at least 3 parts.

The shims on the world's `PATH`:

- **`mint`**, built from this checkout with its version pinned (`mint_version` in `world.sh`), so
  the header shows a release as an installed mint would.
- **`claude`**, the world's `ai_command`. It reads the prompt mint pipes it, waits a moment so the
  spinners show, and answers from canned replies chosen by what the prompt asks: the exclusion
  check, a part of the notes, the merge, single-pass notes, a commit message, or what a failed
  call's output means. `world.sh` lists the text that tells each apart. `DEMO_AI_FAIL_PART=3`
  makes part 3 fail with a usage-limit message, for `failed`. `DEMO_AI_CHECK_TESTS=1` has the
  exclusion check flag `test/` too, for the review tapes. Single-pass notes are chosen by the
  file each version added; v1.2.0's come back empty, so `regenerate --all` skips it.
- **`gh`**: signed in; `release create`, `edit` and `view` answer as GitHub would, from releases
  kept in the world's `github/`, with `https://github.com/acme/pigeon/releases/tag/<tag>`
  where a URL is due.
- **`git`**: the real git, waiting a moment on a `git push`, as the network would, so the spinner
  shows. `git remote get-url origin` answers with the URL as configured: git expands `insteadOf`
  there, which would show mint the bare repo's path and no GitHub to publish to.
- **`npm`**: the `pre_tag` hook's install and build, which takes a moment and writes `dist/`.
- **`demo-edit`**: the review tapes' editor; it waits a moment, adds a few words to the notes'
  first item, and exits.

## The look

agntc's, which is Switchboard's, in Lee's Ghostty theme: JetBrains Mono, Nordfox (Ghostty's own
values, from its `themes/Nordfox`), 100 columns at 28px, frames twice
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
- **The crop.** A still's screen is as tall as its run's tallest moment, such as `commit`'s
  message at its question, so its last frame can end in empty rows. `finalize.sh` finds the
  first and last rows holding a pixel unlike the margin's, reading every pixel, and cuts below
  the last, leaving under it the margin above the first. Clips keep their full height.

`demo/out/`, the world and the frames, is remade each time and never committed.

## Setting up

```bash
brew install vhs ffmpeg webp
ls ~/Library/Fonts /Library/Fonts | grep JetBrainsMono-Regular
```

vhs 0.12 or later. `world.sh` builds mint with `go`, so Go must be on the `PATH`.

## Gotchas

- **The sandbox.** vhs drives a headless browser that connects to `ttyd` over loopback, which an
  agent's sandbox refuses (`could not open ttyd: ... ERR_CONNECTION_REFUSED`): run `record.sh`
  (and `review.sh`) with the sandbox off.
- **`TMPDIR` on another volume.** vhs records a tape's frames into a directory of its own under
  `$TMPDIR`, then moves it to the tape's `Output`. Where `$TMPDIR` is on another volume than the
  repository, the move fails without a word, and no frames come: `record.sh` gives vhs a `TMPDIR`
  in `demo/out`.
- **A screenshot is the next frame.** `Screenshot` saves the next frame vhs captures, and on a tall
  screen the capture lags the terminal, so it can land after the key the tape types next: the
  review tapes wait a second after each one.
- **Silent failures.** vhs can run a tape, report nothing wrong, and write nothing, leaving the
  last recording's frames to be judged as new. `record.sh` deletes a tape's output before it
  records, and `finalize.sh` stops when there are no frames.
- **Settings come first.** A `Set` after any other command is ignored, and `LetterSpacing` takes
  no negative value.
- **Quoted paths.** A path with a `/` in a tape must be quoted, or the tape doesn't parse.
- **`Wait+Screen` reads the top.** It matches against the first screenful of output, not what
  shows once the output has scrolled, so a tape that waits on the screen is tall enough for its
  whole run. `Wait`, which looks for the prompt on the cursor's line, isn't affected.
