# Harden gh output classification against wording and locale changes

mint's GitHub interactions currently classify gh's behaviour by substring-matching its English stderr text, which is brittle against gh upgrades, wording tweaks, and locales. Two concrete sites surfaced in the mint-release-tool final review (idea #18, reports 1-8 and 5-7):

1. `publish.ReleaseExists` (internal/publish/publish.go:118,148) decides "release not found" by scanning gh's stderr for the English "release not found" marker. If gh rewords that message, ships a localised variant, or changes which stream carries it, the create-or-update dispatch in regenerate misclassifies the probe result.

2. `preflight.CheckGhAuth` (internal/preflight/preflight.go:261) probes `gh auth status` with no `--hostname` argument, so authentication against *any* host satisfies the gate — a user logged into a GitHub Enterprise host but not github.com passes preflight and then fails at publish time, after the point where failing loudly was cheap.

The idea is to decide on and adopt a sturdier classification signal rather than English prose: candidates discussed include relying on gh exit codes where they're documented and stable, probing existence via `gh api` and branching on a structured 404, or any other machine-readable signal gh offers — and pinning `--hostname github.com` (or the configured host) on the auth probe so the gate actually checks the host mint will publish to.

Context worth keeping: the review judged the current behaviour working-but-fragile, not broken — both sites are test-guarded with today's wording, so this is robustness investment rather than a bug fix. It also matters slightly more going forward because the commit-command verb will reuse the same preflight/publish seams, widening the blast radius of a silent misclassification. Whatever signal is chosen should remain compatible with the FakeRunner-scripted test approach used across internal/publish and internal/preflight, which currently seeds the English stderr text.
