#!/usr/bin/env bash
# Record the README's demos, and make their art: the tapes named, or every
# one, through vhs, then finalize.sh. Run it from anywhere; it works from the
# repository's root, where the tapes' paths start. Each tape builds the world
# it records in (demo/world.sh), mint included, from this checkout.
#
# vhs records a tape's frames into a directory of its own under $TMPDIR, then
# moves it to the tape's Output. Where $TMPDIR is on another volume than the
# repository, the move fails without a word and no frames come, so vhs is
# given a TMPDIR in demo/out. A tape's old frames are deleted first, so a
# recording that fails without a word can't leave them to be taken for new.
#
# Usage:  demo/record.sh                every tape
#         demo/record.sh tour commit    the tapes named
set -euo pipefail

cd "$(dirname "$0")/.."
stills=" failed dry-run commit regenerate init "

names=("$@")
if [ ${#names[@]} -eq 0 ]; then
	for tape in demo/*.tape; do
		name="$(basename "$tape" .tape)"
		[ "$name" = look ] || names+=("$name")
	done
fi

mkdir -p demo/out/.vhs
for name in "${names[@]}"; do
	[ -f "demo/$name.tape" ] && [ "$name" != look ] || {
		echo "no tape demo/$name.tape" >&2
		exit 1
	}
	rm -rf "demo/out/$name"
	TMPDIR="$PWD/demo/out/.vhs" vhs --quiet "demo/$name.tape"
	case "$stills" in
	*" $name "*) demo/finalize.sh --still "$name" ;;
	*) demo/finalize.sh "$name" ;;
	esac
done
