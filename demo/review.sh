#!/usr/bin/env bash
# Record the review tapes: each of mint's questions caught in a screenshot at
# every state it passes through, for a change to the questions to be looked at
# before review. They make no art: their screenshots go to demo/out/review/,
# never committed, a folder a tape. vhs is given a TMPDIR in demo/out, as
# record.sh gives it, and a tape's old screenshots are deleted first.
#
# Usage:  demo/review.sh                    every review tape
#         demo/review.sh release commit     the review tapes named
set -euo pipefail

cd "$(dirname "$0")/.."

names=("$@")
if [ ${#names[@]} -eq 0 ]; then
	for tape in demo/review/*.tape; do
		names+=("$(basename "$tape" .tape)")
	done
fi

mkdir -p demo/out/.vhs
for name in "${names[@]}"; do
	[ -f "demo/review/$name.tape" ] || {
		echo "no tape demo/review/$name.tape" >&2
		exit 1
	}
	rm -rf "demo/out/review/$name"
	mkdir -p "demo/out/review/$name"
	TMPDIR="$PWD/demo/out/.vhs" vhs --quiet "demo/review/$name.tape"
	shots=(demo/out/review/"$name"/*.png)
	[ -e "${shots[0]}" ] || {
		echo "demo/review/$name.tape took no screenshot" >&2
		exit 1
	}
	printf '%s\n' "${shots[@]}"
done
