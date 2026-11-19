package player

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// nukeKey marks the TNT /nuke drops. Essentials: Commandnuke.NUKE_META_KEY
var nukeKey = plugin.Key{Namespace: "essentials", Path: "ess_tnt_proj"}

// fireballKey marks /fireball's projectiles. Essentials:
// Commandfireball.FIREBALL_META_KEY
var fireballKey = plugin.Key{Namespace: "essentials", Path: "ess_fireball_proj"}

func registerFun(e *core.Essentials) {
	e.RegisterCommand(nukeCmd(), antiochCmd(), fireballCmd(), beezookaCmd(), kittycannonCmd(), breakCmd(), sudoCmd(),
		removeCmd(), spawnerCmd())

	// Nuke TNT explodes without damage. Essentials: TNTExplodeListener
	plugin.Listen(e.Server, func(ev *plugin.EntityExplodeEvent) {
		if ev.Entity == nil || ev.Entity.Type() != entityType("tnt") {
			return
		}
		if v, ok := plugin.GetData[bool](ev.Entity.Data(), nukeKey); !ok || !v {
			return
		}
		ev.SetCancelled(true)
		ev.Entity.World().Explode(ev.Pos, 0, false, plugin.ExplosionKeep, nil)
	}, plugin.ListenOptions{Priority: plugin.Low})

	// /fireball's arrows and tridents cannot be picked up.
	// Essentials: EssentialsPlayerListener.ArrowPickupListener
	plugin.Listen(e.Server, func(ev *plugin.PlayerPickupArrowEvent) {
		if v, ok := plugin.GetData[bool](ev.Arrow.Data(), fireballKey); ok && v {
			ev.SetCancelled(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Low})
}

// entityType answers the entity type of a vanilla name, -1 for none.
func entityType(name string) registry.EntityType {
	t, ok := registry.EntityTypeByName("minecraft:" + name)
	if !ok {
		return -1
	}
	return t
}

// Essentials: Commandnuke
func nukeCmd() core.Cmd {
	return core.Cmd{
		Name: "nuke",
		Run: func(ctx *core.Ctx) error {
			var targets []*core.User
			if len(ctx.Args) > 0 {
				for i := range ctx.Args {
					u, err := ctx.GetPlayer(i, false)
					if err != nil {
						return err
					}
					targets = append(targets, u)
				}
			} else {
				targets = ctx.Ess.OnlineUsers()
			}
			for _, u := range targets {
				p, ok := u.Player()
				if !ok {
					continue
				}
				u.Send("nuke")
				loc := p.Location()
				w := loc.World
				b := loc.Pos.Block()
				top := w.HighestBlockY(b.X, b.Z, plugin.MotionBlocking)
				for x := -10; x <= 10; x += 5 {
					for z := -10; z <= 10; z += 5 {
						at := plugin.Location{World: w, Pos: plugin.Vec3{X: float64(b.X + x), Y: float64(top + 64), Z: float64(b.Z + z)}}
						_, _ = w.Spawn(entityType("tnt"), at, func(ent plugin.Entity) { ent.Data().Set(nukeKey, true) })
					}
				}
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return ctx.CompletePlayers()
			}
			return nil
		},
	}
}

// broadcastText sends text to every player not ignoring from, and the
// console. Essentials: broadcastMessage(IUser, String)
func broadcastText(e *core.Essentials, from *core.User, text string) {
	msg := plugin.Plain(text)
	for _, p := range e.Server.Players() {
		if from != nil && e.Users.Of(p).IsIgnoring(from) {
			continue
		}
		p.SendMessage(msg)
	}
	e.Server.Console().SendMessage(msg)
}

// Essentials: Commandantioch
func antiochCmd() core.Cmd {
	return core.Cmd{
		Name:       "antioch",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) > 0 {
				broadcastText(ctx.Ess, ctx.User, "...lobbest thou thy Holy Hand Grenade of Antioch towards thy foe,")
				broadcastText(ctx.Ess, ctx.User, "who being naughty in My sight, shall snuff it.")
			}
			loc := targetLocation(ctx.Player, 300)
			_, err := loc.World.Spawn(entityType("tnt"), loc, nil)
			return err
		},
	}
}

// fireballTypes are /fireball's projectiles by name.
// Essentials: Commandfireball.types
var fireballTypes = map[string]string{
	"fireball": "fireball", "small": "small_fireball", "large": "fireball", "arrow": "arrow",
	"skull": "wither_skull", "egg": "egg", "snowball": "snowball", "expbottle": "experience_bottle",
	"dragon": "dragon_fireball", "splashpotion": "splash_potion", "lingeringpotion": "lingering_potion",
	"trident": "trident", "windcharge": "wind_charge",
}

// Essentials: Commandfireball
func fireballCmd() core.Cmd {
	return core.Cmd{
		Name:       "fireball",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			kind := "fireball"
			if _, ok := fireballTypes[ctx.Arg(0)]; ok {
				kind = ctx.Arg(0)
			}
			speed := 2.0
			ride := len(ctx.Args) > 2 && strings.EqualFold(ctx.Args[2], "ride") && ctx.Has("essentials.fireball.ride")
			if len(ctx.Args) > 1 {
				if f, ok := parseFloat(ctx.Args[1]); ok {
					speed = math.Max(0, math.Min(f, ctx.Ess.Config.Float("max-projectile-speed", 8)))
				}
			}
			if !ctx.Has("essentials.fireball." + kind) {
				return core.Tlf("noPerm", "essentials.fireball."+kind)
			}
			eye := eyeLocation(ctx.Player)
			dir := eye.Direction().Scale(speed)
			at := eye
			at.Pos = at.Pos.Add(dir)
			proj, err := eye.World.Spawn(entityType(fireballTypes[kind]), at, nil)
			if err != nil {
				return err
			}
			if pr, ok := proj.(plugin.Projectile); ok {
				pr.SetShooter(ctx.Player)
			}
			proj.SetVelocity(dir)
			proj.Data().Set(fireballKey, true)
			if ride {
				proj.AddPassenger(ctx.Player)
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				var out []string
				for k := range fireballTypes {
					if ctx.Has("essentials.fireball." + k) {
						out = append(out, k)
					}
				}
				return out
			case 2:
				return []string{"1", "2", "3", "4", "5"}
			}
			return nil
		},
	}
}

// launchAndPop throws a mob of type name from the player's eyes and pops it
// with a harmless explosion a second later.
// Essentials: Commandbeezooka, Commandkittycannon
func launchAndPop(ctx *core.Ctx, name string, configure func(plugin.Entity)) error {
	eye := eyeLocation(ctx.Player)
	ent, err := eye.World.Spawn(entityType(name), eye, nil)
	if err != nil {
		return core.Tlf("unableToSpawnMob")
	}
	if configure != nil {
		configure(ent)
	}
	ent.SetVelocity(eye.Direction().Scale(2))
	ctx.Ess.Server.Scheduler().RunLater(20, func() {
		if !ent.IsValid() {
			return
		}
		loc := ent.Location()
		ent.Remove()
		loc.World.Explode(loc.Pos, 0, false, plugin.ExplosionKeep, nil)
	})
	return nil
}

// Essentials: Commandbeezooka
func beezookaCmd() core.Cmd {
	return core.Cmd{
		Name:       "beezooka",
		PlayerOnly: true,
		Run:        func(ctx *core.Ctx) error { return launchAndPop(ctx, "bee", nil) },
	}
}

// catVariants are vanilla's cat variants. Bukkit: Cat.Type
var catVariants = []string{"tabby", "black", "red", "siamese", "british_shorthair", "calico", "persian",
	"ragdoll", "white", "jellie", "all_black"}

// Essentials: Commandkittycannon
func kittycannonCmd() core.Cmd {
	return core.Cmd{
		Name:       "kittycannon",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			return launchAndPop(ctx, "cat", func(cat plugin.Entity) {
				if v, ok := cat.(plugin.VariantHolder); ok {
					v.SetVariant(plugin.MinecraftKey(catVariants[rand.IntN(len(catVariants))]))
				}
				if t, ok := cat.(plugin.Tameable); ok {
					t.SetTamed(true) // no owner
					t.SetBaby(true)
				}
			})
		},
	}
}

// Essentials: Commandbreak
func breakCmd() core.Cmd {
	return core.Cmd{
		Name:       "break",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			b := targetBlockExact(ctx.Player, 20)
			s := b.State()
			if isAir(s) {
				return core.ErrNoCharge
			}
			if s.Block() == bedrockBlock && !ctx.Has("essentials.break.bedrock") {
				return core.Tlf("noBreakBedrock")
			}
			// Protection plugins may refuse it. Bukkit: new BlockBreakEvent(block, player)
			ev := &plugin.BlockBreakEvent{Player: ctx.Player, Block: b, State: s, DropItems: true}
			ctx.Ess.Server.Events().Fire(ev)
			if ev.Cancelled() {
				return core.ErrNoCharge
			}
			b.Set(plugin.DefaultState(airBlock))
			return nil
		},
	}
}

// Essentials: Commandsudo
func sudoCmd() core.Cmd {
	return core.Cmd{
		Name: "sudo",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 2 {
				return core.ErrNotEnoughArgs
			}
			command := ctx.FinalArg(1)
			multiple := ctx.User == nil || ctx.Has("essentials.sudo.multiple")
			ctx.Send("sudoRun", ctx.Args[0], command, "")
			return loopOnline(ctx, false, multiple, ctx.Args[0], func(u *core.User) error {
				if u.Name() == ctx.Sender.Name() {
					return nil
				}
				if u.HasPermission("essentials.sudo.exempt") && ctx.User != nil {
					ctx.Send("sudoExempt", u.Name())
					return nil
				}
				p, ok := u.Player()
				if !ok {
					return nil
				}
				if strings.HasPrefix(strings.ToLower(command), "c:") {
					p.Chat(command[2:])
					return nil
				}
				if command != "" {
					ctx.Ess.Server.Scheduler().RunTask(func() {
						if p.IsValid() {
							p.Chat("/" + command)
						}
					})
				}
				return nil
			})
		},
	}
}

// Entity kinds /remove sorts by, after Bukkit's entity interfaces.
var (
	projectileTypes = typeSet("arrow", "spectral_arrow", "trident", "snowball", "egg", "ender_pearl", "fireball",
		"small_fireball", "dragon_fireball", "wither_skull", "shulker_bullet", "llama_spit", "splash_potion",
		"lingering_potion", "experience_bottle", "firework_rocket", "fishing_bobber", "wind_charge", "breeze_wind_charge")
	flyingTypes  = typeSet("ghast", "phantom")
	monsterTypes = typeSet("blaze", "creeper", "enderman", "endermite", "giant", "guardian", "elder_guardian",
		"pillager", "vindicator", "evoker", "illusioner", "ravager", "witch", "silverfish", "skeleton", "stray",
		"wither_skeleton", "bogged", "parched", "spider", "cave_spider", "vex", "wither", "zombie", "husk", "drowned",
		"zombie_villager", "zombified_piglin", "piglin", "piglin_brute", "zoglin", "warden", "breeze", "creaking",
		"ender_dragon", "slime", "magma_cube")
	animalTypes = typeSet("chicken", "cow", "mooshroom", "pig", "sheep", "wolf", "cat", "ocelot", "horse", "donkey",
		"mule", "llama", "trader_llama", "skeleton_horse", "zombie_horse", "rabbit", "polar_bear", "turtle", "panda",
		"fox", "bee", "hoglin", "strider", "goat", "axolotl", "frog", "camel", "camel_husk", "sniffer", "armadillo",
		"parrot", "happy_ghast", "nautilus", "zombie_nautilus", "villager", "wandering_trader", "snow_golem", "squid",
		"glow_squid", "dolphin", "cod", "salmon", "pufferfish", "tropical_fish", "tadpole", "bat")
)

func typeSet(names ...string) map[registry.EntityType]bool {
	m := map[registry.EntityType]bool{}
	for _, n := range names {
		if t := entityType(n); t >= 0 {
			m[t] = true
		}
	}
	return m
}

// removeKinds are /remove's kinds. Essentials: Commandremove.ToRemove
var removeKinds = []string{"drops", "arrows", "boats", "minecarts", "xp", "paintings", "itemframes", "endercrystals",
	"hostile", "monsters", "passive", "animals", "ambient", "mobs", "entities", "all", "custom", "tamed", "named"}

// removes reports whether kind takes ent. Essentials: Commandremove.removeHandler
func removes(kind string, ent plugin.Entity, custom []registry.EntityType) bool {
	t := ent.Type()
	name := strings.TrimPrefix(t.Name(), "minecraft:")
	switch kind {
	case "tamed":
		tm, ok := ent.(plugin.Tameable)
		return ok && tm.Tamed()
	case "named":
		_, living := ent.(plugin.LivingEntity)
		return living && ent.CustomName() != nil
	case "drops":
		return name == "item"
	case "arrows":
		return projectileTypes[t]
	case "boats":
		return strings.HasSuffix(name, "_boat") || strings.HasSuffix(name, "_raft")
	case "minecarts":
		return name == "minecart" || strings.HasSuffix(name, "_minecart")
	case "xp":
		return name == "experience_orb"
	case "paintings":
		return name == "painting"
	case "itemframes":
		return name == "item_frame" || name == "glow_item_frame"
	case "endercrystals":
		return name == "end_crystal"
	case "ambient":
		return flyingTypes[t]
	case "hostile", "monsters":
		return monsterTypes[t] || flyingTypes[t]
	case "passive", "animals":
		return animalTypes[t]
	case "mobs":
		return monsterTypes[t] || flyingTypes[t] || animalTypes[t]
	case "entities", "all":
		return true
	case "custom":
		return slices.Contains(custom, t)
	}
	return false
}

// Essentials: Commandremove
func removeCmd() core.Cmd {
	run := func(ctx *core.Ctx, w plugin.World, worldOK bool, radius int) error {
		if !worldOK || w == nil {
			return core.Tlf("invalidWorld")
		}
		var kinds []string
		var customNames []string
		if ctx.Args[0] == "*" || ctx.Args[0] == "all" {
			kinds = []string{"all"}
		} else {
			for _, s := range strings.Split(ctx.Args[0], ",") {
				low := strings.ToLower(s)
				switch {
				case slices.Contains(removeKinds, low):
					kinds = append(kinds, low)
				case slices.Contains(removeKinds, low+"s"):
					kinds = append(kinds, low+"s")
				default:
					kinds = append(kinds, "custom")
					customNames = append(customNames, s)
				}
			}
		}
		var custom []registry.EntityType
		warn := false
		for _, n := range customNames {
			if m, ok := mobByName(n); ok {
				custom = append(custom, m.typ)
			} else {
				warn = true
			}
		}
		if warn {
			ctx.Send("invalidMob")
		}
		r2 := float64(radius * radius)
		var center plugin.Vec3
		if ctx.Player != nil {
			center = ctx.Player.Location().Pos
		}
		removed := 0
		var ents []plugin.Entity
		for ent := range w.Entities() {
			ents = append(ents, ent)
		}
		for _, ent := range ents {
			if radius > 0 && ctx.Player != nil && center.DistanceSq(ent.Pos()) > r2 {
				continue
			}
			if _, ok := ent.(plugin.Player); ok {
				continue
			}
			for _, k := range kinds {
				if tm, ok := ent.(plugin.Tameable); ok && tm.Tamed() && !slices.Contains(kinds, "tamed") {
					if _, owned := tm.Owner(); owned {
						continue
					}
				}
				if _, living := ent.(plugin.LivingEntity); living && ent.CustomName() != nil && !slices.Contains(kinds, "named") {
					continue
				}
				if ent.IsValid() && removes(k, ent, custom) {
					ent.Remove()
					removed++
				}
			}
		}
		ctx.Send("removed", removed)
		return nil
	}
	return core.Cmd{
		Name: "remove",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if ctx.User == nil {
				if len(args) < 2 {
					return core.ErrNotEnoughArgs
				}
				w, ok := core.WorldByName(ctx.Ess.Server, args[1])
				return run(ctx, w, ok, 0)
			}
			if len(args) < 1 {
				return core.ErrNotEnoughArgs
			}
			w, ok := ctx.Player.World(), true
			radius := 0
			if len(args) >= 2 {
				if n, err := parseInt(args[1]); err == nil {
					radius = n
				} else {
					w, ok = core.WorldByName(ctx.Ess.Server, args[1])
				}
			}
			if len(args) >= 3 {
				radius = 0
				w, ok = core.WorldByName(ctx.Ess.Server, args[2])
			}
			return run(ctx, w, ok, radius)
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return removeKinds
			case 2:
				return core.WorldNames(ctx.Ess.Server)
			}
			return nil
		},
	}
}

// Essentials: Commandspawner
func spawnerCmd() core.Cmd {
	return core.Cmd{
		Name:       "spawner",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 1 || len(ctx.Args[0]) < 2 {
				return usageWith(ctx, "mobsAvailable", strings.Join(mobNames(), ", "))
			}
			loc := targetLocation(ctx.Player, 300)
			pos := loc.Pos.Block()
			if loc.World.BlockState(pos).Block() != spawnerBlock {
				return core.Tlf("mobSpawnTarget")
			}
			m, ok := mobByName(ctx.Args[0])
			if !ok {
				return core.Tlf("invalidMob")
			}
			if ctx.Ess.Config.Bool("protect.prevent.spawn."+m.key, false) {
				return core.Tlf("disabledToSpawnMob")
			}
			if !ctx.Has("essentials.spawner." + strings.ToLower(m.name)) {
				return core.Tlf("noPermToSpawnMob")
			}
			delay := 0
			if len(ctx.Args) > 1 && core.IsInt(ctx.Args[1]) && ctx.Has("essentials.spawner.delay") {
				delay, _ = parseInt(ctx.Args[1])
			}
			costName := "spawner-" + strings.ToLower(m.name)
			var charge *core.Trade
			if cost := ctx.Ess.Settings.CommandCost(costName); cost.Sign() > 0 && !ctx.Has("essentials.nocommandcost."+costName) {
				charge = core.MoneyTrade(cost)
			}
			if err := charge.CheckAffordable(ctx.User); err != nil {
				return err
			}
			be, ok := loc.World.BlockEntity(pos)
			sp, isSpawner := be.(plugin.Spawner)
			if !ok || !isSpawner {
				return core.Tlf("mobSpawnError")
			}
			sp.SetEntityType(m.typ)
			if delay > 0 {
				sp.SetDelays(delay, delay)
			}
			sp.SetDelay(delay)
			if err := charge.Charge(ctx.User); err != nil {
				return err
			}
			ctx.Send("setSpawner", m.name)
			return nil
		},
	}
}
