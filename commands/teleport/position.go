package teleport

import (
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func (g *group) registerPosition() {
	g.e.RegisterCommand(
		core.Cmd{Name: "top", PlayerOnly: true, Run: g.runTop},
		core.Cmd{Name: "bottom", PlayerOnly: true, Run: g.runBottom},
		core.Cmd{Name: "jump", PlayerOnly: true, Run: g.runJump, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.jump.lock") {
				return []string{"lock", "unlock"}
			}
			return nil
		}},
		core.Cmd{Name: "world", PlayerOnly: true, Run: g.runWorld, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			var out []string
			for _, w := range g.e.Server.Worlds() {
				name := core.WorldName(w)
				if !g.e.Settings.WorldTeleportPermissions() || ctx.Has("essentials.worlds."+name) {
					out = append(out, name)
				}
			}
			return out
		}},
		core.Cmd{Name: "compass", PlayerOnly: true, Run: g.runCompass},
		core.Cmd{Name: "depth", PlayerOnly: true, Run: g.runDepth},
		core.Cmd{Name: "getpos", Run: g.runGetpos, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.getpos.others") {
				return ctx.CompletePlayers()
			}
			return nil
		}},
	)
}

// toSafe teleports the user to the safe place nearest y in their column,
// telling them key on arrival. Essentials: Commandtop, Commandbottom
func (g *group) toSafe(ctx *core.Ctx, y int, key string) error {
	here := ctx.Player.Location()
	b := here.Pos.Block()
	unsafe := plugin.Location{World: here.World, Pos: plugin.Vec3{X: float64(b.X), Y: float64(y), Z: float64(b.Z)},
		Yaw: here.Yaw, Pitch: here.Pitch}
	safe, err := g.e.SafeDestination(unsafe)
	if err != nil {
		return err
	}
	if err := g.teleport(ctx.User, ctx.User, core.LocationTarget(safe), ctx.Charge, func() {
		ctx.User.Send(key, locArgs(safe)...)
	}); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// Essentials: Commandtop
func (g *group) runTop(ctx *core.Ctx) error {
	return g.toSafe(ctx, maxHeight(ctx.Player.World()), "teleportTop")
}

// Essentials: Commandbottom
func (g *group) runBottom(ctx *core.Ctx) error {
	return g.toSafe(ctx, ctx.Player.World().MinY(), "teleportBottom")
}

// sessFlyClickJump is the /jump lock flag: a flying left click into the air
// jumps to the block looked at.
const sessFlyClickJump = "teleport.flyClickJump"

// Essentials: Commandjump
func (g *group) runJump(ctx *core.Ctx) error {
	user := ctx.User
	if len(ctx.Args) > 0 && strings.Contains(ctx.Args[0], "lock") && user.HasPermission("essentials.jump.lock") {
		if on, _ := core.SessionValue[bool](user, sessFlyClickJump); on {
			user.Session[sessFlyClickJump] = false
			user.Send("jumpEasterDisable")
		} else {
			user.Session[sessFlyClickJump] = true
			user.Send("jumpEasterEnable")
		}
		return nil
	}
	here := ctx.Player.Location()
	if here.World == nil {
		return core.Tlf("jumpError")
	}
	b, ok := targetBlock(ctx.Player)
	if !ok {
		return core.Tlf("jumpError")
	}
	loc := plugin.Location{World: here.World, Pos: plugin.Vec3{X: float64(b.X), Y: float64(b.Y + 1), Z: float64(b.Z)},
		Yaw: here.Yaw, Pitch: here.Pitch}
	if err := g.teleport(user, user, core.LocationTarget(loc), ctx.Charge, nil); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// onInteract makes a flying left click into the air jump with /jump lock.
// Essentials: EssentialsPlayerListener.useFlyClickJump
func (g *group) onInteract(ev *plugin.PlayerInteractEvent) {
	p := ev.Player
	if ev.Action != plugin.LeftClickAir || !p.Flying() {
		return
	}
	if on, _ := core.SessionValue[bool](g.e.Users.Of(p), sessFlyClickJump); !on {
		return
	}
	target, ok := targetBlock(p)
	if !ok {
		return
	}
	g.e.Server.Scheduler().RunTask(func() {
		if !p.IsValid() {
			return
		}
		loc := p.Location()
		loc.Pos.X, loc.Pos.Z = float64(target.X), float64(target.Z)
		for b := loc.Pos.Block(); g.damaging(loc.World, b.X, b.Y-1, b.Z) && b.Y < maxHeight(loc.World); b = loc.Pos.Block() {
			loc.Pos.Y++
		}
		p.Teleport(loc, plugin.TeleportPlugin)
	})
}

var netherKey, overworldKey = plugin.MinecraftKey("the_nether"), plugin.MinecraftKey("overworld")

// Essentials: Commandworld
func (g *group) runWorld(ctx *core.Ctx) error {
	user := ctx.User
	here := ctx.Player.Location()
	var w plugin.World
	if len(ctx.Args) < 1 {
		worlds := g.e.Server.Worlds()
		var nether plugin.World
		for _, w2 := range worlds {
			if w2.Dimension() == netherKey {
				nether = w2
				break
			}
		}
		if nether == nil {
			return nil
		}
		w = nether
		if sameWorld(here.World, nether) {
			w = worlds[0]
		}
	} else {
		var ok bool
		if w, ok = g.getWorld(ctx.FinalArg(0)); !ok {
			user.Send("invalidWorld")
			user.Send("possibleWorlds", len(g.e.Server.Worlds())-1)
			user.Send("typeWorldName")
			return core.ErrNoCharge
		}
	}
	if node := "essentials.worlds." + core.WorldName(w); g.e.Settings.WorldTeleportPermissions() && !user.HasPermission(node) {
		return core.Tlf("noPerm", node)
	}
	factor := 1.0
	switch from, to := here.World.Dimension(), w.Dimension(); {
	case from == netherKey && to == overworldKey:
		factor = 8
	case from == overworldKey && to == netherKey:
		factor = 1.0 / 8
	}
	b := here.Pos.Block()
	loc := plugin.Location{World: w, Pos: plugin.Vec3{X: float64(b.X)*factor + .5, Y: float64(b.Y), Z: float64(b.Z)*factor + .5}}
	if err := g.teleport(user, user, core.LocationTarget(loc), ctx.Charge, nil); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// compassPoints are the message keys of the bearings, every 45 degrees from
// north.
var compassPoints = []string{"north", "northEast", "east", "southEast", "south", "southWest", "west", "northWest", "north"}

// Essentials: Commandcompass
func (g *group) runCompass(ctx *core.Ctx) error {
	bearing := int(ctx.Player.Location().Yaw+180+360) % 360
	i := 0
	for i < 8 && bearing >= 23+45*i {
		i++
	}
	ctx.Send("compassBearing", core.Raw(g.rawTl(ctx.User, compassPoints[i])), bearing)
	return nil
}

// Essentials: Commanddepth
func (g *group) runDepth(ctx *core.Ctx) error {
	loc := ctx.Player.Location()
	switch depth := loc.Pos.Block().Y - loc.World.SeaLevel(); {
	case depth > 0:
		ctx.Send("depthAboveSea", depth)
	case depth < 0:
		ctx.Send("depthBelowSea", -depth)
	default:
		ctx.Send("depth")
	}
	return nil
}

// Essentials: Commandgetpos
func (g *group) runGetpos(ctx *core.Ctx) error {
	if ctx.User == nil {
		if len(ctx.Args) < 1 {
			return core.ErrNotEnoughArgs
		}
		u, err := g.e.MatchUser(nil, ctx.Args[0], true, false)
		if err != nil {
			return err
		}
		outputPosition(ctx, location(u), nil)
		return nil
	}
	if len(ctx.Args) > 0 && ctx.Has("essentials.getpos.others") {
		other, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		here := ctx.Player.Location()
		outputPosition(ctx, location(other), &here)
		return nil
	}
	outputPosition(ctx, ctx.Player.Location(), nil)
	return nil
}

// outputPosition shows a location, and its distance from from when that is
// in the same world.
func outputPosition(ctx *core.Ctx, l plugin.Location, from *plugin.Location) {
	b := l.Pos.Block()
	ctx.Send("currentWorld", core.WorldName(l.World))
	ctx.Send("posX", b.X)
	ctx.Send("posY", b.Y)
	ctx.Send("posZ", b.Z)
	ctx.Send("posYaw", number(math.Mod(float64(l.Yaw)+360, 360)))
	ctx.Send("posPitch", number(float64(l.Pitch)))
	if from != nil && sameWorld(l.World, from.World) {
		ctx.Send("distance", number(math.Sqrt(l.Pos.DistanceSq(from.Pos))))
	}
}

// number formats a decimal as Java's MessageFormat does: at most three
// fraction digits.
func number(f float64) string {
	s := strconv.FormatFloat(f, 'f', 3, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}
