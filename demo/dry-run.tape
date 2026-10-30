# A still for the dry run: a minor release run up to its review without
# changing the repo. The hooks are skipped, the exclusion check's suggestion
# is shown without being asked, and the notes, written in parts, are printed
# with the plan.
#
# Record it, and make art/dry-run.webp:
#   demo/record.sh dry-run

Output "demo/out/dry-run/"
Source "demo/look.tape"
# 36 rows: room for what it shows.
Set Height 1396

Hide
Type 'demo/world.sh release && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Type "mint release -m --dry-run"
Enter
Wait
# The line only a dry run that reaches its end prints.
Wait+Screen /dry run · nothing changed/
Sleep 600ms
