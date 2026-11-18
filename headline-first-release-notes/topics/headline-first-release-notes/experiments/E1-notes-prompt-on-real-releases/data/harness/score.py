"""Executes E1's pre-registered decision rule against the judges' scores.

Reads <record>/data/judgements/*.json (each a JSON array of per-body judgements, the
shape rubric.md fixes), <record>/data/blind-key.json (moved in after judging) and
<record>/sample.tsv. Writes <record>/data/scores.json and prints the class table and
the one-line verdict. Nothing here is judgement: every threshold is the design's.

Usage: python3 -I score.py
"""

import glob
import json
import math
import os
import statistics

HERE = os.path.dirname(os.path.abspath(__file__))
RECORD = os.path.dirname(HERE)

RATIO_CLASSES = [
    # (class, fields counted, new must be <= this share of current)
    ("order within sections", ["inversions"], 0.5),
    ("section placement", ["misfiled", "duplicates"], 0.4),
    ("user-facing only", ["internal"], 0.3),
    ("true to the release", ["false_claims", "in_release_as_change"], 0.4),
]


def load_sample():
    rows = {}
    for line in open(os.path.join(RECORD, "sample.tsv")):
        if line.startswith("#") or not line.strip():
            continue
        project, _, _, tag, set_, headline, breaking = line.rstrip("\n").split("\t")
        if set_ == "holdout":
            rows[f"{project}-{tag}"] = {"headline": headline, "breaking": breaking != "-"}
    return rows


def load_judgements(key):
    bodies = []
    for path in sorted(glob.glob(os.path.join(RECORD, "data", "judgements", "*.json"))):
        for j in json.load(open(path)):
            meta = key[j["release"]][j["label"]]
            j["arm"], j["repeat"] = meta["arm"], meta["repeat"]
            bodies.append(j)
    return bodies


def residual_verdict(instances):
    if not instances:
        return "needs more prompt work", 0, 0
    absent = sum(1 for present in instances if present is False)
    return ("needs new input" if absent / len(instances) >= 0.5 else "needs more prompt work"), absent, len(instances)


def main():
    sample = load_sample()
    key = json.load(open(os.path.join(RECORD, "data", "blind-key.json")))["key"]
    bodies = load_judgements(key)
    new = [b for b in bodies if b["arm"] == "new"]
    cur = [b for b in bodies if b["arm"] == "current"]
    classes, residuals = {}, {}

    # Lead and headline
    clean = sum(1 for b in new if b["lead"] == "clean")
    match = sum(1 for b in new if b["headline"]["verdict"] == "match")
    wrong = sum(1 for b in new if b["headline"]["verdict"] == "wrong-subject")
    lead_pass = clean >= math.ceil(0.85 * len(new)) and match >= math.ceil(0.80 * len(new)) and wrong <= 1
    classes["lead and headline"] = {
        "pass": lead_pass, "new_clean_lead": clean, "new_headline_match": match, "new_wrong_subject": wrong,
        "new_bodies": len(new), "current_clean_lead": sum(1 for b in cur if b["lead"] == "clean"),
    }
    residuals["lead and headline"] = [b.get("lead_input_present") for b in new if b["lead"] != "clean"] + \
        [b["headline"].get("input_present") for b in new if b["headline"]["verdict"] != "match"]

    # Ratio classes
    for name, fields, share in RATIO_CLASSES:
        n = sum(len(b.get(f, [])) for b in new for f in fields)
        c = sum(len(b.get(f, [])) for b in cur for f in fields)
        passed = n == 0 if c == 0 else n <= share * c
        classes[name] = {"pass": passed, "new": n, "current": c, "limit": share * c}
        residuals[name] = [e.get("input_present") for b in new for f in fields for e in b.get(f, [])]

    # Breaking changes
    on_breaking = [b for b in new if sample[b["release"]]["breaking"]]
    flagged = sum(1 for b in on_breaking if b["breaking"] == "flagged")
    classes["breaking changes"] = {
        "pass": flagged >= math.ceil(0.8 * len(on_breaking)), "new_flagged": flagged, "new_bodies": len(on_breaking),
        "current_flagged": sum(1 for b in cur if sample[b["release"]]["breaking"] and b["breaking"] == "flagged"),
    }
    residuals["breaking changes"] = [b.get("breaking_input_present") for b in on_breaking if b["breaking"] != "flagged"]

    # Guard rails
    missing_new = sum(len(b.get("missing", [])) for b in new)
    missing_cur = sum(len(b.get("missing", [])) for b in cur)
    format_new = sum(len(b.get("format_slips", [])) for b in new)
    format_cur = sum(len(b.get("format_slips", [])) for b in cur)
    prefer, releases = 0, 0
    for stem in sample:
        n_scores = [b["overall"] for b in new if b["release"] == stem]
        c_scores = [b["overall"] for b in cur if b["release"] == stem]
        if n_scores and c_scores:
            releases += 1
            prefer += statistics.mean(n_scores) > statistics.mean(c_scores)
    guards = {
        "missing items": {"hold": missing_new <= math.ceil(1.10 * missing_cur), "new": missing_new, "current": missing_cur},
        "format slips": {"hold": format_new <= format_cur, "new": format_new, "current": format_cur},
        "judges prefer new": {"hold": prefer >= math.ceil(0.70 * releases), "releases_preferring_new": prefer, "releases": releases},
    }

    passed = [c for c, v in classes.items() if v["pass"]]
    failed = {c: residual_verdict(residuals[c]) for c, v in classes.items() if not v["pass"]}
    broken = [g for g, v in guards.items() if not v["hold"]]
    parts = []
    if passed:
        parts.append("passes " + ", ".join(passed))
    need_input = [c for c, (v, _, _) in failed.items() if v == "needs new input"]
    need_prompt = [c for c, (v, _, _) in failed.items() if v == "needs more prompt work"]
    if need_input:
        parts.append(", ".join(need_input) + " need mint to hand the AI more")
    if need_prompt:
        parts.append(", ".join(need_prompt) + " need more prompt work")
    if broken:
        parts.append("guard rail broken (" + ", ".join(broken) + ") — not taken as tested")
    head = "Meets the bar" if not failed and not broken else "Short of the bar"
    verdict = f"{head}: rewritten prompt " + "; ".join(parts)

    out = {"classes": classes, "guards": guards,
           "residuals": {c: {"verdict": v, "input_absent": a, "instances": n} for c, (v, a, n) in failed.items()},
           "headline_shape_slips_new": sum(len(b["headline"].get("shape_slips", [])) for b in new),
           "verdict": verdict}
    with open(os.path.join(RECORD, "data", "scores.json"), "w") as f:
        json.dump(out, f, indent=2)
    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()
