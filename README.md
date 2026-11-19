# EssentialsX for golem

An unofficial port of [EssentialsX](https://essentialsx.net/) to
[golem](https://github.com/golem-mc/golem), the Minecraft: Java Edition
server written in Go. It keeps EssentialsX's commands, permission nodes,
messages and configuration files, so anyone who knows EssentialsX on Bukkit
or Paper will find the same plugin here.

## What's included

| Plugin | Package | What it does |
|---|---|---|
| Essentials | `github.com/golem-mc/essentials` | The core, about 150 commands: homes, warps, teleports and teleport requests, `/tpr`, kits, the economy (`/balance`, `/pay`, `/sell`, `/worth`), mail, private messages, nicknames, AFK, mutes, bans, jails, god mode, vanish, item and player commands, the MOTD and rules |
| EssentialsSpawn | `github.com/golem-mc/essentials/spawn` | `/spawn` and `/setspawn`, where players appear on first join and on respawn, and the first-join announcement and kit |
| EssentialsChat | `github.com/golem-mc/essentials/chat` | Chat formats, local chat within a radius, shouting with `!`, questions with `?`, and `/toggleshout` |
| EssentialsProtect | `github.com/golem-mc/essentials/protect` | Turns off explosions, fire spread, lava and water flow, mob griefing and chosen mob spawns, and blocks chosen kinds of damage |
| EssentialsAntiBuild | `github.com/golem-mc/essentials/antibuild` | Stops players without `essentials.build` from placing, breaking or using blocks and entities |

The modules need the core plugin. Install the core on its own, or the core
with any of the modules.

## Installing

### With the golem launcher

Add the plugins with
[`golem-launcher`](https://github.com/golem-mc/golem#installing-plugins).
Each plugin is a separate entry, and all of them must have the same version:

```bash
./golem-launcher plugin add github.com/golem-mc/essentials@<version>
./golem-launcher plugin add github.com/golem-mc/essentials/spawn@<version>
./golem-launcher plugin add github.com/golem-mc/essentials/chat@<version>
./golem-launcher
```

`<version>` is a release tag, or a commit hash while there are no releases.
Always give it: the modules are packages inside this one Go module, and the
launcher can only look up the latest version of a whole module. The launcher
writes the plugins to `launcher.json`, which you can also edit by hand:

```json
{
  "server": "<golem version>",
  "plugins": [
    "github.com/golem-mc/essentials@<version>",
    "github.com/golem-mc/essentials/chat@<version>",
    "github.com/golem-mc/essentials/spawn@<version>"
  ]
}
```

The first start builds a server with the plugins in it, which takes a few
minutes; later starts are immediate. To update, run `plugin add` again for
each entry with the new version, or change all the versions in
`launcher.json`, then restart. While a module entry is listed,
`./golem-launcher update` and `outdated` stop with an error and
`plugin list` shows its latest version as unknown, because the launcher
looks each entry's versions up as if it were a module of its own.

The plugins need golem at commit `056d502` or newer. If your server is too
old, the launcher says so and tells you how to update it.

### As a ready-built server

This repository also builds a golem server with the core and all four
modules included. You need [Go](https://go.dev/dl/) 1.27.4 or newer, or
[mise](https://mise.jdx.dev/), which installs the version in `mise.toml`:

```bash
git clone https://github.com/golem-mc/essentials.git
cd essentials
go build -o essentials-server ./cmd/server   # with mise: mise exec -- go build ...
./essentials-server --nogui
```

The server writes `server.properties` and the world into the directory it
runs in, as the vanilla server does. Stop it with `stop` or Ctrl+C.

## Configuration

On first start, Essentials writes its files to `plugins/essentials/`:

- `config.yml`, the main configuration. The modules are configured here
  too, in the same sections as in EssentialsX (`chat`, `protect`,
  `newbies`, and so on).
- `messages.properties`, to change any message. All 55 of EssentialsX's
  translations are built in; choose one with `locale` in `config.yml`.
- `kits.yml`, `worth.yml`, `custom_items.yml` and `tpr.yml`.
- `motd.txt`, `rules.txt`, `info.txt`, `book.txt` and `custom.txt`.

These are EssentialsX's own files, so its
[documentation](https://essentialsx.net/wiki/Home.html) applies. Run
`/essentials reload` after editing them.

### Moving from a Bukkit or Paper server

Copy the files from the old server's `plugins/Essentials/` folder into
`plugins/essentials/`. `userdata/*.yml`, `warps/*.yml`, `jail.yml` and
`spawn.yml` load as they are. Saved locations are matched to a world by its
name (`world`, `world_nether`, `world_the_end`) rather than by Bukkit's world
UUID, so keep the world names the same.

## Permissions

The permission nodes are EssentialsX's (`essentials.home`,
`essentials.tpa`, and so on), with the defaults from its `plugin.yml`. As in
EssentialsX, almost every node defaults to operators only. Every player sees
every command; one without the node is told "You do not have access to that
command."

golem itself only knows operators. To let other players use Essentials, run
a permission plugin, one that provides golem's `plugin.PermissionProvider`
service, and grant them the nodes, as you would with LuckPerms on Bukkit.
EssentialsX's [list of permissions](https://essentialsx.net/wiki/Command-Reference.html)
says which node each command needs. Without a permission plugin, only
operators can use Essentials' commands; golem's own help, `/?`, still works
for everyone.

golem has no permission groups yet, so every player is in the `default`
group. That is the group that per-group chat formats, spawn points and
`/list` sections use.

## Differences from EssentialsX on Bukkit

- **Vanilla commands.** As on Bukkit, `/give`, `/tp`, `/msg`, `/ban`,
  `/time` and the rest are Essentials' versions. The vanilla commands stay
  available as `/minecraft:give` and so on, and a command listed in
  `disabled-commands` goes back to vanilla. `/essentials:give` and the
  command aliases work as usual.
- **Item counts.** golem allows at most 99 items in a stack, so
  `oversized-stacksize` is capped at 99.
- **Economy for other plugins.** There is no Vault. Other golem plugins use
  the economy through the `core.EconomyService` service.
- **Not ported:** EssentialsDiscord, EssentialsDiscordLink, EssentialsXMPP
  and EssentialsGeoIP, which connect to outside services; the update
  checker; and metrics.
- **Not yet tried by hand with several players:** `/tpa`, `/pay` to another
  player, `/invsee`, vanish, jail and temporary bans. Automated tests cover
  them.

## Development

```bash
go test ./...                              # every package, about a minute
go test ./commands/items/ -run TestHat     # one test
```

The tests run a real golem server inside the test process (golem's
`plugintest`), join players and check what they are sent. The first run
compiles the server, which takes a few minutes.

`go.mod` pins a golem commit. To move to a newer one:

```bash
go get github.com/golem-mc/golem@<commit>
```

To build against a local golem checkout instead, create a `go.work`, which
git ignores:

```bash
go work init . ../golem
```

`PORTING.md` explains the layout and how a command is ported from the Java
code.

## License

GPL-3.0, the licence of EssentialsX, which this port is derived from; see
`LICENSE`. The command logic is translated from EssentialsX, and the bundled
`config.yml`, `messages*.properties`, `items.json`, `kits.yml`, `worth.yml`
and text files are EssentialsX's own, by the EssentialsX team and
contributors (<https://github.com/EssentialsX/Essentials>). This project is
not affiliated with or endorsed by EssentialsX.
