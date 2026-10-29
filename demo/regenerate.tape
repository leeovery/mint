# A still for regenerate's section: fresh notes for every version, oldest
# first, each written to its GitHub release, then CHANGELOG.md rebuilt,
# committed and pushed. v1.2.0's notes come back empty, so it is skipped and
# keeps the notes it had.
#
# Record it, and make demo/out/art/regenerate.webp:
#   demo/record.sh regenerate

Output "demo/out/regenerate/"
Source "demo/look.tape"
# 55 rows: room for what it shows.
Set Height 2099

Hide
Type 'demo/world.sh regenerate && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Type "mint release regenerate --all --target both -y"
Enter
Wait
# The end only a batch with its one skip lands at.
Wait+Screen /done +6 of 7 versions regenerated · 1 skipped/
Sleep 600ms
