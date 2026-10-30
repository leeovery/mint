# A commit's review: the message with its keys, then accepted.
#
#   demo/review.sh commit

Output "demo/out/review/commit/commit.gif"
Source "demo/look.tape"
Set Height 1100

Hide
Type 'demo/world.sh commit && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Type "mint commit -Ap"
Enter
Wait+Screen /Commit this message/
Sleep 500ms
Screenshot "demo/out/review/commit/01-review.png"
Sleep 1s
Type "y"
Wait
Wait+Screen /pushed/
Sleep 500ms
Screenshot "demo/out/review/commit/02-committed.png"
Sleep 1s
