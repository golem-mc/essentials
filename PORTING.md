# Porting EssentialsX to golem: layout and conventions

This module is EssentialsX (Java, Bukkit) ported to a golem plugin
(golem-mc/golem, formerly minecraft-go; Go). `go.mod` requires a golem
commit; to move to a newer one, run
`mise exec -- go get github.com/golem-mc/golem@<commit>`. To build against a
local golem checkout instead, add a `go.work` (ignored by git):
`go work init . ../golem`.

## Layout

| Path | What | Java counterpart |
|---|---|---|
| `core/` | The foundation every command uses: config, messages, users, teleports, charges, economy, item DB, item meta, warps, kits, jails, worth, text files, the command framework, the core listeners and timer | `Essentials.java`, `User*`, `Settings`, `I18n`, `AsyncTeleport`, `Trade`, `MetaItemStack`, `Warps`, `Kits`, `Jails`, `Worth`, `textreader/`, `EssentialsPlayerListener`, `EssentialsEntityListener`, `EssentialsTimer` |
| `core/resources/` | Essentials' own `config.yml`, `plugin.yml`, `messages*.properties`, `items.json`, `kits.yml`, `worth.yml` and text files, embedded unchanged | `Essentials/src/main/resources` |
| `commands/<group>/` | The commands, grouped; each package has `Register(*core.Essentials)` | `commands/Command*.java` |
| `essentials.go` | The `essentials` plugin: enables core, then every command group | `Essentials.onEnable` |
| `spawn/`, `chat/`, `protect/`, `antibuild/` | The modules, each its own plugin depending on `essentials` | `EssentialsSpawn`, `EssentialsChat`, `EssentialsProtect`, `EssentialsAntiBuild` |
| `internal/esstest/` | Black-box test harness over golem's `plugintest`: a test server with the plugins, fake players | — |
| `dev/` | `launcher.json` for a development server with the local packages (`mise run dev`) | — |

Discord, DiscordLink, XMPP and GeoIP are not ported. They are bridges to
external services (and GeoIP needs MaxMind's database), and none of them is
needed for the in-game features.

## Writing a command

Port `Commandfoo.java` to a `core.Cmd` in its group's package and add it to the
group's `Register`:

```go
e.RegisterCommand(core.Cmd{
	Name: "foo",          // the key in plugin.yml: gives aliases, description
	PlayerOnly: true,     // Java: run(Server, User, ...) only
	Run: func(ctx *core.Ctx) error {
		if len(ctx.Args) < 1 {
			return core.ErrNotEnoughArgs // shows the usage from messages
		}
		target, err := ctx.GetPlayer(0, false) // Java: getPlayer(server, user, args, 0)
		if err != nil {
			return err
		}
		ctx.Send("fooDone", target.DisplayName()) // Java: user.sendTl("fooDone", ...)
		return nil
	},
	Complete: func(ctx *core.Ctx) []string { // Java: getTabCompleteOptions
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return nil
	},
})
```

- **Permission** is `essentials.<name>`, checked by the server before `Run`
  (and the command is hidden from players without it). `ctx.Has(node)` checks
  sub-permissions; the console has every node.
- **Errors.** `core.Tlf(key, args...)` is Java's `TranslatableException`
  (shown in `errorWithMessage`); `core.TlPlain` shows the message as it is;
  `core.ErrNotEnoughArgs` is `NotEnoughArgumentsException`;
  `core.ErrNoCharge` is `NoChargeException`; `core.ErrQuiet` is
  `QuietAbortException`; `core.ErrPlayerNotFound` is `PlayerNotFoundException`.
- **Messages** keep their keys in `core/resources/messages.properties`.
  Arguments are escaped, so pass player text as-is; pass `core.Raw(s)` for a
  pre-built MiniMessage argument (Java's `AdventureUtil.parsed`).
- **Charges.** `ctx.Charge` is the command's cost from `command-costs`,
  charged after `Run` returns nil. A command that teleports passes
  `ctx.Charge` to `User.Teleport` and returns `core.ErrNoCharge`, as the Java
  does, since the teleport charges.
- **Config.** Shared settings are `e.Settings.X()`; anything else is read
  with `e.Config.String/Bool/Int/Float/StringList(path, default)` using
  config.yml's key.
- **Users.** `e.Users.Of(player)`, `e.Users.Get(uuid)`,
  `e.Users.ByName(name)`; `u.Data` is the saved data (call `u.Save()` after
  changing it); `u.Session` holds a group's own per-session state.
- **Listeners** that a command needs (powertools, unlimited, vanish...) are
  registered in the group's `Register` with `plugin.Listen(e.Server, ...)`.
- **Vanilla command names.** Core registers every command with
  `Override`, so Essentials' `/give`, `/tp`, `/time` replace vanilla's, which
  stay reachable as `/minecraft:give`. A name listed in `disabled-commands`
  is left to vanilla. Nothing to do in the command.
- **Teleports.** `u.Teleport(target, charge, cause, then)` runs `then`
  once the teleport has happened (after the warmup), for the success
  messages Essentials chains on its teleport future.
- **Deviations.** Where a command has to behave differently from the Java,
  say so, and why, in a comment at that spot.

## Tests

Black-box tests in each package start a test server with `esstest.Start`,
join fake players and read what they are sent:

```go
env := esstest.Start(t)
c := env.Join("Alice", true) // true: operator
env.Expect(env.Run(c, "foo Alice"), "expected text")
env.Expect(env.Console("foo Alice"), "expected text")
```

A test package registers the plugins in its `init`: a command group's tests
call `esstest.RegisterCore(foo.Register)` (core plus that group only, so they
build while other groups change); a module's tests call
`esstest.RegisterCore()` and import the module package for its own `init`. Run only the tests you changed:
`mise exec -- go test ./commands/foo/ -run TestFoo`. The first build of the
test harness takes a few minutes. `esstest.Client` embeds a `plugintest`
player, so a test acts as a real client: `PlaceBlock`, `Break`, `Attack`,
`Interact`, `UseItem`, `Click`, `Drag`, `Disconnect`, and reads system chat,
titles, action bars, sounds and explosions. `esstest.Start` takes
`plugintest` options, such as `WithUsercache` for names that never joined.

## Toolchain

Go comes from mise (`mise.toml`): `mise exec -- go build ./...`.
