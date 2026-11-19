package teleport

import (
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
)

func (g *group) registerTpa() {
	first := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return nil
	}
	pending := func(ctx *core.Ctx) []string {
		if len(ctx.Args) != 1 || ctx.User == nil {
			return nil
		}
		var out []string
		for _, r := range ctx.User.Teleports {
			out = append(out, r.Name)
		}
		return append(out, "*")
	}
	tptoggle := toggleCmd{others: "essentials.tptoggle.others", toggle: toggleTeleport}
	tpauto := toggleCmd{others: "essentials.tpauto.others", toggle: toggleAuto}
	g.e.RegisterCommand(
		core.Cmd{Name: "tpa", PlayerOnly: true, Run: g.runTpa, Complete: first},
		core.Cmd{Name: "tpahere", PlayerOnly: true, Run: g.runTpahere, Complete: first},
		core.Cmd{Name: "tpaall", Run: g.runTpaall, Complete: first},
		core.Cmd{Name: "tpaccept", PlayerOnly: true, Run: g.runTpaccept, Complete: pending},
		core.Cmd{Name: "tpdeny", PlayerOnly: true, Run: g.runTpdeny, Complete: pending},
		core.Cmd{Name: "tpacancel", PlayerOnly: true, Run: g.runTpacancel},
		core.Cmd{Name: "tptoggle", Run: tptoggle.run, Complete: tptoggle.complete},
		core.Cmd{Name: "tpauto", Run: tpauto.run, Complete: tpauto.complete},
	)
}

// The request queue is core.User.Teleports, newest first, one request per
// requester (Essentials keys it by name in insertion order).

// requestTeleport records from's request to u, at from's location for
// /tpahere and u's for /tpa; a full queue drops its newest request.
// Essentials: User.requestTeleport
func (g *group) requestTeleport(u, from *core.User, here bool) {
	loc := location(u)
	if here {
		loc = location(from)
	}
	u.Teleports = slices.DeleteFunc(u.Teleports, func(r *core.TpRequest) bool { return r.Name == from.Name() })
	if len(u.Teleports) > 0 && len(u.Teleports) >= g.e.Settings.TpaMaxRequests() {
		u.Teleports = u.Teleports[1:]
	}
	r := &core.TpRequest{From: from.ID, Name: from.Name(), Here: here, Location: loc, Time: time.Now()}
	u.Teleports = append([]*core.TpRequest{r}, u.Teleports...)
}

// expired reports whether r is past tpa-accept-cancellation.
func (g *group) expired(r *core.TpRequest) bool {
	d := g.e.Settings.TpaAcceptCancellation()
	return d >= time.Second && time.Since(r.Time) > d
}

func (g *group) timedOut(u *core.User, r *core.TpRequest) {
	u.Send("requestTimedOutFrom", g.e.Users.Get(r.From).DisplayName())
}

// outstandingRequest answers u's unexpired request from name, dropping it
// when it has expired (and telling u with inform).
// Essentials: User.getOutstandingTpaRequest
func (g *group) outstandingRequest(u *core.User, name string, inform bool) *core.TpRequest {
	i := slices.IndexFunc(u.Teleports, func(r *core.TpRequest) bool { return r.Name == name })
	if i < 0 {
		return nil
	}
	r := u.Teleports[i]
	if !g.expired(r) {
		return r
	}
	u.RemoveTpRequest(r)
	if inform {
		g.timedOut(u, r)
	}
	return nil
}

// nextRequest answers u's newest unexpired request (not a /tpahere one with
// excludeHere), dropping the expired ones; with ignoreExpirations it stops
// at the first found. Essentials: User.getNextTpaRequest
func (g *group) nextRequest(u *core.User, inform, ignoreExpirations, excludeHere bool) *core.TpRequest {
	var next *core.TpRequest
	for _, r := range slices.Clone(u.Teleports) {
		if g.expired(r) {
			if inform {
				g.timedOut(u, r)
			}
			u.RemoveTpRequest(r)
			continue
		}
		if excludeHere && r.Here {
			continue
		}
		if ignoreExpirations {
			return r
		}
		if next == nil {
			next = r
		}
	}
	return next
}

// removeRequest drops u's request from name, reporting whether there was one.
func removeRequest(u *core.User, name string) bool {
	n := len(u.Teleports)
	u.Teleports = slices.DeleteFunc(u.Teleports, func(r *core.TpRequest) bool { return r.Name == name })
	return len(u.Teleports) < n
}

// tpaTarget finds and checks /tpa's or /tpahere's player.
func (g *group) tpaTarget(ctx *core.Ctx) (*core.User, error) {
	if len(ctx.Args) < 1 {
		return nil, core.ErrNotEnoughArgs
	}
	player, err := ctx.GetPlayer(0, false)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(ctx.User.Name(), player.Name()) {
		return nil, core.ErrNotEnoughArgs
	}
	if !player.HasPermission("essentials.tpaccept") {
		return nil, core.Tlf("teleportNoAcceptPermission", player.DisplayName())
	}
	if !player.Data.TeleportEnabled {
		return nil, core.Tlf("teleportDisabled", player.DisplayName())
	}
	return player, nil
}

// tellRequest tells player of a request and how to answer it.
func (g *group) tellRequest(player *core.User, key, from string, deny bool) {
	player.Send(key, from)
	player.Send("typeTpaccept")
	if deny {
		player.Send("typeTpdeny")
	}
	if d := g.e.Settings.TpaAcceptCancellation(); d != 0 {
		player.Send("teleportRequestTimeoutInfo", int64(d/time.Second))
	}
}

// Essentials: Commandtpa
func (g *group) runTpa(ctx *core.Ctx) error {
	user := ctx.User
	player, err := g.tpaTarget(ctx)
	if err != nil {
		return err
	}
	if err := g.checkWorld(user, world(user), world(player), world(player)); err != nil {
		return err
	}
	// Don't let the sender request a teleport to the same player twice.
	if r := g.outstandingRequest(player, user.Name(), false); r != nil && !r.Here {
		return core.Tlf("requestSentAlready", player.DisplayName())
	}
	if player.Data.TeleportAuto && !player.IsIgnoring(user) {
		user.Send("teleportToPlayer", player.DisplayName())
		if err := g.teleport(user, user, core.PlayerTarget{User: player}, ctx.Charge, func() {
			player.Send("requestAcceptedAuto", user.DisplayName())
			user.Send("requestAcceptedFromAuto", player.DisplayName())
		}); err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	if !player.IsIgnoring(user) {
		g.requestTeleport(player, user, false)
		g.tellRequest(player, "teleportRequest", user.DisplayName(), true)
	}
	user.Send("requestSent", player.DisplayName())
	if user.HasPermission("essentials.tpacancel") {
		user.Send("typeTpacancel")
	}
	return nil
}

// Essentials: Commandtpahere
func (g *group) runTpahere(ctx *core.Ctx) error {
	user := ctx.User
	player, err := g.tpaTarget(ctx)
	if err != nil {
		return err
	}
	if err := g.checkWorld(user, world(user), world(player), world(user)); err != nil {
		return err
	}
	if r := g.outstandingRequest(player, user.Name(), false); r != nil && r.Here {
		return core.Tlf("requestSentAlready", player.DisplayName())
	}
	if !player.IsIgnoring(user) {
		g.requestTeleport(player, user, true)
		g.tellRequest(player, "teleportHereRequest", user.DisplayName(), true)
	}
	user.Send("requestSent", player.DisplayName())
	user.Send("typeTpacancel")
	return nil
}

// Essentials: Commandtpaall
func (g *group) runTpaall(ctx *core.Ctx) error {
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
	ctx.Send("teleportAAll")
	for _, u := range g.e.OnlineUsers() {
		if u.ID == target.ID || !u.Data.TeleportEnabled {
			continue
		}
		if isSender(ctx, target) && g.checkWorld(target, world(target), world(u), world(target)) != nil {
			continue
		}
		g.requestTeleport(u, target, true)
		g.tellRequest(u, "teleportHereRequest", target.DisplayName(), false)
	}
	return nil
}

// isAll reports whether arg means every request: * or all.
func isAll(args []string) bool {
	return len(args) > 0 && (args[0] == "*" || strings.EqualFold(args[0], "all"))
}

// Essentials: Commandtpaccept
func (g *group) runTpaccept(ctx *core.Ctx) error {
	user := ctx.User
	all := isAll(ctx.Args)
	if g.nextRequest(user, true, false, all) == nil {
		return core.Tlf("noPendingRequest")
	}
	if all {
		count := 0
		for r := g.nextRequest(user, true, true, true); r != nil; r = g.nextRequest(user, true, true, true) {
			if err := g.acceptRequest(ctx, r); err != nil {
				g.e.ShowError(ctx, err)
			} else {
				count++
			}
			user.RemoveTpRequest(r)
		}
		user.Send("requestAcceptedAll", count)
		return core.ErrNoCharge
	}
	user.Send("requestAccepted")
	var r *core.TpRequest
	if len(ctx.Args) > 0 {
		from, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		r = g.outstandingRequest(user, from.Name(), true)
	} else {
		r = g.nextRequest(user, true, false, false)
	}
	if err := g.acceptRequest(ctx, r); err != nil {
		return err
	}
	return core.ErrNoCharge
}

// acceptRequest carries out a request: the requester comes to the user, or
// for /tpahere the user goes where the requester was, the requester paying
// /tpaccept's cost. Essentials: Commandtpaccept.handleTeleport
func (g *group) acceptRequest(ctx *core.Ctx, r *core.TpRequest) error {
	user := ctx.User
	if r == nil {
		return core.Tlf("noPendingRequest")
	}
	requester := g.e.Users.Get(r.From)
	if !requester.Online() {
		user.RemoveTpRequest(r)
		return core.Tlf("noPendingRequest")
	}
	if r.Here {
		if !requester.HasPermission("essentials.tpahere") && !requester.HasPermission("essentials.tpaall") ||
			g.checkWorld(user, world(user), world(requester), world(user)) != nil {
			return core.Tlf("noPendingRequest")
		}
	} else if !requester.HasPermission("essentials.tpa") || g.checkWorld(user, world(user), world(requester), world(requester)) != nil {
		return core.Tlf("noPendingRequest")
	}
	charge := g.costOf(requester, "tpaccept")
	requester.Send("requestAcceptedFrom", user.DisplayName())
	var err error
	if r.Here {
		loc := r.Location
		err = g.teleport(requester, user, core.LocationTarget(loc), charge, func() {
			requester.Send("teleporting", locArgs(loc)...)
		})
	} else {
		requester.Send("teleportToPlayer", user.DisplayName())
		err = g.teleport(requester, requester, core.PlayerTarget{User: user}, charge, nil)
	}
	if err != nil {
		g.showErrorTo(requester, err)
		user.Send("pendingTeleportCancelled")
	}
	user.RemoveTpRequest(r)
	return nil
}

// Essentials: Commandtpdeny
func (g *group) runTpdeny(ctx *core.Ctx) error {
	user := ctx.User
	if g.nextRequest(user, false, false, false) == nil {
		return core.Tlf("noPendingRequest")
	}
	if isAll(ctx.Args) {
		count := 0
		for r := g.nextRequest(user, false, true, false); r != nil; r = g.nextRequest(user, false, true, false) {
			if p := g.e.Users.Get(r.From); p.Online() {
				p.Send("requestDeniedFrom", user.DisplayName())
			}
			user.RemoveTpRequest(r)
			count++
		}
		user.Send("requestDeniedAll", count)
		return nil
	}
	var r *core.TpRequest
	if len(ctx.Args) > 0 {
		from, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		r = g.outstandingRequest(user, from.Name(), false)
	} else {
		r = g.nextRequest(user, false, true, false)
	}
	if r == nil {
		return core.Tlf("noPendingRequest")
	}
	player := g.e.Users.Get(r.From)
	if !player.Online() {
		return core.Tlf("noPendingRequest")
	}
	user.Send("requestDenied")
	player.Send("requestDeniedFrom", user.DisplayName())
	user.RemoveTpRequest(r)
	return nil
}

// Essentials: Commandtpacancel
func (g *group) runTpacancel(ctx *core.Ctx) error {
	user := ctx.User
	if len(ctx.Args) == 0 {
		n := 0
		for _, u := range g.e.OnlineUsers() {
			if u.ID != user.ID && removeRequest(u, user.Name()) {
				n++
			}
		}
		if n == 0 {
			return core.Tlf("noPendingRequest")
		}
		user.Send("teleportRequestAllCancelled", n)
		return nil
	}
	target, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	if removeRequest(target, user.Name()) {
		user.Send("teleportRequestSpecificCancelled", target.Name())
	}
	return nil
}

// Essentials: Commandtptoggle
func toggleTeleport(ctx *core.Ctx, u *core.User, enabled *bool) {
	v := !u.Data.TeleportEnabled
	if enabled != nil {
		v = *enabled
	}
	u.Data.TeleportEnabled = v
	u.Save()
	key := "teleportationDisabled"
	if v {
		key = "teleportationEnabled"
	}
	u.Send(key)
	if !isSender(ctx, u) {
		ctx.Send(key+"For", u.DisplayName())
	}
}

// Essentials: Commandtpauto
func toggleAuto(ctx *core.Ctx, u *core.User, enabled *bool) {
	v := !u.Data.TeleportAuto
	if enabled != nil {
		v = *enabled
	}
	u.Data.TeleportAuto = v
	u.Save()
	key := "autoTeleportDisabled"
	if v {
		key = "autoTeleportEnabled"
	}
	u.Send(key)
	if v && !u.Data.TeleportEnabled {
		u.Send("teleportationDisabledWarning")
	}
	if !isSender(ctx, u) {
		ctx.Send(key+"For", u.DisplayName())
	}
}
