# EssentialsX for golem

An unofficial port of [EssentialsX](https://essentialsx.net/) to
[golem](https://github.com/golem-mc/golem), the Minecraft: Java Edition server
written in Go. It keeps EssentialsX's commands, permission nodes, messages and
configuration files, so anyone who knows EssentialsX on Bukkit or Paper will
find the same plugin here.

## Install

Add the plugins with
[`golem-launcher`](https://github.com/golem-mc/golem#installing-plugins), one
line each, then start the server:

```bash
./golem-launcher plugin add github.com/golem-mc/essentials            # core: required
./golem-launcher plugin add github.com/golem-mc/essentials/spawn      # optional: /spawn, /setspawn, first-join spawn
./golem-launcher plugin add github.com/golem-mc/essentials/chat       # optional: chat formats, local chat, shouting
./golem-launcher plugin add github.com/golem-mc/essentials/protect    # optional: switches off fire spread and the dragon's block damage by default; more in `protect:` in `config.yml`
./golem-launcher plugin add github.com/golem-mc/essentials/antibuild  # optional: build protection
./golem-launcher
```

The first start builds a server with the plugins in it, which takes a few
minutes. To update, run `./golem-launcher update` and restart. The four
optional modules need the core plugin.

## Getting started

1. Start the server once. The plugin writes its files to `plugins/essentials/`.
2. Make yourself an operator (`op <name>` in the console). Operators have every
   node by default; for other players see [Permissions](#permissions).
3. Set a home and a warp, and try them:

   ```
   /sethome base
   /setwarp market
   /home base
   /warp market
   ```

4. Make a kit and hand it out: `/createkit starter 0`, then `/kit starter`.
5. Check your balance and pay a friend: `/balance`, `/pay Alex 50`.
6. Change `config.yml` and run `/essentials reload`.

## Commands

Every command works as in EssentialsX; usage is shown in brackets (`[optional]`,
`<required>`). A command's permission is `essentials.<command>`, and most have
extra nodes for their options, such as `essentials.tp.others` or
`essentials.warps.<name>`; the full list is in
[EssentialsX's permission reference](https://essentialsx.net/permissions.html).
Vanilla's version of a name Essentials also provides (`/give`, `/tp`, `/ban`,
`/time`, ...) is replaced by Essentials'; it stays reachable as
`/minecraft:give`, or put the name in `disabled-commands` to give it back.
Every command also answers to `/essentials:<name>`.

### Homes, warps and teleports

| Command | What it does | Permission |
|---|---|---|
| `/home [player:][name]` | Teleport to your home (also `/homes`) | `essentials.home` |
| `/sethome [[player:]name]` | Set your home to your current location (also `/createhome`) | `essentials.sethome` |
| `/delhome [player:]<name>` | Removes a home (also `/rmhome`, `/remhome`) | `essentials.delhome` |
| `/renamehome <[player:]name> <new name>` | Renames a home | `essentials.renamehome` |
| `/warp <pagenumber\|warp> [player]` | List all warps or warp to the specified location (also `/warps`) | `essentials.warp` |
| `/setwarp <warp>` | Creates a new warp (also `/createwarp`) | `essentials.setwarp` |
| `/delwarp <warp>` | Deletes the specified warp (also `/rmwarp`, `/remwarp`) | `essentials.delwarp` |
| `/warpinfo <warp>` | Finds location information for a specified warp | `essentials.warpinfo` |
| `/back [player]` | Teleports you to your location prior to tp/spawn/warp (also `/return`) | `essentials.back` |
| `/tpa <player>` | Request to teleport to the specified player (also `/call`, `/tpask`) | `essentials.tpa` |
| `/tpahere <player>` | Request that the specified player teleport to you | `essentials.tpahere` |
| `/tpaccept [player\|*]` | Accepts teleport requests (also `/tpyes`) | `essentials.tpaccept` |
| `/tpdeny [player\|*]` | Rejects teleport requests (also `/tpno`) | `essentials.tpdeny` |
| `/tpacancel [player]` | Cancel all outstanding teleport requests. Specify [player] to cancel requests with them | `essentials.tpacancel` |
| `/tpaall <player>` | Requests all players online to teleport to you | `essentials.tpaall` |
| `/tpauto [player]` | Automatically accept teleportation requests | `essentials.tpauto` |
| `/tptoggle [player] [on\|off]` | Blocks all forms of teleportation | `essentials.tptoggle` |
| `/tp <player> [otherplayer]` | Teleport to a player (also `/tele`, `/tp2p`, `/teleport`) | `essentials.tp` |
| `/tphere <player>` | Teleport a player to you (also `/s`) | `essentials.tphere` |
| `/tpall [player]` | Teleport all online players to another player | `essentials.tpall` |
| `/tppos <x> <y> <z> [yaw] [pitch] [world]` | Teleport to coordinates | `essentials.tppos` |
| `/tpo <player> [otherplayer]` | Teleport override for tptoggle | `essentials.tpo` |
| `/tpohere <player>` | Teleport here override for tptoggle | `essentials.tpohere` |
| `/tpoffline <player>` | Teleport to a player's last known logout location (also `/otp`, `/tpoff`, `/offlinetp`) | `essentials.tpoffline` |
| `/tpr` | Teleport randomly (also `/tprandom`) | `essentials.tpr` |
| `/settpr [center\|minrange\|maxrange] [value]` | Set the random teleport location and parameters (also `/settprandom`) | `essentials.settpr` |
| `/top` | Teleport to the highest block at your current position | `essentials.top` |
| `/bottom` | Teleport to the lowest block at your current position | `essentials.bottom` |
| `/depth` | States current depth, relative to sea level (also `/height`) | `essentials.depth` |
| `/jump` | Jumps to the nearest block in the line of sight (also `/j`, `/jumpto`) | `essentials.jump` |
| `/compass` | Describes your current bearing (also `/direction`) | `essentials.compass` |
| `/getpos [player]` | Get your current coordinates or those of a player (also `/coords`, `/getloc`, `/position`, `/whereami`) | `essentials.getpos` |
| `/world [world]` | Switch between worlds | `essentials.world` |
| `/spawn [player]` | Teleports to the spawn point | `essentials.spawn` |
| `/setspawn <group>` | Sets the spawn point to your current position | `essentials.setspawn` |

### Economy

| Command | What it does | Permission |
|---|---|---|
| `/balance [player]` | States the current balance of a player (also `/bal`, `/money`) | `essentials.balance` |
| `/balancetop [page]` | Gets the top balance values (also `/baltop`) | `essentials.balancetop` |
| `/pay <player> <amount>` | Pays another player from your balance | `essentials.pay` |
| `/paytoggle [player]` | Toggles whether you are accepting payments (also `/payon`, `/payoff`) | `essentials.paytoggle` |
| `/payconfirmtoggle` | Toggles whether you are prompted to confirm payments (also `/payconfirm`, `/payconfirmon`, `/payconfirmoff`) | `essentials.payconfirmtoggle` |
| `/eco <give\|take\|set\|reset> <player> <amount>` | Manages the server economy | `essentials.eco` |
| `/sell <<itemname>\|<id>\|hand\|inventory\|blocks> [amount]` | Sells the item currently in your hand | `essentials.sell` |
| `/worth <<itemname>\|<id>\|hand\|inventory\|blocks> [-][amount]` | Calculates the worth of items in hand or as specified (also `/price`) | `essentials.worth` |
| `/setworth [itemname\|id] <price>` | Set the sell value of an item | `essentials.setworth` |

### Kits

| Command | What it does | Permission |
|---|---|---|
| `/kit [kit] [player]` | Obtains the specified kit or views all available kits (also `/kits`) | `essentials.kit` |
| `/kitreset <kit> [player]` | Resets the cooldown on the specified kit (also `/kitr`, `/resetkit`) | `essentials.kitreset` |
| `/showkit <kitname>` | Show contents of a kit (also `/preview`, `/kitshow`, `/kitpreview`) | `essentials.showkit` |
| `/createkit <kitname> <delay>` | Create a kit in game! (also `/kc`, `/ck`, `/createk`, `/kitcreate`) | `essentials.createkit` |
| `/delkit <kit>` | Deletes the specified kit (also `/rmkit`, `/remkit`, `/deletekit`) | `essentials.delkit` |

### Messaging and chat

| Command | What it does | Permission |
|---|---|---|
| `/msg <to> <message>` | Sends a private message to the specified player (also `/w`, `/m`, `/t`, `/pm`) | `essentials.msg` |
| `/r <message>` | Quickly reply to the last player to message you (also `/reply`) | `essentials.r` |
| `/msgtoggle [player] [on\|off]` | Blocks receiving all private messages | `essentials.msgtoggle` |
| `/rtoggle [player] [on\|off]` | Change whether the recipient of the reply is last recipient or last sender (also `/replytoggle`) | `essentials.rtoggle` |
| `/socialspy [player] [on\|off]` | Toggles if you can see msg/mail commands in chat | `essentials.socialspy` |
| `/ignore <player>` | Ignore or unignore other players (also `/unignore`, `/rmignore`, `/delignore`, `/remignore`) | `essentials.ignore` |
| `/mail [read\|clear\|clear [number]\|clear <player> [number]\|send [to] [message]\|sendtemp [to] [expire time] [message]\|sendall [message]]` | Manages inter-player, intra-server mail (also `/memo`) | `essentials.mail` |
| `/nick [player] <nickname\|off>` | Change your nickname or that of another player (also `/nickname`) | `essentials.nick` |
| `/realname <nickname>` | Displays the username of a user based on nick | `essentials.realname` |
| `/me <description>` | Describes an action in the context of the player (also `/action`, `/describe`) | `essentials.me` |
| `/broadcast <msg>` | Broadcasts a message to the entire server (also `/bc`, `/bcast`, `/shout`) | `essentials.broadcast` |
| `/broadcastworld <world> <msg>` | Broadcasts a message to a world (also `/bcw`, `/bcastw`, `/shoutworld`) | `essentials.broadcastworld` |
| `/helpop <message>` | Message online admins (also `/ac`, `/amsg`) | `essentials.helpop` |
| `/toggleshout [player] [on\|off]` | Toggles whether you are talking in shout mode | `essentials.toggleshout` |

### Moderation and jails

| Command | What it does | Permission |
|---|---|---|
| `/ban <player> [reason]` | Bans a player | `essentials.ban` |
| `/unban <player>` | Unbans the specified player (also `/pardon`) | `essentials.unban` |
| `/banip <address> [reason]` | Bans an IP address | `essentials.banip` |
| `/unbanip <address>` | Unbans the specified IP address (also `/pardonip`) | `essentials.unbanip` |
| `/tempban <playername> <datediff> [reason]` | Temporary ban a user | `essentials.tempban` |
| `/tempbanip <playername> <datediff> [reason]` | Temporarily ban an IP Address | `essentials.tempbanip` |
| `/kick <player> [reason]` | Kicks a specified player with a reason | `essentials.kick` |
| `/kickall [reason]` | Kicks all players off the server except the issuer | `essentials.kickall` |
| `/mute <player> [datediff] [reason]` | Mutes or unmutes a player (also `/unmute`, `/silence`) | `essentials.mute` |
| `/setjail <jailname>` | Creates a jail where you specified named [jailname] (also `/createjail`) | `essentials.setjail` |
| `/deljail <jailname>` | Removes a jail (also `/rmjail`, `/remjail`) | `essentials.deljail` |
| `/jails` | List all jails | `essentials.jails` |
| `/togglejail <player> <jailname> [datediff]` | Jails/Unjails a player, TPs them to the jail specified (also `/jail`, `/tjail`, `/unjail`) | `essentials.togglejail` |
| `/jailedplayers` | List all jailed players | `essentials.jailedplayers` |
| `/sudo <player> <command [args]>` | Make another user perform a command | `essentials.sudo` |

### Player state

| Command | What it does | Permission |
|---|---|---|
| `/god [player] [on\|off]` | Enables your godly powers (also `/tgm`, `/godmode`) | `essentials.god` |
| `/vanish [player] [on\|off]` | Hide yourself from other players (also `/v`) | `essentials.vanish` |
| `/afk [player/message...]` | Marks you as away-from-keyboard (also `/away`) | `essentials.afk` |
| `/fly [player] [on\|off]` | Take off, and soar! | `essentials.fly` |
| `/gamemode <survival\|creative\|adventure\|spectator> [player]` | Change player gamemode (also `/gm`, `/sp`, `/gma`, `/gmc`) | `essentials.gamemode` |
| `/heal [player]` | Heals you or the given player | `essentials.heal` |
| `/feed [player]` | Satisfy the hunger | `essentials.feed` |
| `/speed [type] <speed> [player]` | Change your speed limits (also `/fspeed`, `/wspeed`, `/flyspeed`, `/walkspeed`) | `essentials.speed` |
| `/burn <player> <seconds>` | Set a player on fire | `essentials.burn` |
| `/suicide` | Causes you to perish | `essentials.suicide` |
| `/kill <player>` | Kills specified player | `essentials.kill` |
| `/exp [reset\|show\|set\|give] [playername [amount]]` | Give, set, reset, or look at a players experience (also `/xp`) | `essentials.exp` |
| `/rest [player]` | Rests you or the given player | `essentials.rest` |
| `/ice [player]` | Cools a player off | `essentials.ice` |

### Items and inventories

| Command | What it does | Permission |
|---|---|---|
| `/give <player> <item\|numeric> [amount [itemmeta...]]` | Give a player an item | `essentials.give` |
| `/item <item\|numeric> [amount [itemmeta...]]` | Spawn an item (also `/i`) | `essentials.item` |
| `/itemdb <item>` | Searches for an item (also `/dura`, `/itemno`, `/durability`) | `essentials.itemdb` |
| `/itemname [name]` | Names an item (also `/iname`, `/irename`, `/itemrename`) | `essentials.itemname` |
| `/itemlore <add/set/clear> [text/line] [text]` | Edit the lore of an item (also `/lore`, `/ilore`) | `essentials.itemlore` |
| `/enchant <enchantmentname> [level]` | Enchants the item the user is holding | `essentials.enchant` |
| `/potion <clear\|apply\|effect:<effect> power:<power> duration:<duration>>` | Adds custom potion effects to a potion | `essentials.potion` |
| `/firework <<meta param>\|power [amount]\|clear\|fire [amount]>` | Allows you to modify a stack of fireworks | `essentials.firework` |
| `/skull [owner] [player]` | Set the owner of a player skull (also `/head`, `/playerskull`) | `essentials.skull` |
| `/book [title\|author [name]]` | Allows reopening and editing of sealed books | `essentials.book` |
| `/hat [remove]` | Get some cool new headgear (also `/head`) | `essentials.hat` |
| `/more [amount]` | Fills the item stack in hand to specified amount, or to maximum size if none is specified | `essentials.more` |
| `/repair [hand\|all]` | Repairs the durability of one or all items (also `/fix`) | `essentials.repair` |
| `/condense [itemname]` | Condenses items into a more compact blocks (also `/blocks`, `/compact`, `/toblocks`) | `essentials.condense` |
| `/recipe <item> [number]` | Displays how to craft items (also `/method`, `/formula`, `/recipes`) | `essentials.recipe` |
| `/unlimited <list\|item\|clear> [player]` | Allows the unlimited placing of items (also `/ul`, `/unl`) | `essentials.unlimited` |
| `/powertool [l:\|a:\|r:\|c:\|d:][command] [arguments] - {player} can be replaced by name of a clicked player.` | Assigns a command to the item in hand (also `/pt`) | `essentials.powertool` |
| `/powertoollist` | Lists all current powertools (also `/ptlist`) | `essentials.powertoollist` |
| `/powertooltoggle` | Enables or disables all current powertools (also `/ptt`, `/pttoggle`) | `essentials.powertooltoggle` |
| `/clearinventory [player\|*] [item[:<data>]\|*\|**] [amount]` | Clear all items in your inventory (also `/ci`, `/clean`, `/clear`, `/clearinvent`) | `essentials.clearinventory` |
| `/clearinventoryconfirmtoggle` | Toggles whether you are prompted to confirm inventory clears (also `/clearconfirm`, `/clearconfirmon`, `/clearconfirmoff`, `/clearinventoryconfirmoff`) | `essentials.clearinventoryconfirmtoggle` |
| `/invsee <player>` | See the inventory of other players | `essentials.invsee` |
| `/enderchest [player]` | Lets you see inside an enderchest | `essentials.enderchest` |
| `/disposal` | Opens a portable disposal menu (also `/trash`) | `essentials.disposal` |
| `/editsign <set/clear/copy/paste> [line number] [text]` | Edits a sign in the world (also `/sign`) | `essentials.editsign` |
| `/anvil` | Opens up an Anvil | `essentials.anvil` |
| `/cartographytable` | Opens up a cartography table (also `/carttable`) | `essentials.cartographytable` |
| `/grindstone` | Opens up a grindstone | `essentials.grindstone` |
| `/loom` | Opens up a loom | `essentials.loom` |
| `/smithingtable` | Opens up a smithing table (also `/smithtable`) | `essentials.smithingtable` |
| `/stonecutter` | Opens up a stonecutter | `essentials.stonecutter` |
| `/workbench` | Opens up a workbench (also `/wb`, `/craft`, `/wbench`) | `essentials.workbench` |

### World and fun

| Command | What it does | Permission |
|---|---|---|
| `/time [set\|add] [day\|night\|dawn\|17:30\|4pm\|4000ticks] [worldname\|all]` | Display/Change the world time. Defaults to current world (also `/day`, `/night`) | `essentials.time` |
| `/ptime [list\|reset\|day\|night\|dawn\|17:30\|4pm\|4000ticks] [player\|*]` | Adjust player's client time. Add @ prefix to fix (also `/playertime`) | `essentials.ptime` |
| `/weather <storm/sun> [duration]` | Sets the weather (also `/sky`, `/sun`, `/rain`, `/storm`) | `essentials.weather` |
| `/pweather [list\|reset\|storm\|sun\|clear] [player\|*]` | Adjust a player's weather (also `/playerweather`) | `essentials.pweather` |
| `/thunder <true/false> [duration]` | Enable/disable thunder | `essentials.thunder` |
| `/lightning [player] [power]` | The power of Thor. Strike at cursor or player (also `/thor`, `/shock`, `/smite`, `/strike`) | `essentials.lightning` |
| `/remove <all\|tamed\|named\|drops\|arrows\|boats\|minecarts\|xp\|paintings\|itemframes\|endercrystals\|monsters\|animals\|ambient\|mobs\|[mobType]> [radius\|world]` | Removes entities in your world (also `/butcher`, `/killall`, `/mobkill`) | `essentials.remove` |
| `/spawnmob <mob>[:data][,<mount>[:data]] [amount] [player]` | Spawns a mob (also `/mob`, `/spawnentity`) | `essentials.spawnmob` |
| `/spawner <mob> [delay]` | Change the mob type of a spawner (also `/changems`, `/mobspawner`) | `essentials.spawner` |
| `/tree <tree\|birch\|redwood\|redmushroom\|brownmushroom\|jungle\|junglebush\|swamp>` | Spawn a tree where you are looking | `essentials.tree` |
| `/bigtree <tree\|redwood\|jungle\|darkoak>` | Spawn a big tree where you are looking (also `/largetree`) | `essentials.bigtree` |
| `/break` | Breaks the block you are looking at | `essentials.break` |
| `/nuke [player]` | May death rain upon them | `essentials.nuke` |
| `/antioch [message]` | A little surprise for operators (also `/tnt`, `/grenade`) | `essentials.antioch` |
| `/fireball [fireball\|small\|large\|arrow\|skull\|egg\|snowball\|expbottle\|dragon\|splashpotion\|lingeringpotion\|trident] [speed]` | Throw a fireball or other assorted projectiles (also `/fireskull`, `/fireentity`) | `essentials.fireball` |
| `/kittycannon` | Throw an exploding kitten at your opponent | `essentials.kittycannon` |
| `/beezooka` | Throw an exploding bee at your opponent (also `/beecannon`) | `essentials.beezooka` |

### Information and admin

| Command | What it does | Permission |
|---|---|---|
| `/help [search term] [page]` | Views a list of available commands | `essentials.help` |
| `/list [group]` | List all online players (also `/who`, `/plist`, `/online`, `/playerlist`) | `essentials.list` |
| `/seen <playername>` | Shows the last logout time of a player (also `/alts`) | `essentials.seen` |
| `/whois <nickname>` | Determine basic information about the specified player | `essentials.whois` |
| `/near [playername] [radius]` | Lists the players near by or around a player (also `/nearby`) | `essentials.near` |
| `/ping` | Pong! (also `/pong`) | `essentials.ping` |
| `/playtime [player]` | Shows a player's time played in game | `essentials.playtime` |
| `/motd [chapter] [page]` | Views the Message Of The Day | `essentials.motd` |
| `/rules [chapter] [page]` | Views the server rules | `essentials.rules` |
| `/info [chapter] [page]` | Shows information set by the server owner (also `/ifo`, `/news`, `/about`, `/inform`) | `essentials.info` |
| `/<alias> - Define in bukkit.yml` | Allows you to create custom text commands | `essentials.customtext` |
| `/essentials` | Reloads essentials | `essentials.essentials` |
| `/backup` | Runs the backup if configured | `essentials.backup` |
| `/gc` | Reports memory, uptime and tick info (also `/lag`, `/mem`, `/tps`, `/memory`) | `essentials.gc` |
| `/ext [player]` | Extinguish players | `essentials.ext` |

## Permissions

Permission nodes are EssentialsX's. golem only knows operators by itself, so
without a permission plugin only operators can use these commands. To let
other players use them, install a permission plugin for golem (such as
[perms](https://github.com/golem-mc/perms)) and grant the nodes. Players
without a node are told "You do not have access to that command."

| Node | Default | Allows |
|---|---|---|
| `essentials.<command>` | operators | the command (see the tables above) |
| `essentials.<command>.others` | operators | using it on other players |
| `essentials.back.onteleport` | everyone | `/back` remembers where `/tp` and `/warp` took you from |
| `essentials.chat.local`, `essentials.chat.receive.local`, `essentials.chat.receive.shout`, `essentials.chat.receive.question` | everyone | sending and receiving local chat, shouts and questions (chat module) |
| `essentials.chat.color`, `.format`, `.magic`, `.rgb`, `.url` | operators | colour, format, magic, RGB codes and links in chat |
| `essentials.chat.shout`, `essentials.chat.question` | operators | `!` shouting and `?` questions when `chat.radius` is set |
| `essentials.build` | operators | placing and breaking blocks under the antibuild module |
| `essentials.spawn-on-join.exempt` | nobody | skipping the spawn teleport on join |
| `essentials.exempt` | nobody | exemptions from bans, kicks, mutes and sudo (a bundle of the `.exempt` nodes) |
| `essentials.vanish.pickup`, `essentials.silentjoin`, `essentials.silentquit` | nobody | picking up items while vanished, joining and leaving silently |
| `essentials.sethome.multiple.<name>` | nobody | the home limit set in `sethome-multiple` |
| `essentials.nocommandcost.<command>` | nobody | not paying the command's `command-costs` fee |
| `essentials.warps.<name>` | operators | `/warp <name>` when `per-warp-permission` is on |

## Configuration

On first start the plugin writes its files, the same as EssentialsX's, to
`plugins/essentials/`. Run `/essentials reload` after editing them.

| File | What it holds |
|---|---|
| `config.yml` | every setting, documented in the file itself, including the chat, protect and newbies sections |
| `messages.properties` | message overrides; all EssentialsX translations are built in |
| `kits.yml` | the kits (also made with `/createkit`) |
| `worth.yml` | item prices for `/sell` and `/worth` |
| `motd.txt`, `rules.txt`, `info.txt`, `custom.txt` | text for `/motd`, `/rules`, `/info`, `/customtext` |
| `userdata/`, `warps/`, `jail.yml`, `spawn.yml` | saved players, warps, jails and spawn points |

The main options in `config.yml`, with their defaults:

| Option | Default | What it does |
|---|---|---|
| `locale` | `en` | language of the messages |
| `teleport-delay` / `teleport-cooldown` | `0` / `0` | seconds to wait before, and between, teleports |
| `teleport-safety` | `true` | move teleports to a safe spot |
| `heal-cooldown` | `60` | seconds between `/heal` uses |
| `sethome-multiple` | `default: 3`, `vip: 5`, `staff: 10` | homes per permission group |
| `spawnmob-limit` | `10` | most mobs one `/spawnmob` makes |
| `auto-afk` | `300` | seconds idle before a player is marked AFK |
| `starting-balance` | `0` | balance of a new player |
| `currency-symbol` | `$` | symbol in money amounts |
| `max-money` / `min-money` | `10000000000000` / `-10000` | balance limits |
| `command-costs` | none | a fee per command |
| `disabled-commands` | none | commands handed back to vanilla or other plugins |
| `per-warp-permission` | `false` | require `essentials.warps.<name>` for each warp |
| `chat.radius` | `0` | local chat range in blocks; `0` is global (chat module) |
| `chat.format` | `<{DISPLAYNAME}> {MESSAGE}` | chat line format (chat module) |
| `protect.prevent.*` | `fire-spread`, `lava-fire-spread`, `lightning-fire-spread`, `enderdragon-blockdamage` on | behaviours the protect module switches off |
| `newbies.announce-format` | `&dWelcome {DISPLAYNAME}&d to the server!` | first-join announcement |
| `newbies.kit` | `tools` | kit given on first join |
| `respawn-at-home` | `false` | respawn at a home instead of the spawn point |

## Migrating from Bukkit or Paper

Copy the old server's `plugins/Essentials/` folder into `plugins/essentials/`
and keep the world names (`world`, `world_nether`, `world_the_end`) the same.
`config.yml`, `userdata/`, `warps/`, `jail.yml`, `spawn.yml`, `kits.yml` and
`worth.yml` load as they are. Saved locations are matched to a world by name.

## Works with other golem plugins

- **[Vault](https://github.com/golem-mc/vault):** Essentials provides its
  economy to other plugins as "EssentialsX Economy"; the Vault plugin itself
  is not needed. Plugins that use Vault's economy (shops, for instance) use
  Essentials' balances.
- **Permission plugins:** any plugin providing golem's permission interface
  (such as [perms](https://github.com/golem-mc/perms)) decides who may use
  Essentials' commands. Every player is in the `default` group for per-group
  chat formats, spawn points and `/list` sections.

## Differences from EssentialsX

- Item stacks hold at most 99 items, so `oversized-stacksize` is capped at 99.
- Every player sees every command in tab completion; one without the node is
  told they have no access.
- Players are all in the `default` group until a permission plugin says otherwise.
- Discord, DiscordLink, XMPP and GeoIP are not included, and neither are the
  update checker and metrics.
- Locations are matched to worlds by name, not by Bukkit's world UUID.

## License

GPL-3.0, the licence of EssentialsX, which this port is derived from; see
`LICENSE`. This project is not affiliated with or endorsed by EssentialsX.
