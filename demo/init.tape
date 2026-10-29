# A still for init's section: mint init in an empty repo, creating
# .mint.toml and the release shim.
#
# Record it, and make demo/out/art/init.webp:
#   demo/record.sh init

Output "demo/out/init/"
Source "demo/look.tape"
# 6 rows: room for what it shows.
Set Height 286

Hide
Type "demo/world.sh init && source demo/out/world/profile.sh" Enter
Wait
Show

Type "mint init"
Enter
Wait
Sleep 600ms
