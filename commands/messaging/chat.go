package messaging

import (
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func (g *group) registerChat() {
	e := g.e
	none := func(*core.Ctx) []string { return nil }
	e.RegisterCommand(core.Cmd{
		// Essentials: Commandme
		Name:     "me",
		Run:      g.me,
		Complete: none,
	}, core.Cmd{
		// Essentials: Commandhelpop
		Name:     "helpop",
		Run:      g.helpop,
		Complete: none,
	}, core.Cmd{
		// Essentials: Commandbroadcast
		Name: "broadcast",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 1 {
				return core.ErrNotEnoughArgs
			}
			msg := strings.ReplaceAll(replaceFormat(ctx.FinalArg(0)), `\n`, "\n")
			e.Broadcast(nil, "", "broadcast", core.Raw(withLinks(sectionMini(msg))), ctx.SenderName())
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandbroadcastworld
		Name: "broadcastworld",
		Run:  g.broadcastWorld,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && (ctx.User == nil || e.Config.Bool("allow-world-in-broadcastworld", true)) {
				return core.WorldNames(e.Server)
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandmute
		Name: "mute",
		Run:  g.mute,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return ctx.CompletePlayers()
			}
			return core.Durations
		},
	}, core.Cmd{
		// Essentials: Commandafk
		Name: "afk",
		Run:  g.afk,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.afk.others") {
				return ctx.CompletePlayers()
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandignore
		Name:       "ignore",
		PlayerOnly: true,
		Run:        g.ignore,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return ctx.CompletePlayers()
			}
			return nil
		},
	})
}

// me shows an action to the players within chat.radius, or to everyone.
func (g *group) me(ctx *core.Ctx) error {
	u := ctx.User
	if u == nil {
		if len(ctx.Args) < 1 {
			return core.ErrNotEnoughArgs
		}
		g.e.Broadcast(nil, "", "action", "@", replaceFormat(ctx.FinalArg(0)))
		return nil
	}
	if u.IsMuted() {
		return muteError(u)
	}
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	msg := g.formatMessage(u, "essentials.chat", ctx.FinalArg(0))
	u.UpdateDisplayName()
	radius := g.e.Config.Int("chat.radius", g.e.Config.Int("chat-radius", 0))
	if radius < 1 {
		g.e.Broadcast(nil, "", "action", u.DisplayName(), msg)
		return nil
	}
	r2 := float64(radius) * float64(radius)
	loc := ctx.Player.Location()
	var out []*core.User
	for _, o := range g.e.OnlineUsers() {
		if o.ID == u.ID {
			out = append(out, o)
			continue
		}
		p, ok := o.Player()
		if !ok {
			continue
		}
		pl := p.Location()
		in := pl.World == loc.World && !o.IsIgnoring(u) && pl.Pos.DistanceSq(loc.Pos) <= r2
		if in || o.HasPermission("essentials.chat.spy") {
			out = append(out, o)
		}
	}
	// localNoOne is blank by default, which Essentials does not send.
	if len(out) < 2 && g.e.I18n.Format(g.locale(ctx.Sender), "localNoOne") != "" {
		ctx.Send("localNoOne")
	}
	for _, o := range out {
		o.Send("action", u.DisplayName(), msg)
	}
	return nil
}

// helpop sends a message to the players with essentials.helpop.receive.
func (g *group) helpop(ctx *core.Ctx) error {
	from := g.from(ctx)
	if ctx.User != nil {
		ctx.User.UpdateDisplayName()
	}
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	msg := stripFormat(ctx.FinalArg(0))
	g.e.Log.Info(g.e.TlString("helpOp", from.displayName(), msg))
	var recipients []*core.User
	for _, u := range g.e.OnlineUsers() {
		if u.HasPermission("essentials.helpop.receive") {
			recipients = append(recipients, u)
		}
	}
	if ctx.User != nil && !slices.ContainsFunc(recipients, func(r *core.User) bool { return r.ID == ctx.User.ID }) {
		ctx.Send("helpOp", from.displayName(), msg)
	}
	for _, r := range recipients {
		r.Send("helpOp", from.displayName(), msg)
	}
	return nil
}

// broadcastWorld broadcasts to the players of one world.
func (g *group) broadcastWorld(ctx *core.Ctx) error {
	if len(ctx.Args) < 2 {
		return core.ErrNotEnoughArgs
	}
	var world plugin.World
	var name, msg string
	if ctx.User != nil {
		world = ctx.Player.World()
		name = ctx.User.Nick()
		msg = ctx.FinalArg(0)
		if g.e.Config.Bool("allow-world-in-broadcastworld", true) {
			if w, ok := core.WorldByName(g.e.Server, ctx.Args[0]); ok {
				world, msg = w, ctx.FinalArg(1)
			}
		}
	} else {
		w, ok := core.WorldByName(g.e.Server, ctx.Args[0])
		if !ok {
			return core.Tlf("invalidWorld")
		}
		world, name, msg = w, core.EscapeTags(ctx.Sender.Name()), ctx.FinalArg(1)
	}
	if msg == "" {
		return core.ErrNotEnoughArgs
	}
	formatted := strings.ReplaceAll(replaceFormat(msg), `\n`, "\n")
	g.broadcastIn(world, "broadcast", core.Raw(withLinks(sectionMini(formatted))), core.Raw(name))
	return nil
}

// mute mutes or unmutes a player, for a time and with a reason; the label
// unmute only unmutes.
func (g *group) mute(ctx *core.Ctx) error {
	e := g.e
	unmute := strings.Contains(strings.ToLower(ctx.Label), "unmute")
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	nomatch := false
	u, err := e.MatchUser(nil, ctx.Args[0], true, true)
	if err != nil {
		// Essentials mutes an unknown name as an OfflinePlayerStub, whose
		// player is Bukkit's getOfflinePlayer(name).
		nomatch = true
		u = g.offlineUser(ctx.Args[0])
	}
	muted := u.IsMuted()
	if unmute && !muted {
		ctx.Send("playerNotMuted", u.DisplayName())
		return nil
	}
	if !u.Online() && ctx.User != nil {
		if !ctx.Has("essentials.mute.offline") {
			return core.Tlf("muteExemptOffline")
		}
	} else if !unmute && u.HasPermission("essentials.mute.exempt") {
		return core.Tlf("muteExempt")
	}
	var until time.Time
	var reason *string
	if !unmute && len(ctx.Args) > 1 {
		r := ctx.FinalArg(1)
		if t, err := core.ParseDateDiff(ctx.Args[1], true); err == nil {
			until, r = t, ctx.FinalArg(2)
		}
		reason = &r
		if limit := e.Settings.MaxMuteTime(); limit > 0 && time.Until(until) > limit && ctx.User != nil && !ctx.Has("essentials.mute.unlimited") {
			ctx.Send("oversizedMute")
			return core.ErrNoCharge
		}
	}
	newReason := u.Data.MuteReason
	if reason != nil {
		newReason = *reason
		muted = true
	} else {
		muted = !muted
		if !muted {
			newReason = ""
		}
	}
	u.SetMuted(muted, until, newReason)
	timed := !until.IsZero()
	muteTime := ""
	if timed {
		muteTime = core.FormatDateDiff(until)
	}
	if nomatch {
		ctx.Send("userUnknown", u.Name())
	}
	by := ctx.Sender.Name()
	if !muted {
		ctx.Send("unmutedPlayer", u.DisplayName())
		u.Send("playerUnmuted")
		e.Broadcast(nil, "essentials.mute.notify", "unmuteNotify", by, u.Name())
		return nil
	}
	switch {
	case timed && newReason == "":
		ctx.Send("mutedPlayerFor", u.DisplayName(), muteTime)
		u.Send("playerMutedFor", muteTime)
	case timed:
		ctx.Send("mutedPlayerForReason", u.DisplayName(), muteTime, newReason)
		u.Send("playerMutedForReason", muteTime, newReason)
	case newReason == "":
		ctx.Send("mutedPlayer", u.DisplayName())
		u.Send("playerMuted")
	default:
		ctx.Send("mutedPlayerReason", u.DisplayName(), newReason)
		u.Send("playerMutedReason", newReason)
	}
	key, args := "muteNotify", []any{by, u.Name(), muteTime}
	switch {
	case newReason != "" && timed:
		key, args = "muteNotifyForReason", []any{by, u.Name(), muteTime, newReason}
	case newReason != "":
		// Essentials passes the reason to muteNotify, which does not show it.
		args = []any{by, u.Name(), newReason}
	case timed:
		key = "muteNotifyFor"
	}
	e.Broadcast(nil, "essentials.mute.notify", key, args...)
	return nil
}

// afk toggles away status, for the sender or, with essentials.afk.others,
// another player.
func (g *group) afk(ctx *core.Ctx) error {
	if ctx.User == nil {
		if len(ctx.Args) == 0 {
			return core.ErrNotEnoughArgs
		}
		u, err := g.e.MatchUser(nil, ctx.Args[0], true, false)
		if err != nil {
			return err
		}
		var msg *string
		if len(ctx.Args) > 1 {
			msg = new(ctx.FinalArg(1))
		}
		return g.toggleAfk(nil, u, msg)
	}
	if len(ctx.Args) > 0 && ctx.Has("essentials.afk.others") {
		target := ctx.User
		var msg *string
		if u, err := g.getPlayer(ctx, ctx.Args[0], false); err == nil {
			target = u
			if len(ctx.Args) > 1 {
				msg = new(ctx.FinalArg(1))
			}
		} else {
			msg = new(ctx.FinalArg(0))
		}
		return g.toggleAfk(ctx.User, target, msg)
	}
	var msg *string
	if len(ctx.Args) > 0 {
		msg = new(ctx.FinalArg(0))
	}
	return g.toggleAfk(ctx.User, ctx.User, msg)
}

// toggleAfk flips u's away status; a message needs essentials.afk.message
// and a sender who is not muted. Essentials: Commandafk.toggleAfk
func (g *group) toggleAfk(from, u *core.User, msg *string) error {
	if msg != nil && from != nil {
		if from.IsMuted() {
			return muteError(from)
		}
		if !from.HasPermission("essentials.afk.message") {
			return core.Tlf("noPermToAFKMessage")
		}
	}
	u.UpdateDisplayName()
	m := ""
	if msg != nil {
		m = *msg
	}
	if u.IsAfk() {
		u.LastActivity = time.Now()
		u.SetAfk(false, "")
	} else {
		u.SetAfk(true, m)
	}
	u.UpdateDisplayName()
	return nil
}

// ignore lists, adds or removes ignored players.
func (g *group) ignore(ctx *core.Ctx) error {
	u := ctx.User
	if len(ctx.Args) == 0 {
		var names []string
		for _, s := range u.Data.Ignore {
			id, err := parseUUID(s)
			if err != nil {
				continue
			}
			if n := strings.TrimSpace(g.e.Users.Get(id).Name()); n != "" {
				names = append(names, n)
			}
		}
		if list := strings.Join(names, " "); list != "" {
			ctx.Send("ignoredList", list)
		} else {
			ctx.Send("noIgnored", list)
		}
		return nil
	}
	other, err := g.e.MatchUser(nil, ctx.Args[0], false, true)
	if err != nil {
		return err
	}
	switch {
	case other.HasPermission("essentials.chat.ignoreexempt"):
		ctx.Send("ignoreExempt")
	case u.IsIgnoring(other):
		u.SetIgnoring(other, false)
		ctx.Send("unignorePlayer", other.Name())
	case u.ID == other.ID:
		ctx.Send("ignoreYourself")
	default:
		u.SetIgnoring(other, true)
		ctx.Send("ignorePlayer", other.Name())
	}
	return nil
}
