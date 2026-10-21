"""Post-exclusion diff bytes per sampled release (a setup measurement, not an outcome).

Mirrors mint's notes diff: `git diff --src-prefix=a/ --dst-prefix=b/ --no-color
--no-ext-diff prev..tag -- .` with the built-in CHANGELOG.md and .mint.toml excludes,
the at-tag shared and [release] diff_exclude globs as plain :(exclude) pathspecs, and
the plain-mode version_file exclude. Usage: python3 -I sizes.py sample.tsv
"""

import subprocess
import sys
import tomllib


def at_tag_config(repo, tag):
    out = subprocess.run(["git", "-C", repo, "show", f"{tag}:.mint.toml"], capture_output=True, text=True).stdout
    return tomllib.loads(out) if out else {}


def pick(cfg, key, default):
    rel = cfg.get("release", {})
    return rel.get(key, cfg.get(key, default))


def main(path):
    total = 0
    for line in open(path):
        if line.startswith("#") or not line.strip():
            continue
        project, repo, prev, tag = line.rstrip("\n").split("\t")[:4]
        cfg = at_tag_config(repo, tag)
        globs = list(cfg.get("diff_exclude", [])) + list(cfg.get("release", {}).get("diff_exclude", []))
        excludes = [":(exclude)CHANGELOG.md", ":(exclude).mint.toml"] + [":(exclude)" + g for g in globs]
        version_file, version_pattern = pick(cfg, "version_file", ""), pick(cfg, "version_pattern", "")
        if version_file and not version_pattern:
            excludes.append(":(exclude)" + version_file)
        diff = subprocess.run(
            ["git", "-C", repo, "diff", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", "--no-ext-diff",
             f"{prev}..{tag}", "--", ".", *excludes],
            capture_output=True,
        ).stdout
        total += len(diff)
        print(f"{project}\t{tag}\t{len(diff)}")
    print(f"total\t-\t{total}")


if __name__ == "__main__":
    main(sys.argv[1])
