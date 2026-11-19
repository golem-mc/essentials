package admin

import (
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func registerBans(e *core.Essentials) {
	firstPlayers := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return nil
	}
	playersThenDurations := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return core.Durations
	}
	e.RegisterCommand(
		core.Cmd{Name: "ban", Run: runBan, Complete: firstPlayers},
		core.Cmd{Name: "tempban", Run: runTempban, Complete: playersThenDurations},
		core.Cmd{Name: "banip", Run: runBanIP, Complete: firstPlayers},
		core.Cmd{Name: "tempbanip", Run: runTempbanIP, Complete: playersThenDurations},
		core.Cmd{Name: "unban", Run: runUnban},
		core.Cmd{Name: "unbanip", Run: runUnbanIP},
	)
	plugin.Listen(e.Server, func(ev *plugin.PlayerLoginEvent) { onLoginBanned(e, ev) },
		plugin.ListenOptions{Priority: plugin.Highest})
}

// banReason answers the reason typed from args[from] on, or
// defaultBanReason.
func banReason(ctx *core.Ctx, from int) string {
	if len(ctx.Args) > from {
		return reasonText(ctx.FinalArg(from))
	}
	return ctx.Ess.TlString("defaultBanReason")
}

// kickBanned kicks the online user with Essentials' message, after the ban
// is recorded.
func kickBanned(u *core.User, reason plugin.Text) {
	if p, ok := u.Player(); ok {
		p.Kick(reason)
	}
}

// kickAddress kicks every player connected from ip.
func kickAddress(e *core.Essentials, ip string, reason plugin.Text) {
	for _, p := range e.Server.Players() {
		if strings.EqualFold(hostOf(p), ip) {
			p.Kick(reason)
		}
	}
}

// cachedUser answers the user of a player the server knows by name from
// usercache.json, for names Essentials' own user map does not know.
func cachedUser(e *core.Essentials, name string) (*core.User, bool) {
	op, ok := e.Server.OfflinePlayerNamed(name)
	if !ok {
		return nil, false
	}
	return e.Users.Get(op.UUID()), true
}

// Essentials: Commandban
func runBan(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	user, err := e.MatchUser(nil, ctx.Args[0], true, true)
	nomatch := false
	if err != nil {
		// A name the server has never seen is banned by name, as Bukkit's
		// NAME ban list does; it takes the UUID at the player's first login.
		user, nomatch = cachedUser(e, ctx.Args[0])
		nomatch = !nomatch
	}
	name, target := ctx.Args[0], ctx.Args[0]
	if !nomatch {
		name, target = user.Name(), user.ID.String()
	}
	if nomatch || !user.Online() {
		if isPlayerWithout(ctx, "essentials.ban.offline") {
			return core.Tlf("banExemptOffline")
		}
	} else if ctx.User != nil && user.HasPermission("essentials.ban.exempt") {
		return core.Tlf("banExempt")
	}
	sender := ctx.SenderName()
	reason := banReason(ctx, 1)
	e.Server.Bans().Ban(target, reason, time.Time{}, sourceName(ctx))
	if !nomatch {
		kickBanned(user, e.Tl("banFormat", reason, sender))
	}
	if nomatch {
		ctx.Send("userUnknown", name)
	}
	e.Broadcast(nil, "essentials.ban.notify", "playerBanned", sender, name, reason)
	return nil
}

// tempbanTime reads a tempban's time and reason from args[1] on; ok is
// false when the sender may not ban for that long (oversizedTempban was
// sent).
func tempbanTime(ctx *core.Ctx) (until time.Time, reason string, ok bool, err error) {
	t := ctx.FinalArg(1)
	until, err = core.ParseDateDiff(t, true)
	if err != nil {
		return time.Time{}, "", false, err
	}
	reason = replaceFormat(removeTimePattern(t))
	if max := ctx.Ess.Settings.MaxTempbanTime(); max > 0 && time.Until(until) > max &&
		isPlayerWithout(ctx, "essentials.tempban.unlimited") {
		ctx.Send("oversizedTempban")
		return time.Time{}, "", false, nil
	}
	if utf8.RuneCountInString(reason) < 2 {
		reason = ctx.Ess.TlString("defaultBanReason")
	}
	return until, reason, true, nil
}

// Essentials: Commandtempban
func runTempban(ctx *core.Ctx) error {
	if len(ctx.Args) < 2 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	user, err := e.MatchUser(nil, ctx.Args[0], true, true)
	if err != nil {
		var cached bool
		if user, cached = cachedUser(e, ctx.Args[0]); !cached {
			return err
		}
	}
	if !user.Online() && isPlayerWithout(ctx, "essentials.tempban.offline") {
		ctx.Send("tempbanExemptOffline")
		return nil
	} else if ctx.User != nil && user.HasPermission("essentials.tempban.exempt") {
		ctx.Send("tempbanExempt")
		return nil
	}
	until, reason, ok, err := tempbanTime(ctx)
	if !ok {
		return err
	}
	sender := ctx.SenderName()
	expiry := core.FormatDateDiff(until)
	e.Server.Bans().Ban(user.ID.String(), reason, until, sourceName(ctx))
	kickBanned(user, user.Tl("tempBanned", expiry, sender, reason))
	e.Broadcast(nil, "essentials.ban.notify", "playerTempBanned", sender, user.Name(), expiry, reason)
	return nil
}

// Essentials: Commandbanip
func runBanIP(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	sender := ctx.SenderName()
	ip, err := ipOf(ctx)
	if err != nil {
		return err
	}
	reason := banReason(ctx, 1)
	e.Server.IPBans().Ban(ip, reason, time.Time{}, sourceName(ctx))
	kickAddress(e, ip, e.Tl("banFormat", reason, sender))
	e.Broadcast(nil, "essentials.banip.notify", "playerBanIpAddress", sender, ip, reason)
	return nil
}

// Essentials: Commandtempbanip
func runTempbanIP(ctx *core.Ctx) error {
	if len(ctx.Args) < 2 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	sender := ctx.SenderName()
	ip, err := ipOf(ctx)
	if err != nil {
		return err
	}
	until, reason, ok, err := tempbanTime(ctx)
	if !ok {
		return err
	}
	e.Server.IPBans().Ban(ip, reason, until, sourceName(ctx))
	kickAddress(e, ip, e.Tl("banFormat", reason, sender))
	// Essentials passes the reason as {2} and the time as {3}.
	e.Broadcast(nil, "essentials.banip.notify", "playerTempBanIpAddress", sender, ip, reason, core.FormatDateDiff(until), reason)
	return nil
}

// Essentials: Commandunban
func runUnban(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	var target, name string
	if user, err := e.MatchUser(nil, ctx.Args[0], true, true); err == nil {
		target, name = user.ID.String(), user.Name()
	} else {
		var ok bool
		if target, name, ok = bannedNamed(e, ctx.Args[0]); !ok {
			return core.Tlf("playerNeverOnServer", ctx.Args[0])
		}
	}
	e.Server.Bans().Pardon(target)
	e.Broadcast(nil, "essentials.ban.notify", "playerUnbanned", ctx.SenderName(), name)
	return nil
}

// bannedNamed finds the ban of the player named name: by the UUID
// usercache.json gives the name, else by the name the ban list holds.
// It answers the target to pardon and the player's name.
func bannedNamed(e *core.Essentials, name string) (target, display string, ok bool) {
	if op, ok := e.Server.OfflinePlayerNamed(name); ok {
		if _, banned := e.Server.Bans().Entry(op.UUID().String()); banned {
			return op.UUID().String(), op.Name(), true
		}
	}
	ban, ok := e.Server.Bans().Entry(name)
	if !ok {
		return "", "", false
	}
	if ban.Target != "" {
		return ban.Target, ban.Name, true
	}
	return ban.Name, ban.Name, true
}

// Essentials: Commandunbanip
func runUnbanIP(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	ip, err := ipOf(ctx)
	if err != nil {
		return err
	}
	ctx.Ess.Server.IPBans().Pardon(ip)
	ctx.Ess.Broadcast(nil, "essentials.banip.notify", "playerUnbanIpAddress", ctx.SenderName(), ip)
	return nil
}

// onLoginBanned shows a banned player why, with Essentials' messages. It
// runs after core's listener, which uses the in-game ban messages.
// Essentials: EssentialsPlayerListener.onPlayerKickBanned
func onLoginBanned(e *core.Essentials, ev *plugin.PlayerLoginEvent) {
	if ev.Result != plugin.LoginBanned {
		return
	}
	key, ipKey := "banJoin", ""
	b, ok := e.Server.Bans().Entry(ev.Player.UUID().String())
	if !ok {
		ip := ""
		if ev.Address != nil {
			if tcp, isTCP := ev.Address.(*net.TCPAddr); isTCP {
				ip = tcp.IP.String()
			} else if h, _, err := net.SplitHostPort(ev.Address.String()); err == nil {
				ip = h
			}
		}
		if b, ok = e.Server.IPBans().Entry(ip); !ok {
			return
		}
		key, ipKey = "banIpJoin", "tempbanIpJoin"
	}
	if !b.Expires.IsZero() {
		if ipKey != "" {
			key = ipKey
		} else {
			key = "tempbanJoin"
		}
		ev.Reason = e.Tl(key, core.FormatDateDiff(b.Expires), b.Reason)
		return
	}
	ev.Reason = e.Tl(key, b.Reason)
}
