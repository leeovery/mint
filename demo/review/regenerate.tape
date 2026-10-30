# Regenerate's typed questions: Source, its choices words, read as a typed
# line and answered tag; Target, Enter taking its default; then the review.
#
#   demo/review.sh regenerate

Output "demo/out/review/regenerate/regenerate.gif"
Source "demo/look.tape"
Set Height 1100

Hide
Type 'demo/world.sh regenerate && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Type "mint release regenerate v1.4.0"
Enter
Wait+Screen /Source\?/
Sleep 500ms
Screenshot "demo/out/review/regenerate/01-source.png"
Sleep 1s
Type "tag"
Sleep 300ms
Screenshot "demo/out/review/regenerate/02-source-typed.png"
Sleep 1s
Enter
Wait+Screen /Target\?/
Sleep 500ms
Screenshot "demo/out/review/regenerate/03-target.png"
Sleep 1s
Enter
Wait+Screen /Use these notes/
Sleep 500ms
Screenshot "demo/out/review/regenerate/04-review.png"
Sleep 1s
Type "y"
Wait
Wait+Screen /regenerated/
Sleep 500ms
Screenshot "demo/out/review/regenerate/05-regenerated.png"
Sleep 1s
