# A dry run's questions, shown where a real run asks them and skipped: the
# exclusion check's two inside the check, and the review's after the notes.
#
#   demo/review.sh dry-run

Output "demo/out/review/dry-run/dry-run.gif"
Source "demo/look.tape"
Set Height 2284

Hide
Type 'demo/world.sh release && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh" && export DEMO_AI_CHECK_TESTS=1' Enter
Wait
Show

Type "mint release -m --dry-run"
Enter
Wait
Wait+Screen /dry run · nothing changed/
Sleep 500ms
Screenshot "demo/out/review/dry-run/01-dry-run.png"
Sleep 1s
