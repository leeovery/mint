# A still for the dry run: a minor release run up to its review without
# changing the repo. The hooks are skipped, the exclusion check's suggestion
# is shown without being asked, and the notes, written in parts, are printed
# with the plan.
#
# Record it, and make demo/out/art/dry-run.webp:
#   demo/record.sh dry-run

Output "demo/out/dry-run/"
Source "demo/look.tape"
# 32 rows: room for what it shows.
Set Height 1248

Hide
Type "demo/world.sh release && source demo/out/world/profile.sh" Enter
Wait
Show

Type "mint release -m --dry-run"
Enter
Wait
Sleep 600ms
