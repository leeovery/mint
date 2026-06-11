# AI transport validation accepts an exit-0 polite refusal

`internal/ai/transport.go:170-179` — `isValid` only checks that the body is non-empty / not whitespace-only. An exit-0 polite refusal (e.g. "I can't help with that") therefore passes validation and would be used as the release-notes body.

Decide whether minimal refusal detection is intended (current behaviour, with the interactive review gate as the human backstop) or whether a lightweight refusal heuristic should be added to the transport's validation.

Non-blocking. A decision item — confirm intent or add a heuristic.

Source: review of mint-release-tool/mint-release-tool (Recommendation #21)
