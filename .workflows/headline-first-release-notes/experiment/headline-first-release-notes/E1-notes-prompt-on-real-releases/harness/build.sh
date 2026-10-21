#!/usr/bin/env bash
# Builds e1harness into the mint module through a go build overlay, so the harness
# source stays in the experiment record and imports mint's internal packages at the
# checked-out SHA. Prints the binary path and the SHA it was built against.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mint="$(git -C "$here" rev-parse --show-toplevel)"
cache="$mint/.workflows/.cache/headline-first-release-notes/experiment/headline-first-release-notes"
mkdir -p "$cache"

cat > "$cache/overlay.json" <<EOF
{"Replace": {"$mint/cmd/e1harness/main.go": "$here/main.go"}}
EOF

(cd "$mint" && go build -overlay "$cache/overlay.json" -o "$cache/e1harness" ./cmd/e1harness)
echo "binary: $cache/e1harness"
echo "mint sha: $(git -C "$mint" rev-parse --short HEAD)"
