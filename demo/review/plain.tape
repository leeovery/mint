# A release in plain, asked at a terminal: each question one line, its key
# hint marking the default, each answered with a typed line. The version, the
# exclusion check's two questions, answered e then k, then the review,
# answered n.
#
#   demo/review.sh plain

Output "demo/out/review/plain/plain.gif"
Source "demo/look.tape"
Set Height 2284

Hide
Type 'demo/world.sh release && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh" && export DEMO_AI_CHECK_TESTS=1' Enter
Wait
Show

Type "mint release -m --plain"
Enter
Wait+Screen /\[Y\/n\]/
Sleep 500ms
Screenshot "demo/out/review/plain/01-version.png"
Sleep 1s
Type "y"
Enter
Wait+Screen /Exclude docs\/specs\/\*\* from the notes.*\[e\/K\]/
Sleep 500ms
Screenshot "demo/out/review/plain/02-exclude-specs.png"
Sleep 1s
Type "e"
Enter
Wait+Screen /Exclude test\/ from the notes/
Sleep 500ms
Screenshot "demo/out/review/plain/03-exclude-tests.png"
Sleep 1s
Type "k"
Enter
Wait+Screen /Release with these notes\? \[Y\/n\/e\/r\]/
Sleep 500ms
Screenshot "demo/out/review/plain/04-review.png"
Sleep 1s
Type "n"
Enter
Wait
Sleep 500ms
Screenshot "demo/out/review/plain/05-aborted.png"
Sleep 1s
