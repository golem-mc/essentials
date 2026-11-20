package core

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
)

// SpawnProvider answers where a user's spawn is; the spawn module sets it,
// and without it the world spawn is used. Essentials: SpawnStorage
type SpawnProvider func(u *User) (plugin.Location, bool)

// SetSpawnProvider sets where SpawnFor looks.
func (e *Essentials) SetSpawnProvider(f SpawnProvider) { e.spawn = f }

// SpawnFor answers the user's spawn: the spawn module's, else the
// overworld's spawn point.
func (e *Essentials) SpawnFor(u *User) (plugin.Location, bool) {
	if e.spawn != nil {
		if loc, ok := e.spawn(u); ok {
			return loc, true
		}
	}
	return e.Server.Overworld().SpawnPoint(), true
}

// registerCoreListeners registers the listeners every command relies on:
// user lifecycle, AFK, mutes, ignores, god mode, /back on death, command
// cooldowns and social spy. Essentials: EssentialsPlayerListener and
// EssentialsEntityListener
func (e *Essentials) registerCoreListeners() {
	s := e.Server
	if eff, ok := s.Registries().Effect(plugin.MinecraftKey("invisibility")); ok {
		e.invisibility = eff
	}
	plugin.Listen(s, e.onJoin, plugin.ListenOptions{Priority: plugin.Highest})
	plugin.Listen(s, e.onQuit, plugin.ListenOptions{Priority: plugin.Highest})
	plugin.Listen(s, e.onChat, plugin.ListenOptions{Priority: plugin.Lowest})
	plugin.Listen(s, e.onMove, plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true})
	plugin.Listen(s, e.onCommand, plugin.ListenOptions{Priority: plugin.Lowest, IgnoreCancelled: true})
	plugin.Listen(s, e.onTeleport, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	plugin.Listen(s, e.onDeath, plugin.ListenOptions{Priority: plugin.Low})
	plugin.Listen(s, e.onDamage, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, func(ev *plugin.FoodLevelChangeEvent) {
		u := e.Users.Of(ev.Player)
		if u.IsGodMode() {
			ev.Player.SetFood(20)
			ev.Player.SetSaturation(10)
			ev.SetCancelled(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, func(ev *plugin.EntityRegainHealthEvent) {
		if p, ok := ev.Entity.(plugin.Player); ok && ev.Reason == plugin.RegainSatiated &&
			e.Users.Of(p).IsAfk() && e.Settings.FreezeAfkPlayers() {
			ev.SetCancelled(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, func(ev *plugin.PlayerInteractEvent) {
		if e.Settings.CancelAfkOnInteract() {
			e.Users.Of(ev.Player).UpdateActivity(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor})
	plugin.Listen(s, func(ev *plugin.EntityPickupItemEvent) {
		p, ok := ev.Entity.(plugin.Player)
		if !ok {
			return
		}
		u := e.Users.Of(p)
		if (e.Settings.DisableItemPickupAfk() && u.IsAfk()) || u.Vanished && !u.HasPermission("essentials.vanish.interact") {
			ev.SetCancelled(true)
		}
	}, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, func(ev *plugin.PlayerChangedWorldEvent) {
		u := e.Users.Of(ev.Player)
		e.updateCompass(u, ev.Player.Location())
		if u.IsGodMode() || !u.Data.GodMode {
			return
		}
		u.Send("noGodWorldWarning")
	}, plugin.ListenOptions{Priority: plugin.Monitor})
	plugin.Listen(s, func(ev *plugin.PlayerRespawnEvent) {
		e.updateCompass(e.Users.Of(ev.Player), ev.Location)
	})
	// The ban messages on login are the admin group's (bans.go).
	plugin.Listen(s, func(ev *plugin.PlayerLoginEvent) {
		if ev.Result == plugin.LoginFull && ev.Player.HasPermission("essentials.joinfullserver") {
			ev.Result = plugin.LoginAllowed
		}
	}, plugin.ListenOptions{Priority: plugin.High})
}

// updateCompass points the user's compass at their first home in the
// world of at (else their first home), or their bed when they have none.
// Essentials: EssentialsPlayerListener.updateCompass
func (e *Essentials) updateCompass(u *User, at plugin.Location) {
	p, ok := u.Player()
	if !ok || e.Config.Bool("compass-towards-home-perm", false) && !u.HasPermission("essentials.home.compass") {
		return
	}
	var first *plugin.Location
	for _, name := range u.HomeNames() {
		home, _ := u.Home(name)
		loc, ok := home.Resolve(e.Server)
		if !ok {
			continue
		}
		if loc.World == at.World {
			p.SetCompassTarget(&loc)
			return
		}
		if first == nil {
			first = &loc
		}
	}
	if first == nil {
		if bed, ok := p.RespawnPoint(); ok {
			first = &bed
		}
	}
	if first != nil {
		p.SetCompassTarget(first)
	}
}

func (e *Essentials) onJoin(ev *plugin.PlayerJoinEvent) {
	p := ev.Player
	u := e.Users.Of(p)
	now := time.Now()
	prev := u.Data.LastAccountName
	u.Data.LastAccountName = p.Name()
	newName := prev != "" && prev != p.Name()
	if newName {
		if !slices.Contains(u.Data.PastUsernames, prev) {
			u.Data.PastUsernames = append(u.Data.PastUsernames, prev)
		}
		if e.Config.Bool("reset-nick-on-name-change", false) {
			u.Data.Nickname = ""
		}
	}
	e.Users.Rename(u.ID, p.Name())
	u.Data.IPAddress = hostOf(p)
	u.Data.Timestamps.Login = now.UnixMilli()
	u.Session = map[string]any{}
	u.LastActivity = now
	u.Data.Afk = false
	u.Teleports = nil
	u.Save()
	u.UpdateDisplayName()

	for _, o := range e.OnlineUsers() {
		if o.Vanished && o.ID != u.ID && !p.HasPermission("essentials.vanish.see") {
			if op, ok := o.Player(); ok {
				p.Hide(op)
			}
		}
	}
	silent := e.Config.Bool("allow-silent-join-quit", false)
	switch {
	case silent && (u.HasPermission("essentials.silentjoin") || u.HasPermission("essentials.silentjoin.vanish")):
		if u.HasPermission("essentials.silentjoin.vanish") {
			u.SetVanished(true)
		}
		ev.Message = nil
	case e.hideJoinQuit():
		ev.Message = nil
	case ev.Message != nil && e.customMessage("custom-join-message") != "":
		format := e.customMessage("custom-join-message")
		if newName && e.customMessage("custom-new-username-message") != "" {
			format = e.customMessage("custom-new-username-message")
		}
		msg := strings.NewReplacer("{PLAYER}", u.Nick(), "{USERNAME}", u.Name(), "{OLDUSERNAME}", prev,
			"{UNIQUE}", itoa(len(e.Users.All())), "{ONLINE}", itoa(len(e.Server.Players())),
			"{UPTIME}", FormatDateDiff(startTime), "{PREFIX}", "", "{SUFFIX}", "").Replace(format)
		if msg == "" {
			ev.Message = nil
		} else {
			t := e.Mini(msg)
			ev.Message = &t
		}
	}

	if d := e.Config.Int("delay-motd", 0); d >= 0 && p.HasPermission("essentials.motd") {
		show := func() {
			if !u.Online() {
				return
			}
			if lines := e.TextLines("motd.txt", u); len(lines) > 0 {
				e.ShowText(p, lines, "", "", "", true)
			}
		}
		if d/50 > 0 {
			e.Server.Scheduler().RunLater(d/50, show)
		} else {
			show()
		}
	}
	if !slices.Contains(e.Settings.DisabledCommands(), "mail") && p.HasPermission("essentials.mail") {
		if len(u.UnreadMail()) == 0 {
			if e.Config.Bool("notify-no-new-mail", true) {
				u.Send("noNewMail")
			}
		} else {
			e.NotifyOfMail(u)
		}
	}
	if u.Data.FlyMode && p.HasPermission("essentials.fly") {
		p.SetAllowFlight(true)
		if e.Config.Bool("send-fly-enable-on-join", true) {
			u.Send("flyMode", e.TlString("enabled"), u.DisplayName())
		}
	}
	if p.HasPermission("essentials.fly.safelogin") && e.shouldFly(p.Location()) {
		p.SetAllowFlight(true)
		p.SetFlying(true)
	}
	if !p.HasPermission("essentials.speed") {
		p.SetFlySpeed(0.1)
		p.SetWalkSpeed(0.2)
	}
	if u.Data.SocialSpy && !p.HasPermission("essentials.socialspy") {
		u.Data.SocialSpy = false
	}
	if u.Data.FlyMode && !p.HasPermission("essentials.fly") {
		u.Data.FlyMode = false
	}
	if u.Data.GodMode && !p.HasPermission("essentials.god") {
		u.Data.GodMode = false
	}
}

func (e *Essentials) customMessage(key string) string {
	m := e.Config.String(key, "none")
	if strings.EqualFold(m, "none") {
		return ""
	}
	return m
}

func (e *Essentials) hideJoinQuit() bool {
	n := e.Config.Int("hide-join-quit-messages-above", -1)
	return n >= 0 && len(e.Server.Players()) > n
}

func itoa(n int) string { return FormatAmount(decimalOf(n)) }

func (e *Essentials) onQuit(ev *plugin.PlayerQuitEvent) {
	p := ev.Player
	u := e.Users.Of(p)
	if e.hideJoinQuit() || e.Config.Bool("allow-silent-join-quit", false) && u.HasPermission("essentials.silentquit") {
		ev.Message = nil
	} else if f := e.customMessage("custom-quit-message"); f != "" && ev.Message != nil {
		msg := strings.NewReplacer("{PLAYER}", u.Nick(), "{USERNAME}", u.Name(), "{ONLINE}", itoa(len(e.Server.Players())-1),
			"{UPTIME}", FormatDateDiff(startTime), "{PREFIX}", "", "{SUFFIX}", "").Replace(f)
		if msg == "" {
			ev.Message = nil
		} else {
			t := e.Mini(msg)
			ev.Message = &t
		}
	}
	if e.Settings.RemoveGodOnDisconnect() && u.Data.GodMode {
		u.Data.GodMode = false
	}
	if u.Vanished {
		u.Vanished = false
		for _, o := range e.Server.Players() {
			o.Show(p)
		}
	}
	l := LazyOf(p.Location())
	u.Data.LogoutLocation = &l
	u.cancelPending(false)
	u.Data.Afk = false
	if !u.Hidden {
		u.Data.Timestamps.Logout = time.Now().UnixMilli()
	}
	u.Save()
}

func (e *Essentials) onChat(ev *plugin.PlayerChatEvent) {
	u := e.Users.Of(ev.Player)
	if u.IsMuted() {
		ev.SetCancelled(true)
		e.NotifyMuted(u)
		e.Log.Info(e.TlString("mutedUserSpeaks", u.Name(), ev.Message))
	}
	ev.Recipients = slices.DeleteFunc(ev.Recipients, func(r plugin.Player) bool {
		return e.Users.Of(r).IsIgnoring(u)
	})
	if e.Settings.CancelAfkOnChat() {
		u.UpdateActivity(true)
	}
	u.UpdateDisplayName()
}

// NotifyMuted tells a muted user why they cannot talk. Essentials:
// User.notifyMuted
func (e *Essentials) NotifyMuted(u *User) {
	reason := u.Data.MuteReason
	switch {
	case u.Data.Timestamps.Mute > 0 && reason != "":
		u.Send("voiceSilencedReasonTime", FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Mute)), reason)
	case u.Data.Timestamps.Mute > 0:
		u.Send("voiceSilencedTime", FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Mute)))
	case reason != "":
		u.Send("voiceSilencedReason", reason)
	default:
		u.Send("voiceSilenced")
	}
}

func (e *Essentials) onMove(ev *plugin.PlayerMoveEvent) {
	from, to := ev.From.Pos.Block(), ev.To.Pos.Block()
	if from == to {
		return
	}
	u := e.Users.Of(ev.Player)
	if !e.Settings.CancelAfkOnMove() && !e.Settings.FreezeAfkPlayers() {
		return
	}
	if u.IsAfk() && e.Settings.FreezeAfkPlayers() {
		if ev.To.Pos.Y >= float64(from.Y+1) {
			u.UpdateActivity(true)
			return
		}
		t := ev.To
		t.Pos = ev.From.Pos
		if !ev.Player.AllowFlight() {
			if safe, err := e.SafeDestination(t); err == nil {
				t = safe
			}
		}
		ev.To = t
		return
	}
	afk := u.AfkPosition
	if afk.World == nil || afk.World != ev.To.World || afk.Pos.DistanceSq(ev.To.Pos) > 9 {
		if e.Settings.CancelAfkOnMove() {
			u.UpdateActivity(true)
		}
	}
}

// commandCooldown is a command-cooldowns entry from config.yml.
type commandCooldown struct {
	pattern *regexp.Regexp
	raw     string
	d       time.Duration
}

// cooldowns reads command-cooldowns: a regex (between ^ and $, or a
// command name with an optional *) to seconds. Essentials:
// Settings._getCommandCooldowns
func (e *Essentials) cooldowns() []commandCooldown {
	var out []commandCooldown
	for k, v := range e.Config.Map("command-cooldowns") {
		secs, ok := toFloat(v)
		if !ok || secs <= 0 {
			continue
		}
		var pat string
		if strings.HasPrefix(k, "^") {
			pat = k[1:]
		} else {
			pat = regexp.QuoteMeta(strings.TrimPrefix(k, "/"))
			if before, ok0 := strings.CutSuffix(pat, `\*`); ok0 {
				pat = before + ".*"
			} else {
				pat += "( .*)?"
			}
		}
		re, err := regexp.Compile("^(?i:" + pat + ")$")
		if err != nil {
			continue
		}
		out = append(out, commandCooldown{re, k, time.Duration(secs * float64(time.Second))})
	}
	return out
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// commandName answers the Essentials command a typed name runs, or the name
// itself.
func (e *Essentials) commandName(typed string) string {
	typed = strings.ToLower(typed)
	if _, after, ok := strings.Cut(typed, ":"); ok {
		typed = after
	}
	for name, r := range e.commands {
		if slices.Contains(r.names, typed) {
			return name
		}
	}
	return typed
}

func (e *Essentials) onCommand(ev *plugin.PlayerCommandEvent) {
	p := ev.Player
	u := e.Users.Of(p)
	typed, args, _ := strings.Cut(strings.TrimPrefix(ev.Command, "/"), " ")
	cmd := e.commandName(typed)
	spyCmds := e.Settings.SocialSpyCommands()
	if (slices.Contains(spyCmds, strings.ToLower(typed)) || slices.Contains(spyCmds, "*")) && cmd != "msg" && cmd != "r" &&
		!u.HasPermission("essentials.chat.spy.exempt") {
		name := p.Name()
		if e.Config.Bool("socialspy-uses-displaynames", true) {
			name = u.DisplayName()
		}
		for _, spy := range e.OnlineUsers() {
			if spy.Data.SocialSpy && spy.ID != u.ID {
				prefix := "socialSpyPrefix"
				if u.IsMuted() && e.Config.Bool("socialspy-listen-muted-players", true) {
					prefix = "socialSpyMutedPrefix"
				}
				spy.SendText(spy.Tl(prefix).Append(spy.Tl("socialSpyCmdFormat", name, "/"+ev.Command)))
			}
		}
	}
	mute := e.Settings.MuteCommands()
	if u.IsMuted() && (slices.Contains(mute, cmd) || slices.Contains(mute, strings.ToLower(typed)) || slices.Contains(mute, "*")) {
		ev.SetCancelled(true)
		e.NotifyMuted(u)
		e.Log.Info(e.TlString("mutedUserSpeaks", p.Name(), "/"+ev.Command))
		return
	}
	if cmd != "afk" {
		u.UpdateActivity(cmd != "vanish")
	}
	if u.HasPermission("essentials.commandcooldowns.bypass") || u.HasPermission("essentials.commandcooldowns.bypass."+cmd) {
		return
	}
	full := cmd
	if args != "" {
		full += " " + args
	}
	now := time.Now().UnixMilli()
	cds := u.Data.Timestamps.CommandCooldowns[:0]
	found := false
	for _, c := range u.Data.Timestamps.CommandCooldowns {
		if c.Value <= now {
			continue
		}
		cds = append(cds, c)
		if re, err := regexp.Compile(c.Pattern); err == nil && re.MatchString(full) {
			u.Send("commandCooldown", FormatDateDiff(time.UnixMilli(c.Value)))
			ev.SetCancelled(true)
			found = true
		}
	}
	u.Data.Timestamps.CommandCooldowns = cds
	if found {
		return
	}
	for _, c := range e.cooldowns() {
		if c.pattern.MatchString(full) {
			u.Data.Timestamps.CommandCooldowns = append(u.Data.Timestamps.CommandCooldowns,
				CommandCooldown{Pattern: c.pattern.String(), Value: time.Now().Add(c.d).UnixMilli()})
			u.Save()
			break
		}
	}
}

func (e *Essentials) onTeleport(ev *plugin.EntityTeleportEvent) {
	p, ok := ev.Entity.(plugin.Player)
	if !ok || (ev.Cause != plugin.TeleportPlugin && ev.Cause != plugin.TeleportCommand) {
		return
	}
	u := e.Users.Of(p)
	if e.Settings.RegisterBackInListener() && u.HasPermission("essentials.back.onteleport") {
		u.SetLastLocation(ev.From)
	}
}

func (e *Essentials) onDeath(ev *plugin.PlayerDeathEvent) {
	u := e.Users.Of(ev.Player)
	loc := ev.Player.Location()
	if e.Config.Bool("send-info-after-death", false) {
		b := loc.Pos.Block()
		u.Send("infoAfterDeath", WorldName(loc.World), b.X, b.Y, b.Z)
	}
	if u.HasPermission("essentials.back.ondeath") && !slices.Contains(e.Settings.DisabledCommands(), "back") {
		u.SetLastLocation(loc)
		u.Send("backAfterDeath")
	}
	if !e.Config.Bool("death-messages", true) {
		ev.Message = nil
	}
	if u.HasPermission("essentials.keepxp") {
		ev.KeepLevel = true
		ev.Experience = 0
	}
	if u.HasPermission("essentials.keepinv") {
		ev.KeepInventory = true
		inv := ev.Player.Inventory().Contents()
		for _, s := range inv {
			if s.Empty() {
				continue
			}
			if i := slices.IndexFunc(ev.Drops, func(d plugin.ItemStack) bool { return e.Server.Items().Similar(d, s) && d.Count == s.Count }); i >= 0 {
				ev.Drops = slices.Delete(ev.Drops, i, i+1)
			}
		}
	}
}

func (e *Essentials) onDamage(ev *plugin.EntityDamageEvent) {
	if p, ok := ev.Entity.(plugin.Player); ok && e.Users.Of(p).IsGodMode() {
		ev.SetCancelled(true)
		p.SetFireTicks(0)
		p.SetAir(300)
		return
	}
	attacker, ok := ev.Source.Causing.(plugin.Player)
	if !ok {
		return
	}
	if _, victimIsPlayer := ev.Entity.(plugin.Player); !victimIsPlayer {
		return
	}
	a := e.Users.Of(attacker)
	if a.IsGodMode() && !a.HasPermission("essentials.god.pvp") {
		ev.SetCancelled(true)
	}
	if a.Vanished && !a.HasPermission("essentials.vanish.pvp") {
		ev.SetCancelled(true)
	}
}
