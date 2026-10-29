# A still for commit's section: commit -Ap over the retry change, four files
# not yet staged. The message the AI wrote, accepted, then everything staged,
# committed and pushed.
#
# Record it, and make demo/out/art/commit.webp:
#   demo/record.sh commit

Output "demo/out/commit/"
Source "demo/look.tape"
# 20 rows: room for what it shows.
Set Height 804

Hide
Type "demo/world.sh commit && source demo/out/world/profile.sh" Enter
Wait
Show

Type "mint commit -Ap"
Enter
Wait+Screen /Commit this message/
Sleep 600ms
Type "y"
Wait
Sleep 600ms
