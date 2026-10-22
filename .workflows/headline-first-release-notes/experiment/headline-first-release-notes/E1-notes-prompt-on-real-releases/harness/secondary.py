"""Secondary measures for E1 (reported, not in the decision rule): per-arm field counts,
headline and lead distributions, repeat agreement, per-release preference, Lee's read.
Writes <record>/data/secondary.json. Usage: python3 -I secondary.py"""
import collections, glob, json, os, statistics
HERE = os.path.dirname(os.path.abspath(__file__)); RECORD = os.path.dirname(HERE)
key = json.load(open(os.path.join(RECORD, "data", "blind-key.json")))["key"]
bodies = []
for f in sorted(glob.glob(os.path.join(RECORD, "data", "judgements", "*.json"))):
    for j in json.load(open(f)):
        j.update(key[j["release"]][j["label"]]); bodies.append(j)
fields = ["inversions", "misfiled", "duplicates", "internal", "false_claims", "in_release_as_change", "missing", "format_slips"]
out = {"field_counts": {}, "lead": {}, "headline_verdict": {}, "overall_mean": {}, "breaking": {}}
for arm in ("current", "new"):
    b = [x for x in bodies if x["arm"] == arm]
    out["field_counts"][arm] = {f: sum(len(x.get(f, [])) for x in b) for f in fields}
    out["field_counts"][arm]["bodies_with_any_false_claim"] = sum(1 for x in b if x.get("false_claims"))
    out["lead"][arm] = dict(collections.Counter(x["lead"] for x in b))
    out["headline_verdict"][arm] = dict(collections.Counter(x["headline"]["verdict"] for x in b))
    out["overall_mean"][arm] = round(statistics.mean(x["overall"] for x in b), 2)
    out["breaking"][arm] = dict(collections.Counter(x["breaking"] for x in b if x["breaking"] != "n/a"))
per_release = {}
for stem in sorted({x["release"] for x in bodies}):
    r = {arm: sorted([x for x in bodies if x["release"] == stem and x["arm"] == arm], key=lambda x: x["repeat"]) for arm in ("current", "new")}
    per_release[stem] = {
        "overall": {arm: [x["overall"] for x in r[arm]] for arm in r},
        "prefers_new": statistics.mean(x["overall"] for x in r["new"]) > statistics.mean(x["overall"] for x in r["current"]),
        "new_headline": [x["headline"]["verdict"] for x in r["new"]],
        "new_headline_present_agree": r["new"][0]["headline"]["present"] == r["new"][1]["headline"]["present"],
        "false_claims": {arm: [len(x.get("false_claims", [])) for x in r[arm]] for arm in r},
        "missing": {arm: [len(x.get("missing", [])) for x in r[arm]] for arm in r},
    }
out["per_release"] = per_release
out["new_headline_presence_agreement"] = sum(v["new_headline_present_agree"] for v in per_release.values())
lee_key = json.load(open(os.path.join(RECORD, "data", "lee-key.json")))["key"]
picks = json.load(open(os.path.join(RECORD, "data", "lee-picks.json")))["picks"]
lee = {stem: lee_key[stem][label] for stem, label in picks.items()}
out["lee"] = {"picked_arm": lee, "new_preferred": sum(1 for a in lee.values() if a == "new"),
              "agrees_with_judges": sum(1 for s, a in lee.items() if (a == "new") == per_release[s]["prefers_new"])}
json.dump(out, open(os.path.join(RECORD, "data", "secondary.json"), "w"), indent=2)
print(json.dumps({k: v for k, v in out.items() if k != "per_release"}, indent=2))
for s, v in per_release.items():
    print(f"{s:28} cur {v['overall']['current']} new {v['overall']['new']} prefers_new={v['prefers_new']} hl={v['new_headline']} fc={v['false_claims']} miss={v['missing']}")
