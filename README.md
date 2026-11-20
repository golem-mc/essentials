# EssentialsX for golem

An unofficial port of [EssentialsX](https://essentialsx.net/) to
[golem](https://github.com/golem-mc/golem), the Minecraft: Java Edition server
written in Go. It keeps EssentialsX's commands, permission nodes, messages and
configuration files, so anyone who knows EssentialsX on Bukkit or Paper will
find the same plugin here.

## Installing

Add the plugins with
[`golem-launcher`](https://github.com/golem-mc/golem#installing-plugins), one
line each, then start the server:

```bash
./golem-launcher plugin add github.com/golem-mc/essentials
./golem-launcher plugin add github.com/golem-mc/essentials/spawn
./golem-launcher plugin add github.com/golem-mc/essentials/chat
./golem-launcher
```

The first start builds a server with the plugins in it, which takes a few
minutes. To update, run `./golem-launcher update` and restart. The modules
(`spawn`, `chat`, `protect`, `antibuild`) need the core plugin.

## What's included

- **Homes and warps:** `/home`, `/sethome`, `/warp`, `/setwarp`
- **Teleports:** `/tpa`, `/tpaccept`, `/back`, `/tpr`
- **Kits:** `/kit`, `/createkit`
- **Economy:** `/balance`, `/pay`, `/sell`, `/worth`, also offered to other plugins through Vault
- **Messaging:** `/msg`, `/reply`, `/mail`, `/nick`
- **Moderation:** `/ban`, `/mute`, `/jail`, `/kick`
- **Player state:** `/god`, `/vanish`, `/afk`, `/fly`, `/heal`
- **Items and inventories:** `/give`, `/item`, `/more`, `/invsee`, `/hat`
- **Information:** the MOTD, `/rules`, `/info`, `/list`, `/seen`
- **EssentialsSpawn:** `/spawn`, `/setspawn`, first-join spawn and announcement
- **EssentialsChat:** chat formats, local chat, `!` shouting, `?` questions
- **EssentialsProtect:** switches off explosions, fire spread, fluid flow and mob griefing
- **EssentialsAntiBuild:** stops players without `essentials.build` from building

About 150 commands in all. Each works as in the original; see the
[EssentialsX documentation](https://essentialsx.net/wiki/Home.html) for usage.

## Configuration

On first start the plugin writes its files, the same as EssentialsX's, to
`plugins/essentials/`: `config.yml`, `messages.properties`, `kits.yml`,
`worth.yml` and the text files. Run `/essentials reload` after editing them.
All of EssentialsX's translations are built in; pick one with `locale` in
`config.yml`.

To move from a Bukkit or Paper server, copy its `plugins/Essentials/` folder
into `plugins/essentials/` and keep the world names the same.

## Permissions

Permission nodes are EssentialsX's, and most default to operators only. To let
other players use commands, run a permission plugin for golem and grant them
the nodes.

## License

GPL-3.0, the licence of EssentialsX, which this port is derived from; see
`LICENSE`. This project is not affiliated with or endorsed by EssentialsX.
