package teleport

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
)

func (g *group) registerWarps() {
	g.e.RegisterCommand(
		core.Cmd{Name: "warp", Run: g.runWarp, Complete: g.completeWarp},
		core.Cmd{Name: "setwarp", PlayerOnly: true, Run: g.runSetWarp},
		core.Cmd{Name: "delwarp", Run: g.runDelWarp, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return g.e.Warps.Names()
			}
			return nil
		}},
		core.Cmd{Name: "warpinfo", Run: g.runWarpInfo, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			if g.perWarpPermission() && ctx.User != nil {
				return g.availableWarps(ctx.User)
			}
			return g.e.Warps.Names()
		}},
	)
}

// warpsPerPage is how many warps a /warp page lists.
const warpsPerPage = 20

var digits = regexp.MustCompile(`^[0-9]+$`)

// reservedName matches the file names Windows reserves.
// Essentials: StringUtil.isReservedFileName
var reservedName = regexp.MustCompile(`^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`)

func (g *group) perWarpPermission() bool { return g.e.Config.Bool("per-warp-permission", false) }

// availableWarps answers the warps u may use: all, or with
// per-warp-permission those u has essentials.warps.<warp> for; all for the
// console (nil u).
func (g *group) availableWarps(u *core.User) []string {
	names := g.e.Warps.Names()
	if !g.perWarpPermission() || u == nil {
		return names
	}
	var out []string
	for _, n := range names {
		if u.HasPermission("essentials.warps." + n) {
			out = append(out, n)
		}
	}
	return out
}

// Essentials: Commandwarp
func (g *group) runWarp(ctx *core.Ctx) error {
	args := ctx.Args
	user := ctx.User
	if user == nil {
		if len(args) < 2 || core.IsInt(args[0]) {
			if err := g.warpList(ctx, nil); err != nil {
				return err
			}
			return core.ErrNoCharge
		}
		other, err := g.e.MatchUser(nil, args[1], true, false)
		if err != nil {
			return err
		}
		if err := g.warp(other, other, args[0], nil); err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	if len(args) == 0 || digits.MatchString(args[0]) {
		if !user.HasPermission("essentials.warp.list") {
			return core.Tlf("warpListPermission")
		}
		if err := g.warpList(ctx, user); err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	target := user
	if len(args) == 2 && (user.HasPermission("essentials.warp.otherplayers") || user.HasPermission("essentials.warp.others")) {
		other, err := ctx.GetPlayer(1, false)
		if err != nil {
			return err
		}
		target = other
	}
	if err := g.warpUser(user, target, args[0]); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// warpList shows a page of the warps. Essentials: Commandwarp.warpList
func (g *group) warpList(ctx *core.Ctx, u *core.User) error {
	names := g.availableWarps(u)
	if len(names) == 0 {
		return core.Tlf("noWarpsDefined")
	}
	page := 1
	if len(ctx.Args) > 0 && core.IsInt(ctx.Args[0]) {
		page, _ = strconv.Atoi(ctx.Args[0])
	}
	maxPages := int(math.Ceil(float64(len(names)) / warpsPerPage))
	page = max(min(page, maxPages), 1)
	start := (page - 1) * warpsPerPage
	list := joinList(names[start:min(len(names), start+warpsPerPage)])
	if len(names) > warpsPerPage {
		ctx.Send("warpsCount", len(names), page, maxPages)
		ctx.Send("warpList", list)
	} else {
		ctx.Send("warps", list)
	}
	return nil
}

// warpUser sends user to the warp, owner paying warp's and warp-<name>'s
// command costs. Essentials: Commandwarp.warpUser
func (g *group) warpUser(owner, user *core.User, name string) error {
	cost := g.commandCost(user, "warp-"+strings.ReplaceAll(strings.ToLower(name), "_", "-")).Add(g.commandCost(user, "warp"))
	var charge *core.Trade
	if cost.Sign() > 0 {
		charge = core.MoneyTrade(cost)
	}
	if err := charge.CheckAffordable(owner); err != nil {
		return err
	}
	if g.perWarpPermission() && !owner.HasPermission("essentials.warps."+name) {
		return core.Tlf("warpUsePermission")
	}
	return g.warp(owner, user, name, charge)
}

// warp teleports user to the warp with owner's warmup and charge, telling
// both warpingTo. Essentials: AsyncTeleport.warp
func (g *group) warp(owner, user *core.User, name string, charge *core.Trade) error {
	loc, err := g.e.Warps.Get(name)
	if err != nil {
		return err
	}
	return g.teleport(owner, user, core.LocationTarget(loc), charge, func() {
		args := append([]any{name}, locArgs(loc)...)
		user.Send("warpingTo", args...)
		if owner.ID != user.ID {
			owner.Send("warpingTo", args...)
		}
	})
}

func (g *group) completeWarp(ctx *core.Ctx) []string {
	u := ctx.User
	if u == nil {
		switch len(ctx.Args) {
		case 1:
			return g.e.Warps.Names()
		case 2:
			return ctx.CompletePlayers()
		}
		return nil
	}
	switch {
	case len(ctx.Args) == 1 && u.HasPermission("essentials.warp.list"):
		return g.availableWarps(u)
	case len(ctx.Args) == 2 && (u.HasPermission("essentials.warp.otherplayers") || u.HasPermission("essentials.warp.others")):
		return ctx.CompletePlayers()
	}
	return nil
}

// Essentials: Commandsetwarp
func (g *group) runSetWarp(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	name := ctx.Args[0]
	if core.IsInt(name) || name == "" || reservedName.MatchString(strings.ToLower(name)) {
		return core.Tlf("invalidWarpName")
	}
	if _, err := g.e.Warps.Get(name); err == nil && !ctx.User.HasPermission("essentials.warp.overwrite."+safeString(name)) {
		return core.Tlf("warpOverwrite")
	}
	if err := g.e.Warps.Set(name, ctx.Player.Location(), ctx.User); err != nil {
		return err
	}
	ctx.Send("warpSet", name)
	return nil
}

// Essentials: Commanddelwarp
func (g *group) runDelWarp(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	if !g.e.Warps.Exists(ctx.Args[0]) {
		return core.Tlf("warpNotExist")
	}
	if err := g.e.Warps.Delete(ctx.Args[0]); err != nil {
		return err
	}
	ctx.Send("deleteWarp", ctx.Args[0])
	return nil
}

// Essentials: Commandwarpinfo
func (g *group) runWarpInfo(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	name := ctx.Args[0]
	loc, err := g.e.Warps.Get(name)
	if err != nil {
		return err
	}
	ctx.Send("warpInfo", name)
	ctx.Send("whoisLocation", locArgs(loc)...)
	return nil
}
