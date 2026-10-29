# A still for when the notes fail: an unattended minor release whose notes
# are written in parts, part 3 failing on a spent usage limit. The release
# stops on its notes with the AI's output, the saved part notes and the next
# moves, prints the exclusion check's suggestion, and unwinds the pre_tag
# artifacts commit.
#
# Record it, and make demo/out/art/failed.webp:
#   demo/record.sh failed

Output "demo/out/failed/"
Source "demo/look.tape"
# 30 rows: room for what it shows.
Set Height 1174

Hide
Type "demo/world.sh release && source demo/out/world/profile.sh && export DEMO_AI_FAIL_PART=3" Enter
Wait
Show

Type "mint release -m -y"
Enter
Wait
Sleep 600ms
