# A still for commit's section: commit -Ap over the retry change, four files
# not yet staged. The message the AI wrote, accepted, then everything staged,
# committed and pushed.
#
# Record it, and make demo/out/art/commit.webp:
#   demo/record.sh commit

Output "demo/out/commit/"
Source "demo/look.tape"
# 22 rows: room for what it shows, the message and the stages to come under its
# question included, so nothing scrolls away.
Set Height 878

Hide
Type 'demo/world.sh commit && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Type "mint commit -Ap"
Enter
Wait+Screen /Commit this message/
Sleep 600ms
Type "y"
Wait
# The end only a commit that lands and pushes lands at.
Wait+Screen /main +[0-9a-f]{7} · pushed/
Sleep 600ms
