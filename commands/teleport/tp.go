package teleport

import (
	"math"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

func (g *group) registerBack() {
	g.e.RegisterCommand(core.Cmd{Name: "back", Run: g.runBack, Complete: func(ctx *core.Ctx) []string {
		if ctx.User == nil {
			return ctx.CompletePlayers()
		}
		if len(ctx.Args) == 1 && ctx.Has("essentials.back.others") {
			return ctx.CompletePlayers()
		}
		return nil
	}})
}

func (g *group) registerTp() {
	firstOrOthers := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 || len(ctx.Args) == 2 && ctx.Has("essentials.tp.others") {
			return ctx.CompletePlayers()
		}
		return nil
	}
	first := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return nil
	}
	g.e.RegisterCommand(
		core.Cmd{Name: "tp", Run: g.runTp, Complete: firstOrOthers},
		core.Cmd{Name: "tphere", PlayerOnly: true, Run: g.runTphere, Complete: first},
		core.Cmd{Name: "tpo", PlayerOnly: true, Run: g.runTpo, Complete: firstOrOthers},
		core.Cmd{Name: "tpohere", PlayerOnly: true, Run: g.runTpohere, Complete: first},
		core.Cmd{Name: "tpall", Run: g.runTpall, Complete: first},
		core.Cmd{Name: "tppos", Run: g.runTppos, Complete: g.completeTppos},
		core.Cmd{Name: "tpoffline", PlayerOnly: true, Run: g.runTpoffline},
	)
}

// commandCost answers what command costs u: command-costs, unless u has
// essentials.nocommandcost.<command>. Essentials: Trade.getCommandCost
func (g *group) commandCost(u *core.User, command string) decimal.Decimal {
	if u.HasPermission("essentials.nocommandcost." + command) {
		return decimal.Zero
	}
	return g.e.Settings.CommandCost(command)
}

// costOf answers command's cost to u as a charge, nil when free.
func (g *group) costOf(u *core.User, command string) *core.Trade {
	if c := g.commandCost(u, command); c.Sign() > 0 {
		return core.MoneyTrade(c)
	}
	return nil
}

// Essentials: Commandback
func (g *group) runBack(ctx *core.Ctx) error {
	if ctx.User != nil && (len(ctx.Args) == 0 || !ctx.Has("essentials.back.others")) {
		return g.teleportBack(ctx, ctx.User)
	}
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	u, err := g.e.MatchUser(nil, ctx.Args[0], true, false)
	if err != nil {
		return err
	}
	ctx.Send("backOther", u.Name())
	return g.teleportBack(ctx, u)
}

// teleportBack returns user to their last location; a player sending it
// pays, and needs essentials.back.into.<world>. Essentials:
// Commandback.teleportBack, AsyncTeleport.back
func (g *group) teleportBack(ctx *core.Ctx, user *core.User) error {
	loc, ok := user.LastLocation()
	if !ok {
		return core.Tlf("noLocationFound")
	}
	name := core.WorldName(loc.World)
	requester := ctx.User
	if requester != nil {
		if err := g.checkWorld(user, world(user), loc.World, loc.World); err != nil {
			return err
		}
		if node := "essentials.back.into." + name; !requester.HasPermission(node) {
			return core.Tlf("noPerm", node)
		}
	}
	user.Send("backUsageMsg", locArgs(loc)...)
	to := core.LocationTarget(loc)
	if requester == nil {
		if err := g.now(user, to, true, nil); err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	if err := g.teleport(requester, user, to, ctx.Charge, nil); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// Essentials: Commandtp
func (g *group) runTp(ctx *core.Ctx) error {
	args := ctx.Args
	user := ctx.User
	if user == nil {
		return g.runTpConsole(ctx)
	}
	switch len(args) {
	case 0:
		return core.ErrNotEnoughArgs
	case 1:
		player, err := g.e.MatchUser(user, args[0], false, true)
		if err != nil {
			return err
		}
		if !player.Data.TeleportEnabled {
			return core.Tlf("teleportDisabled", player.DisplayName())
		}
		if !player.Online() {
			if user.HasPermission("essentials.tpoffline") {
				return core.Tlf("teleportOffline", player.DisplayName())
			}
			return core.ErrPlayerNotFound
		}
		if err := g.checkWorld(user, world(user), world(player), world(player)); err != nil {
			return err
		}
		user.Send("teleportToPlayer", player.DisplayName())
		if err := g.teleport(user, user, core.PlayerTarget{User: player}, ctx.Charge, nil); err != nil {
			return err
		}
		return core.ErrNoCharge
	case 3:
		if !user.HasPermission("essentials.tp.position") {
			return core.Tlf("noPerm", "essentials.tp.position")
		}
		here := ctx.Player.Location()
		pos, err := coords(args, here.Pos)
		if err != nil {
			return err
		}
		if outOfBounds(pos) {
			return invalidLocation(ctx)
		}
		loc := plugin.Location{World: here.World, Pos: pos, Yaw: here.Yaw, Pitch: here.Pitch}
		return g.now(user, core.LocationTarget(loc), true, func() {
			user.Send("teleporting", locArgs(loc)...)
		})
	case 4:
		for _, node := range []string{"essentials.tp.others", "essentials.tp.position"} {
			if !user.HasPermission(node) {
				return core.Tlf("noPerm", node)
			}
		}
		target, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		return g.tpToPosition(ctx, target, args[1:])
	}
	if !user.HasPermission("essentials.tp.others") {
		return core.Tlf("noPerm", "essentials.tp.others")
	}
	target, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	to, err := ctx.GetPlayer(1, false)
	if err != nil {
		return err
	}
	for _, u := range []*core.User{target, to} {
		if !u.Data.TeleportEnabled {
			return core.Tlf("teleportDisabled", u.DisplayName())
		}
	}
	if err := g.checkWorld(user, world(target), world(to), world(to)); err != nil {
		return err
	}
	target.Send("teleportAtoB", user.DisplayName(), to.DisplayName())
	return g.nowTo(target, to, nil)
}

// tpToPosition teleports target to the coordinates in args, relative to
// them, telling the sender and them teleporting.
func (g *group) tpToPosition(ctx *core.Ctx, target *core.User, args []string) error {
	at := location(target)
	pos, err := coords(args, at.Pos)
	if err != nil {
		return err
	}
	if outOfBounds(pos) {
		return invalidLocation(ctx)
	}
	loc := plugin.Location{World: at.World, Pos: pos, Yaw: at.Yaw, Pitch: at.Pitch}
	if ctx.User != nil && !target.Data.TeleportEnabled {
		return core.Tlf("teleportDisabled", target.DisplayName())
	}
	ctx.Send("teleporting", locArgs(loc)...)
	return g.now(target, core.LocationTarget(loc), false, func() {
		target.Send("teleporting", locArgs(loc)...)
	})
}

func (g *group) runTpConsole(ctx *core.Ctx) error {
	args := ctx.Args
	if len(args) < 2 {
		return core.ErrNotEnoughArgs
	}
	target, err := g.e.MatchUser(nil, args[0], true, false)
	if err != nil {
		return err
	}
	switch {
	case len(args) == 2:
		to, err := g.e.MatchUser(nil, args[1], true, false)
		if err != nil {
			return err
		}
		target.Send("teleportAtoB", ctx.SenderName(), to.DisplayName())
		return g.nowTo(target, to, nil)
	case len(args) > 3:
		return g.tpToPosition(ctx, target, args[1:4])
	}
	return core.ErrNotEnoughArgs
}

// Essentials: Commandtphere
func (g *group) runTphere(ctx *core.Ctx) error {
	user := ctx.User
	player, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	if !player.Data.TeleportEnabled {
		return core.Tlf("teleportDisabled", player.DisplayName())
	}
	if err := g.checkWorld(user, world(user), world(player), world(user)); err != nil {
		return err
	}
	to := core.PlayerTarget{User: user}
	if err := g.teleport(user, player, to, ctx.Charge, func() {
		loc, _ := to.Location()
		player.Send("teleporting", locArgs(loc)...)
		user.Send("teleporting", locArgs(loc)...)
	}); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// Essentials: Commandtpo
func (g *group) runTpo(ctx *core.Ctx) error {
	user := ctx.User
	switch len(ctx.Args) {
	case 0:
		return core.ErrNotEnoughArgs
	case 1:
		player, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		if err := g.checkWorld(user, world(user), world(player), world(player)); err != nil {
			return err
		}
		return g.nowTo(user, player, nil)
	}
	if !user.HasPermission("essentials.tp.others") {
		return core.Tlf("noPerm", "essentials.tp.others")
	}
	target, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	to, err := ctx.GetPlayer(1, false)
	if err != nil {
		return err
	}
	if err := g.checkWorld(user, world(target), world(to), world(to)); err != nil {
		return err
	}
	return g.nowTo(target, to, func() {
		target.Send("teleportAtoB", user.DisplayName(), to.DisplayName())
	})
}

// Essentials: Commandtpohere
func (g *group) runTpohere(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	user := ctx.User
	player, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	if err := g.checkWorld(user, world(user), world(player), world(user)); err != nil {
		return err
	}
	return g.nowTo(player, user, nil)
}

// Essentials: Commandtpall
func (g *group) runTpall(ctx *core.Ctx) error {
	target := ctx.User
	if len(ctx.Args) > 0 {
		t, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		target = t
	} else if target == nil {
		return core.ErrNotEnoughArgs
	}
	ctx.Send("teleportAll")
	loc := location(target)
	for _, u := range g.e.OnlineUsers() {
		if u.ID == target.ID {
			continue
		}
		if isSender(ctx, target) && g.checkWorld(target, world(target), world(u), world(target)) != nil {
			continue
		}
		if err := g.now(u, core.LocationTarget(loc), false, nil); err != nil {
			g.e.ShowError(ctx, err)
		}
	}
	return nil
}

// Essentials: Commandtppos
func (g *group) runTppos(ctx *core.Ctx) error {
	user := ctx.User
	args := ctx.Args
	if user == nil {
		if len(args) < 4 {
			return core.ErrNotEnoughArgs
		}
		target, err := g.e.MatchUser(nil, args[0], true, false)
		if err != nil {
			return err
		}
		user, args = target, args[1:]
	} else if len(args) < 3 {
		return core.ErrNotEnoughArgs
	}
	at := location(user)
	pos, err := coords(args, at.Pos)
	if err != nil {
		return err
	}
	loc := plugin.Location{World: at.World, Pos: pos, Yaw: at.Yaw, Pitch: at.Pitch}
	setWorld := func(name string) error {
		w, ok := g.getWorld(name)
		if !ok {
			return core.Tlf("invalidWorld")
		}
		if ctx.User != nil {
			if err := g.checkWorld(user, at.World, w, w); err != nil {
				return err
			}
		}
		loc.World = w
		return nil
	}
	if len(args) == 4 {
		if err := setWorld(args[3]); err != nil {
			return err
		}
	}
	if len(args) > 4 {
		yaw, err := parseFloat(args[3])
		if err != nil {
			return err
		}
		pitch, err := parseFloat(args[4])
		if err != nil {
			return err
		}
		loc.Yaw = float32(math.Mod(float64(yaw)+360, 360))
		loc.Pitch = pitch
	}
	if len(args) > 5 {
		if err := setWorld(args[5]); err != nil {
			return err
		}
	}
	if outOfBounds(pos) {
		return invalidLocation(ctx)
	}
	ctx.Send("teleporting", locArgs(loc)...)
	if ctx.User == nil {
		user.Send("teleporting", locArgs(loc)...)
		return g.teleport(user, user, core.LocationTarget(loc), nil, nil)
	}
	if err := g.teleport(user, user, core.LocationTarget(loc), ctx.Charge, nil); err != nil {
		return err
	}
	return core.ErrNoCharge
}

func (g *group) completeTppos(ctx *core.Ctx) []string {
	n := len(ctx.Args)
	if ctx.User == nil {
		if n == 1 {
			return ctx.CompletePlayers()
		}
		n--
	}
	switch n {
	case 1, 2, 3:
		return []string{"~0"}
	case 4, 5:
		return []string{"0"}
	case 6:
		return core.WorldNames(g.e.Server)
	}
	return nil
}

// Essentials: Commandtpoffline
func (g *group) runTpoffline(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	user := ctx.User
	target, err := g.e.MatchUser(nil, ctx.Args[0], true, true)
	if err != nil {
		return err
	}
	var logout plugin.Location
	ok := false
	if l := target.Data.LogoutLocation; l != nil {
		logout, ok = l.Resolve(g.e.Server)
	}
	if !ok {
		user.Send("teleportOfflineUnknown", target.DisplayName())
		return core.ErrNoCharge
	}
	if err := g.checkWorld(user, world(user), logout.World, logout.World); err != nil {
		return err
	}
	user.Send("teleporting", locArgs(logout)...)
	return g.now(user, core.LocationTarget(logout), false, nil)
}
