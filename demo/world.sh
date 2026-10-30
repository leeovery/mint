#!/usr/bin/env bash
# The world the tapes record in, built under /tmp/mint-demo (DEMO_WORLD moves
# it): a home folder with pigeon in it at ~/Code/pigeon, the bare repo it
# pushes to, the shims mint calls, and a shell setup the tapes source, the
# world's profile.sh. Nothing reaches the network, the AI, GitHub, or the real
# home folder or git config.
#
# pigeon is a webhook delivery service released with mint from v1.0.0 to
# v1.3.2. Its origin is a local bare repo that git reaches as
# https://github.com/acme/pigeon through a url.insteadOf in a gitconfig of the
# world's own (GIT_CONFIG_GLOBAL). Every commit and tag the world is built
# with is dated, so its hashes are the same in every recording, and so are
# the commits mint makes (GIT_AUTHOR_DATE, GIT_COMMITTER_DATE in the profile),
# all but a release's bookkeeping commit, whose CHANGELOG.md entry mint dates
# by the clock, so its hash changes from day to day. No frame shows it.
#
# Shims on the world's PATH:
#   mint    built from this checkout, carrying the release the art shows
#   claude  the world's ai_command: canned replies chosen by what the prompt
#           asks, after a pause so the spinners show (see ai() below)
#   gh      signed in; the release calls answer as GitHub would, from
#           releases kept in the world's github/
#   git     the real git, waiting a moment on a push, as the network would,
#           and naming origin by its GitHub URL (see shims() below)
#   npm    the pre_tag hook's `npm ci && npm run build`: writes dist/
#   demo-edit  an editor for the review tapes' e: after a moment, it adds
#           a few words to the notes' first item and exits
#
# Usage:  demo/world.sh release      pigeon at v1.3.2 with work since, ready
#                                    for v1.4.0: release, failed, dry-run
#         demo/world.sh commit       the same, its last change not yet
#                                    committed: commit -Ap
#         demo/world.sh regenerate   pigeon released to v1.4.0, a GitHub
#                                    release for every tag: regenerate --all
#         demo/world.sh init         an empty repo at ~/Code/pigeon: init
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
# The world sits at a short path outside the checkout, so every path mint
# prints, its saved part notes' among them, reads the same on any machine and
# from any checkout. DEMO_WORLD moves it, so that two checkouts can record at
# once. It is named by its physical path, as git names the repo's root (macOS
# keeps /tmp at /private/tmp), so HOME is a prefix of every path mint prints.
mkdir -p "${DEMO_WORLD:-/tmp/mint-demo}"
world="$(cd -P "${DEMO_WORLD:-/tmp/mint-demo}" && pwd)"
bin="$world/bin"
home="$world/home"
repo="$home/Code/pigeon"
origin="$world/origin/pigeon.git"
github="$world/github"

# The mint release the header shows.
mint_version=0.1.2
# When every commit mint makes in the world is dated.
now="2026-10-21T14:30:00Z"

# What the shell running this or the tapes carries of the user's own git and
# config never reaches the world.
leaks="GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE XDG_CONFIG_HOME"
# shellcheck disable=SC2086 # the names, one word each
unset $leaks
export GIT_CONFIG_GLOBAL="$world/gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export TZ=UTC

# --- the shims -------------------------------------------------------------

shims() {
	local git
	git="$(command -v git)"
	mkdir -p "$bin"
	(cd "$root" && go build -ldflags "-X mint/internal/buildinfo.Version=$mint_version" -o "$bin/mint" ./cmd/mint)

	# mint reads the remote's URL to find the provider, and `git remote get-url`
	# expands insteadOf, which would show it the bare repo's file:// path, so
	# that one read answers with the URL as configured.
	# mint runs every push as `git push …`, with no option before it.
	cat >"$bin/git" <<EOF
#!/usr/bin/env bash
if [ "\$*" = "remote get-url origin" ]; then
	exec "$git" config --get remote.origin.url
fi
[ "\${1:-}" = push ] && sleep 1.2
exec "$git" "\$@"
EOF

	cat >"$bin/gh" <<EOF
#!/usr/bin/env bash
# GitHub, as far as mint asks it: auth status, and release create, edit and
# view against the releases kept in $github.
set -euo pipefail
store="$github"
EOF
	cat >>"$bin/gh" <<'EOF'
url() { echo "https://github.com/acme/pigeon/releases/tag/$1"; }
case "${1:-} ${2:-}" in
"auth status")
	echo "github.com" >&2
	echo "  ✓ Logged in to github.com account acme (keyring)" >&2
	;;
"release create" | "release edit")
	sleep 0.8
	mkdir -p "$store"
	cat >"$store/$3.md"
	url "$3"
	;;
"release view")
	[ -f "$store/$3.md" ] || {
		echo "release not found" >&2
		exit 1
	}
	case " $* " in
	*" --json body "*) cat "$store/$3.md" ;;
	*) printf 'title:\t%s\ntag:\t%s\nurl:\t%s\n' "$3" "$3" "$(url "$3")" ;;
	esac
	;;
*)
	echo "gh (demo): not faked: gh $*" >&2
	exit 1
	;;
esac
EOF

	# The pre_tag hook's build: dist/ from src/ and the version being released.
	cat >"$bin/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
ci) sleep 1.6 ;;
"run build")
	sleep 1.8
	version="${MINT_NEW_VERSION:-$(sed -n 's/.*"version": "\(.*\)".*/\1/p' package.json)}"
	digest="$(cat src/*.ts src/*/*.ts | shasum -a 256 | cut -c1-16)"
	mkdir -p dist
	printf '// pigeon %s (%s)\n"use strict";\nrequire("./server.js").start(process.env);\n' "$version" "$digest" >dist/pigeon.js
	printf '// pigeon %s\nexport declare function start(env: Record<string, string | undefined>): void;\n' "$version" >dist/pigeon.d.ts
	printf '{\n  "name": "pigeon",\n  "version": "%s",\n  "source": "%s"\n}\n' "$version" "$digest" >dist/manifest.json
	;;
*)
	echo "npm (demo): not faked: npm $*" >&2
	exit 1
	;;
esac
EOF

	# The review tapes' editor, which the README's tapes never open.
	cat >"$bin/demo-edit" <<'EOF'
#!/usr/bin/env bash
sleep 1
perl -pi -e 's/for up to an hour\./for up to an hour, then give up./' "$1"
EOF

	ai >"$bin/claude"
	chmod +x "$bin"/*
}

# The AI command. It reads the prompt on stdin, as mint pipes it, and answers
# by what the prompt asks, telling the calls apart by text each of mint's
# prompts carries, tried in this order, since the merge's and the parts'
# prompts carry the notes prompt too:
#
#   the classify call   "has failed. Classify the failure"  →  OTHER
#   the exclusion check "about to be written from its diff" →  docs/specs/**,
#                       where the list holds it, else none; with
#                       DEMO_AI_CHECK_TESTS set, test/ too, for the
#                       review tapes' second question
#   the merge           "so it was read in N parts"         →  v1.4.0's notes
#   a part              "What follows is part N of M"       →  that part's notes
#   a commit message    "writing the git commit message"    →  the message
#   single-pass notes   "writing the release notes"         →  the notes of the
#                       version whose diff it reads, told by the file that
#                       version added; v1.2.0's come back empty, so
#                       regenerate --all skips it
#
# The replies are the files canned() writes. With DEMO_AI_FAIL_PART=3, part 3
# fails as a spent usage limit does, and the classify call reads it as OTHER,
# not SIZE, so the release stops on its notes rather than cutting the part
# again. Its two attempts and the classify call take about a second, less
# than any other part's reply, so it fails while parts 1, 2 and 4 are still in
# flight: mint lets them finish, and saves their notes, but sends no part 5,
# whatever the timing.
ai() {
	printf '#!/usr/bin/env bash\ncanned="%s"\n' "$world/ai"
	cat <<'EOF'
prompt="$(cat)"
has() { [[ "$prompt" == *"$1"* ]]; }
reply() {
	sleep "$1"
	cat "$canned/$2"
}

if has "has failed. Classify the failure"; then
	sleep 0.3
	echo OTHER
elif has "about to be written from its diff"; then
	if has "- docs/specs/"; then
		reply 2.2 check.txt
		if [ -n "${DEMO_AI_CHECK_TESTS:-}" ] && has "- test/"; then
			echo "test/: tests and their fixtures"
		fi
	else
		sleep 1.2
		echo none
	fi
elif has "so it was read in "; then
	reply 2.4 v1.4.0.md
elif has "What follows is part "; then
	part="$(sed -n 's/.*What follows is part \([0-9]*\) of.*/\1/p' <<<"$prompt" | head -n1)"
	if [ "$part" = "${DEMO_AI_FAIL_PART:-}" ]; then
		sleep 0.3
		echo "Claude usage limit reached. Your limit will reset at 4pm (Europe/London)."
		exit 1
	fi
	i=$(((part - 1) % 4))
	waits=(2.4 2.9 2.6 3.1)
	reply "${waits[i]}" "part-$((i + 1)).md"
elif has "writing the git commit message"; then
	reply 2.1 commit.txt
elif has "writing the release notes"; then
	if has "diff --git a/src/metrics.ts"; then
		sleep 1.6
	elif has "diff --git a/src/deliveries/batch.ts"; then
		reply 1.4 v1.1.0.md
	elif has "diff --git a/src/dashboard/auth.ts"; then
		reply 1.0 v1.2.1.md
	elif has "diff --git a/src/dashboard/filters.ts"; then
		reply 1.8 v1.3.0.md
	elif has "diff --git a/src/deliveries/timeout.ts"; then
		reply 1.2 v1.3.2.md
	else
		reply 2.4 v1.4.0.md
	fi
else
	echo "claude (demo): no canned reply for this prompt" >&2
	exit 1
fi
EOF
}

# The AI's replies under ai/, and under shipped/ the notes pigeon's releases
# were made with, terser than the AI's fresh ones, so regenerate has a
# CHANGELOG.md to rewrite.
canned() {
	local ai="$world/ai" shipped="$world/shipped"
	mkdir -p "$ai" "$shipped"
	echo "Initial release." >"$shipped/v1.0.0.md"
	lines "$shipped/v1.1.0.md" "✨ Added" "- Batched deliveries."
	lines "$shipped/v1.2.0.md" "✨ Added" "- Metrics endpoint."
	lines "$shipped/v1.2.1.md" "🐛 Fixed" "- Session refresh logout."
	lines "$shipped/v1.3.0.md" "✨ Added" "- Dashboard filters."
	lines "$shipped/v1.3.2.md" "🐛 Fixed" "- Slow endpoints blocking deliveries."

	lines "$ai/v1.1.0.md" "✨ Added" "- Deliveries to the same endpoint go out in batches — fewer requests for busy receivers."
	lines "$ai/v1.2.1.md" "🐛 Fixed" "- The dashboard no longer signs you out when your session refreshes."
	lines "$ai/v1.3.0.md" "✨ Added" "- Filter the dashboard's deliveries by endpoint, status and date."
	lines "$ai/v1.3.2.md" "🐛 Fixed" "- A slow endpoint no longer holds up deliveries to the others."
	lines "$ai/v1.4.0.md" \
		"Webhooks retry on their own now, so a flaky endpoint no longer drops events." \
		"" \
		"✨ Added" \
		"- Failed webhook deliveries retry with backoff for up to an hour." \
		"- \`pigeon replay <id>\` resends any delivery from the last 30 days." \
		"" \
		"🐛 Fixed" \
		"- The dashboard no longer shows a delivery twice after a reconnect." \
		"- Signing secrets with a trailing newline are accepted again."
	lines "$ai/part-1.md" "🐛 Fixed" "- The dashboard no longer shows a delivery twice after a reconnect."
	lines "$ai/part-2.md" "🐛 Fixed" "- Signing secrets with a trailing newline are accepted again."
	lines "$ai/part-3.md" "✨ Added" "- \`pigeon replay <id>\` resends any delivery from the last 30 days."
	lines "$ai/part-4.md" "✨ Added" "- Failed webhook deliveries retry with backoff for up to an hour."
	lines "$ai/check.txt" "docs/specs/**: design documents"
	lines "$ai/commit.txt" \
		"feat: retry failed deliveries with backoff" \
		"" \
		"Deliveries that fail now retry on an exponential schedule for up to an" \
		"hour before they're marked dead, so a short outage at the receiving end" \
		"no longer loses events. \`pigeon replay <id>\` resends one by hand."
}

# A file ($1) of the lines given.
lines() {
	local file="$1"
	shift
	printf '%s\n' "$@" >"$file"
}

gitconfig() {
	mkdir -p "$world"
	cat >"$GIT_CONFIG_GLOBAL" <<EOF
[user]
	name = Ada Webb
	email = ada@acme.dev
[commit]
	gpgSign = false
[tag]
	gpgSign = false
[init]
	defaultBranch = main
[advice]
	detachedHead = false
[url "file://$origin"]
	insteadOf = https://github.com/acme/pigeon
EOF
}

# What a tape sources, in the shell vhs opens: the world's home, PATH and git
# config, the date mint's commits carry, colour as a truecolor terminal
# reports it, and a bare prompt.
profile() {
	cat >"$world/profile.sh" <<EOF
export DEMO="$here"
export HOME="$home"
export PATH="$bin:\$PATH"
export GIT_CONFIG_GLOBAL="$GIT_CONFIG_GLOBAL"
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_DATE="$now" GIT_COMMITTER_DATE="$now"
export TZ=UTC
export COLORTERM=truecolor
unset $leaks NO_COLOR FORCE_COLOR DEMO_AI_FAIL_PART DEMO_AI_CHECK_TESTS
PS1='\[\e[34m\]❯\[\e[0m\] '
cd "$repo"
clear
EOF
}

# --- pigeon ----------------------------------------------------------------

# A commit dated $1, of everything in the tree.
commit() {
	git add -A
	GIT_AUTHOR_DATE="$1" GIT_COMMITTER_DATE="$1" git commit -qm "$2"
}

# A release as mint made it, dated $2, of the notes in file $3: the pre_tag
# build's artifacts commit, the version and the notes recorded, the tag
# carrying the notes, and the lot pushed, with a GitHub release of the notes.
release() {
	artifacts "$1" "$2"
	record "$@"
}

# The pre_tag build's artifacts for version $1, committed at $2.
artifacts() {
	MINT_NEW_VERSION="$1" "$bin/npm" run build >/dev/null
	commit "$2" "chore(release): pre-tag artifacts for v$1"
}

# The rest of release(): the version and the notes recorded, tagged, pushed and
# published.
record() {
	local version="$1" date="$2" notes
	notes="$(cat "${3:-$world/shipped/v$version.md}")"
	sed "s/\"version\": \".*\"/\"version\": \"$version\"/" package.json >package.json.new
	mv package.json.new package.json
	changelog "$version" "${date%%T*}" "$notes"
	commit "$date" "🌿 Release v$version"
	GIT_COMMITTER_DATE="$date" git tag -a --cleanup=verbatim -m "🌿 Release v$version

$notes" "v$version"
	git push -q origin main "v$version"
	mkdir -p "$github"
	printf '%s' "$notes" >"$github/v$version.md"
}

changelog() {
	local entry
	entry="$(printf '## [%s] - %s\n\n%s\n' "$1" "$2" "$3")"
	if [ ! -f CHANGELOG.md ]; then
		cat >CHANGELOG.md <<'EOF'
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
EOF
	fi
	entry="$entry" awk '
		!done && /^## \[/ { print ENVIRON["entry"]; print ""; done = 1 }
		{ print }
		END { if (!done) { print ""; print ENVIRON["entry"] } }
	' CHANGELOG.md >CHANGELOG.md.new
	mv CHANGELOG.md.new CHANGELOG.md
}

# A design document under docs/specs/: its title, then its sections, each a
# heading and a paragraph.
spec() {
	local file="docs/specs/$1" title="$2"
	shift 2
	mkdir -p docs/specs
	{
		printf '# %s\n\n' "$title"
		printf 'Status: accepted · Owner: Ada Webb\n'
		while [ $# -gt 1 ]; do
			printf '\n## %s\n\n%s\n' "$1" "$2"
			shift 2
		done
	} >"$file"
}

mint_toml() {
	cat >.mint.toml <<'EOF'
# .mint.toml — mint configuration. Only keys that differ from mint's compiled
# defaults are set; everything else rides on the defaults. Full key reference and
# defaults: run `mint setup`.

ai_command = "claude -p"
# Keep each AI call small, so a big release's notes are written in parts.
max_prompt_bytes = 20800

[release]
version_file = "package.json"
version_pattern = '"version": "{version}"'

[release.hooks]
pre_tag = "npm ci && npm run build"
post_release = "./scripts/notify.sh"
EOF
}

# pigeon's first release, v1.0.0: the delivery queue, request signing and the
# dashboard's live stream.
v1_0_0() {
	cat >package.json <<'EOF'
{
  "name": "pigeon",
  "version": "1.0.0",
  "description": "Webhook delivery that doesn't drop events",
  "license": "MIT",
  "main": "dist/pigeon.js",
  "bin": { "pigeon": "dist/pigeon.js" },
  "scripts": {
    "build": "tsc -p .",
    "test": "vitest run"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vitest": "^2.1.0"
  }
}
EOF
	cat >README.md <<'EOF'
# pigeon

Webhook delivery that doesn't drop events. pigeon queues each event your app
emits, signs it, and delivers it to every endpoint subscribed to it, with a
dashboard that shows each delivery as it happens.

```bash
npm install -g pigeon
pigeon start
```
EOF
	mkdir -p src/deliveries src/dashboard src/cli scripts test
	cat >src/server.ts <<'EOF'
import { createServer } from "node:http";
import { loadConfig } from "./config";
import { Queue } from "./deliveries/queue";
import { stream } from "./dashboard/stream";

export function start(env: Record<string, string | undefined>): void {
  const config = loadConfig(env);
  const queue = new Queue(config);
  createServer((req, res) => {
    if (req.url === "/stream") return stream(queue, req, res);
    if (req.method === "POST" && req.url === "/events") return queue.accept(req, res);
    res.writeHead(404).end();
  }).listen(config.port);
}
EOF
	cat >src/config.ts <<'EOF'
export interface Config {
  port: number;
  secret: string;
  timeoutMs: number;
}

export function loadConfig(env: Record<string, string | undefined>): Config {
  return {
    port: Number(env.PIGEON_PORT ?? 8080),
    secret: env.PIGEON_SECRET ?? "",
    timeoutMs: Number(env.PIGEON_TIMEOUT_MS ?? 10_000),
  };
}
EOF
	cat >src/deliveries/queue.ts <<'EOF'
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Config } from "../config";
import { sign } from "../signing";

export interface Delivery {
  id: string;
  endpoint: string;
  body: string;
  attempts: number;
  status: "pending" | "delivered" | "dead";
}

export class Queue {
  private deliveries = new Map<string, Delivery>();
  private listeners = new Set<(d: Delivery) => void>();

  constructor(private config: Config) {}

  accept(req: IncomingMessage, res: ServerResponse): void {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      const delivery = this.enqueue(String(req.headers["x-endpoint"]), body);
      res.writeHead(202, { "content-type": "application/json" });
      res.end(JSON.stringify({ id: delivery.id }));
    });
  }

  enqueue(endpoint: string, body: string): Delivery {
    const delivery: Delivery = { id: crypto.randomUUID(), endpoint, body, attempts: 0, status: "pending" };
    this.deliveries.set(delivery.id, delivery);
    void this.send(delivery);
    return delivery;
  }

  onChange(listener: (d: Delivery) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private async send(delivery: Delivery): Promise<void> {
    delivery.attempts++;
    const response = await fetch(delivery.endpoint, {
      method: "POST",
      body: delivery.body,
      headers: { "x-pigeon-signature": sign(this.config.secret, delivery.body) },
      signal: AbortSignal.timeout(this.config.timeoutMs),
    }).catch(() => undefined);
    delivery.status = response?.ok ? "delivered" : "dead";
    this.listeners.forEach((listener) => listener(delivery));
  }
}
EOF
	cat >src/signing.ts <<'EOF'
import { createHmac } from "node:crypto";

export function sign(secret: string, body: string): string {
  if (secret === "" || /\s$/.test(secret)) throw new Error("invalid signing secret");
  return "sha256=" + createHmac("sha256", secret).update(body).digest("hex");
}
EOF
	cat >src/dashboard/stream.ts <<'EOF'
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Queue } from "../deliveries/queue";

export function stream(queue: Queue, req: IncomingMessage, res: ServerResponse): void {
  res.writeHead(200, { "content-type": "text/event-stream" });
  const stop = queue.onChange((delivery) => {
    res.write(`data: ${JSON.stringify(delivery)}\n\n`);
  });
  req.on("close", stop);
}
EOF
	cat >scripts/notify.sh <<'EOF'
#!/bin/sh
# Tells the team's channel a release is out.
sleep 0.8
echo "announced pigeon $MINT_VERSION_TAG"
EOF
	chmod +x scripts/notify.sh
	cat >test/queue.test.ts <<'EOF'
import { describe, expect, it } from "vitest";
import { Queue } from "../src/deliveries/queue";

describe("Queue", () => {
  it("accepts a delivery as pending", () => {
    const queue = new Queue({ port: 0, secret: "s", timeoutMs: 10 });
    expect(queue.enqueue("http://localhost:1", "{}").status).toBe("pending");
  });
});
EOF
	mint_toml
	commit "2026-03-02T09:00:00Z" "feat: deliver signed webhooks from a queue"
	release 1.0.0 "2026-03-02T11:00:00Z"
}

v1_1_0() {
	cat >src/deliveries/batch.ts <<'EOF'
import type { Delivery } from "./queue";

// Deliveries bound for one endpoint within a window go out as one request.
export function batches(pending: Delivery[], max = 50): Delivery[][] {
  const byEndpoint = new Map<string, Delivery[]>();
  for (const delivery of pending) {
    const group = byEndpoint.get(delivery.endpoint) ?? [];
    group.push(delivery);
    byEndpoint.set(delivery.endpoint, group);
  }
  const out: Delivery[][] = [];
  for (const group of byEndpoint.values()) {
    for (let i = 0; i < group.length; i += max) out.push(group.slice(i, i + max));
  }
  return out;
}
EOF
	commit "2026-04-10T15:20:00Z" "feat: batch deliveries to the same endpoint"
	release 1.1.0 "2026-04-14T10:00:00Z"
}

v1_2_0() {
	cat >src/metrics.ts <<'EOF'
import type { Delivery } from "./deliveries/queue";

// Success rate and latency per endpoint, served at /metrics.
export class Metrics {
  private byEndpoint = new Map<string, { delivered: number; dead: number; totalMs: number }>();

  record(delivery: Delivery, ms: number): void {
    const m = this.byEndpoint.get(delivery.endpoint) ?? { delivered: 0, dead: 0, totalMs: 0 };
    if (delivery.status === "delivered") m.delivered++;
    if (delivery.status === "dead") m.dead++;
    m.totalMs += ms;
    this.byEndpoint.set(delivery.endpoint, m);
  }

  render(): string {
    return [...this.byEndpoint].map(([endpoint, m]) =>
      `pigeon_delivered{endpoint="${endpoint}"} ${m.delivered}\npigeon_dead{endpoint="${endpoint}"} ${m.dead}`,
    ).join("\n");
  }
}
EOF
	commit "2026-05-21T13:05:00Z" "feat: serve delivery metrics"
	release 1.2.0 "2026-05-26T10:00:00Z"
}

v1_2_1() {
	cat >src/dashboard/auth.ts <<'EOF'
import type { IncomingMessage } from "node:http";

// A session stays signed in across its refresh: the old token is honoured
// until the new one is first seen.
export function signedIn(req: IncomingMessage, tokens: Set<string>, previous: Set<string>): boolean {
  const token = String(req.headers.authorization ?? "").replace(/^Bearer /, "");
  return tokens.has(token) || previous.has(token);
}
EOF
	commit "2026-06-08T17:40:00Z" "fix(dashboard): keep the session across a token refresh"
	release 1.2.1 "2026-06-09T10:00:00Z"
}

v1_3_0() {
	cat >src/dashboard/filters.ts <<'EOF'
import type { Delivery } from "../deliveries/queue";

export interface Filter {
  endpoint?: string;
  status?: Delivery["status"];
}

export function matches(delivery: Delivery, filter: Filter): boolean {
  if (filter.endpoint && delivery.endpoint !== filter.endpoint) return false;
  if (filter.status && delivery.status !== filter.status) return false;
  return true;
}
EOF
	commit "2026-07-16T11:25:00Z" "feat(dashboard): filter deliveries by endpoint and status"
	release 1.3.0 "2026-07-21T10:00:00Z"
}

v1_3_2() {
	cat >src/deliveries/timeout.ts <<'EOF'
// Each endpoint gets its own concurrency slot, so one slow receiver can't hold
// up deliveries to the others.
export class Slots {
  private busy = new Set<string>();

  take(endpoint: string): boolean {
    if (this.busy.has(endpoint)) return false;
    this.busy.add(endpoint);
    return true;
  }

  free(endpoint: string): void {
    this.busy.delete(endpoint);
  }
}
EOF
	commit "2026-09-04T09:50:00Z" "fix: stop a slow endpoint holding up the others"
	release 1.3.2 "2026-09-08T10:00:00Z"
}

# The work since v1.3.2, all but the retry change commit -Ap records.
since_v1_3_2() {
	spec retry-backoff.md "Retrying failed deliveries" \
		"Problem" "A delivery that fails is marked dead at once. A receiver that is down for a minute loses every event sent to it in that minute, and its owner has no way to get them back short of asking the sender to emit them again. Support sees this every week, and it is the most common reason a team stops using pigeon." \
		"Proposal" "Retry a failed delivery on an exponential schedule: 10 seconds, then doubling each time, capped at 15 minutes between attempts, for up to an hour in all. A delivery is marked dead only once that hour has passed. A 4xx other than 408 and 429 is not retried, since sending the same request again won't change the answer." \
		"Jitter" "Each wait is jittered by up to a fifth either way, so the deliveries that failed together in one outage don't all come back at the same moment and knock the receiver over again as it recovers." \
		"Out of scope" "Per-endpoint schedules, and retrying across a restart of pigeon itself. Both are worth having and both need the queue to be durable first, which is its own piece of work."
	spec replay.md "Replaying a delivery by hand" \
		"Problem" "Once a delivery is dead it stays dead. Even with retries, a receiver that is down for longer than an hour loses events, and the only recovery today is to find them in the logs and post them again by hand, signature and all." \
		"Proposal" "A pigeon replay <id> command that sends a delivery again, exactly as it was first sent, with a fresh signature. Deliveries are kept for 30 days, so any delivery in that window can be replayed. Replaying a delivered one is allowed: the receiver is expected to be idempotent." \
		"Interface" "pigeon replay <id> prints the new attempt's status and exits 0 on a 2xx. pigeon replay --dead --since 2h replays every dead delivery in the window, oldest first, one at a time."
	spec dashboard-reconnect.md "The dashboard after a reconnect" \
		"Problem" "When the dashboard's event stream drops and reconnects, it is sent the deliveries it already holds, and draws them a second time. Users read it as pigeon sending the event twice and file it as a bug against the receiver." \
		"Proposal" "The stream carries each delivery's id and attempt, and the dashboard keeps the last of each it has drawn, so a delivery it already holds is updated in place rather than added again."
	spec signing-secrets.md "Signing secrets" \
		"Problem" "Secrets copied from a password manager often end in a newline. Since 1.0 pigeon rejects a secret with trailing whitespace, and the error doesn't say why, so the fix isn't obvious." \
		"Proposal" "Trim trailing whitespace from the secret when it is read, and reject only an empty one."
	spec storage.md "Keeping deliveries for 30 days" \
		"Problem" "Replay needs a delivery's body after it has been sent, and today pigeon forgets it at once." \
		"Proposal" "Keep each delivery's body, headers and attempts in an append-only log, one file a day, and drop a day's file once it is older than 30 days. A day of a busy installation is around 200 MB, so the log is compressed as each day closes." \
		"Rejected" "SQLite: it would serve, but pigeon has no other reason to carry it, and the log is read in order only."
	spec rollout.md "Rolling out retries" \
		"Plan" "Retries ship on by default in 1.4.0. Teams that need the old behaviour, a delivery dead on its first failure, set PIGEON_RETRY_FOR_MS=0." \
		"Risk" "A receiver that answers slowly and fails could now see six or seven attempts an hour where it saw one. The jitter and the cap keep that gentle, and the dashboard shows each attempt so its owner can see why."
	commit "2026-09-22T10:15:00Z" "docs: specs for retries, replay and the dashboard reconnect"

	cat >src/dashboard/stream.ts <<'EOF'
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Delivery, Queue } from "../deliveries/queue";

// A reconnecting dashboard sends the last event id it drew, and is sent only
// what changed after it, so no delivery is drawn twice.
export function stream(queue: Queue, req: IncomingMessage, res: ServerResponse): void {
  res.writeHead(200, { "content-type": "text/event-stream" });
  const lastSeen = String(req.headers["last-event-id"] ?? "");
  const send = (delivery: Delivery) => {
    const id = `${delivery.id}:${delivery.attempts}`;
    if (id <= lastSeen) return;
    res.write(`id: ${id}\ndata: ${JSON.stringify(delivery)}\n\n`);
  };
  const stop = queue.onChange(send);
  req.on("close", stop);
}
EOF
	commit "2026-09-30T16:45:00Z" "fix(dashboard): draw each delivery once after a reconnect"

	cat >src/signing.ts <<'EOF'
import { createHmac } from "node:crypto";

// A secret pasted from a password manager often ends in a newline: it is
// trimmed, and only an empty secret is refused.
export function sign(secret: string, body: string): string {
  const key = secret.trimEnd();
  if (key === "") throw new Error("PIGEON_SECRET is empty");
  return "sha256=" + createHmac("sha256", key).update(body).digest("hex");
}
EOF
	commit "2026-10-06T12:10:00Z" "fix(signing): accept secrets with a trailing newline"

	cat >src/cli/replay.ts <<'EOF'
import { loadConfig } from "../config";
import { Queue } from "../deliveries/queue";
import { Store } from "../deliveries/store";

// pigeon replay <id>: sends a kept delivery again, as it was first sent, with
// a fresh signature. Deliveries are kept for 30 days.
export async function replay(argv: string[], env: Record<string, string | undefined>): Promise<number> {
  const [id] = argv;
  if (!id) {
    console.error("usage: pigeon replay <id>");
    return 2;
  }
  const store = new Store(env.PIGEON_DATA ?? ".pigeon");
  const kept = await store.find(id);
  if (!kept) {
    console.error(`no delivery ${id} in the last 30 days`);
    return 1;
  }
  const queue = new Queue(loadConfig(env));
  const delivery = queue.enqueue(kept.endpoint, kept.body);
  const settled = await new Promise<string>((resolve) => {
    const stop = queue.onChange((d) => {
      if (d.id !== delivery.id || d.status === "pending") return;
      stop();
      resolve(d.status);
    });
  });
  console.log(`${id} → ${kept.endpoint}: ${settled}`);
  return settled === "delivered" ? 0 : 1;
}
EOF
	cat >src/deliveries/store.ts <<'EOF'
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";

export interface Kept {
  id: string;
  endpoint: string;
  body: string;
  at: string;
}

const keepDays = 30;

// Deliveries are kept in one log a day, newest first when read back.
export class Store {
  constructor(private dir: string) {}

  async find(id: string): Promise<Kept | undefined> {
    const days = (await readdir(this.dir)).sort().reverse().slice(0, keepDays);
    for (const day of days) {
      const lines = (await readFile(join(this.dir, day), "utf8")).split("\n");
      for (const line of lines) {
        if (!line.includes(id)) continue;
        const kept = JSON.parse(line) as Kept;
        if (kept.id === id) return kept;
      }
    }
    return undefined;
  }
}
EOF
	commit "2026-10-13T14:00:00Z" "feat(cli): add pigeon replay"
}

# The retry change: four files, the one commit -Ap writes the message for.
retry_change() {
	cat >src/deliveries/retry.ts <<'EOF'
// A failed delivery is tried again on an exponential schedule, jittered so a
// receiver coming back from an outage isn't met by every retry at once.
export interface Schedule {
  firstMs: number;
  capMs: number;
  forMs: number;
}

export const defaultSchedule: Schedule = {
  firstMs: 10_000,
  capMs: 15 * 60_000,
  forMs: 60 * 60_000,
};

// The wait before attempt n (from 1), or undefined once the schedule's time
// is spent.
export function nextWait(schedule: Schedule, attempt: number, elapsedMs: number): number | undefined {
  const base = Math.min(schedule.firstMs * 2 ** (attempt - 1), schedule.capMs);
  const wait = base * (0.8 + Math.random() * 0.4);
  return elapsedMs + wait > schedule.forMs ? undefined : wait;
}

// A 4xx says sending the same request again won't help, except a timeout or a
// rate limit.
export function retryable(status: number | undefined): boolean {
  if (status === undefined) return true;
  if (status === 408 || status === 429) return true;
  return status >= 500;
}
EOF
	cat >src/deliveries/queue.ts <<'EOF'
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Config } from "../config";
import { sign } from "../signing";
import { nextWait, retryable } from "./retry";

export interface Delivery {
  id: string;
  endpoint: string;
  body: string;
  attempts: number;
  firstAt: number;
  status: "pending" | "retrying" | "delivered" | "dead";
}

export class Queue {
  private deliveries = new Map<string, Delivery>();
  private listeners = new Set<(d: Delivery) => void>();

  constructor(private config: Config) {}

  accept(req: IncomingMessage, res: ServerResponse): void {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      const delivery = this.enqueue(String(req.headers["x-endpoint"]), body);
      res.writeHead(202, { "content-type": "application/json" });
      res.end(JSON.stringify({ id: delivery.id }));
    });
  }

  enqueue(endpoint: string, body: string): Delivery {
    const delivery: Delivery = { id: crypto.randomUUID(), endpoint, body, attempts: 0, firstAt: Date.now(), status: "pending" };
    this.deliveries.set(delivery.id, delivery);
    void this.send(delivery);
    return delivery;
  }

  onChange(listener: (d: Delivery) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private async send(delivery: Delivery): Promise<void> {
    delivery.attempts++;
    const response = await fetch(delivery.endpoint, {
      method: "POST",
      body: delivery.body,
      headers: { "x-pigeon-signature": sign(this.config.secret, delivery.body) },
      signal: AbortSignal.timeout(this.config.timeoutMs),
    }).catch(() => undefined);
    if (response?.ok) {
      delivery.status = "delivered";
    } else {
      const wait = retryable(response?.status)
        ? nextWait(this.config.retry, delivery.attempts, Date.now() - delivery.firstAt)
        : undefined;
      delivery.status = wait === undefined ? "dead" : "retrying";
      if (wait !== undefined) setTimeout(() => void this.send(delivery), wait);
    }
    this.listeners.forEach((listener) => listener(delivery));
  }
}
EOF
	cat >src/config.ts <<'EOF'
import { defaultSchedule, type Schedule } from "./deliveries/retry";

export interface Config {
  port: number;
  secret: string;
  timeoutMs: number;
  retry: Schedule;
}

export function loadConfig(env: Record<string, string | undefined>): Config {
  return {
    port: Number(env.PIGEON_PORT ?? 8080),
    secret: env.PIGEON_SECRET ?? "",
    timeoutMs: Number(env.PIGEON_TIMEOUT_MS ?? 10_000),
    retry: { ...defaultSchedule, forMs: Number(env.PIGEON_RETRY_FOR_MS ?? defaultSchedule.forMs) },
  };
}
EOF
	cat >test/retry.test.ts <<'EOF'
import { describe, expect, it } from "vitest";
import { defaultSchedule, nextWait, retryable } from "../src/deliveries/retry";

describe("retry", () => {
  it("waits longer after each attempt, up to the cap", () => {
    const first = nextWait(defaultSchedule, 1, 0)!;
    const fifth = nextWait(defaultSchedule, 5, 0)!;
    expect(first).toBeLessThan(fifth);
    expect(nextWait(defaultSchedule, 20, 0)).toBeLessThanOrEqual(defaultSchedule.capMs * 1.2);
  });

  it("gives up once the hour is spent", () => {
    expect(nextWait(defaultSchedule, 3, defaultSchedule.forMs)).toBeUndefined();
  });

  it("doesn't retry a request the receiver refused", () => {
    expect(retryable(400)).toBe(false);
    expect(retryable(429)).toBe(true);
    expect(retryable(503)).toBe(true);
  });
});
EOF
}

# pigeon released with mint to v1.3.2, and pushed.
history() {
	git init -q --bare -b main "$origin"
	mkdir -p "$repo"
	cd "$repo"
	git init -q -b main
	git remote add origin https://github.com/acme/pigeon
	v1_0_0
	git push -q -u origin main
	git remote set-head origin main
	v1_1_0
	v1_2_0
	v1_2_1
	v1_3_0
	v1_3_2
	since_v1_3_2
}

# The retry change committed with the message commit -Ap writes for it.
retry_commit() {
	retry_change
	commit "2026-10-20T15:30:00Z" "$(head -n1 "$world/ai/commit.txt")"
}

# v1.4.0 released as the tour releases it: the pre_tag artifacts, then
# docs/specs/** answered Exclude at the exclusion check's pause, its line
# placed where mint's config.AppendReleaseGlobs places it, after the
# [release] table's last key, then the release.
v1_4_0() {
	local date="2026-10-21T14:30:00Z"
	artifacts 1.4.0 "$date"
	awk '{ print } /^version_pattern/ { print "diff_exclude = [\"docs/specs/**\"]" }' .mint.toml >.mint.toml.new
	mv .mint.toml.new .mint.toml
	commit "$date" "chore(release): release-notes path answers for v1.4.0"
	record 1.4.0 "$date" "$world/ai/v1.4.0.md"
}

# A world built afresh. mint is built with the user's own HOME, where Go keeps
# its caches; the history is made with the world's, so the user's git ignore
# and attributes files can't change what is committed.
fresh() {
	[ ! -e "$world" ] || [ -z "$(ls -A "$world")" ] || [ -f "$world/profile.sh" ] || {
		echo "$world isn't a demo world; set DEMO_WORLD elsewhere" >&2
		exit 1
	}
	rm -rf "$world"
	gitconfig
	canned
	shims
	profile
	export HOME="$home"
}

case "${1:-}" in
release)
	fresh
	history
	retry_commit
	git push -q origin main
	;;
commit)
	fresh
	history
	git push -q origin main
	retry_change
	;;
regenerate)
	fresh
	history
	retry_commit
	v1_4_0
	;;
init)
	fresh
	mkdir -p "$repo"
	cd "$repo"
	git init -q -b main
	;;
*)
	echo "usage: demo/world.sh release | commit | regenerate | init" >&2
	exit 2
	;;
esac
