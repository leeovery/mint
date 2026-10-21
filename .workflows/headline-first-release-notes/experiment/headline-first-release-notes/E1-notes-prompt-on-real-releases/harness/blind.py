"""Blinds the held-out notes for the judges and builds Lee's side-by-side read.

Judges: each held-out release's four bodies (current r1, r2; new r1, r2) are copied to
<cache>/blind/<project>-<tag>/<label>.md under labels A-D in a seeded random order.
Lee: six pre-registered releases, repeat 1 of each arm, labelled A/B by a seeded coin
flip, written to <record>/data/lee-read.md. Both keys go to <cache> and move into
<record>/data/ only once the judgements and Lee's picks are in.

Usage: python3 -I blind.py --new-version v3
"""

import argparse
import json
import os
import random
import shutil
import subprocess

SEED = 20261021
HERE = os.path.dirname(os.path.abspath(__file__))
RECORD = os.path.dirname(HERE)
MINT = subprocess.run(["git", "-C", HERE, "rev-parse", "--show-toplevel"], capture_output=True, text=True).stdout.strip()
CACHE = os.path.join(MINT, ".workflows/.cache/headline-first-release-notes/experiment/headline-first-release-notes")

LEE_READ = [
    ("portal-v0.11.1", "Fixes resume hooks that could silently vanish, so a restored pane came back to a bare shell; hooks saved in the old format stop firing after upgrade."),
    ("tick-v0.3.0", "Every TOON document rebuilt so a standard TOON parser reads it and free text survives read-edit-write; task descriptions change format (breaking for agent parsers)."),
    ("switchboard-v0.2.6", "Performance: finished days stored as per-day digests so the sessions list, session pages and the Log's earlier days open fast; plus a small Runway tweak."),
    ("agentic-workflows-v0.8.0", "A Claude Code mod draws the workflow gate menus as clickable buttons above the prompt."),
    ("agntc-v0.5.2", "Packs' support folders, which 0.5.0 and 0.5.1 dropped, install again, with an update safety net; plus git retries and one-line errors."),
    ("mint-v0.0.2", "Two co-equal bodies: dry run becomes a true preview whose notes the real run reuses, and release output is redesigned around single-keypress gates."),
]


def holdout_stems():
    stems = []
    for line in open(os.path.join(RECORD, "sample.tsv")):
        if line.startswith("#") or not line.strip():
            continue
        cols = line.rstrip("\n").split("\t")
        if cols[4] == "holdout":
            stems.append(f"{cols[0]}-{cols[3]}")
    return stems


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--new-version", required=True)
    args = ap.parse_args()
    rng = random.Random(SEED)
    bodies = os.path.join(RECORD, "data", "bodies", "holdout")
    new_dir = f"new-{args.new_version}"

    key = {}
    for stem in holdout_stems():
        sources = [("current", 1), ("current", 2), (new_dir, 1), (new_dir, 2)]
        present = [(arm, n) for arm, n in sources if os.path.exists(os.path.join(bodies, arm, f"{stem}-r{n}.md"))]
        labels = ["A", "B", "C", "D"][: len(present)]
        rng.shuffle(present)
        out = os.path.join(CACHE, "blind", stem)
        os.makedirs(out, exist_ok=True)
        key[stem] = {}
        for label, (arm, n) in zip(labels, present):
            shutil.copyfile(os.path.join(bodies, arm, f"{stem}-r{n}.md"), os.path.join(out, f"{label}.md"))
            key[stem][label] = {"arm": "new" if arm == new_dir else "current", "repeat": n}
    with open(os.path.join(CACHE, "blind-key.json"), "w") as f:
        json.dump({"seed": SEED, "new_version": args.new_version, "key": key}, f, indent=2)

    lee_key = {}
    parts = ["# Release notes: blind side-by-side\n",
             "For each release, pick the notes that better meet the bar: good, readable, coherent, useful, accurate, well ordered. "
             "Reply with six picks, e.g. `1 A, 2 B, …`.\n"]
    for i, (stem, truth) in enumerate(LEE_READ, 1):
        pair = [("current", os.path.join(bodies, "current", f"{stem}-r1.md")),
                ("new", os.path.join(bodies, new_dir, f"{stem}-r1.md"))]
        if rng.random() < 0.5:
            pair.reverse()
        lee_key[stem] = {"A": pair[0][0], "B": pair[1][0]}
        parts.append(f"\n---\n\n## {i}. {stem.replace('-v', ' v', 1)}\n\n*What the release was:* {truth}\n")
        for label, (_, path) in zip("AB", pair):
            parts.append(f"\n### {label}\n\n{open(path).read().strip()}\n")
    with open(os.path.join(RECORD, "data", "lee-read.md"), "w") as f:
        f.write("".join(parts))
    with open(os.path.join(CACHE, "lee-key.json"), "w") as f:
        json.dump({"seed": SEED, "new_version": args.new_version, "key": lee_key}, f, indent=2)


if __name__ == "__main__":
    main()
