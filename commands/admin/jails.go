package admin

import (
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func registerJails(e *core.Essentials) {
	jailNames := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.Ess.Jails.Names()
		}
		return nil
	}
	e.RegisterCommand(
		core.Cmd{Name: "jails", Run: runJails},
		core.Cmd{Name: "setjail", Run: runSetJail, PlayerOnly: true},
		core.Cmd{Name: "deljail", Run: runDelJail, Complete: jailNames},
		core.Cmd{Name: "togglejail", Run: runToggleJail, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return ctx.CompletePlayers()
			case 2:
				return ctx.Ess.Jails.Names()
			}
			return core.Durations
		}},
		core.Cmd{Name: "jailedplayers", Run: runJailedPlayers},
	)
	plugin.Listen(e.Server, func(ev *plugin.PlayerJoinEvent) { convertOnlineJail(e.Users.Of(ev.Player)) },
		plugin.ListenOptions{Priority: plugin.Lowest})
}

// Essentials: Commandjails
func runJails(ctx *core.Ctx) error {
	names := ctx.Ess.Jails.Names()
	if len(names) == 0 {
		ctx.Send("noJailsDefined")
		return nil
	}
	ctx.Send("jailList", strings.Join(names, " "))
	return nil
}

// sanitize replaces the characters Essentials does not show with _.
// Essentials: StringUtil.sanitizeString
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r',
			r >= 0x20 && r <= 0x7e, r == 0x85,
			r >= 0xa0 && r <= 0xd7ff, r >= 0xe000 && r <= 0xfffc:
			return r
		}
		return '_'
	}, s)
}

// Essentials: Commandsetjail
func runSetJail(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	if err := ctx.Ess.Jails.Set(ctx.Args[0], ctx.Player.Location()); err != nil {
		return err
	}
	ctx.Send("jailSet", sanitize(ctx.Args[0]))
	return nil
}

// Essentials: Commanddeljail
func runDelJail(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	if err := ctx.Ess.Jails.Remove(ctx.Args[0]); err != nil {
		return err
	}
	ctx.Send("deleteJail", ctx.Args[0])
	return nil
}

// Essentials: Commandjailedplayers
func runJailedPlayers(ctx *core.Ctx) error {
	var names []string
	for _, id := range ctx.Ess.Users.All() {
		if u := ctx.Ess.Users.Get(id); u.Data.Jailed {
			names = append(names, u.Name())
		}
	}
	if len(names) == 0 {
		ctx.Send("jailedPlayersEmpty")
		return nil
	}
	slices.Sort(names)
	ctx.Send("jailedPlayersList", strings.Join(names, ", "))
	return nil
}

// Essentials: Commandtogglejail
func runToggleJail(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	player, err := e.MatchUser(nil, ctx.Args[0], true, true)
	if err != nil {
		return err
	}
	by := ctx.Sender.Name()
	if !player.Data.Jailed {
		if !player.Online() && isPlayerWithout(ctx, "essentials.togglejail.offline") {
			ctx.Send("mayNotJailOffline")
			return nil
		}
		if player.HasPermission("essentials.jail.exempt") {
			ctx.Send("mayNotJail")
			return nil
		}
		var jail string
		switch names := e.Jails.Names(); {
		case len(ctx.Args) > 1:
			jail = ctx.Args[1]
		case len(names) == 1:
			jail = names[0]
		default:
			return core.ErrNotEnoughArgs
		}
		if _, err := e.Jails.Get(jail); err != nil {
			return err
		}
		var until time.Time
		if len(ctx.Args) > 2 {
			if until, err = core.ParseDateDiff(ctx.FinalArg(2), true); err != nil {
				return err
			}
		}
		if player.Online() {
			if err := e.Jails.SendToJail(player, jail); err != nil {
				return err
			}
		}
		player.Data.Jailed = true
		player.Send("userJailed")
		player.Data.Jail = jail
		if len(ctx.Args) > 2 {
			setJailTime(e, player, until)
		}
		player.Save()
		if len(ctx.Args) > 2 {
			expiry := core.FormatDateDiff(until)
			ctx.Send("playerJailedFor", player.Name(), expiry)
			e.Broadcast(nil, "essentials.jail.notify", "jailNotifyJailedFor", player.Name(), expiry, by)
		} else {
			ctx.Send("playerJailed", player.Name())
			e.Broadcast(nil, "essentials.jail.notify", "jailNotifyJailed", player.Name(), by, by)
		}
		return nil
	}
	if len(ctx.Args) >= 2 && !strings.EqualFold(ctx.Args[1], player.Data.Jail) {
		ctx.Send("jailAlreadyIncarcerated", player.Data.Jail)
		return nil
	}
	if len(ctx.Args) >= 2 {
		until, err := core.ParseDateDiff(ctx.FinalArg(2), true)
		if err != nil {
			return err
		}
		setJailTime(e, player, until)
		player.Save()
		expiry := core.FormatDateDiff(until)
		ctx.Send("jailSentenceExtended", expiry)
		e.Broadcast(nil, "essentials.jail.notify", "jailNotifySentenceExtended", player.Name(), expiry, by)
		return nil
	}
	player.Data.Jailed = false
	player.Data.Timestamps.Jail = 0
	player.Data.Timestamps.OnlineJail = 0
	player.Send("jailReleasedPlayerNotify")
	player.Data.Jail = ""
	player.Save()
	if p, ok := player.Player(); ok {
		switch strings.ToLower(e.Config.String("teleport-when-freed", "back")) {
		case "back":
			loc, ok := player.LastLocation()
			if !ok || player.TeleportNow(core.LocationTarget(loc), plugin.TeleportCommand) != nil {
				respawn(e, player, p)
			}
		case "spawn":
			respawn(e, player, p)
		}
	}
	ctx.Send("jailReleased", player.Name())
	return nil
}

// respawn sends the user where they would respawn: their bed or anchor,
// else the spawn. Essentials: AsyncTeleport.respawn
func respawn(e *core.Essentials, u *core.User, p plugin.Player) {
	loc, ok := p.RespawnPoint()
	if !ok {
		loc, ok = e.SpawnFor(u)
	}
	if !ok {
		loc = p.World().SpawnPoint()
	}
	u.TeleportNow(core.LocationTarget(loc), plugin.TeleportPlugin)
}

// setJailTime sets when the user is released. With jail-online-time,
// timestamps.jail is the sentence as a duration (a time after 1970, which
// has always passed) and timestamps.onlinejail the play_time statistic at
// which it is served; core's timer releases on both. Essentials:
// Commandtogglejail
func setJailTime(e *core.Essentials, u *core.User, until time.Time) {
	if !e.Config.Bool("jail-online-time", false) {
		u.Data.Timestamps.Jail = until.UnixMilli()
		u.Data.Timestamps.OnlineJail = 0
		return
	}
	// Essentials parses the time from the epoch; the time from now, to the
	// second the durations are written in, is the same sentence.
	d := time.Until(until).Round(time.Second).Milliseconds()
	u.Data.Timestamps.Jail = d
	u.Data.Timestamps.OnlineJail = int64(u.PlayTime()) + d/50
}

// legacyOnlineJail is the userdata key under which earlier versions of the
// port kept the sentence, in ticks, of a player jailed offline with
// jail-online-time.
const legacyOnlineJail = "jail-online-pending-ticks"

// convertOnlineJail turns an online-time sentence saved by earlier versions
// of the port into Essentials' form when the player joins: those kept
// timestamps.jail at the wall-clock time the sentence would end if the
// player stayed online, and an offline player's sentence in
// extra.jail-online-pending-ticks.
func convertOnlineJail(u *core.User) {
	d := &u.Data.Timestamps
	if v, ok := u.Data.Extra[legacyOnlineJail]; ok {
		delete(u.Data.Extra, legacyOnlineJail)
		if ticks := toInt64(v); u.Data.Jailed && ticks > 0 {
			d.OnlineJail = int64(u.PlayTime()) + ticks
			d.Jail = ticks * 50
		}
		u.Save()
		return
	}
	// A duration is at most ten years; a wall-clock time is after 2001.
	if u.Data.Jailed && d.OnlineJail > 0 && d.Jail > 1e12 {
		d.Jail = max(d.OnlineJail-int64(u.PlayTime()), 1) * 50
		u.Save()
	}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case uint64:
		return int64(n)
	}
	return 0
}
