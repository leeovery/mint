# The README's lead: a release from start to finish. pigeon goes v1.3.2 →
# v1.4.0: the version confirmed, preflight, the pre_tag build, the exclusion
# check's pause on docs/specs/**, answered exclude, the notes written in 4
# parts and merged, their review, answered release, then the record, the
# push, the GitHub release and the post_release hook.
#
# Record it, and make demo/out/art/tour.webp and tour.mp4:
#   demo/record.sh tour

Output "demo/out/tour/"
Source "demo/look.tape"
# 44 rows: the whole run, from the command to the prompt after it, and an
# even number, as the MP4's encoder needs an even height. vhs's Wait+Screen
# reads the screen's first rows of output, not the ones it shows once it has
# scrolled, so a tape that waits on the screen fits its run.
Set Height 1692

Hide
Type 'demo/world.sh release && source "${DEMO_WORLD:-/tmp/mint-demo}/profile.sh"' Enter
Wait
Show

Sleep 600ms
Type "mint release -m"
Sleep 400ms
Enter
Wait+Screen /Release v1\.3\.2/
Sleep 1500ms
Type "y"
Wait+Screen /Exclude it from the notes/
Sleep 2500ms
Type "e"
Wait+Screen /Release with these notes/
Sleep 3500ms
Type "y"
Wait
# The line only a release that lands prints.
Wait+Screen /v1\.4\.0 +released/
Sleep 4s
