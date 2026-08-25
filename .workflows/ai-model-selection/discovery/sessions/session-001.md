# Discovery Session 001

Date: 2026-06-13
Work unit: ai-model-selection

## Description (as of session)

Pin a model in mint's default AI command and decide the AI model/command config shape per verb (top-level vs per-verb override).

## Seed

- seeds/2026-06-11-ai-model-selection.md (inbox:idea)

## Imports

(none)

## Map State at Start

(n/a — single-topic work)

## Exploration

Originated from an inbox idea: mint's default `ai_command` (`claude -p`) inherits whatever default model the operator's Claude CLI is set to — likely an overpowered, slower, more expensive model than these tasks need. Immediate want is to pin a model in the shipped default, probably via the alias form (`claude -p --model sonnet`) rather than a stale-prone full model ID.

Shaping settled this as a single coherent feature with a real design question to talk through (so the pipeline runs through discussion). The core question: should the two verbs (release notes, commit) use different models at all? Release notes are salience-heavy (Sonnet/Opus); commit messages are frequent and latency-sensitive (Haiku may suffice). That surfaces the config-shape decision — keep a single top-level `ai_command`, add a top-level command with per-verb override, or go straight to per-verb config.

User also raised an "ideal world" framing: rather than configuring a raw command string, pick which AI you're using and set the model as config, with the tool knowing how to invoke each AI — i.e. a driver-based pattern. User explicitly flagged this as likely YAGNI right now (no desire for multiple AIs today), so it stays in the option space for discussion to weigh and probably defer.

Code-health angle noted in the seed: the default command string is duplicated across three sites (`internal/config/config.go`, `internal/ai/transport.go`, `internal/initgen/initgen.go`) plus test pins and both specs — worth considering a single source of truth so model/default changes are one edit. Related constraint: the transport's per-attempt timeout is 60s and timeouts are fatal (not retried), so slower models couple to that deadline.

## Edits

(none)

## Topics Identified

(none)

## Conclusion

Routed to discussion.
