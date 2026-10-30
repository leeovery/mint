# A release's every question, a screenshot at each state: the version asked,
# the exclusion check's two questions inside the check, the first answered
# exclude, the second keep, the notes running once both are answered, the
# review with its keys, edited with e and back, then aborted with n, the
# unwind and the close.
#
#   demo/review.sh release

Output "demo/out/review/release/release.gif"
Source "demo/look.tape"
# 60 rows: Wait+Screen reads the screen's first rows, so the run fits.
Set Height 2284

Hide
Type 'demo/world.sh release && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh" && export DEMO_AI_CHECK_TESTS=1 VISUAL=demo-edit EDITOR=demo-edit' Enter
Wait
Show

Type "mint release -m"
Enter
Wait+Screen /Release v1\.3\.2/
Sleep 500ms
Screenshot "demo/out/review/release/01-version.png"
Sleep 1s
Type "y"
Wait+Screen /Exclude docs\/specs\/\*\* from the notes/
Sleep 500ms
Screenshot "demo/out/review/release/02-exclude-specs.png"
Sleep 1s
Type "e"
Wait+Screen /Exclude test\/ from the notes/
Sleep 500ms
Screenshot "demo/out/review/release/03-exclude-tests.png"
Sleep 1s
Type "k"
Wait+Screen /parts done/
Sleep 1500ms
Screenshot "demo/out/review/release/04-notes-running.png"
Sleep 1s
Wait+Screen /Release with these notes/
Sleep 500ms
Screenshot "demo/out/review/release/05-review.png"
Sleep 1s
Type "e"
Wait+Screen /then give up/
Sleep 500ms
Screenshot "demo/out/review/release/06-review-edited.png"
Sleep 1s
Type "n"
Wait
Wait+Screen /nothing pushed/
Sleep 500ms
Screenshot "demo/out/review/release/07-aborted.png"
Sleep 1s
