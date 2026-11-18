# Publish failure swallows gh's stderr — real cause invisible

During the agentic-workflows v0.6.0 release (2026-07-20), the run got all the way through preflight, hooks, notes approval, and the atomic branch+tag push, then failed at the publish step with only this to go on:

```
⚠ publish failed  tag is already published; heal with regenerate --source tag
creating GitHub release for tag "v0.6.0": running "gh": exited with code 1: exit status 1
```

The actual error from `gh` was never shown. Running the equivalent `gh release create` by hand revealed it immediately: `HTTP 403: API rate limit exceeded` — the release day had burned the full 5,000/hr GitHub API budget on a 75-PR stack landing, and the publish was simply the first call to hit the empty bucket. Nothing was wrong with the tag, the release, or mint's sequencing.

The impact compounds: the healing path failed the same way for the same reason, with the same silent swallowing — `mint release regenerate v0.6.0 --source tag --target release -y` reported `publish: FAILED - creating GitHub release … exited with code 1` and nothing else. So both the primary path and its documented recovery dead-ended with "exit status 1" while the underlying condition was transient and self-resolving (the limit reset ~7 minutes later and a manual `gh release create` succeeded with the tag annotation as notes).

Symptoms in short: any gh failure during publish surfaces as a bare exit code; the user cannot distinguish a transient API condition from a genuine conflict or auth problem; and the misdiagnosis in the accompanying hint text (see the separate bug about the hint) sends them down the wrong recovery path. Everything else in the release had already succeeded — CHANGELOG committed, version file mirrored, tag pushed atomically — so the failure looks scarier than it is, and the one piece of information that would have defused it (gh's own stderr) was discarded.
