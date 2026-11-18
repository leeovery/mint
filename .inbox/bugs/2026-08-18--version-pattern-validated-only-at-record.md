# version_pattern without {version} passes preflight, fails at record after the user approves the release

Setting up mint on a fresh repo (dex-engine, first-ever release, v0.0.1), the
`.mint.toml` was written with:

```toml
[release]
version_file = "pyproject.toml"
version_pattern = 'version = "'
```

Running `./release` with this config: preflight passed ("tree clean, on main,
v0.0.1 free, origin in sync"), the pre_tag hook ran, release notes were
generated and presented at the review gate, the user read the plan
(commit/tag/publish) and confirmed with `y` — and only then did the run fail:

```
✗ record     record: version_pattern "version = \"" has no {version} placeholder
```

So a config value that is statically checkable — the placeholder is either
present in the string or it isn't — was accepted through config load and
preflight, and surfaced as an error only at the record stage, after the user
had reviewed notes and explicitly approved the release. The release aborted at
that point; correcting the pattern to `version = "{version}"` required a new
commit (preflight requires a clean tree), then a full re-run including a fresh
notes review.

A second facet of the same experience: the `{version}` placeholder requirement
appears nowhere in `mint setup`'s output. The config-reference table describes
version_pattern only as "version line replaced inside version_file (empty
treats the whole file as the version)", and no example anywhere in the guide
shows the placeholder syntax. The setup guide positions itself as
self-contained ("you never need to fetch anything external") and is explicitly
addressed to an AI assistant configuring the tool, so the assistant writing
this config followed the guide, flagged the pattern syntax as uncertain to the
user, guessed the natural line-prefix form — and the guess was only falsifiable
by running a release to the record stage.

Impact: on a first release the failure is confusing but cheap. On a routine
release it wastes a full gate cycle (notes generation, human review, approval)
before reporting a problem that existed in the config before the run started,
and the operator pays it again on the re-run.
