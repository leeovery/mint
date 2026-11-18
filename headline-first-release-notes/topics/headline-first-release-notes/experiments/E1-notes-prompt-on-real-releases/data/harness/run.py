"""Runs e1harness over one set of the sample.

One unit per (release, arm). A unit runs its repeats one after another, so the second
repeat sends a byte-identical prompt within the cache window and reads it from cache.
Up to --parallel units run at once, arms interleaved release by release. The run is
resumable: a repeat whose body already exists is skipped. A failed repeat is retried
once; a second failure is left recorded in its .meta.json.

Usage (from the mint repo root):
  python3 -I <dir>/harness/run.py --set tune --arms current,new --new-version v0
  python3 -I <dir>/harness/run.py --set holdout --arms current,new --new-version v3 --repeats 2
"""

import argparse
import concurrent.futures
import datetime
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
RECORD = os.path.dirname(HERE)
MINT = subprocess.run(["git", "-C", HERE, "rev-parse", "--show-toplevel"], capture_output=True, text=True).stdout.strip()
CACHE = os.path.join(MINT, ".workflows/.cache/headline-first-release-notes/experiment/headline-first-release-notes")


def log(msg):
    print(f"{datetime.datetime.now().strftime('%H:%M:%S')} {msg}", flush=True)


def load_sample(path, wanted_set):
    rows = []
    for line in open(path):
        if line.startswith("#") or not line.strip():
            continue
        project, repo, prev, tag, set_, headline, breaking = line.rstrip("\n").split("\t")
        if set_ == wanted_set:
            rows.append(dict(project=project, repo=repo, prev=prev, tag=tag))
    return rows


def run_repeat(binary, rel, arm, arm_dir, set_, n, new_version):
    stem = f"{rel['project']}-{rel['tag']}-r{n}"
    out_dir = os.path.join(RECORD, "data", "bodies", set_, arm_dir)
    os.makedirs(out_dir, exist_ok=True)
    body = os.path.join(out_dir, stem + ".md")
    meta = os.path.join(out_dir, stem + ".meta.json")
    if os.path.exists(body):
        return "skipped"

    cmd = [binary, "-repo", rel["repo"], "-prev", rel["prev"], "-tag", rel["tag"], "-arm", arm,
           "-body-out", body, "-meta-out", meta]
    if arm == "new":
        cmd += ["-instructions", os.path.join(RECORD, "prompts", f"new-{new_version}.md"),
                "-reminder", os.path.join(RECORD, "prompts", f"new-{new_version}-reminder.txt")]
    if n == 1:
        prompt_dir = os.path.join(CACHE, "prompts", set_, arm_dir)
        os.makedirs(prompt_dir, exist_ok=True)
        cmd += ["-prompt-out", os.path.join(prompt_dir, f"{rel['project']}-{rel['tag']}.txt")]
        if arm == "current":
            input_dir = os.path.join(CACHE, "inputs")
            os.makedirs(input_dir, exist_ok=True)
            cmd += ["-input-out", os.path.join(input_dir, f"{rel['project']}-{rel['tag']}.txt")]

    for attempt in (1, 2):
        res = subprocess.run(cmd, cwd=rel["repo"], capture_output=True, text=True)
        if res.returncode == 0:
            return "ok"
        log(f"FAIL {arm_dir} {stem} attempt {attempt}: {res.stderr.strip()[:300]}")
    return "failed"


def run_unit(binary, rel, arm, set_, repeats, new_version):
    arm_dir = "current" if arm == "current" else f"new-{new_version}"
    results = []
    for n in range(1, repeats + 1):
        log(f"start {arm_dir} {rel['project']} {rel['tag']} r{n}")
        status = run_repeat(binary, rel, arm, arm_dir, set_, n, new_version)
        log(f"{status} {arm_dir} {rel['project']} {rel['tag']} r{n}")
        results.append(status)
    return results


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--set", required=True, choices=["tune", "holdout"])
    ap.add_argument("--arms", default="current,new")
    ap.add_argument("--new-version", default="v0")
    ap.add_argument("--repeats", type=int, default=1)
    ap.add_argument("--parallel", type=int, default=4)
    ap.add_argument("--only", default="", help="comma-separated project-tag stems to restrict to")
    args = ap.parse_args()

    binary = os.path.join(CACHE, "e1harness")
    rows = load_sample(os.path.join(RECORD, "sample.tsv"), args.set)
    if args.only:
        keep = set(args.only.split(","))
        rows = [r for r in rows if f"{r['project']}-{r['tag']}" in keep]
    arms = args.arms.split(",")
    units = [(rel, arm) for rel in rows for arm in arms]
    log(f"{len(units)} units, {args.repeats} repeat(s), parallel {args.parallel}")

    failed = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.parallel) as pool:
        futures = [pool.submit(run_unit, binary, rel, arm, args.set, args.repeats, args.new_version) for rel, arm in units]
        for fut in concurrent.futures.as_completed(futures):
            failed += fut.result().count("failed")
    log(f"done, {failed} failed repeat(s)")
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
